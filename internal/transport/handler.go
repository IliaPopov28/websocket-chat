package transport

import (
	"encoding/json"
	"net/http"

	"github.com/IliaPopov28/websocket-chat/internal/auth"
	"github.com/IliaPopov28/websocket-chat/internal/domain"
	"github.com/IliaPopov28/websocket-chat/internal/hub"
	"github.com/IliaPopov28/websocket-chat/pkg/protocol"
)

type Handler struct {
	hub  *hub.Hub
	auth *auth.Service
}

func NewHandler(h *hub.Hub, a *auth.Service) *Handler {
	return &Handler{hub: h, auth: a}
}

// HandleRegister — POST /api/register
// Body: {"nickname": "...", "password": "..."}
// Returns: {"token": "..."}
func (h *Handler) HandleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Nickname string `json:"nickname"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	if req.Nickname == "" || req.Password == "" {
		http.Error(w, `{"error":"nickname and password are required"}`, http.StatusBadRequest)
		return
	}

	err := h.auth.Register(r.Context(), req.Nickname, req.Password)
	if err != nil {
		if err == auth.ErrUserAlreadyExists {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"nickname is already taken"}`))
			return
		}
		http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
		return
	}

	token, err := h.auth.Login(r.Context(), req.Nickname, req.Password)
	if err != nil {
		http.Error(w, `{"error":"registration succeeded but login failed"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// HandleLogin — POST /api/login
// Body: {"nickname": "...", "password": "..."}
// Returns: {"token": "..."}
func (h *Handler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Nickname string `json:"nickname"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
		return
	}

	token, err := h.auth.Login(r.Context(), req.Nickname, req.Password)
	if err != nil {
		if err == auth.ErrUserNotFound {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"error":"user not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"invalid credentials"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

// HandleWebSocket — GET /ws?token=...
// Извлекает nickname из JWT-токена, подключает к чату.
func (h *Handler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, "token is required", http.StatusBadRequest)
		return
	}

	nickname, err := h.auth.ValidateToken(tokenStr)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	if h.hub.HasUser(nickname) {
		http.Error(w, "already connected", http.StatusConflict)
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
