# AI Soft Skills Trainer

Веб-тренажер сложных разговоров: React-фронтенд, Go backend и генерация через `ml-service`.

## Что внутри

- `frontend/` - React + Vite интерфейс с главной страницей, меню тренировок и личным кабинетом.
- `cmd/server` - Go + Gin API.
- `internal/services` - бизнес-логика с пользователями, лимитами, сценариями, сессиями и разбором диалогов.
- `internal/ai` - HTTP-клиент для inference-сервиса `ml-service`.
- `docker-compose.yml` - frontend, backend и inference-сервис.

## Запуск через Docker

1. Убедитесь, что `ml-service` настроен и доступен.
2. Запустите проект через Docker Compose:

```bash
docker compose up --build
```

После запуска:

- frontend: `http://localhost:5173`
- backend: `http://localhost:8080`
- inference: `http://localhost:8087`

Проверка работы inference-сервиса:

```bash
curl http://localhost:8080/api/check-models
```

## Переменные окружения

Вставляйте свои значения в корневой файл `.env`.
Если вы не используете Docker Compose, можно скопировать `.env.example` в `.env` и заменить нужные параметры.

```env
PORT=8080
DAILY_TRAINING_LIMIT=5
ML_SERVICE_URL=http://ml-service:8087
MODEL_NAME=GigaChat
SBER_AUTH=
MAX_NEW_TOKENS=300
TEMPERATURE=0.55
TOP_P=0.85
```

Объяснение переменных:
- `ML_SERVICE_URL` — URL inference-сервиса `ml-service`. В Docker Compose это `http://ml-service:8087`.
- `MODEL_NAME` — имя модели, которая используется `ml-service` (по умолчанию `GigaChat`).
- `SBER_AUTH` — ключ/токен доступа к Gigachat/Sber, если ваш inference-сервис требует авторизации. Вставляйте сюда строку токена, иначе оставьте пустым.
- `MAX_NEW_TOKENS`, `TEMPERATURE`, `TOP_P` — параметры генерации для модели.

Если `ml-service` запускается локально на той же машине, используйте `http://localhost:8087` в `ML_SERVICE_URL`.

## Возможности

- Регистрация и вход.
- Личный кабинет с дневным лимитом тренировок.
- Автоматический сброс лимита по UTC-дате.
- Сохранение истории диалогов в backend-памяти текущего процесса.
- Меню сценариев и случайный диалог с новым контекстом на каждую попытку.
- Разбор завершенного диалога через тот же inference-сервис.

## API

- `POST /api/register` - создать пользователя.
- `POST /api/login` - войти.
- `GET /api/profile` - профиль, лимиты и история. Требует `X-User-ID`.
- `GET /api/scenarios` - список тренировочных сценариев.
- `POST /api/start-session` - создать сессию и получить первую реплику модели. Требует `X-User-ID`.
- `POST /api/send-message` - отправить ответ пользователя в модель через `ml-service`. Требует `X-User-ID`.
- `GET /api/sessions/:id` - открыть сохраненный диалог. Требует `X-User-ID`.
- `GET /api/get-feedback?session_id=...` - получить JSON-разбор диалога. Требует `X-User-ID`.
- `GET /api/check-models` - проверить metadata inference-сервиса.

## Важно

Mock-режим удален. Если `ml-service` недоступен, backend возвращает ошибку, а UI показывает проблему со статусом inference server.

Сейчас пользователи, лимиты и история хранятся в памяти Go-процесса. Для production следующим шагом стоит заменить `internal/storage` на PostgreSQL или SQLite.
