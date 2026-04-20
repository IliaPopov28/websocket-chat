# WebSocket Chat — Статус проекта

> Последнее обновление: 20 апреля 2026
> Ветка: `feat/auth-postgres`

---

## Текущее состояние

### Реализовано
- WebSocket Hub с моделью `ReadPump`/`WritePump`
- Публичные и приватные сообщения (JSON через WebSocket)
- Graceful shutdown с закрытием зарегистрированных клиентов
- Атомарная регистрация nickname (`RegisterWithResult`) с защитой от TOCTOU
- JWT-аутентификация: регистрация, логин, валидация токена
- PostgreSQL-хранилище пользователей через `pgxpool`
- Origin-проверка WebSocket:
  - same-origin разрешён
  - cross-origin только из `ALLOWED_ORIGINS`
- Встроенный фронтенд через `//go:embed *.html`
- Docker-инфраструктура (`app + postgres`)

### Проверки качества
- `go test ./...`
- `go build ./...`
- `go vet ./...`
- `golangci-lint run ./...`

### Текущие тесты
- `internal/hub/hub_test.go`
  - отключение клиентов при `Shutdown()`
  - отклонение дубликата nickname
- `pkg/protocol/ws_test.go`
  - same-origin handshake проходит
  - foreign origin по умолчанию отклоняется
  - allowlist origin принимается

---

## Статус прошлых проблем (ревью 15.04.2026)

### Закрыто
1. `send on closed channel` в `Client.Send()`
2. конкурентные записи в WebSocket при закрытии
3. race condition при `Shutdown()`
4. TOCTOU при проверке дубликата nickname
5. `CheckOrigin: return true` (origin-защита восстановлена)
6. ошибки `errcheck` для дедлайнов/записи

### Остаётся актуальным
1. JWT всё ещё передаётся в query-параметре (`/ws?token=...`)
2. нет rate limiting на `/api/register` и `/api/login`
3. нет CI-пайплайна для `test/build/vet/lint`
4. тестов нет для `internal/auth`, `internal/transport`, `internal/store/postgres`

---

## Константы runtime (актуально)

```go
// internal/hub/client.go
writeWait      = 20 * time.Second
pongWait       = 20 * time.Second
pingPeriod     = pongWait * 2 / 3
maxMessageSize = 1024 * 1024
```

---

## Структура проекта

```
websocket-chat/
  cmd/server/main.go              # Точка входа, конфигурация, graceful shutdown
  internal/
    auth/service.go               # JWT аутентификация
    domain/                       # Доменные модели
    hub/                          # Hub и клиентские pump-горутины
    store/postgres/               # Хранилище пользователей в PostgreSQL
    transport/handler.go          # HTTP/WebSocket хендлеры
  migrations/001_create_users.sql
  pkg/protocol/ws.go              # WebSocket-обёртка и origin-check
  web/index.html                  # Встроенный фронтенд
  web/web.go                      # go:embed *.html
  docker-compose.yml
  Dockerfile
  AGENTS.md
  QWEN.md
  CODEX.md
```
