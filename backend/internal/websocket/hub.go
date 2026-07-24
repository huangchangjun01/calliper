package websocket

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer.
	maxMessageSize = 4096
)

// Message represents a WebSocket message.
type Message struct {
	Type    string          `json:"type"`
	Channel string          `json:"channel,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// Hub maintains the set of active clients and broadcasts messages to clients.
type Hub struct {
	// Registered clients by channel.
	channels map[string]map[*Client]bool

	// Inbound messages from the clients.
	Broadcast chan *Message

	// Register requests from the clients.
	register chan *Client

	// Unregister requests from clients.
	unregister chan *Client

	mu sync.RWMutex
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		channels:   make(map[string]map[*Client]bool),
		Broadcast:  make(chan *Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run starts the Hub's main loop.
func (h *Hub) Run() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			for _, channel := range client.subscriptions {
				if h.channels[channel] == nil {
					h.channels[channel] = make(map[*Client]bool)
				}
				h.channels[channel][client] = true
			}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			for _, channel := range client.subscriptions {
				if clients, ok := h.channels[channel]; ok {
					if _, ok := clients[client]; ok {
						delete(clients, client)
						close(client.send)
						// Clean up empty channels
						if len(clients) == 0 {
							delete(h.channels, channel)
						}
					}
				}
			}
			h.mu.Unlock()

		case message := <-h.Broadcast:
			h.mu.RLock()
			if clients, ok := h.channels[message.Channel]; ok {
				for client := range clients {
					select {
					case client.send <- message:
					default:
						// Client send buffer is full, remove it
						h.mu.RUnlock()
						h.mu.Lock()
						h.removeClient(client)
						h.mu.Unlock()
						h.mu.RLock()
					}
				}
			}
			h.mu.RUnlock()

		case <-ticker.C:
			h.mu.RLock()
			for _, clients := range h.channels {
				for client := range clients {
					client.conn.SetWriteDeadline(time.Now().Add(writeWait))
					if err := client.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
						h.mu.RUnlock()
						h.mu.Lock()
						h.removeClient(client)
						h.mu.Unlock()
						h.mu.RLock()
					}
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Register adds a client to the Hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the Hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Subscribe adds a client to a channel.
func (h *Hub) Subscribe(client *Client, channel string) {
	client.mu.Lock()
	// Check if already subscribed
	for _, ch := range client.subscriptions {
		if ch == channel {
			client.mu.Unlock()
			return
		}
	}
	client.subscriptions = append(client.subscriptions, channel)
	client.mu.Unlock()

	h.mu.Lock()
	if h.channels[channel] == nil {
		h.channels[channel] = make(map[*Client]bool)
	}
	h.channels[channel][client] = true
	h.mu.Unlock()
}

// Unsubscribe removes a client from a channel.
func (h *Hub) Unsubscribe(client *Client, channel string) {
	client.mu.Lock()
	for i, ch := range client.subscriptions {
		if ch == channel {
			client.subscriptions = append(client.subscriptions[:i], client.subscriptions[i+1:]...)
			break
		}
	}
	client.mu.Unlock()

	h.mu.Lock()
	if clients, ok := h.channels[channel]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.channels, channel)
		}
	}
	h.mu.Unlock()
}

func (h *Hub) removeClient(client *Client) {
	for _, channel := range client.subscriptions {
		if clients, ok := h.channels[channel]; ok {
			if _, ok := clients[client]; ok {
				delete(clients, client)
				if len(clients) == 0 {
					delete(h.channels, channel)
				}
			}
		}
	}
	close(client.send)
}