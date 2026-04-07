package hub

import (
	"encoding/json"
	"log"
	"time"

	"github.com/IliaPopov28/websocket-chat/internal/domain"
	"github.com/IliaPopov28/websocket-chat/pkg/protocol"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 20 * time.Second
	pongWait       = 20 * time.Second
	pingPeriod     = pongWait * 2 / 3
	maxMessageSize = 1024 * 1024
)

type Client struct {
	nickname string
	hub      *Hub
	conn     *protocol.Connection
	send     chan domain.Message
}

func NewClient(nickname string, hub *Hub, conn *protocol.Connection) *Client {
	return &Client{nickname: nickname,
		hub:  hub,
		conn: conn,
		send: make(chan domain.Message, 256),
	}
}

func (c *Client) Nickname() string {
	return c.nickname
}

func (c *Client) Send(message domain.Message) {
	c.send <- message
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.RawConn().SetReadLimit(maxMessageSize)

	c.conn.RawConn().SetPongHandler(func(string) error {
		c.conn.RawConn().SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		var msg domain.Message
		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("error of read: %v \n", err)
			}
			break
		}

		msg.Sender = c.nickname
		msg.Timestamp = time.Now()

		switch msg.Type {
		case domain.PublicMessage:
			c.hub.Broadcast(msg)
		case domain.PrivateMessage:
			if !c.hub.SendTo(msg.Recipient, msg) {
				errMsg := domain.Message{
					Type:      domain.ErrorMessage,
					Sender:    "system",
					Content:   "user not found:" + msg.Recipient,
					Timestamp: time.Now(),
				}
				c.send <- errMsg
			}
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.conn.WriteControl(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error Marshaling: %v \n", err)
				return
			}

			c.conn.RawConn().SetWriteDeadline(time.Now().Add(writeWait))
			w, err := c.conn.RawConn().NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(data)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				data, err := json.Marshal(<-c.send)
				if err != nil {
					log.Printf("Error Marshaling batch: %v \n", err)
					break
				}
				w.Write(data)
			}

			if err := w.Close(); err != nil {
				log.Println("Error closing writer")
				return
			}

		case <-ticker.C:
			c.conn.RawConn().SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.RawConn().WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
