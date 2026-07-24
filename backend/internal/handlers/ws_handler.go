package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/services"
	ws "github.com/quant-trading/backend/internal/websocket"
)

const (
	wsWriteWait  = 10 * time.Second
	wsPongWait   = 60 * time.Second
	wsMaxMsgSize = 4096
)

// WsHandler handles WebSocket connections for real-time quote streaming.
type WsHandler struct {
	hub          *ws.Hub
	upgrader     websocket.Upgrader
	jwtSecret    string
	subManager   *services.QuoteSubscriptionManager
	quoteService *services.QuotePushService
}

// NewWsHandler creates a new WsHandler.
func NewWsHandler(hub *ws.Hub, jwtSecret string, subManager *services.QuoteSubscriptionManager, quoteService *services.QuotePushService) *WsHandler {
	return &WsHandler{
		hub:          hub,
		jwtSecret:    jwtSecret,
		subManager:   subManager,
		quoteService: quoteService,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
}

// HandleWebSocket handles the WebSocket upgrade request.
// Validates JWT from the "token" query parameter and registers the client.
func (h *WsHandler) HandleWebSocket(c *gin.Context) {
	tokenString := c.Query("token")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing token query parameter"})
		return
	}

	claims, err := h.parseToken(tokenString)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[WsHandler] Upgrade error: %v", err)
		return
	}

	userID := claims.UserID
	log.Printf("[WsHandler] WebSocket connected: user=%s", userID)

	client := ws.NewClient(h.hub, conn)
	h.hub.Register(client)

	go client.WritePump()
	go h.readPump(client, conn, userID)
}

// readPump reads messages from the WebSocket connection and dispatches them.
func (h *WsHandler) readPump(client *ws.Client, conn *websocket.Conn, userID string) {
	defer func() {
		h.hub.Unregister(client)
		conn.Close()
		log.Printf("[WsHandler] WebSocket disconnected: user=%s", userID)
	}()

	conn.SetReadLimit(wsMaxMsgSize)
	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	for {
		_, messageBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WsHandler] WebSocket error (user=%s): %v", userID, err)
			}
			break
		}

		var msg services.WSMessage
		if err := json.Unmarshal(messageBytes, &msg); err != nil {
			log.Printf("[WsHandler] Invalid message (user=%s): %v", userID, err)
			continue
		}

		switch msg.Type {
		case "subscribe":
			if msg.Channel != "" {
				h.hub.Subscribe(client, msg.Channel)
				symbol := extractSymbol(msg.Channel)
				if symbol != "" {
					h.subManager.Subscribe(symbol)
				}
				log.Printf("[WsHandler] User %s subscribed to %s", userID, msg.Channel)
			}

		case "unsubscribe":
			if msg.Channel != "" {
				h.hub.Unsubscribe(client, msg.Channel)
				symbol := extractSymbol(msg.Channel)
				if symbol != "" {
					h.subManager.Unsubscribe(symbol)
				}
				log.Printf("[WsHandler] User %s unsubscribed from %s", userID, msg.Channel)
			}

		case "ping":
			pongMsg := services.NewWSMessage("pong", "", nil)
			conn.SetWriteDeadline(time.Now().Add(wsWriteWait))
			if err := conn.WriteJSON(pongMsg); err != nil {
				log.Printf("[WsHandler] Failed to send pong (user=%s): %v", userID, err)
				return
			}
		}
	}
}

// parseToken validates a JWT token string and returns the claims.
func (h *WsHandler) parseToken(tokenString string) (*middleware.Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &middleware.Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*middleware.Claims)
	if !ok || !token.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

// extractSymbol extracts the stock symbol from a channel string.
// "stock:AAPL" → "AAPL", "market:NASDAQ" → ""
func extractSymbol(channel string) string {
	if strings.HasPrefix(channel, "stock:") {
		return strings.TrimPrefix(channel, "stock:")
	}
	return ""
}