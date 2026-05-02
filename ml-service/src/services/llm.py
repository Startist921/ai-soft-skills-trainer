import inspect

from langchain_gigachat import GigaChat


DEFAULT_GIGACHAT_PARAMS = {
    "model": "GigaChat",
    "temperature": 1.0,
    "profanity_check": False,
    "verify_ssl_certs": False,
    "top_p": 0.1,
    "max_tokens": 128,
    "timeout": 120,
}


def create_gigachat(**kwargs,) -> GigaChat:
    params = {**kwargs,}

    allowed = set(inspect.signature(GigaChat).parameters)
    unknown = set(params) - allowed
    if unknown:
        raise TypeError(f"Unsupported GigaChat arguments: {sorted(unknown)}")

    return GigaChat(**params)