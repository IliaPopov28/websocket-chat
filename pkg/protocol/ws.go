package protocol

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

type Connection struct {
	conn *websocket.Conn
}

func NewConnection(conn *websocket.Conn) *Connection {
	return &Connection{conn: conn}
}

func (c *Connection) ReadJSON(v interface{}) error {
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))

	_, message, err := c.conn.ReadMessage()
	if err != nil {
		return err
	}

	return json.Unmarshal(message, v)
}

func (c *Connection) WriteJSON(v interface{}) error {
	c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second))

	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	return c.conn.WriteMessage(websocket.TextMessage, data)
}

func (c *Connection) WriteControl(messageType int, data []byte) error {
	c.conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	return c.conn.WriteControl(messageType, data, time.Now().Add(20*time.Second))
}

func (c *Connection) Close() error {
	return c.conn.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(5*time.Second),
	)
}

func (c *Connection) RawConn() *websocket.Conn {
	return c.conn
}
