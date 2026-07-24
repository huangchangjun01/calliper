package services

import (
	"time"

	"github.com/shopspring/decimal"
)

// Broker defines the abstract interface for broker API operations.
type Broker interface {
	PlaceOrder(req PlaceOrderRequest) (*PlaceOrderResponse, error)
	CancelOrder(orderID string) error
	QueryOrder(orderID string) (*OrderStatus, error)
	QueryPositions() ([]PositionInfo, error)
	QueryAccount() (*AccountInfo, error)
	GetBrokerName() string
}

// PlaceOrderRequest represents a request to place a trading order.
type PlaceOrderRequest struct {
	Symbol    string          `json:"symbol"`
	Action    string          `json:"action"`     // buy / sell
	OrderType string          `json:"order_type"` // market / limit
	Price     decimal.Decimal `json:"price"`
	Quantity  int             `json:"quantity"`
	TradeType string          `json:"trade_type"` // real / simulated
}

// PlaceOrderResponse represents the response from placing an order.
type PlaceOrderResponse struct {
	OrderID        string          `json:"order_id"`
	Status         string          `json:"status"`
	FilledPrice    decimal.Decimal `json:"filled_price"`
	FilledQuantity int             `json:"filled_quantity"`
	Message        string          `json:"message"`
}

// OrderStatus represents the current status of an order queried from the broker.
type OrderStatus struct {
	OrderID           string    `json:"order_id"`
	Symbol            string    `json:"symbol"`
	Status            string    `json:"status"`
	FilledPrice       float64   `json:"filled_price"`
	FilledQuantity    int       `json:"filled_quantity"`
	RemainingQuantity int       `json:"remaining_quantity"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// PositionInfo represents a holding position returned by the broker.
type PositionInfo struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Quantity      int     `json:"quantity"`
	AvgCost       float64 `json:"avg_cost"`
	CurrentPrice  float64 `json:"current_price"`
	MarketValue   float64 `json:"market_value"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
}

// AccountInfo represents account asset information returned by the broker.
type AccountInfo struct {
	TotalAssets  float64 `json:"total_assets"`
	AvailableCash float64 `json:"available_cash"`
	FrozenCash   float64 `json:"frozen_cash"`
	MarketValue  float64 `json:"market_value"`
	TotalPnL     float64 `json:"total_pnl"`
	TodayPnL     float64 `json:"today_pnl"`
}