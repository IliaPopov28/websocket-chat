# Code Review — ветка `feat/auth-postgres` vs `origin/main`

**Дата:** 15 апреля 2026  
**Файлов изменено:** 16 | **+657 / -88 строк**  
**Вердикт:** 🔴 Request Changes

---

## Deterministic Analysis

| Проверка | Результат |
|----------|-----------|
| `go vet ./...` | ✅ Без ошибок |
| `golangci-lint run ./...` | ⚠️ 12 предупреждений (errcheck) |
| `go build ./...` | ✅ Собралось |
| `go test ./...` | ✅ Прошли |

### Linter: errcheck (12 warnings, Nice to have)

| Файл | Строка | Что не проверено |
|------|--------|------------------|
| `pkg/protocol/ws.go` | 26 | `c.conn.SetReadDeadline` |
| `pkg/protocol/ws.go` | 37 | `c.conn.SetWriteDeadline` |
| `pkg/protocol/ws.go` | 48 | `c.conn.SetWriteDeadline` |
| `internal/hub/client.go` | 62 | `SetReadDeadline` (raw conn) |
| `internal/hub/client.go` | 109 | `c.conn.WriteControl` |
| `internal/hub/client.go` | 118 | `SetWriteDeadline` (raw conn) |
| `internal/hub/client.go` | 123 | `w.Write` |
| `internal/hub/client.go` | 127 | `w.Write` |
| `internal/hub/client.go` | 133 | `w.Write` |
| `internal/hub/client.go` | 142 | `SetWriteDeadline` (raw conn) |
| `internal/transport/handler.go` | 65 | `json.NewEncoder(w).Encode` |
| `internal/transport/handler.go` | 102 | `json.NewEncoder(w).Encode` |

---

## Critical (4) — Блокируют мерж

### 1. Send on closed channel — паника `Client.Send()`

**Файлы:** `internal/hub/client.go:41-43`, `internal/hub/client.go:84`

**Проблема:** `Send()` делает блокирующий `c.send <- message` без проверки, закрыт ли канал. `Close()` вызывает `close(c.send)` через `sync.Once`. Если `Send()` вызван конкурентно с или после `Close()` — **паника «send on closed channel»**, которая уронит весь сервер.

**Сценарии:**
- ReadPump шлёт приватное сообщение во время Shutdown
- Hub Broadcast шлёт клиенту, который только что закрылся

**Исправление:**
```go
func (c *Client) Send(message domain.Message) {
    select {
    case c.send <- message:
    case <-c.done:
        return
    }
}
```
Или: добавить `sync.Mutex` + флаг `closed` и проверять перед отправкой.

---

### 2. Concurrent WebSocket writes — паника в WritePump

**Файлы:** `internal/hub/client.go:45-49`

**Проблема:** `Close()` вызывает `c.conn.Close()` (пишет CloseMessage через `WriteControl`). Одновременно `WritePump` может быть в середине `NextWriter`/`WriteMessage`. **Gorilla WebSocket не поддерживает конкурентные записи** — паника или битый фрейм.

**Исправление:** Убрать `c.conn.Close()` из `Client.Close()`. Закрытие `send` канала уже сигнализирует WritePump выйти и отправить CloseMessage. Пусть только WritePump управляет записью в соединение.

---

### 3. Race condition: `Shutdown()` модифицирует map из другой горутины

**Файлы:** `internal/hub/hub.go:43-49`

**Проблема:** `Shutdown()` вызывается из горутины signal handler и напрямую переназначает `h.registered = make(map[string]ClientInterface)`. В это же время `Run()` читает этот map. **Конкурентное чтение/запись map** — undefined behavior, паника «concurrent map read and map write».

**Исправление:** Не модифицировать map напрямую. После `close(h.done)` перебрать клиентов и вызвать `Close()`, но **не** переназначать map. Или использовать `sync.RWMutex`.

---

### 4. TOCTOU: дубликат nickname при одновременном подключении

**Файлы:** `internal/transport/handler.go:120-134`, `internal/hub/hub.go:57`

**Проблема:** `HandleWebSocket` сначала проверяет `HasUser()`, потом шлёт `Register()`. Между проверкой и регистрацией другой клиент с тем же nickname может зарегистрироваться. `handleRegister` просто перезаписывает entry — первый клиент остаётся с работающими горутинами, но «осиротевшим» entry в map. **Утечка горутин + соединений.**

**Исправление:** Сделать регистрацию атомарной. В `handleRegister` проверять наличие nickname и отклонять дубликат, закрывая новое соединение. Или возвращать результат регистрации через ответный канал.

---

## Suggestion (8) — Рекомендуется исправить

### 5. JWT токен в URL query параметре — утечка в логах

**Файлы:** `internal/transport/handler.go:112`, `web/index.html`

**Проблема:** Токен передаётся как `?token=...`. URL логируются reverse proxy, load balancer, access-логами сервера, сохраняются в истории браузера и Referer header.

**Исправление:** Передавать токен в заголовке (через `Sec-WebSocket-Protocol`) или в первом WebSocket-сообщении.

---

### 6. Захардкоженный JWT secret в `main.go` и `docker-compose.yml`

**Файлы:** `cmd/server/main.go:69-71`, `docker-compose.yml:30`

**Проблема:** `"super-secret-key-change-in-production"` закоммичено. Если `JWT_SECRET` не установлен — используется известный секрет, позволяющий подделывать токены.

**Исправление:** Fail fast при пустом `JWT_SECRET`. Убрать хардкод из `docker-compose.yml`, использовать `.env`.

---

### 7. Дублирование кода парсинга auth-запроса

**Файлы:** `internal/transport/handler.go:30-34` и `73-77`

**Проблема:** Анонимная struct + JSON decode + валидация дублируются в `HandleRegister` и `HandleLogin`.

**Исправление:** Вынести в `decodeAuthRequest(r *http.Request) (nickname, password string, err error)` или именованный тип `authRequest`.

---

### 8. No rate limiting на `/api/register` и `/api/login`

**Файлы:** `internal/transport/handler.go:25-95`

**Проблема:** Bcrypt намеренно дорогой (~100ms). Без рейт-лимитинга 100 параллельных запросов = DoS-вектор.

**Исправление:** Добавить `golang.org/x/time/rate` или простой mutex + счётчик.

---

### 9. `CheckOrigin: return true` — любой источник

**Файлы:** `pkg/protocol/ws.go:14`

**Проблема:** Любая веб-страница может открыть WebSocket к серверу от имени авторизованного пользователя.

**Исправление:** Проверять `Origin` header против allowlist из конфигурации.

---

### 10. `embed *` может засветить `.go` файлы

**Файлы:** `web/web.go:5-6`

**Проблема:** `//go:embed *` включает `web.go` и любые `.go` файлы. `http.FileServer` может отдать их как статику.

**Исправление:** `//go:embed *.html *.css *.js`

---

### 11. `pool.Ping(context.Background())` без таймаута

**Файлы:** `cmd/server/main.go:48`

**Проблема:** Если БД принимает TCP но не отвечает — `Ping` зависнет навечно, блокируя старт сервера.

**Исправление:** `context.WithTimeout(context.Background(), 5*time.Second)`

---

### 12. Postgres port `5432` экспонирован на host

**Файлы:** `docker-compose.yml:10`

**Проблема:** Любой процесс на хосте может подключиться к БД с известными креденциалами.

**Исправление:** Убрать `ports` для postgres. Для отладки: `docker exec -it ws-chat-db psql -U wschat`.

---

## Needs Human Review

### 13. Registration auto-login — двойная работа БД (Possibly)

**Файл:** `internal/transport/handler.go:53-57`

После `Register()` сразу вызывается `Login()`, что делает лишний SELECT + bcrypt compare. Можно генерировать токен сразу в `Register()`.

### 14. Двойное закрытие соединения (Possibly)

**Файл:** `internal/hub/client.go:54-56`

`ReadPump` defer вызывает `c.conn.Close()` после `Client.Close()`. Gorilla обычно это переживает, но стоит проверить.

---

## Итог

| Severity | Кол-во |
|----------|--------|
| 🔴 Critical | 4 |
| 🟡 Suggestion | 8 |
| 🔵 Nice to have (linter) | 12 |
| ⚪ Needs Human Review | 2 |

**Без исправления 4 Critical проблем — не мержить.**
