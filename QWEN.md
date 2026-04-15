# Qwen Code — Project Instructions

## Общие правила для ИИ

1. **Всегда отвечай на русском языке**, если пользователь явно не попросил другой язык.
2. **Всегда проверяй актуальность версий** (языков, фреймворков, библиотек) через поиск в интернете перед тем, как утверждать что-либо о версиях. Сейчас апрель 2026 года — не предполагай, что последняя стабильная версия — это то, что было на момент твоего обучения.
3. **Не предлагай косметические изменения** (стиль, форматирование, именование), если они соответствуют конвенциям окружающего кода.
4. **Не повторяй уже обсуждённые проблемы.** Если проблема уже была поднята в ходе текущего диалога — не возвращайся к ней.
5. **Будь конкретен.** Вместо «можно улучшить» — давай конкретный код или команду.
6. **Коммить после каждого изменения файла.** Каждое логически завершённое изменение должно быть зафиксировано через `git add` + `git commit` с понятным сообщением. Это даёт удобные точки отката. Если нужно — делай `git push`.

## О проекте

- **Название:** websocket-chat
- **Язык:** Go 1.26
- **Технологии:** WebSocket (gorilla/websocket), JWT-аутентификация, PostgreSQL (pgxpool), embed для веб-ресурсов, Docker
- **Структура:**
  - `cmd/server/` — точка входа
  - `internal/auth/` — сервис аутентификации
  - `internal/hub/` — WebSocket Hub (управление клиентами, broadcast)
  - `internal/store/postgres/` — слой хранения (PostgreSQL)
  - `internal/transport/` — HTTP/WebSocket хендлеры
  - `internal/domain/` — доменные модели
  - `pkg/protocol/` — низкоуровневая обёртка над WebSocket
  - `web/` — фронтенд (HTML/JS, встраивается через embed)
  - `migrations/` — SQL-миграции

## Команды

- `go build ./...` — сборка
- `go test ./...` — тесты
- `go vet ./...` — статический анализ
- `golangci-lint run ./...` — линтер
- `docker-compose up` — запуск с PostgreSQL

---

## GRACE-документация (архитектура и инварианты)

> GRACE: Guidelines for Recursive AI Code Evaluation.
> Документация внутри кода, объясняющая «почему» и «зачем», а не «что».

### Hub (`internal/hub/hub.go`)

**Цель:** единый оркестратор всех WebSocket-соединений. Реализует паттерн **single-owner goroutine** — вся работа с map `registered` происходит строго в одной горутине (`Run()`).

**Инварианты:**
- `registered` map читается и модифицируется **только** внутри `Run()` — это гарантирует отсутствие гонок без мьютексов
- Все публичные методы (`Register`, `Broadcast`, `SendTo`) отправляют сообщения через каналы — они thread-safe
- `Shutdown()` **не модифицирует** `registered` напрямую — только закрывает `done` и вызывает `client.Close()`. Map может продолжать читаться, пока `Run()` не выйдет

**DECISION: `RegisterWithResult`** — атомарная регистрация. Нужна, чтобы избежать TOCTOU-гонки: два клиента с одинаковым nickname не могут оба пройти `HasUser()` → `Register()`. Результат возвращается через buffered channel внутри `Run()`.

**DECISION: `ClientInterface`** — абстракция над `*Client`. Позволяет подменять клиента в тестах и передавать метаданные (например, `registeredClient` для RegisterWithResult).

**Нельзя менять:**
- Сигнатуры каналов `register`, `unregister`, `broadcast` — от них зависит типизация в `Run()` select
- Порядок обработки select-кейсов в `Run()` — Go выбирает случайный при готовности нескольких, но это не влияет на корректность

### Client (`internal/hub/client.go`)

**Цель:** пара горутин (ReadPump / WritePump), управляющих одним WebSocket-соединением.

**Инварианты:**
- **Только WritePump пишет в `conn`.** `Client.Close()` НЕ вызывает `c.conn.Close()` — он лишь закрывает каналы `done` и `send`. Это предотвращает конкурентные записи (gorilla/websocket не thread-safe для Write)
- `Send()` использует `select` с `<-c.done` — никогда не паникует на закрытом канале
- `Close()` защищён `sync.Once` — безопасен для вызова из нескольких горутин

**Поток закрытия:**
1. `Close()` → `close(done)` + `close(send)`
2. WritePump видит `ok == false` на `<-c.send` → отправляет WebSocket CloseMessage → выходит
3. ReadPump видит ошибку чтения (соединение закрыто) → выходит, посылает unregister в Hub (non-blocking)
4. defer WritePump → `c.conn.Close()` (единственный, кто пишет в conn при закрытии)

**DECISION: буфер `send = 256`** — достаточен для burst-нагрузки. Если переполняется — `Send()` блокируется на `select`, но `<-c.done` разблокирует и тихо отбрасывает сообщение.

**DECISION: non-blocking unregister в ReadPump defer** — Hub может быть уже остановлен (`Shutdown()` закрыл `done`). Без `select` с `<-h.hub.done` goroutine зависнет навсегда.

### Connection (`pkg/protocol/ws.go`)

**Цель:** тонкая обёртка над `*websocket.Conn` для JSON-сериализации и управления дедлайнами.

**Инварианты:**
- `ReadJSON` / `WriteJSON` устанавливают дедлайны перед каждой операцией
- `Close()` отправляет WebSocket CloseMessage — **не закрывает TCP-соединение** (это делает вызывающий через `conn.Close()`)
- `RawConn()` даёт доступ к сырому `*websocket.Conn` для настроек (SetReadLimit, SetPongHandler и т.д.)

**DECISION: `CheckOrigin: return true`** — для разработки. В production нужно заменить на проверку Origin header.

### Handler (`internal/transport/handler.go`)

**Цель:** HTTP-эндпоинты (register, login) и WebSocket upgrade.

**Поток WebSocket-подключения:**
1. Извлечь JWT из query `?token=...`
2. Валидировать токен → получить nickname
3. Upgrade HTTP → WebSocket
4. Создать `Client` → атомарно зарегистрировать через `RegisterWithResult`
5. Если регистрация неудачна (ник занят) → закрыть клиента и вернуть
6. Запустить `WritePump` и `ReadPump` в отдельных горутинах

**DECISION: upgrade ПЕРЕД регистрацией** — WebSocket upgrade может упасть (сетевая ошибка). Если регистрировать до upgrade, при失败-е клиент останется «зарегистрированным» без реального соединения. Сейчас: upgrade → register → если register fail → close client.

### Auth Service (`internal/auth/service.go`)

**Цель:** регистрация, логин, JWT-токены.

**Поток:**
- `Register()` → INSERT в БД с bcrypt hash → если 23505 (unique violation) → `ErrUserAlreadyExists`
- `Login()` → SELECT по nickname → `bcrypt.CompareHashAndPassword` → `jwt.Sign`
- `ValidateToken()` → `jwt.Parse` с проверкой подписи и expiration

**DECISION: JWT без refresh** — упрощение для pet-проекта. Токен на 24 часа. В production нужен refresh endpoint.
