package services

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/quant-trading/backend/internal/models"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// MockBroker is a simulated broker adapter that uses real market data for pricing.
type MockBroker struct {
	db    *gorm.DB
	rng   *rand.Rand
}

// NewMockBroker creates a new MockBroker with a database connection for real prices.
func NewMockBroker(db *gorm.DB) *MockBroker {
	return &MockBroker{
		db:  db,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetBrokerName returns the name of this broker.
func (m *MockBroker) GetBrokerName() string {
	return "MockBroker"
}

// PlaceOrder simulates placing an order using real market prices from the database.
func (m *MockBroker) PlaceOrder(req PlaceOrderRequest) (*PlaceOrderResponse, error) {
	orderID := fmt.Sprintf("MOCK-%d-%d", time.Now().UnixNano(), m.rng.Intn(10000))

	var fillPrice decimal.Decimal
	if req.OrderType == "market" || req.Price.IsZero() {
		// Look up real market price from database
		realPrice := m.getRealPrice(req.Symbol)
		if realPrice > 0 {
			fillPrice = decimal.NewFromFloat(realPrice)
		} else {
			return nil, fmt.Errorf("无法获取 %s 的实时价格", req.Symbol)
		}
	} else {
		// Limit order: use the specified price with slight slippage
		slippage := decimal.NewFromFloat((m.rng.Float64() - 0.5) * 0.002)
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
	return nil
}

// QueryOrder queries an order from the database.
func (m *MockBroker) QueryOrder(orderID string) (*OrderStatus, error) {
	if m.db == nil {
		return nil, fmt.Errorf("数据库不可用")
	}

	var order models.Order
	if err := m.db.Preload("Stock").Where("id = ?", orderID).First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在: %s", orderID)
	}

	symbol := ""
	if order.Stock.ID != 0 {
		symbol = order.Stock.Symbol
	}

	return &OrderStatus{
		OrderID:           orderID,
		Symbol:            symbol,
		Status:            order.Status,
		FilledPrice:       order.Price,
		FilledQuantity:    order.FilledQuantity,
		RemainingQuantity: order.Quantity - order.FilledQuantity,
		CreatedAt:         order.CreatedAt,
		UpdatedAt:         order.UpdatedAt,
	}, nil
}

// QueryPositions returns actual positions from the database.
func (m *MockBroker) QueryPositions() ([]PositionInfo, error) {
	if m.db == nil {
		return nil, nil
	}

	var positions []models.Position
	if err := m.db.Preload("Stock").Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("查询持仓失败: %w", err)
	}

	result := make([]PositionInfo, 0, len(positions))
	for _, p := range positions {
		symbol := ""
		name := ""
		if p.Stock.ID != 0 {
			symbol = p.Stock.Symbol
			name = p.Stock.Name
		}

		currentPrice := m.getRealPrice(symbol)
		marketValue := currentPrice * float64(p.Quantity)
		unrealizedPnL := (currentPrice - p.AvgCost) * float64(p.Quantity)

		result = append(result, PositionInfo{
			Symbol:        symbol,
			Name:          name,
			Quantity:      p.Quantity,
			AvgCost:       p.AvgCost,
			CurrentPrice:  currentPrice,
			MarketValue:   marketValue,
			UnrealizedPnL: unrealizedPnL,
		})
	}
	return result, nil
}

// QueryAccount returns account information from the database.
func (m *MockBroker) QueryAccount() (*AccountInfo, error) {
	if m.db == nil {
		return &AccountInfo{
			TotalAssets:   1000000.00,
			AvailableCash: 1000000.00,
		}, nil
	}

	var account models.SimAccount
	if err := m.db.First(&account).Error; err != nil {
		// Return default account if none exists
		return &AccountInfo{
			TotalAssets:   1000000.00,
			AvailableCash: 1000000.00,
		}, nil
	}

	// Calculate market value from positions
	var positions []models.Position
	marketValue := 0.0
	if err := m.db.Where("is_real = ?", false).Find(&positions).Error; err == nil {
		for _, p := range positions {
			price := m.getRealPrice("")
			if p.Stock.ID != 0 {
				// Look up price by stock symbol
				var stock models.Stock
				if err := m.db.First(&stock, p.StockID).Error; err == nil {
					price = m.getRealPrice(stock.Symbol)
				}
			}
			marketValue += price * float64(p.Quantity)
		}
	}

	totalAssets := account.AvailableCash + marketValue

	return &AccountInfo{
		TotalAssets:   totalAssets,
		AvailableCash: account.AvailableCash,
		FrozenCash:    0,
		MarketValue:   marketValue,
		TotalPnL:      account.TotalPnL,
		TodayPnL:      account.TodayPnL,
	}, nil
}

// getRealPrice looks up the latest price for a stock symbol from the database.
// Returns 0 if the price cannot be determined.
func (m *MockBroker) getRealPrice(symbol string) float64 {
	if m.db == nil || symbol == "" {
		return 0
	}

	// Join with stocks table to look up by symbol
	var daily models.StockPriceDaily
	err := m.db.Table("stock_price_dailies").
		Joins("JOIN stocks ON stocks.id = stock_price_dailies.stock_id").
		Where("stocks.symbol = ?", symbol).
		Order("stock_price_dailies.time DESC").
		Select("stock_price_dailies.*").
		First(&daily).Error
	if err == nil && daily.Close > 0 {
		return daily.Close
	}

	return 0
}