# AI Soft Skills Trainer

Веб-тренажер сложных разговоров: React-фронтенд, Go backend и локальный Triton Inference Server с генеративной LLM `softskill_generator`.

## Что внутри

- `frontend/` - React + Vite интерфейс с главной страницей, меню тренировок и личным кабинетом.
- `cmd/server` - Go + Gin API.
- `internal/services` - аккаунты, дневные лимиты, сценарии, сессии, промпты и разбор диалогов.
- `internal/ai` - HTTP-клиент Triton Inference Server.
- `triton/model_repository/softskill_generator` - Triton Python backend, который запускает Hugging Face causal LLM.
- `docker-compose.yml` - frontend, backend и Triton одной командой.

## Запуск через Docker

```bash
cp .env.example .env
docker compose up --build
```

После запуска:

- frontend: `http://localhost:5173`
- backend: `http://localhost:8080`
- Triton HTTP API: `http://localhost:8000`

Проверка модели:

```bash
curl http://localhost:8080/api/check-models
```

## Переменные окружения

```env
PORT=8080
DAILY_TRAINING_LIMIT=5
TRITON_URL=http://localhost:8000
TRITON_MODEL_NAME=softskill_generator
HF_MODEL_ID=Qwen/Qwen2.5-1.5B-Instruct
MAX_NEW_TOKENS=300
TEMPERATURE=0.55
TOP_P=0.85
```

В Docker Compose backend использует внутренний адрес `http://triton:8000`.

По умолчанию Triton загружает `Qwen/Qwen2.5-1.5B-Instruct`. Это настоящая генеративная instruction-tuned LLM, а не набор правил. Для более сильной генерации можно заменить `HF_MODEL_ID` на совместимую causal/chat модель крупнее, если хватает CPU/GPU и памяти.

## Возможности

- Регистрация и вход.
- Личный кабинет с дневным лимитом тренировок.
- Автоматический сброс лимита по UTC-дате.
- Сохранение истории диалогов в backend-памяти текущего процесса.
- Меню сценариев и случайный диалог с новым контекстом на каждую попытку.
- Разбор завершенного диалога через ту же LLM в Triton.

## API

- `POST /api/register` - создать пользователя.
- `POST /api/login` - войти.
- `GET /api/profile` - профиль, лимиты и история. Требует `X-User-ID`.
- `GET /api/scenarios` - список тренировочных сценариев.
- `POST /api/start-session` - создать сессию и получить первую реплику модели. Требует `X-User-ID`.
- `POST /api/send-message` - отправить ответ пользователя в Triton-модель. Требует `X-User-ID`.
- `GET /api/sessions/:id` - открыть сохраненный диалог. Требует `X-User-ID`.
- `GET /api/get-feedback?session_id=...` - получить JSON-разбор диалога. Требует `X-User-ID`.
- `GET /api/check-models` - проверить metadata модели в Triton.

## Важно

Mock-режим удален. Если Triton или Hugging Face модель недоступны, backend возвращает ошибку, а UI показывает проблему со статусом inference server.

Сейчас пользователи, лимиты и история хранятся в памяти Go-процесса. Для production следующим шагом стоит заменить `internal/storage` на PostgreSQL или SQLite.
