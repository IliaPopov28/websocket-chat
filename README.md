# WebSocket Chat

Учебный проект для практики WebSocket на Go: чат с JWT-аутентификацией, PostgreSQL и встроенным фронтендом.

## Возможности

- Публичные и приватные сообщения по WebSocket
- Регистрация и логин с JWT (HS256, срок 24 часа)
- Хранение пользователей в PostgreSQL (`pgxpool`)
- Graceful shutdown с закрытием активных WebSocket-клиентов
- Origin-проверка для WebSocket (same-origin + allowlist через `ALLOWED_ORIGINS`)
- Фронтенд на vanilla JS, встроенный через `go:embed`

## Быстрый старт (Docker)

1. Создай `.env` из шаблона:
```bash
cp .env.example .env
```
2. Заполни `JWT_SECRET` в `.env`.
3. Запусти сервисы:
```bash
docker compose up --build
```
4. Открой `http://localhost:8081`.

## Локальный запуск (без Docker)

Минимально нужен доступный PostgreSQL и переменная `JWT_SECRET`.

PowerShell пример:

```powershell
$env:JWT_SECRET = "change-me"
$env:DATABASE_URL = "postgres://wschat:wschat_secret@localhost:5432/wschat?sslmode=disable"
$env:ALLOWED_ORIGINS = "http://localhost:8081"
go run cmd/server/main.go
```

Если `DATABASE_URL` не задан, используется `postgres://wschat:wschat_secret@localhost:5432/wschat`.

## Проверка качества

```bash
go test ./...
go build ./...
go vet ./...
golangci-lint run ./...
```

## Текущие тесты

- `internal/hub/hub_test.go`: shutdown закрывает клиентов, дубликат nickname отклоняется
- `pkg/protocol/ws_test.go`: same-origin разрешён, чужой origin режется, allowlist origin разрешается

## Структура

```
cmd/server/main.go        — точка входа
internal/auth/            — JWT-аутентификация
internal/hub/             — управление клиентами и broadcast
internal/store/postgres/  — хранение пользователей
internal/transport/       — HTTP + WebSocket хендлеры
pkg/protocol/             — WebSocket-протокол и origin-check
web/                      — фронтенд (embed)
```

## Стек

Go 1.26 · gorilla/websocket · JWT · bcrypt · PostgreSQL (pgxpool) · Docker
