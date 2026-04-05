package hub

import "example.com/m/v2/internal/domain"

type Client interface {
	Send(message domain.Message)
}

type Hub struct {
	registered map[string]Client
	register   chan Client
	unregister chan Client
	broadcast  chan domain.Message
}

func NewHub() *Hub {
	return &Hub{
		registered: make(map[string]Client),
		register:   make(chan Client),
		unregister: make(chan Client),
		broadcast:  make(chan domain.Message),
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
		}
	}
}

func (h *Hub) handleRegister(client Client) {
	h.registered[clientNickname(client)] = client
}

func (h *Hub) handleUnregister(client Client) {
	nick := clientNickname(client)
	if _, ok := h.registered[nick]; ok {
		delete(h.registered, nick)
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

func clientNickname(c Client) string {
	if cn, ok := c.(interface{ Nickname() string }); ok {
		return cn.Nickname()
	}
	return "unknow"
}
