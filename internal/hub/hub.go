package hub

import "github.com/IliaPopov28/websocket-chat/internal/domain"

type ClientInterface interface {
	Send(message domain.Message)
	Close()
}

type Hub struct {
	registered map[string]ClientInterface
	register   chan ClientInterface
	unregister chan ClientInterface
	broadcast  chan domain.Message
	done       chan struct{}
}

func NewHub() *Hub {
	return &Hub{
		registered: make(map[string]ClientInterface),
		register:   make(chan ClientInterface),
		unregister: make(chan ClientInterface),
		broadcast:  make(chan domain.Message),
		done:       make(chan struct{}),
	}
}

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

// Shutdown закрывает все активные соединения и останавливает Hub.
// Не модифицирует map напрямую — это безопасно, т.к. Run() уже получает сигнал через done.
func (h *Hub) Shutdown() {
	close(h.done)
	// Закрываем клиентов — это разорвёт ReadPump/WritePump, и они завершатся.
	for _, client := range h.registered {
		client.Close()
	}
	// map НЕ переназначаем — Run() может ещё читать его перед выходом.
}

// Register отправляет клиента на регистрацию (без проверки результата).
func (h *Hub) Register(client ClientInterface) {
	h.register <- client
}

// RegisterWithResult регистрирует клиента и возвращает false, если nickname уже занят.
func (h *Hub) RegisterWithResult(client ClientInterface) bool {
	resultCh := make(chan bool, 1)
	h.register <- &registeredClient{ClientInterface: client, resultCh: resultCh}
	return <-resultCh
}

func (h *Hub) handleRegister(client ClientInterface) {
	nick := clientNickname(client)
	if _, exists := h.registered[nick]; exists {
		// Ник уже занят — отклоняем.
		if rc, ok := client.(*registeredClient); ok {
			rc.resultCh <- false
		}
		return
	}
	h.registered[nick] = client
	// Если это RegisterWithResult — сообщаем об успехе.
	if rc, ok := client.(*registeredClient); ok {
		rc.resultCh <- true
	}
}

func (h *Hub) handleUnregister(client ClientInterface) {
	nick := clientNickname(client)
	if _, ok := h.registered[nick]; ok {
		delete(h.registered, nick)
		h.handleBroadcast(domain.Message{
			Type:  domain.UserListMessage,
			Users: h.RegisteredUsers(),
		})
	}
}

func (h *Hub) handleBroadcast(message domain.Message) {
	for _, client := range h.registered {
		client.Send(message)
	}
}

func (h *Hub) Broadcast(message domain.Message) {
	h.broadcast <- message
}

func (h *Hub) SendTo(nickname string, message domain.Message) bool {
	client, ok := h.registered[nickname]
	if !ok {
		return false
	}
	client.Send(message)
	return true
}

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

func (rc *registeredClient) Send(msg domain.Message) {
	rc.ClientInterface.Send(msg)
}

func (rc *registeredClient) Close() {
	rc.ClientInterface.Close()
}
