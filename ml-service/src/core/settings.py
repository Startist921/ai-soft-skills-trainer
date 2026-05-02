from pydantic_settings import BaseSettings


class Settings(BaseSettings):
    APP_NAME: str = "AI Lawyer Inference Server"
    APP_VERSION: str = "1.0.0"
    DEBUG: bool = True


settings = Settings()