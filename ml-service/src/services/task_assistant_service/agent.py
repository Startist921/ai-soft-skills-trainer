import json
import os
import logging

from langchain_core.prompts import ChatPromptTemplate

from .prompts import TASKS_SYSTEM, TASKS_PROMPT, create_tasks_prompt_input_d

from ..llm import create_gigachat, DEFAULT_GIGACHAT_PARAMS

from ...schemas import (
    TaskNameDescriptionGenerationRequest,
    TaskNameDescriptionGenerationResponse,
)

logger = logging.getLogger("TaskAssistantService")
logger.setLevel(os.getenv("LOGGER_LEVEL", "INFO"))


class TaskAssistantService:
    def __init__(self):
        self.params = {
            **DEFAULT_GIGACHAT_PARAMS,
        }
        if credentials := os.getenv("SBER_AUTH"):
            self.params["credentials"] = credentials
        self.runnable = create_gigachat(**self.params)

    async def process(
        self,
        request: TaskNameDescriptionGenerationRequest,
    ) -> TaskNameDescriptionGenerationResponse:
        prompt = ChatPromptTemplate.from_messages([
            ("system", TASKS_SYSTEM),
            ("human", TASKS_PROMPT),
        ])
        chain = prompt | self.runnable
        input_d = create_tasks_prompt_input_d(**request.model_dump())
        response = await chain.ainvoke(input_d)

        try:
            response_d = json.loads(response.content)
        except (ValueError, TypeError):
            logger.exception("Failed to parse task assistant response as JSON")
            response_d = {}

        return TaskNameDescriptionGenerationResponse(
            request_id=request.request_id,
            raw_task_descriprion=request.raw_task_description,
            task_name=response_d.get("task_name", ""),
            task_description=response_d.get("task_description", ""),
        )