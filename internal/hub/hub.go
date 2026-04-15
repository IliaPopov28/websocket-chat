// GRACE: Hub — единый оркестратор всех WebSocket-соединений.
// Паттерн single-owner goroutine: вся работа с registered map происходит строго
// в одной горутине (Run()), что гарантирует отсутствие гонок без мьютексов.
//
// ИНВАРИАНТЫ:
//   - registered читается/модифицируется ТОЛЬКО внутри Run()
//   - Все публичные методы (Register, Broadcast, SendTo) thread-safe — шлют через каналы
//   - Shutdown() НЕ модифицирует registered напрямую — только закрывает done и вызывает client.Close()
package hub

import "github.com/IliaPopov28/websocket-chat/internal/domain"

// GRACE: ClientInterface — абстракция над *Client.
// DECISION: позволяет подменять клиента в тестах и передавать метаданные
// (например, registeredClient для RegisterWithResult).
type ClientInterface interface {
	Send(message *domain.Message)
	Close()
}

// GRACE: Hub управляет подписками клиентов.
// Нельзя менять сигнатуры каналов — от них зависит типизация в Run() select.
type Hub struct {
	registered map[string]ClientInterface
	register   chan ClientInterface
	unregister chan ClientInterface
	broadcast  chan *domain.Message
	done       chan struct{}
}

// NewHub создаёт новый Hub. Каналы небуферизованы — синхронная обработка в Run().
func NewHub() *Hub {
	return &Hub{
		registered: make(map[string]ClientInterface),
		register:   make(chan ClientInterface),
		unregister: make(chan ClientInterface),
		broadcast:  make(chan *domain.Message, 256),
		done:       make(chan struct{}),
	}
}

// GRACE: Run — единственная горутина, которая работает с registered map.
// Блокируется до закрытия done (сигнал от Shutdown()).
// Порядок select-кейсов: Go выбирает случайный при готовности нескольких,
// но это не влияет на корректность — регистрация/отписка/бродкаст идемпотентны.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)
		case client := <-h.unregister:
			h.handleUnregister(client)
		case message := <-h.broadcast:
			h.handleBroadcast(message)
		case <-h.done:
			return
		}
	}
}

// GRACE: Shutdown закрывает Hub.
// DECISION: не переназначает registered = make(...) — это race condition с Run(),
// который может ещё читать map перед выходом. Вместо этого: close(done) + client.Close().
func (h *Hub) Shutdown() {
	close(h.done)
	for _, client := range h.registered {
		client.Close()
	}
}

// GRACE: Register — fire-and-forget регистрация. Результат не проверяется.
func (h *Hub) Register(client ClientInterface) {
	h.register <- client
}

// GRACE: RegisterWithResult — атомарная регистрация.
// DECISION: решает TOCTOU-проблему — два клиента с одинаковым nickname не могут
// оба пройти проверку. Результат возвращается через buffered channel внутри Run().
// Вызывающий блокируется, пока Run() не обработает регистрацию.
func (h *Hub) RegisterWithResult(client ClientInterface) bool {
	resultCh := make(chan bool, 1)
	h.register <- &registeredClient{ClientInterface: client, resultCh: resultCh}
	return <-resultCh
}

func (h *Hub) handleRegister(client ClientInterface) {
	nick := clientNickname(client)
	if _, exists := h.registered[nick]; exists {
		if rc, ok := client.(*registeredClient); ok {
			rc.resultCh <- false
		}
		return
	}
	h.registered[nick] = client
	if rc, ok := client.(*registeredClient); ok {
		rc.resultCh <- true
	}
}

func (h *Hub) handleUnregister(client ClientInterface) {
	nick := clientNickname(client)
	if _, ok := h.registered[nick]; ok {
		delete(h.registered, nick)
		h.handleBroadcast(&domain.Message{
			Type:  domain.UserListMessage,
			Users: h.RegisteredUsers(),
		})
	}
}

func (h *Hub) handleBroadcast(message *domain.Message) {
	for _, client := range h.registered {
		client.Send(message)
	}
}

// GRACE: Broadcast — thread-safe, посылает сообщение через канал.
func (h *Hub) Broadcast(message *domain.Message) {
	h.broadcast <- message
}

// GRACE: SendTo — thread-safe отправка конкретному клиенту.
// Возвращает false, если клиент не найден.
func (h *Hub) SendTo(nickname string, message *domain.Message) bool {
	client, ok := h.registered[nickname]
	if !ok {
		return false
	}
	client.Send(message)
	return true
}

// GRACE: HasUser — НЕ thread-safe в изоляции, но безопасна в контексте
// одного вызывающего (одна горутина handler.go). Для атомарной проверки
// используйте RegisterWithResult.
func (h *Hub) HasUser(nickname string) bool {
	_, ok := h.registered[nickname]
	return ok
}

func (h *Hub) RegisteredUsers() []string {
	users := make([]string, 0, len(h.registered))
	for nick := range h.registered {
		users = append(users, nick)
	}
	return users
}

func clientNickname(c ClientInterface) string {
	if cn, ok := c.(interface{ Nickname() string }); ok {
		return cn.Nickname()
	}
	return "unknown"
}

// registeredClient — обёртка для RegisterWithResult, передаёт результат обратно вызывающему.
type registeredClient struct {
	ClientInterface
	resultCh chan<- bool
}

func (rc *registeredClient) Send(msg *domain.Message) {
	rc.ClientInterface.Send(msg)
}

func (rc *registeredClient) Close() {
	rc.ClientInterface.Close()
}
