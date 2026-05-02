import datetime
from functools import lru_cache
from typing import Any, Dict, Optional
from uuid import uuid4

from fastapi import Depends, APIRouter, Request
from pydantic import BaseModel, Field

from ..schemas import (
    TaskNameDescriptionGenerationRequest,
    TaskNameDescriptionGenerationResponse,
)
from ..services import InferenceService, TaskAssistantService


task_router = APIRouter(tags=["tasks"])
inference_router = APIRouter(tags=["inference"])


@lru_cache(maxsize=1)
def get_task_assistant_agent(request: Request):
    return request.app.state.task_assistant_service


class CompletionChoice(BaseModel):
    text: str
    message: Optional[Dict[str, Any]] = None


class CompletionRequest(BaseModel):
    model: Optional[str] = Field(default="GigaChat")
    prompt: str
    temperature: Optional[float] = None
    max_tokens: Optional[int] = None
    top_p: Optional[float] = None
    stop: Optional[list[str]] = None


class CompletionResponse(BaseModel):
    id: str
    object: str = "text_completion"
    created: int
    model: str
    choices: list[CompletionChoice]


@lru_cache(maxsize=1)
def get_inference_service(request: Request):
    return request.app.state.inference_service


@task_router.get("/health", response_model=Dict[str, Any])
async def health():
    return {
        "status": "ok",
        "service": "Tasks assistant",
        "timestamp": datetime.now().isoformat()
    }


@task_router.get("/task_name_description", response_model=TaskNameDescriptionGenerationResponse)
async def generate_task_name_description(
    request: TaskNameDescriptionGenerationRequest,
    agent: TaskAssistantService = Depends(get_task_assistant_agent),
):
    return agent.process(request)


@inference_router.post("/completions", response_model=CompletionResponse)
async def create_completion(
    request: CompletionRequest,
    inference_service: InferenceService = Depends(get_inference_service),
):
    text = await inference_service.generate(
        request.prompt,
        model=request.model,
        temperature=request.temperature,
        max_tokens=request.max_tokens,
        top_p=request.top_p,
        stop=request.stop,
    )
    return CompletionResponse(
        id=str(uuid4()),
        created=int(datetime.datetime.utcnow().timestamp()),
        model=request.model or "GigaChat",
        choices=[CompletionChoice(text=text, message={"content": text})],
    )


@inference_router.get("/models")
async def list_models(inference_service: InferenceService = Depends(get_inference_service)):
    return {"models": inference_service.list_models()}
