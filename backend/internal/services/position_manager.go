package services

import (
	"fmt"
	"math"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// PositionManager handles simulated position management.
type PositionManager struct {
	db *gorm.DB
}

// NewPositionManager creates a new PositionManager.
func NewPositionManager(db *gorm.DB) *PositionManager {
	return &PositionManager{db: db}
}

// GetPositions retrieves all current simulated positions.
func (pm *PositionManager) GetPositions() ([]models.Position, error) {
	var positions []models.Position
	if err := pm.db.Where("is_real = ?", false).Preload("Stock").Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("查询模拟持仓失败: %w", err)
	}
	return positions, nil
}

// UpdatePosition updates or creates a simulated position for a symbol.
// Positive quantity means buy, negative means sell.
func (pm *PositionManager) UpdatePosition(symbol string, quantity int, price decimal.Decimal) error {
	// Find stock
	var stock models.Stock
	if err := pm.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return fmt.Errorf("股票不存在: %s", symbol)
	}

	// Find existing position (simulated only)
	var existing models.Position
	err := pm.db.Where("stock_id = ? AND user_id = ? AND is_real = ?", stock.ID, 1, false).First(&existing).Error

	if err == gorm.ErrRecordNotFound {
		// No existing position, create new (only for buy)
		if quantity <= 0 {
			return fmt.Errorf("无法卖空: %s 无持仓", symbol)
		}

		position := models.Position{
			UserID:         1,
			StockID:        stock.ID,
			Quantity:       quantity,
			AvgCost:        price.InexactFloat64(),
			CurrentValue:   price.InexactFloat64() * float64(quantity),
			UnrealizedPnL:  0,
			RealizedPnL:    0,
			IsReal:         false,
		}
		return pm.db.Create(&position).Error
	}

	if err != nil {
		return fmt.Errorf("查询持仓失败: %w", err)
	}

	// Update existing position
	newQty := existing.Quantity + quantity
	if newQty < 0 {
		return fmt.Errorf("持仓不足: 当前 %d 股, 尝试卖出 %d 股", existing.Quantity, -quantity)
	}

	if newQty == 0 {
		// Position fully closed
		realizedPnL := (price.InexactFloat64() - existing.AvgCost) * float64(existing.Quantity)
		return pm.db.Model(&existing).Updates(map[string]interface{}{
			"quantity":        0,
			"current_value":   0,
			"unrealized_pnl":  0,
			"realized_pnl":    existing.RealizedPnL + realizedPnL,
		}).Error
	}

	// Partial update
	if quantity > 0 {
		// Buying more: recalculate average cost
		totalCost := existing.AvgCost*float64(existing.Quantity) + price.InexactFloat64()*float64(quantity)
		newAvgCost := totalCost / float64(newQty)
		newAvgCost = math.Round(newAvgCost*10000) / 10000

		currentValue := price.InexactFloat64() * float64(newQty)
		unrealizedPnL := (price.InexactFloat64() - newAvgCost) * float64(newQty)

		return pm.db.Model(&existing).Updates(map[string]interface{}{
			"quantity":       newQty,
			"avg_cost":       newAvgCost,
			"current_value":  currentValue,
			"unrealized_pnl": unrealizedPnL,
		}).Error
	}

	// Selling (quantity < 0)
	currentValue := price.InexactFloat64() * float64(newQty)
	unrealizedPnL := (price.InexactFloat64() - existing.AvgCost) * float64(newQty)
	realizedPnL := (price.InexactFloat64() - existing.AvgCost) * float64(-quantity)

	return pm.db.Model(&existing).Updates(map[string]interface{}{
		"quantity":       newQty,
		"current_value":  currentValue,
		"unrealized_pnl": unrealizedPnL,
		"realized_pnl":   existing.RealizedPnL + realizedPnL,
	}).Error
}

// CalculateUnrealizedPnL calculates the unrealized profit/loss for a position.
func (pm *PositionManager) CalculateUnrealizedPnL(symbol string, currentPrice decimal.Decimal) (decimal.Decimal, error) {
	// Find stock
	var stock models.Stock
	if err := pm.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return decimal.Zero, fmt.Errorf("股票不存在: %s", symbol)
	}

	// Find position
	var position models.Position
	if err := pm.db.Where("stock_id = ? AND user_id = ? AND is_real = ?", stock.ID, 1, false).First(&position).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return decimal.Zero, nil
		}
		return decimal.Zero, fmt.Errorf("查询持仓失败: %w", err)
	}

	if position.Quantity == 0 {
		return decimal.Zero, nil
	}

	unrealized := currentPrice.Sub(decimal.NewFromFloat(position.AvgCost)).Mul(decimal.NewFromInt(int64(position.Quantity)))
	return unrealized, nil
}

// GetIndustryExposure returns the total market value per industry for all simulated positions.
func (pm *PositionManager) GetIndustryExposure() (map[string]float64, error) {
	positions, err := pm.GetPositions()
	if err != nil {
		return nil, err
	}

	exposure := make(map[string]float64)
	for _, pos := range positions {
		industry := pos.Stock.Industry
		if industry == "" {
			industry = "其他"
		}
		exposure[industry] += pos.CurrentValue
	}

	return exposure, nil
}

// CheckPositionLimit checks if adding proposedQty shares would exceed the 20% single-stock limit.
func (pm *PositionManager) CheckPositionLimit(symbol string, proposedQty int) (bool, error) {
	// Find stock
	var stock models.Stock
	if err := pm.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return false, fmt.Errorf("股票不存在: %s", symbol)
	}

	// Find existing position
	var existing models.Position
	err := pm.db.Where("stock_id = ? AND user_id = ? AND is_real = ?", stock.ID, 1, false).First(&existing).Error
	currentQty := 0
	if err == nil {
		currentQty = existing.Quantity
	} else if err != gorm.ErrRecordNotFound {
		return false, fmt.Errorf("查询持仓失败: %w", err)
	}

	// Get current price from the position or use avg cost
	currentPrice := 0.0
	if existing.Quantity > 0 {
		if existing.CurrentValue > 0 {
			currentPrice = existing.CurrentValue / float64(existing.Quantity)
		} else {
			currentPrice = existing.AvgCost
		}
	}

	// Get total assets for limit calculation
	var account models.SimAccount
	if err := pm.db.First(&account, 1).Error; err != nil {
		// If no account, allow the trade
		return true, nil
	}

	maxPositionValue := account.TotalAssets * 0.20
	proposedValue := currentPrice * float64(currentQty+proposedQty)

	return proposedValue <= maxPositionValue, nil
}