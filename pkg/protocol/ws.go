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
	"time"

	"github.com/gorilla/websocket"
)

// GRACE: Upgrader — HTTP → WebSocket upgrade.
// DECISION: CheckOrigin: return true — для разработки. В production заменить на проверку Origin.
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(_ *http.Request) bool { return true },
}

type Connection struct {
	conn *websocket.Conn
}

func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{conn: conn}
}

func (c *Connection) ReadJSON(v interface{}) error {
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

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
	_ = c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second))

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
	_ = c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
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
