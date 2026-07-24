package services

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	ws "github.com/quant-trading/backend/internal/websocket"
)

// QuotePushService handles real-time quote pushing to WebSocket clients
// and Redis caching.
type QuotePushService struct {
	hub  *ws.Hub
	redis *redis.Client
	tsdb  *gorm.DB
}

// NewQuotePushService creates a new QuotePushService.
func NewQuotePushService(hub *ws.Hub, rdb *redis.Client, tsdb *gorm.DB) *QuotePushService {
	return &QuotePushService{
		hub:   hub,
		redis: rdb,
		tsdb:  tsdb,
	}
}

// Start subscribes to Redis Pub/Sub for quote updates and forwards them to
// WebSocket clients. It runs until the context is cancelled.
func (s *QuotePushService) Start(ctx context.Context) {
	if s.redis == nil {
		log.Println("[QuotePushService] Redis not available, skipping Pub/Sub")
		return
	}

	go func() {
		pubsub := s.redis.Subscribe(ctx, "quote:updates")
		defer pubsub.Close()

		ch := pubsub.Channel()
		log.Println("[QuotePushService] Subscribed to Redis channel: quote:updates")

		for {
			select {
			case <-ctx.Done():
				log.Println("[QuotePushService] Stopping Pub/Sub listener")
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var md MarketData
				if err := json.Unmarshal([]byte(msg.Payload), &md); err != nil {
					log.Printf("[QuotePushService] Failed to unmarshal message: %v", err)
					continue
				}
				s.PushQuote(md.Symbol, md)
			}
		}
	}()

	log.Println("[QuotePushService] Started")
}

// PushQuote pushes a single stock quote to WebSocket clients subscribed to
// the "stock:{symbol}" channel and updates the Redis cache.
func (s *QuotePushService) PushQuote(symbol string, data MarketData) {
	// Update Redis cache
	s.UpdateRedisCache(data)

	// Serialize and broadcast via WebSocket
	jsonData, err := ToJSON(data)
	if err != nil {
		log.Printf("[QuotePushService] Failed to serialize quote for %s: %v", symbol, err)
		return
	}

	msg := &ws.Message{
		Type:    "quote",
		Channel: "stock:" + symbol,
		Data:    jsonData,
	}
	s.hub.Broadcast <- msg
}

// PushBatch pushes multiple stock quotes.
func (s *QuotePushService) PushBatch(quotes []MarketData) {
	for i := range quotes {
		s.PushQuote(quotes[i].Symbol, quotes[i])
	}
}

// UpdateRedisCache updates the real-time quote cache in Redis.
// Key format: "quote:{symbol}", TTL: 60 seconds.
func (s *QuotePushService) UpdateRedisCache(data MarketData) {
	if s.redis == nil {
		return
	}

	jsonData, err := ToJSON(data)
	if err != nil {
		log.Printf("[QuotePushService] Failed to serialize for cache: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	key := "quote:" + data.Symbol
	if err := s.redis.Set(ctx, key, jsonData, 60*time.Second).Err(); err != nil {
		log.Printf("[QuotePushService] Failed to update Redis cache for %s: %v", data.Symbol, err)
	}
}

// GetCachedQuote retrieves a cached quote from Redis.
func (s *QuotePushService) GetCachedQuote(symbol string) (*MarketData, error) {
	if s.redis == nil {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	data, err := s.redis.Get(ctx, "quote:"+symbol).Bytes()
	if err != nil {
		return nil, err
	}

	return FromJSON(data)
}