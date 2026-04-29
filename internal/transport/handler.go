// GRACE: transport — HTTP-эндпоинты (register, login) и WebSocket upgrade.
//
// ПОТОК WebSocket-подключения:
//  1. Извлечь JWT из query ?token=...
//  2. Валидировать токен → получить nickname
//  3. Upgrade HTTP → WebSocket
//  4. Создать Client → атомарно зарегистрировать через RegisterWithResult
//  5. Если регистрация неудачна (ник занят) → закрыть клиента и вернуть
//  6. Запустить WritePump и ReadPump в отдельных горутинах.
//
// DECISION: upgrade ПЕРЕД регистрацией — upgrade может упасть (сетевая ошибка).
// Если регистрировать до upgrade, при ошибке клиент останется «зарегистрированным»
// без реального соединения. Сейчас: upgrade → register → если register fail → close client.
package transport

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"github.com/IliaPopov28/websocket-chat/internal/auth"
	"github.com/IliaPopov28/websocket-chat/internal/domain"
	"github.com/IliaPopov28/websocket-chat/internal/hub"
	"github.com/IliaPopov28/websocket-chat/pkg/protocol"
	"github.com/gorilla/websocket"
)

type Handler struct {
	hub      *hub.Hub
	auth     *auth.Service
	upgrader *websocket.Upgrader
}

func NewHandler(h *hub.Hub, a *auth.Service, upgrader *websocket.Upgrader) *Handler {
	return &Handler{hub: h, auth: a, upgrader: upgrader}
}

// Returns: {"token": "..."}.
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
		if errors.Is(err, auth.ErrUserAlreadyExists) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			_, _ = w.Write([]byte(`{"error":"nickname is already taken"}`))
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
	if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
		log.Printf("Failed to encode token: %v", err)
	}
}

// Returns: {"token": "..."}.
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
		if errors.Is(err, auth.ErrUserNotFound) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":"user not found"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"token": token}); err != nil {
		log.Printf("Failed to encode token: %v", err)
	}
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

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	wsConn := protocol.NewConnection(conn)

	client := hub.NewClient(nickname, h.hub, wsConn)

	// Атомарная регистрация — возвращает false, если nickname уже занят.
	if !h.hub.RegisterWithResult(client) {
		client.Close()
		return
	}

	h.hub.Broadcast(&domain.Message{
		Type:    domain.SystemMessage,
		Sender:  "system",
		Content: nickname + " joined the chat",
	})

	h.hub.Broadcast(&domain.Message{
		Type:  domain.UserListMessage,
		Users: h.hub.RegisteredUsers(),
	})

	go client.WritePump()
	go client.ReadPump()
}
