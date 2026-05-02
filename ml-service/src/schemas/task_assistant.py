from pydantic import BaseModel


class TaskNameDescriptionGenerationRequest(BaseModel):
    request_id: str
    raw_task_description: str


class TaskNameDescriptionGenerationResponse(BaseModel):
    request_id: str
    raw_task_descriprion: str
    task_name: str
    task_description: str
