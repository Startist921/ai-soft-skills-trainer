import os
from typing import Any

from langchain_core.messages import HumanMessage
from langchain_gigachat import GigaChat

from .llm import create_gigachat, DEFAULT_GIGACHAT_PARAMS


class InferenceService:
    def __init__(self):
        self.params: dict[str, Any] = {
            **DEFAULT_GIGACHAT_PARAMS,
        }
        if credentials := os.getenv("SBER_AUTH"):
            self.params["credentials"] = credentials
        self.runnable: GigaChat = create_gigachat(**self.params)

    async def generate(
        self,
        prompt: str,
        model: str | None = None,
        temperature: float | None = None,
        max_tokens: int | None = None,
        top_p: float | None = None,
        stop: list[str] | None = None,
    ) -> str:
        if model and model.lower() != "gigachat":
            raise ValueError("Unsupported model: " + model)

        kwargs: dict[str, Any] = {}
        if temperature is not None:
            kwargs["temperature"] = temperature
        if max_tokens is not None:
            kwargs["max_tokens"] = max_tokens
        if top_p is not None:
            kwargs["top_p"] = top_p
        if stop is not None:
            kwargs["stop"] = stop

        messages = [[HumanMessage(content=prompt)]]
        result = await self.runnable.agenerate(messages, **kwargs)
        if not result.generations or not result.generations[0]:
            return ""

        generation = result.generations[0][0]
        return getattr(generation, "text", "").strip()

    def list_models(self) -> list[dict[str, str]]:
        return [
            {
                "id": "GigaChat",
                "name": "GigaChat",
                "description": "Gigachat remote inference via ml-service",
            }
        ]
