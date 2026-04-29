// GRACE: protocol — тонкая обёртка над gorilla/websocket.
// Предоставляет JSON-сериализацию и управление дедлайнами.
//
// ИНВАРИАНТЫ:
//   - ReadJSON/WriteJSON устанавливают дедлайны перед каждой операцией
//   - Close() отправляет WS CloseMessage, но НЕ закрывает TCP-соединение
//   - RawConn() даёт доступ к *websocket.Conn для SetReadLimit/SetPongHandler
package protocol

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// UpgraderConfig — конфигурация для WebSocket upgrade.
type UpgraderConfig struct {
	AllowedOrigins []string // Дополнительные разрешённые Origin помимо same-origin.
}

// NewUpgrader создаёт Upgrader с проверкой Origin.
// Same-origin запросы и запросы без Origin разрешаются всегда.
func NewUpgrader(cfg UpgraderConfig) *websocket.Upgrader {
	allowed := normalizeOrigins(cfg.AllowedOrigins)
	checkOrigin := func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		if isSameOrigin(origin, r.Host) {
			return true
		}
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		return false
	}

	return &websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     checkOrigin,
	}
}

func normalizeOrigins(origins []string) []string {
	result := make([]string, 0, len(origins))
	for _, o := range origins {
		o = strings.TrimSpace(o)
		if o != "" {
			result = append(result, o)
		}
	}
	return result
}

func isSameOrigin(origin, host string) bool {
	parsedOrigin, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return parsedOrigin.Host == host
}

type Connection struct {
	conn *websocket.Conn
}

func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{conn: conn}
}

func (c *Connection) ReadJSON(v interface{}) error {
	if err := c.conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("read message: %w", err)
	}

	if err := json.Unmarshal(message, v); err != nil {
		return fmt.Errorf("unmarshal JSON: %w", err)
	}
	return nil
}

func (c *Connection) WriteJSON(v interface{}) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}

	if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
		return fmt.Errorf("write message: %w", err)
	}
	return nil
}

func (c *Connection) WriteControl(messageType int, data []byte) error {
	if err := c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second)); err != nil {
		return fmt.Errorf("set write deadline: %w", err)
	}
	if err := c.conn.WriteControl(messageType, data, time.Now().Add(20*time.Second)); err != nil {
		return fmt.Errorf("write control: %w", err)
	}
	return nil
}

func (c *Connection) Close() error {
	if err := c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(5*time.Second),
	); err != nil {
		return fmt.Errorf("close: %w", err)
	}
	return nil
}

func (c *Connection) RawConn() *websocket.Conn {
	return c.conn
}
