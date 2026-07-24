package services

import (
	"encoding/json"
	"time"
)

// WSMessage represents a WebSocket message exchanged between server and clients.
type WSMessage struct {
	Type    string      `json:"type"`    // "quote", "subscribe", "unsubscribe", "ping", "pong"
	Channel string      `json:"channel"` // "stock:AAPL", "market:NASDAQ"
	Data    interface{} `json:"data"`
	Time    int64       `json:"time"`
}

// NewWSMessage creates a new WSMessage with the current timestamp.
func NewWSMessage(msgType, channel string, data interface{}) WSMessage {
	return WSMessage{
		Type:    msgType,
		Channel: channel,
		Data:    data,
		Time:    time.Now().UnixMilli(),
	}
}

// ToJSON serializes a MarketData struct to JSON bytes.
func ToJSON(data MarketData) ([]byte, error) {
	return json.Marshal(data)
}

// FromJSON deserializes JSON bytes into a MarketData struct.
func FromJSON(data []byte) (*MarketData, error) {
	var md MarketData
	if err := json.Unmarshal(data, &md); err != nil {
		return nil, err
	}
	return &md, nil
}