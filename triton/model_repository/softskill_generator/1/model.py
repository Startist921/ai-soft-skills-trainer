import json
import os

import numpy as np
import triton_python_backend_utils as pb_utils


class TritonPythonModel:
    def initialize(self, args):
        import torch
        from transformers import AutoModelForCausalLM, AutoTokenizer

        self.model_id = os.getenv("HF_MODEL_ID", "Qwen/Qwen2.5-1.5B-Instruct")
        self.max_new_tokens = int(os.getenv("MAX_NEW_TOKENS", "300"))
        self.temperature = float(os.getenv("TEMPERATURE", "0.55"))
        self.top_p = float(os.getenv("TOP_P", "0.85"))

        self.tokenizer = AutoTokenizer.from_pretrained(
            self.model_id,
            trust_remote_code=True,
        )

        has_cuda = torch.cuda.is_available()
        dtype = torch.float16 if has_cuda else torch.float32
        device_map = "auto" if has_cuda else None

        self.model = AutoModelForCausalLM.from_pretrained(
            self.model_id,
            torch_dtype=dtype,
            device_map=device_map,
            trust_remote_code=True,
        )
        if not has_cuda:
            self.model.to("cpu")
        self.model.eval()

    def execute(self, requests):
        responses = []
        for request in requests:
            prompt_input = pb_utils.get_input_tensor_by_name(request, "PROMPT")
            payload = _decode_prompt(prompt_input.as_numpy()[0])
            text = self._generate(payload)
            output = pb_utils.Tensor("TEXT", np.array([text.encode("utf-8")], dtype=np.object_))
            responses.append(pb_utils.InferenceResponse(output_tensors=[output]))
        return responses

    def _generate(self, payload):
        import torch

        messages = _build_chat(payload)
        prompt = self.tokenizer.apply_chat_template(
            messages,
            tokenize=False,
            add_generation_prompt=True,
        )
        inputs = self.tokenizer(prompt, return_tensors="pt")
        inputs = {key: value.to(self.model.device) for key, value in inputs.items()}

        with torch.inference_mode():
            generated = self.model.generate(
                **inputs,
                max_new_tokens=self.max_new_tokens,
                do_sample=self.temperature > 0,
                temperature=self.temperature,
                top_p=self.top_p,
                repetition_penalty=1.1,
                pad_token_id=self.tokenizer.eos_token_id,
            )

        new_tokens = generated[0][inputs["input_ids"].shape[-1]:]
        text = self.tokenizer.decode(new_tokens, skip_special_tokens=True)
        return _clean_generation(text)


def _decode_prompt(value):
    if isinstance(value, np.ndarray):
        value = value.item()
    if isinstance(value, bytes):
        value = value.decode("utf-8")
    try:
        return json.loads(value)
    except json.JSONDecodeError:
        return {"system_prompt": str(value), "messages": []}


def _build_chat(payload):
    system_prompt = payload.get("system_prompt", "")
    history = payload.get("messages", [])
    quality_prompt = (
        "Ты сильная русскоязычная LLM для тренировки деловой коммуникации. "
        "Пиши естественно, конкретно и без шаблонных фраз. "
        "Не выдумывай системные сообщения, не используй markdown, не повторяй инструкцию. "
        "Если нужно вернуть JSON, верни только JSON."
    )
    messages = [{"role": "system", "content": quality_prompt + "\n\n" + system_prompt}]

    for item in history:
        role = item.get("role", "user")
        if role not in {"user", "assistant"}:
            role = "user"
        text = item.get("text", "")
        if text:
            messages.append({"role": role, "content": text})

    if len(messages) == 1:
        messages.append({
            "role": "user",
            "content": "Начни сцену первой репликой. Не объясняй правила, сразу говори из роли.",
        })

    return messages


def _clean_generation(text):
    text = text.strip()
    for prefix in ("assistant:", "Ассистент:", "Ответ:", "Тренажер:", "Тренажёр:"):
        if text.startswith(prefix):
            text = text[len(prefix):].strip()
    return text
