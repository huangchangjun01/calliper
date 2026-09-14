package services

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quant-trading/backend/internal/models"
)

// TradeService is the core trading service that orchestrates order placement,
// cancellation, querying, and account management.
type TradeService struct {
	db     *gorm.DB
	broker Broker
	redis  *redis.Client
	risk   *RiskManager
	audit  *AuditService
}

// NewTradeService creates a new TradeService.
func NewTradeService(db *gorm.DB, broker Broker, redis *redis.Client) *TradeService {
	risk := NewRiskManager(redis)
	audit := NewAuditService(db)
	return &TradeService{
		db:     db,
		broker: broker,
		redis:  redis,
		risk:   risk,
		audit:  audit,
	}
}

// PlaceOrder handles the full order placement workflow.
func (s *TradeService) PlaceOrder(ctx context.Context, userID uint, req PlaceOrderRequest) (*models.Order, error) {
	// Look up stock by symbol
	var stock models.Stock
	if err := s.db.Where("symbol = ?", req.Symbol).First(&stock).Error; err != nil {
		return nil, fmt.Errorf("股票不存在: %s", req.Symbol)
	}

	// Validate user exists
	var user models.User
	if err := s.db.First(&user, userID).Error; err != nil {
		return nil, fmt.Errorf("用户不存在")
	}

	// Risk validation
	if err := s.risk.ValidateOrder(ctx, userID, req); err != nil {
		return nil, err
	}

	// Call broker to place order
	resp, err := s.broker.PlaceOrder(req)
	if err != nil {
		return nil, fmt.Errorf("下单失败: %w", err)
	}

	// Build order model
	order := models.Order{
		UserID:         userID,
		StockID:        stock.ID,
		OrderType:      req.OrderType,
		OrderKind:      req.Action,
		Price:          req.Price.InexactFloat64(),
		Quantity:       req.Quantity,
		FilledQuantity: resp.FilledQuantity,
		Status:         resp.Status,
		IsReal:         req.TradeType == "real",
	}

	// 保存 order + 成交后更新持仓，放在同一事务中保证一致
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&order).Error; err != nil {
			return fmt.Errorf("保存订单失败: %w", err)
		}

		// 仅成交订单更新持仓
		if resp.FilledQuantity > 0 {
			if err := s.updatePosition(tx, userID, stock.ID, order.IsReal, req.Action,
				resp.FilledQuantity, req.Price.InexactFloat64()); err != nil {
				return err
			}
		}

		// 模拟成交后对模拟账户资金勾稽：买入扣款、卖出加款（仅模拟交易）
		if !order.IsReal && resp.FilledQuantity > 0 {
			amount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity))).InexactFloat64()
			var acc models.SimAccount
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&acc, 1).Error; err != nil {
				return fmt.Errorf("查询模拟账户失败: %w", err)
			}
			if req.Action == "sell" {
				if err := tx.Model(&acc).Update("available_cash", acc.AvailableCash+amount).Error; err != nil {
					return fmt.Errorf("更新模拟账户余额失败: %w", err)
				}
			} else {
				if acc.AvailableCash < amount {
					return fmt.Errorf("资金不足: 需要 %.2f, 可用 %.2f", amount, acc.AvailableCash)
				}
				if err := tx.Model(&acc).Update("available_cash", acc.AvailableCash-amount).Error; err != nil {
					return fmt.Errorf("更新模拟账户余额失败: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Record daily trade amount
	tradeAmount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
	if recordErr := s.risk.RecordTrade(ctx, userID, tradeAmount); recordErr != nil {
		// Non-fatal: log but don't fail the order
	}

	// Audit log
	s.audit.LogTrade(userID, "place_order", "order", fmt.Sprintf("%d", order.ID), map[string]interface{}{
		"symbol":     req.Symbol,
		"action":     req.Action,
		"order_type": req.OrderType,
		"price":      req.Price.InexactFloat64(),
		"quantity":   req.Quantity,
		"status":     resp.Status,
		"trade_type": req.TradeType,
	})

	return &order, nil
}

// CancelOrder cancels an existing order.
func (s *TradeService) CancelOrder(ctx context.Context, userID uint, orderID string) error {
	// Verify order exists and belongs to user
	var order models.Order
	if err := s.db.Where("id = ? AND user_id = ?", orderID, userID).Preload("Stock").First(&order).Error; err != nil {
		return fmt.Errorf("订单不存在或无权操作")
	}

	// Check if order can be cancelled
	if order.Status != "pending" && order.Status != "submitted" {
		return fmt.Errorf("订单状态为 %s，无法撤单", order.Status)
	}

	// Call broker to cancel
	if err := s.broker.CancelOrder(orderID); err != nil {
		return fmt.Errorf("撤单失败: %w", err)
	}

	// Update order status
	if err := s.db.Model(&order).Update("status", "cancelled").Error; err != nil {
		return fmt.Errorf("更新订单状态失败: %w", err)
	}

	// Audit log
	symbol := ""
	if order.Stock.ID != 0 {
		symbol = order.Stock.Symbol
	}
	s.audit.LogTrade(userID, "cancel_order", "order", orderID, map[string]interface{}{
		"symbol": symbol,
		"status": "cancelled",
	})

	return nil
}

// GetOrders retrieves a paginated list of orders for a user, optionally filtered by status.
func (s *TradeService) GetOrders(ctx context.Context, userID uint, status string, limit, offset int) ([]models.Order, int64, error) {
	var orders []models.Order
	var total int64

	query := s.db.Model(&models.Order{}).Where("user_id = ?", userID)

	if status != "" && status != "all" {
		query = query.Where("status = ?", status)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询订单总数失败: %w", err)
	}

	if err := query.Order("created_at DESC").Limit(limit).Offset(offset).Preload("Stock").Find(&orders).Error; err != nil {
		return nil, 0, fmt.Errorf("查询订单列表失败: %w", err)
	}

	return orders, total, nil
}

// GetOrderByID retrieves a single order by ID, verifying ownership.
func (s *TradeService) GetOrderByID(ctx context.Context, userID uint, orderID string) (*models.Order, error) {
	var order models.Order
	if err := s.db.Where("id = ? AND user_id = ?", orderID, userID).Preload("Stock").First(&order).Error; err != nil {
		return nil, fmt.Errorf("订单不存在")
	}
	return &order, nil
}

// updatePosition 在成交后按订单方向更新持仓（需在事务 tx 中调用）。
// buy：行锁 upsert，累加数量并重算 avg_cost/current_value/unrealized_pnl。
// sell：行锁校验数量充足，扣减数量并累加 realized_pnl；数量耗尽则删除该持仓。
func (s *TradeService) updatePosition(tx *gorm.DB, userID, stockID uint, isReal bool, action string, filled int, price float64) error {
	var pos models.Position
	query := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND stock_id = ? AND is_real = ?", userID, stockID, isReal)

	if action == "sell" {
		if err := query.First(&pos).Error; err != nil {
			return fmt.Errorf("无持仓或数量不足")
		}
		if pos.Quantity < filled {
			return fmt.Errorf("无持仓或数量不足")
		}

		realized := (price - pos.AvgCost) * float64(filled)
		newQty := pos.Quantity - filled

		// 扣减后数量耗尽：删除持仓
		if newQty <= 0 {
			if err := tx.Delete(&pos).Error; err != nil {
				return fmt.Errorf("更新持仓失败: %w", err)
			}
			return nil
		}

		pos.Quantity = newQty
		pos.RealizedPnL += realized
		pos.CurrentValue = price * float64(newQty)
		pos.UnrealizedPnL = (price - pos.AvgCost) * float64(newQty)
		if err := tx.Save(&pos).Error; err != nil {
			return fmt.Errorf("更新持仓失败: %w", err)
		}
		return nil
	}

	// buy：upsert 持仓
	err := query.First(&pos).Error
	if err == gorm.ErrRecordNotFound {
		newPos := models.Position{
			UserID:       userID,
			StockID:      stockID,
			IsReal:       isReal,
			Quantity:     filled,
			AvgCost:      price,
			CurrentValue: price * float64(filled),
		}
		if cerr := tx.Create(&newPos).Error; cerr != nil {
			return fmt.Errorf("更新持仓失败: %w", cerr)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("更新持仓失败: %w", err)
	}

	newQty := pos.Quantity + filled
	newAvg := (pos.AvgCost*float64(pos.Quantity) + price*float64(filled)) / float64(newQty)
	pos.Quantity = newQty
	pos.AvgCost = newAvg
	pos.CurrentValue = price * float64(newQty)
	pos.UnrealizedPnL = (price - newAvg) * float64(newQty)
	if err := tx.Save(&pos).Error; err != nil {
		return fmt.Errorf("更新持仓失败: %w", err)
	}
	return nil
}

// GetPositions retrieves positions for a user, optionally filtered by isReal.
func (s *TradeService) GetPositions(ctx context.Context, userID uint, isReal bool) ([]models.Position, error) {
	var positions []models.Position

	query := s.db.Where("user_id = ?", userID)
	if isReal {
		query = query.Where("is_real = ?", true)
	}

	if err := query.Preload("Stock").Find(&positions).Error; err != nil {
		return nil, fmt.Errorf("查询持仓失败: %w", err)
	}

	return positions, nil
}

// GetAccount retrieves account information from the broker.
func (s *TradeService) GetAccount(ctx context.Context, userID uint) (*AccountInfo, error) {
	account, err := s.broker.QueryAccount()
	if err != nil {
		return nil, fmt.Errorf("查询账户失败: %w", err)
	}

	// Audit log
	s.audit.LogAccess(userID, "account", "query")

	return account, nil
}
