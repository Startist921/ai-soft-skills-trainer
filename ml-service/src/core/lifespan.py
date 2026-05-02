from contextlib import asynccontextmanager
from fastapi import FastAPI

from ..services import InferenceService, TaskAssistantService


@asynccontextmanager
async def lifespan(app: FastAPI):
    task_assistant_service = TaskAssistantService()
    inference_service = InferenceService()
    app.state.task_assistant_service = task_assistant_service
    app.state.inference_service = inference_service
    yield