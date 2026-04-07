package transport

import (
	"net/http"

	"github.com/IliaPopov28/websocket-chat/internal/domain"
	"github.com/IliaPopov28/websocket-chat/internal/hub"
	"github.com/IliaPopov28/websocket-chat/pkg/protocol"
)

type Handler struct {
	hub *hub.Hub
}

func NewHandler(h *hub.Hub) *Handler {
	return &Handler{hub: h}
}

// HandleCheckNick — HTTP endpoint для проверки доступности ника.
// GET /api/check-nick?nick=...
// Возвращает 200 OK если ник свободен, 409 Conflict если занят.
func (h *Handler) HandleCheckNick(w http.ResponseWriter, r *http.Request) {
	nickname := r.URL.Query().Get("nick")
	if nickname == "" {
		http.Error(w, `{"error":"nickname is required"}`, http.StatusBadRequest)
		return
	}

	if h.hub.HasUser(nickname) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(`{"error":"nickname is already taken"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"available":true}`))
}

func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	nickname := r.URL.Query().Get("nick")
	if nickname == "" {
		http.Error(w, "nickname is required", http.StatusBadRequest)
		return
	}

	if h.hub.HasUser(nickname) {
		http.Error(w, "nickname is already taken", http.StatusConflict)
		return
	}

	conn, err := protocol.Upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	wsConn := protocol.NewConnection(conn)

	client := hub.NewClient(nickname, h.hub, wsConn)
	h.hub.Register(client)

	h.hub.Broadcast(domain.Message{
		Type:    domain.SystemMessage,
		Sender:  "system",
		Content: nickname + " joined the chat",
	})

	h.hub.Broadcast(domain.Message{
		Type:  domain.UserListMessage,
		Users: h.hub.RegisteredUsers(),
	})

	go client.WritePump()
	go client.ReadPump()
}
