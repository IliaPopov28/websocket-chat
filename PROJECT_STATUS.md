# WebSocket Chat — Статус проекта

> Формат работы: код пишется в чате с подробными объяснениями, пользователь переписывает в файлы.
> Go-опыт у пользователя есть, объяснения фокусируются на WebSocket-специфике и архитектуре.

---

## Текущее состояние (выполнено)

### Шаг 1. Инициализация модуля и исправление багов ✅
- Создан `go.mod` — `github.com/IliaPopov28/websocket-chat`
- Исправлен json-тег `reciptient` → `recipient` в `internal/domain/message.go`
- Исправлен import `example.com/m/v2/internal/domain` → `github.com/IliaPopov28/websocket-chat/internal/domain` в `internal/hub/hub.go`
- Опечатка `"unknow"` → `"unknown"` уже была исправлена ранее
- Установлен `gorilla/websocket`

### Шаг 2. Пакет `pkg/protocol/ws.go` — обёртка над WebSocket ✅
- `Connection` — обёртка над `*websocket.Conn`
- `Upgrader` — HTTP → WebSocket upgrade (CheckOrigin: true для разработки)
- `ReadJSON` / `WriteJSON` — JSON-сериализация с дедлайнами
- `WriteControl` — отправка управляющих сообщений (ping/pong/close)
- `Close` — корректное закрытие с close-кодом
- `RawConn` — доступ к сырому `*websocket.Conn`

### Шаг 3. Пакет `internal/hub/client.go` — Client с ReadPump/WritePump ✅
- `Client` — структура: nickname, hub, conn, send (буферизованный канал)
- `NewClient` — конструктор с буфером send = 256
- `Nickname()` — реализация интерфейса для hub.go
- `ReadPump` — горутина чтения:
  - defer: unregister + Close
  - PongHandler для продления read deadline
  - Читает JSON, заполняет Sender/Timestamp
  - PublicMessage → hub.Broadcast()
  - PrivateMessage → hub.SendTo() или ErrorMessage обратно
- `WritePump` — горутина записи:
  - select: send канал + ping-тикер (pingPeriod = 40с)
  - Batch-оптимизация: склейка сообщений в один WS-фрейм
  - Ping для проверки живости, CloseNormalClosure при выходе

---

## Осталось реализовать

### Шаг 4. Пакет `internal/transport/handler.go` — HTTP обработчики
- WebSocket upgrade handler (`/ws?nick=...`)
- HTTP handler для раздачи статики (`/`)
- Парсинг nickname из query параметров
- Валидация ника (не пустой, проверка на существование через Hub)

### Шаг 5. Файл `cmd/server/main.go` — точка входа
- Создание Hub, запуск `hub.Run()` в горутине
- Настройка HTTP сервера (mux, static files, ws handler)
- Graceful shutdown (os/signal + context)

### Шаг 6. Файл `web/index.html` — фронтенд чата
- HTML + CSS + vanilla JS
- WebSocket API, отправка/получение JSON-сообщений
- UI: ввод ника, поле сообщений, список пользователей

### Шаг 7. Запуск и тестирование
- `go run cmd/server/main.go`
- Несколько вкладок, проверка чата, приватных сообщений

### Шаг 8. Улучшения (опционально)
- Команды в чате (`/list`, `/msg <nick> <text>`)
- Логирование через slog
- Конфигурация через env-переменные
- Тесты на Hub

---

## Структура проекта

```
websocket-chat/
  cmd/
    server/
      main.go              # TODO (Шаг 5)
  internal/
    domain/
      errors.go            # ✅ Готово
      message.go           # ✅ Готово (исправлен тег)
      user.go              # ✅ Готово
    hub/
      client.go            # ✅ Готово (Шаг 3)
      hub.go               # ✅ Готово (исправлен import)
    transport/
      handler.go           # TODO (Шаг 4)
  pkg/
    protocol/
      ws.go                # ✅ Готово (Шаг 2)
  web/
    index.html             # TODO (Шаг 6)
  go.mod                   # ✅ Создан
  go.sum                   # ✅ (после go mod tidy)
  LICENSE                  # ✅ MIT
```

---

## Константы (из client.go)

```go
writeWait      = 20 * time.Second  // дедлайн записи
pongWait       = 60 * time.Second  // макс. время ожидания pong
pingPeriod     = pongWait * 2 / 3  // ~40с, интервал ping
maxMessageSize = 1024 * 1024       // 1MB
```

---

## Как продолжить в новой сессии

1. Загрузить этот файл как контекст
2. Следующий шаг — **Шаг 4** (transport handler)
3. Формат: код в чате с объяснениями → пользователь переписывает в файлы
