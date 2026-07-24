package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
)

// RiskManager handles risk control for trading operations.
type RiskManager struct {
	redis           *redis.Client
	singleTradeLimit decimal.Decimal
	dailyTradeLimit  decimal.Decimal
	maxTradesPerMin  int
}

// NewRiskManager creates a new RiskManager with default limits.
func NewRiskManager(redis *redis.Client) *RiskManager {
	return &RiskManager{
		redis:            redis,
		singleTradeLimit: decimal.NewFromFloat(500000.00),
		dailyTradeLimit:  decimal.NewFromFloat(5000000.00),
		maxTradesPerMin:  60,
	}
}

// ValidateOrder performs all risk checks on a trade order.
func (r *RiskManager) ValidateOrder(ctx context.Context, userID uint, req PlaceOrderRequest) error {
	// Calculate trade amount
	tradeAmount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))

	// Check single trade limit
	if tradeAmount.GreaterThan(r.singleTradeLimit) {
		return fmt.Errorf("单笔交易金额超过限额: %.2f > %.2f", tradeAmount.InexactFloat64(), r.singleTradeLimit.InexactFloat64())
	}

	// Check daily limit
	if err := r.CheckDailyLimit(ctx, userID, tradeAmount); err != nil {
		return err
	}

	// Check anomaly
	if r.DetectAnomaly(ctx, userID, req) {
		return fmt.Errorf("检测到异常交易行为，订单被拒绝")
	}

	return nil
}

// CheckDailyLimit checks whether the user's daily trading limit has been exceeded.
func (r *RiskManager) CheckDailyLimit(ctx context.Context, userID uint, newAmount decimal.Decimal) error {
	if r.redis == nil {
		return nil
	}

	todayKey := fmt.Sprintf("daily_trade:%d:%s", userID, time.Now().Format("2006-01-02"))
	current, err := r.redis.Get(ctx, todayKey).Float64()
	if err != nil && err != redis.Nil {
		return fmt.Errorf("查询当日交易限额失败: %w", err)
	}

	currentTotal := decimal.NewFromFloat(current)
	projected := currentTotal.Add(newAmount)

	if projected.GreaterThan(r.dailyTradeLimit) {
		return fmt.Errorf("当日累计交易金额超过限额: %.2f > %.2f", projected.InexactFloat64(), r.dailyTradeLimit.InexactFloat64())
	}

	return nil
}

// RecordTrade records the trade amount towards the daily limit in Redis.
func (r *RiskManager) RecordTrade(ctx context.Context, userID uint, amount decimal.Decimal) error {
	if r.redis == nil {
		return nil
	}

	todayKey := fmt.Sprintf("daily_trade:%d:%s", userID, time.Now().Format("2006-01-02"))
	if err := r.redis.IncrByFloat(ctx, todayKey, amount.InexactFloat64()).Err(); err != nil {
		return fmt.Errorf("记录当日交易金额失败: %w", err)
	}

	// Set expiration to end of day
	r.redis.ExpireAt(ctx, todayKey, endOfDay())

	return nil
}

// DetectAnomaly checks for abnormal trading patterns.
func (r *RiskManager) DetectAnomaly(ctx context.Context, userID uint, req PlaceOrderRequest) bool {
	if r.redis == nil {
		return false
	}

	// Check trading frequency (trades per minute)
	freqKey := fmt.Sprintf("trade_freq:%d:%d", userID, time.Now().Unix()/60)
	count, err := r.redis.Incr(ctx, freqKey).Result()
	if err != nil {
		return false
	}
	r.redis.Expire(ctx, freqKey, 2*time.Minute)

	if count > int64(r.maxTradesPerMin) {
		return true
	}

	// Check for unusually large trade
	tradeAmount := req.Price.Mul(decimal.NewFromInt(int64(req.Quantity)))
	if tradeAmount.GreaterThan(r.singleTradeLimit.Mul(decimal.NewFromFloat(0.8))) {
		// Large trade near limit - flag for review
		r.redis.Incr(ctx, fmt.Sprintf("large_trade_flag:%d", userID))
	}

	return false
}

// endOfDay returns the time at the end of the current day.
func endOfDay() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
}