// GRACE: Client — пара горутин (ReadPump/WritePump), управляющих одним WebSocket-соединением.
//
// КРИТИЧЕСКИЙ ИНВАРИАНТ: только WritePump пишет в conn.
// Client.Close() НЕ вызывает c.conn.Close() — он лишь закрывает каналы done и send.
// Это предотвращает конкурентные записи (gorilla/websocket НЕ thread-safe для Write).
//
// ПОТОК ЗАКРЫТИЯ:
//  1. Close() → close(done) + close(send)
//  2. WritePump видит ok == false → отправляет CloseMessage → выходит
//  3. ReadPump видит ошибку чтения → выходит, посылает unregister в Hub (non-blocking)
//  4. defer WritePump → c.conn.Close() (единственный, кто пишет в conn при закрытии)
package hub

import (
	"encoding/json"
	"log"
	"sync"
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
	send     chan *domain.Message
	done     chan struct{}
	once     sync.Once
}

func NewClient(nickname string, hub *Hub, conn *protocol.Connection) *Client {
	return &Client{
		nickname: nickname,
		hub:      hub,
		conn:     conn,
		send:     make(chan *domain.Message, 256),
		done:     make(chan struct{}),
	}
}

func (c *Client) Nickname() string {
	return c.nickname
}

// GRACE: Send отправляет сообщение через буферизованный канал.
// DECISION: select с <-c.done предотвращает панику «send on closed channel».
// Если клиент закрыт — сообщение тихо отбрасывается.
func (c *Client) Send(message *domain.Message) {
	select {
	case c.send <- message:
	case <-c.done:
		// Client is closed, silently drop the message.
	}
}

// GRACE: Close сигнализирует об остановке.
// DECISION: не пишет в conn — это делает только WritePump. Закрывает done (для Send)
// и send (для WritePump). sync.Once гарантирует однократность.
func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		close(c.send)
	})
}

// GRACE: ReadPump — горутина чтения.
// DECISION: non-blocking unregister в defer — Hub может быть уже остановлен (Shutdown закрыл done).
// Без select с <-h.hub.done горутина зависнет навсегда.
func (c *Client) ReadPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		case <-c.hub.done:
		}
	}()

	c.conn.RawConn().SetReadLimit(maxMessageSize)

	c.conn.RawConn().SetPongHandler(func(string) error {
		_ = c.conn.RawConn().SetReadDeadline(time.Now().Add(pongWait))
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
			c.hub.Broadcast(&msg)
		case domain.PrivateMessage:
			// Отправляем себе тоже, чтобы видеть свои сообщения.
			c.Send(&msg)
			if !c.hub.SendTo(msg.Recipient, &msg) {
				errMsg := &domain.Message{
					Type:      domain.ErrorMessage,
					Sender:    "system",
					Content:   "user not found: " + msg.Recipient,
					Timestamp: time.Now(),
				}
				c.Send(errMsg)
			}
		default:
			// SystemMessage, ErrorMessage, UserListMessage — только от сервера,
			// клиент не должен их отправлять. Игнорируем.
		}
	}
}

// GRACE: WritePump — горутина записи. ЕДИНСТВЕННЫЙ, кто пишет в conn.
// DECISION: batch-оптимизация — склеивает несколько сообщений в один WS-фрейм.
// При закрытии send-канала отправляет CloseMessage и закрывает conn.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close() // WritePump — единственный, кто пишет в conn при закрытии
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Канал закрыт — отправляем CloseMessage и выходим.
				_ = c.conn.WriteControl(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(message)
			if err != nil {
				log.Printf("Error Marshaling: %v \n", err)
				return
			}

			if err := c.conn.RawConn().SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			w, err := c.conn.RawConn().NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(data); err != nil {
				return
			}

			n := len(c.send)
			for i := 0; i < n; i++ {
				if _, err := w.Write([]byte{'\n'}); err != nil {
					return
				}
				data, err := json.Marshal(<-c.send)
				if err != nil {
					log.Printf("Error Marshaling batch: %v \n", err)
					break
				}
				if _, err := w.Write(data); err != nil {
					return
				}
			}

			if err := w.Close(); err != nil {
				log.Println("Error closing writer")
				return
			}

		case <-ticker.C:
			if err := c.conn.RawConn().SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.RawConn().WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
