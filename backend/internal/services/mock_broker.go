package services

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/shopspring/decimal"
)

// MockBroker is a simulated broker adapter for development and testing.
type MockBroker struct {
	baseAccount *AccountInfo
	rng         *rand.Rand
}

// NewMockBroker creates a new MockBroker with a default initial account.
func NewMockBroker() *MockBroker {
	return &MockBroker{
		baseAccount: &AccountInfo{
			TotalAssets:   1000000.00,
			AvailableCash: 1000000.00,
			FrozenCash:    0,
			MarketValue:   0,
			TotalPnL:      0,
			TodayPnL:      0,
		},
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetBrokerName returns the name of this broker.
func (m *MockBroker) GetBrokerName() string {
	return "MockBroker"
}

// PlaceOrder simulates placing an order with 80% fill probability.
func (m *MockBroker) PlaceOrder(req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	m.simulateLatency()

	orderID := fmt.Sprintf("MOCK-%d-%d", time.Now().UnixNano(), m.rng.Intn(10000))

	filled := m.rng.Float64() < 0.8

	if !filled {
		return &PlaceOrderResponse{
			OrderID:        orderID,
			Status:         "pending",
			FilledPrice:    decimal.Zero,
			FilledQuantity: 0,
			Message:        "order submitted, waiting for fill",
		}, nil
	}

	var fillPrice decimal.Decimal
	if req.OrderType == "market" || req.Price.IsZero() {
		fillPrice = decimal.NewFromFloat(100.0 + m.rng.Float64()*10.0)
	} else {
		// Simulate slight slippage for limit orders
		slippage := decimal.NewFromFloat((m.rng.Float64() - 0.5) * 0.02)
		fillPrice = req.Price.Add(req.Price.Mul(slippage))
	}

	return &PlaceOrderResponse{
		OrderID:        orderID,
		Status:         "filled",
		FilledPrice:    fillPrice,
		FilledQuantity: req.Quantity,
		Message:        "order filled",
	}, nil
}

// CancelOrder simulates order cancellation.
func (m *MockBroker) CancelOrder(orderID string) error {
	m.simulateLatency()
	return nil
}

// QueryOrder simulates querying an order status.
func (m *MockBroker) QueryOrder(orderID string) (*OrderStatus, error) {
	m.simulateLatency()

	now := time.Now()
	return &OrderStatus{
		OrderID:           orderID,
		Symbol:            "AAPL",
		Status:            "filled",
		FilledPrice:       150.0,
		FilledQuantity:    100,
		RemainingQuantity: 0,
		CreatedAt:         now.Add(-5 * time.Minute),
		UpdatedAt:         now,
	}, nil
}

// QueryPositions returns simulated holding positions.
func (m *MockBroker) QueryPositions() ([]PositionInfo, error) {
	m.simulateLatency()

	return []PositionInfo{
		{
			Symbol:        "AAPL",
			Name:          "Apple Inc.",
			Quantity:      100,
			AvgCost:       145.00,
			CurrentPrice:  150.00,
			MarketValue:   15000.00,
			UnrealizedPnL: 500.00,
		},
		{
			Symbol:        "GOOGL",
			Name:          "Alphabet Inc.",
			Quantity:      50,
			AvgCost:       140.00,
			CurrentPrice:  142.00,
			MarketValue:   7100.00,
			UnrealizedPnL: 100.00,
		},
	}, nil
}

// QueryAccount returns simulated account asset information.
func (m *MockBroker) QueryAccount() (*AccountInfo, error) {
	m.simulateLatency()

	return &AccountInfo{
		TotalAssets:   m.baseAccount.TotalAssets,
		AvailableCash: m.baseAccount.AvailableCash,
		FrozenCash:    m.baseAccount.FrozenCash,
		MarketValue:   m.baseAccount.MarketValue,
		TotalPnL:      m.baseAccount.TotalPnL,
		TodayPnL:      m.baseAccount.TodayPnL,
	}, nil
}

// simulateLatency adds a random delay between 50 and 200 ms.
func (m *MockBroker) simulateLatency() {
	delay := time.Duration(50+m.rng.Intn(150)) * time.Millisecond
	time.Sleep(delay)
}