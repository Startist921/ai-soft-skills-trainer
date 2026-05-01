SHELL := /bin/bash

.PHONY: init up down build check-models

init:
	@if [ ! -f .env ]; then cp .env.example .env; echo "Created .env from .env.example"; fi
	@if [ ! -f frontend/.env ]; then echo "VITE_API_URL=http://localhost:8080/api" > frontend/.env; echo "Created frontend/.env"; fi

up: init
	docker compose up --build

build: init
	docker compose build

down:
	docker compose down

check-models:
	curl -s http://localhost:8080/api/check-models | python3 -m json.tool
