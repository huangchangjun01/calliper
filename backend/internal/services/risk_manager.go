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
	redis            *redis.Client
	singleTradeLimit decimal.Decimal
	dailyTradeLimit  decimal.Decimal
	maxTradesPerMin  int
}

// dailyLimitScript 原子执行"当日累计交易金额的限额判断 + 自增"。
// 返回 -1 表示本次金额会使当日累计超过限额（未写入）；否则写入后返回新的累计值。
// 这样可避免 check-then-incr 并发竞态，且仅在订单成功回调处调用，失败订单不占额度。
var dailyLimitScript = redis.NewScript(`
local key = KEYS[1]
local amount = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local ttlMs = tonumber(ARGV[3])
local current = tonumber(redis.call('GET', key) or '0')
if current + amount > limit then
  return -1
end
redis.call('SET', key, current + amount, 'PX', ttlMs)
return current + amount
`)

// dailyTradeKey 生成当日累计交易金额的 Redis key，日期统一使用 Asia/Shanghai。
func dailyTradeKey(userID uint) string {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	return fmt.Sprintf("daily_trade:%d:%s", userID, time.Now().In(loc).Format("2006-01-02"))
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
// 这是一个下单前的只读预检；最终原子"判断+自增"在订单成功回调处的 RecordTrade
// （Redis Lua 脚本）中完成，二者共用同一把日 key，日期统一 Asia/Shanghai。
func (r *RiskManager) CheckDailyLimit(ctx context.Context, userID uint, newAmount decimal.Decimal) error {
	if r.redis == nil {
		return nil
	}

	todayKey := dailyTradeKey(userID)
	current, err := r.redis.Get(ctx, todayKey).Float64()
	if err != nil {
		if err == redis.Nil {
			current = 0
		} else {
			return fmt.Errorf("查询当日交易限额失败: %w", err)
		}
	}

	currentTotal := decimal.NewFromFloat(current)
	projected := currentTotal.Add(newAmount)

	if projected.GreaterThan(r.dailyTradeLimit) {
		return fmt.Errorf("当日累计交易金额超过限额: %.2f > %.2f", projected.InexactFloat64(), r.dailyTradeLimit.InexactFloat64())
	}

	return nil
}

// RecordTrade atomically increments the daily traded amount in Redis via a Lua
// script, enforcing the day limit in the same atomic op. Only call this from the
// successful order callback so failed orders do not consume the quota.
func (r *RiskManager) RecordTrade(ctx context.Context, userID uint, amount decimal.Decimal) error {
	if r.redis == nil {
		return nil
	}

	todayKey := dailyTradeKey(userID)
	ttlMs := int64(time.Until(endOfDay()).Milliseconds())
	if ttlMs < 0 {
		ttlMs = 0
	}

	res, err := dailyLimitScript.Run(ctx, r.redis, []string{todayKey},
		amount.InexactFloat64(), r.dailyTradeLimit.InexactFloat64(), ttlMs).Result()
	if err != nil {
		return fmt.Errorf("记录当日交易金额失败: %w", err)
	}

	if v, ok := res.(int64); ok && v < 0 {
		return fmt.Errorf("当日累计交易金额超过限额")
	}

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

// endOfDay returns the time at the end of the current day in Asia/Shanghai.
func endOfDay() time.Time {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
}
