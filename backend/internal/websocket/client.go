package websocket

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a single WebSocket connection.
type Client struct {
	hub *Hub

	// The WebSocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan *Message

	// Channels the client is subscribed to.
	subscriptions []string

	mu sync.Mutex

	// sendClosed tracks whether the send channel has been closed, so that
	// Send never writes to a closed channel. Guarded by mu.
	sendClosed bool

	// closeOnce guarantees the send channel is closed exactly once,
	// even when unregister/removal happen from multiple code paths.
	closeOnce *sync.Once
}

// NewClient creates a new Client instance.
func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:           hub,
		conn:          conn,
		send:          make(chan *Message, 256),
		subscriptions: make([]string, 0),
		closeOnce:     &sync.Once{},
	}
}

// closeSend closes the outbound channel exactly once. It is safe to call
// from multiple goroutines or repeatedly on the same client.
func (c *Client) closeSend() {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.sendClosed = true
		close(c.send)
		c.mu.Unlock()
	})
}

// Send attempts to enqueue a message for the WritePump to encode and write.
// It never blocks and returns false if the send buffer is full or the channel
// has been closed (in which case the message is dropped). This is the only
// enqueue path that may race with closeSend; it is serialized via mu.
func (c *Client) Send(msg *Message) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sendClosed {
		return false
	}
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

// SnapshotSubscriptions returns a copy of the subscribed channels under the
// client mutex, so callers can iterate without racing Subscribe/Unsubscribe.
func (c *Client) SnapshotSubscriptions() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	subs := make([]string, len(c.subscriptions))
	copy(subs, c.subscriptions)
	return subs
}

// ReadPump pumps messages from the WebSocket connection to the hub.
//
// The application runs ReadPump in a per-connection goroutine. It ensures
// that there is at most one reader on a connection by executing all reads
// from this goroutine.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, messageBytes, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("websocket error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("websocket: invalid message format: %v", err)
			continue
		}

		switch msg.Type {
		case "subscribe":
			if msg.Channel != "" {
				c.hub.Subscribe(c, msg.Channel)
			}
		case "unsubscribe":
			if msg.Channel != "" {
				c.hub.Unsubscribe(c, msg.Channel)
			}
		case "message":
			// Forward the message to the hub for broadcasting
			if msg.Channel != "" {
				c.hub.Broadcast <- &msg
			}
		}
	}
}

// WritePump pumps messages from the hub to the WebSocket connection.
//
// A goroutine running WritePump is started for each connection. The
// application ensures that there is at most one writer to a connection by
// executing all writes from this goroutine.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(message); err != nil {
				return
			}
			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
