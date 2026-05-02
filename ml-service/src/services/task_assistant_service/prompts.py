import logging
import os
import string

logger = logging.getLogger("TaskAssistantServicePrompts")
logger.setLevel(os.getenv("LOGGER_LEVEL", "INFO"))

TASKS_SYSTEM = """
Ты - AI-помощник по составлению задач. 
Твоя задача - по предоставленному сырому описанию задачи и прочего контекста составить название и подробное описание задачи.
Твой вывод должен быть строго структурирован в виде JSON. Ключи должны быть строго task_name и task_description.
Пример:
{{
    "task_name": "...",
    "task_description": "...",
}}
"""

TASKS_PROMPT = """
Сырое описание задачи: {raw_description}.
Прочий контекст: {context}.
"""

def create_tasks_prompt_input_d(**kwargs) -> dict:
    formatter = string.Formatter()
    placeholders = list(dict.fromkeys(
        name for _, name, _, _ in formatter.parse(TASKS_PROMPT) if name
    ))
    
    result = {}
    for field in placeholders:
        if field in kwargs:
            result[field] = kwargs[field]
        else:
            logger.info("[Prompt] Field '%s' not in request, using ''", field)
            result[field] = ""
    return result