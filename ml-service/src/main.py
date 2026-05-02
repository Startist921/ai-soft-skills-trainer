from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware

from .core import lifespan, settings
from .api import inference_router, task_router


def create_app() -> FastAPI:
    app = FastAPI(
        title=settings.APP_NAME,
        version=settings.APP_VERSION,
        debug=settings.DEBUG,
        lifespan=lifespan,
    )

    app.add_middleware(
        CORSMiddleware,
        allow_origins=["*"],
        allow_credentials=True,
        allow_methods=["*"],
        allow_headers=["*"],
    )

    app.include_router(task_router, prefix="/api/tasks")
    app.include_router(inference_router, prefix="/v1")
    return app


app = create_app()