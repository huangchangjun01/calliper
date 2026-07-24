package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// AccountService manages the simulated trading account.
type AccountService struct {
	db    *gorm.DB
	redis *redis.Client
}

// NewAccountService creates a new AccountService.
func NewAccountService(db *gorm.DB, redis *redis.Client) *AccountService {
	return &AccountService{
		db:    db,
		redis: redis,
	}
}

// GetAccount retrieves the simulated account (creates one if not exists).
func (s *AccountService) GetAccount() (*models.SimAccount, error) {
	var account models.SimAccount
	err := s.db.First(&account, 1).Error
	if err == gorm.ErrRecordNotFound {
		// Auto-create with default 1,000,000 initial capital
		return s.initializeAccount(decimal.NewFromFloat(1000000.00))
	}
	if err != nil {
		return nil, fmt.Errorf("查询模拟账户失败: %w", err)
	}
	return &account, nil
}

// InitializeAccount initializes the simulated account with the given capital.
func (s *AccountService) InitializeAccount(initialCapital decimal.Decimal) error {
	_, err := s.initializeAccount(initialCapital)
	return err
}

// initializeAccount creates a new SimAccount record.
func (s *AccountService) initializeAccount(initialCapital decimal.Decimal) (*models.SimAccount, error) {
	capital, _ := initialCapital.Float64()
	account := models.SimAccount{
		TotalAssets:   capital,
		AvailableCash: capital,
		FrozenCash:    0,
		MarketValue:   0,
		TotalPnL:      0,
		TodayPnL:      0,
		TodayReturn:   0,
		StartDate:     time.Now().Format("2006-01-02"),
		IsRunning:     false,
	}

	if err := s.db.Create(&account).Error; err != nil {
		return nil, fmt.Errorf("创建模拟账户失败: %w", err)
	}

	return &account, nil
}

// UpdateBalance updates the available cash balance.
// Positive amount adds cash, negative deducts.
func (s *AccountService) UpdateBalance(amount decimal.Decimal) error {
	var account models.SimAccount
	if err := s.db.First(&account, 1).Error; err != nil {
		return fmt.Errorf("查询模拟账户失败: %w", err)
	}

	newBalance, _ := decimal.NewFromFloat(account.AvailableCash).Add(amount).Float64()
	if newBalance < 0 {
		return fmt.Errorf("资金不足: 当前可用 %.2f, 需要 %.2f", account.AvailableCash, amount.Neg().InexactFloat64())
	}

	newTotal, _ := decimal.NewFromFloat(account.TotalAssets).Add(amount).Float64()

	return s.db.Model(&account).Updates(map[string]interface{}{
		"available_cash": newBalance,
		"total_assets":   newTotal,
	}).Error
}

// FreezeFunds moves funds from available to frozen.
func (s *AccountService) FreezeFunds(amount decimal.Decimal) error {
	var account models.SimAccount
	if err := s.db.First(&account, 1).Error; err != nil {
		return fmt.Errorf("查询模拟账户失败: %w", err)
	}

	amountF, _ := amount.Float64()
	if account.AvailableCash < amountF {
		return fmt.Errorf("资金不足: 当前可用 %.2f, 需要冻结 %.2f", account.AvailableCash, amountF)
	}

	newAvailable := account.AvailableCash - amountF
	newFrozen := account.FrozenCash + amountF

	return s.db.Model(&account).Updates(map[string]interface{}{
		"available_cash": newAvailable,
		"frozen_cash":    newFrozen,
	}).Error
}

// UnfreezeFunds moves funds from frozen back to available.
func (s *AccountService) UnfreezeFunds(amount decimal.Decimal) error {
	var account models.SimAccount
	if err := s.db.First(&account, 1).Error; err != nil {
		return fmt.Errorf("查询模拟账户失败: %w", err)
	}

	amountF, _ := amount.Float64()
	if account.FrozenCash < amountF {
		newAvailable := account.AvailableCash + account.FrozenCash
		newFrozen := 0.0
		return s.db.Model(&account).Updates(map[string]interface{}{
			"available_cash": newAvailable,
			"frozen_cash":    newFrozen,
		}).Error
	}

	newAvailable := account.AvailableCash + amountF
	newFrozen := account.FrozenCash - amountF

	return s.db.Model(&account).Updates(map[string]interface{}{
		"available_cash": newAvailable,
		"frozen_cash":    newFrozen,
	}).Error
}

// RecordDailyPnL records the daily profit/loss in Redis.
func (s *AccountService) RecordDailyPnL(date string, pnl decimal.Decimal) error {
	if s.redis == nil {
		return nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("sim:daily_pnl:%s", date)
	pnlF, _ := pnl.Float64()

	if err := s.redis.Set(ctx, key, pnlF, 30*24*time.Hour).Err(); err != nil {
		return fmt.Errorf("记录每日盈亏到Redis失败: %w", err)
	}

	return nil
}

// GetDailyPnL retrieves the daily PnL from Redis for a given date.
func (s *AccountService) GetDailyPnL(date string) (decimal.Decimal, error) {
	if s.redis == nil {
		return decimal.Zero, nil
	}

	ctx := context.Background()
	key := fmt.Sprintf("sim:daily_pnl:%s", date)
	val, err := s.redis.Get(ctx, key).Float64()
	if err == redis.Nil {
		return decimal.Zero, nil
	}
	if err != nil {
		return decimal.Zero, fmt.Errorf("查询每日盈亏失败: %w", err)
	}

	return decimal.NewFromFloat(val), nil
}