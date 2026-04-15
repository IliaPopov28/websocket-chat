# WebSocket Chat

Учебный проект для понимания работы с WebSocket на Go.

## Что внутри

- WebSocket-чат с публичными и приватными сообщениями
- JWT-аутентификация (регистрация + логин, bcrypt)
- Хранение пользователей в PostgreSQL (pgxpool)
- Graceful shutdown, ping/pong keepalive
- Фронтенд на vanilla JS, встраивается через `go:embed`
- Docker-инфраструктура (app + postgres)

## Быстрый старт

```bash
docker-compose up
```

Или локально:

```bash
go run cmd/server/main.go
```

Открой `http://localhost:8080` в двух вкладках.

## Структура

```
cmd/server/main.go        — точка входа
internal/auth/            — JWT-аутентификация
internal/hub/             — управление клиентами, broadcast
internal/store/postgres/  — хранение пользователей
internal/transport/       — HTTP + WebSocket хендлеры
pkg/protocol/             — обёртка над gorilla/websocket
web/                      — фронтенд (embed)
```

## Стек

Go 1.26 · gorilla/websocket · JWT · bcrypt · PostgreSQL (pgxpool) · Docker
