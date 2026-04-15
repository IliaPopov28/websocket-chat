# WebSocket Chat — Статус проекта

> Последнее обновление: 15 апреля 2026
> Ветка: `feat/auth-postgres`

---

## Что реализовано ✅

### Ядро
- WebSocket Hub с goroutine-per-client (ReadPump / WritePump)
- Public и private сообщения (JSON через WebSocket)
- Batch-оптимизация записи, ping/pong keepalive
- Graceful shutdown (os/signal + context)

### Аутентификация
- Регистрация + логин с JWT-токенами (HS256, 24h)
- Bcrypt хеширование паролей
- WebSocket-подключение через токен в query-параметре (`/ws?token=...`)

### Хранение данных
- PostgreSQL (pgxpool) для пользователей
- Миграции через `/docker-entrypoint-initdb.d/`
- Таблица `users` (nickname, password_hash)

### Веб
- Встраивание фронтенда через `//go:embed`
- HTML + vanilla JS фронтенд (index.html)
- UI: логин/регистрация, чат, список пользователей

### Инфраструктура
- Dockerfile (multi-stage: go build → alpine)
- docker-compose.yml (app + postgres)
- .dockerignore, .gitignore

---

## Известные проблемы (из code review 15.04.2026)

### 🔴 Critical (блокируют мерж)
1. **Send on closed channel** — `Client.Send()` паникует после `Close()` (`internal/hub/client.go`)
2. **Concurrent WebSocket writes** — `Close()` и `WritePump` пишут в соединение одновременно
3. **Race condition в `Shutdown()`** — signal handler модифицирует map, который читает `Run()`
4. **TOCTOU дубликат nickname** — одновременное подключение с одним ником приводит к утечке горутин

### 🟡 Suggestion
5. JWT токен в URL — утечка в логах
6. Захардкоженный JWT secret
7. Дублирование кода парсинга auth-запроса
8. Нет rate limiting на auth-эндпоинтах
9. `CheckOrigin: return true` — любой источник
10. `embed *` может засветить `.go` файлы
11. `pool.Ping()` без таймаута
12. Postgres port экспонирован на host

### 🔵 Linter (errcheck)
12 непроверенных возвращаемых значений (`SetReadDeadline`, `SetWriteDeadline`, `w.Write`, `json.Encode`)

> Полный отчёт: `.qwen/reviews/2026-04-15-153000-feat-auth-postgres.md`

---

## Что можно добавить (TODO)

- [ ] Исправить 4 Critical проблемы из ревью
- [ ] Rate limiting на `/api/register` и `/api/login`
- [ ] Передавать JWT через `Sec-WebSocket-Protocol` вместо URL
- [ ] Вынести JWT secret в обязательную env-переменную (fail fast)
- [ ] Тесты (hub, auth service, handlers)
- [ ] Логирование через `slog`
- [ ] Команды в чате (`/list`, `/msg`)
- [ ] Proper migration tool (golang-migrate / goose) при росте миграций
- [ ] CI/CD (GitHub Actions: build + test + lint)

---

## Структура проекта

```
websocket-chat/
  cmd/server/main.go              # Точка входа, инициализация, graceful shutdown
  internal/
    auth/service.go               # JWT аутентификация (register, login, validate)
    domain/
      errors.go                   # Доменные ошибки
      message.go                  # Модель сообщения
      user.go                     # Модель пользователя
    hub/
      client.go                   # Client: ReadPump, WritePump, Send
      hub.go                      # Hub: register, unregister, broadcast
    store/postgres/
      user_store.go               # PostgreSQL хранилище пользователей
    transport/
      handler.go                  # HTTP/WebSocket хендлеры (register, login, ws)
  migrations/
    001_create_users.sql          # Создание таблицы users
  pkg/protocol/
    ws.go                         # Низкоуровневая обёртка над gorilla/websocket
  web/
    index.html                    # Фронтенд чата
    web.go                        // go:embed для веб-ресурсов
  docker-compose.yml              # App + PostgreSQL
  Dockerfile                      # Multi-stage build
  QWEN.md                         # Инструкции для ИИ
```

---

## Константы

```go
// client.go
writeWait      = 20 * time.Second
pongWait       = 60 * time.Second
pingPeriod     = pongWait * 2 / 3  // ~40с
maxMessageSize = 1024 * 1024       // 1MB
```

---

## Команды

```bash
go run cmd/server/main.go          # Запуск
go build ./...                     # Сборка
go test ./...                      # Тесты
go vet ./...                       # Статический анализ
golangci-lint run ./...            # Линтер
docker-compose up                  # Запуск с PostgreSQL
```
