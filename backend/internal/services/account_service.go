package services

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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
	// 兼容旧账户：initial_capital 为空时以当前总资产为初始资金
	if account.InitialCapital <= 0 {
		account.InitialCapital = account.TotalAssets
		s.db.Model(&models.SimAccount{}).Where("id = ?", account.ID).Update("initial_capital", account.TotalAssets)
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
		TotalAssets:    capital,
		InitialCapital: capital,
		AvailableCash:  capital,
		FrozenCash:     0,
		MarketValue:    0,
		TotalPnL:       0,
		TodayPnL:       0,
		TodayReturn:    0,
		StartDate:      time.Now().Format("2006-01-02"),
		IsRunning:      false,
	}

	if err := s.db.Create(&account).Error; err != nil {
		return nil, fmt.Errorf("创建模拟账户失败: %w", err)
	}

	return &account, nil
}

// UpdateBalance updates the available cash balance.
// Positive amount adds cash, negative deducts.
// 事务内对账户行加排他锁（SELECT ... FOR UPDATE）后再做读-改-写，避免并发覆盖。
func (s *AccountService) UpdateBalance(amount decimal.Decimal) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var account models.SimAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, 1).Error; err != nil {
			return fmt.Errorf("查询模拟账户失败: %w", err)
		}

		newBalance := decimal.NewFromFloat(account.AvailableCash).Add(amount)
		if newBalance.LessThan(decimal.Zero) {
			return fmt.Errorf("资金不足: 当前可用 %.2f, 需要 %.2f", account.AvailableCash, amount.Neg().InexactFloat64())
		}

		newTotal := decimal.NewFromFloat(account.TotalAssets).Add(amount)

		return tx.Model(&account).Updates(map[string]interface{}{
			"available_cash": newBalance.InexactFloat64(),
			"total_assets":   newTotal.InexactFloat64(),
		}).Error
	})
}

// FreezeFunds moves funds from available to frozen.
func (s *AccountService) FreezeFunds(amount decimal.Decimal) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var account models.SimAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, 1).Error; err != nil {
			return fmt.Errorf("查询模拟账户失败: %w", err)
		}

		amountF, _ := amount.Float64()
		if account.AvailableCash < amountF {
			return fmt.Errorf("资金不足: 当前可用 %.2f, 需要冻结 %.2f", account.AvailableCash, amountF)
		}

		newAvailable := account.AvailableCash - amountF
		newFrozen := account.FrozenCash + amountF

		return tx.Model(&account).Updates(map[string]interface{}{
			"available_cash": newAvailable,
			"frozen_cash":    newFrozen,
		}).Error
	})
}

// UnfreezeFunds moves funds from frozen back to available.
func (s *AccountService) UnfreezeFunds(amount decimal.Decimal) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		var account models.SimAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, 1).Error; err != nil {
			return fmt.Errorf("查询模拟账户失败: %w", err)
		}

		amountF, _ := amount.Float64()
		if account.FrozenCash < amountF {
			newAvailable := account.AvailableCash + account.FrozenCash
			newFrozen := 0.0
			return tx.Model(&account).Updates(map[string]interface{}{
				"available_cash": newAvailable,
				"frozen_cash":    newFrozen,
			}).Error
		}

		newAvailable := account.AvailableCash + amountF
		newFrozen := account.FrozenCash - amountF

		return tx.Model(&account).Updates(map[string]interface{}{
			"available_cash": newAvailable,
			"frozen_cash":    newFrozen,
		}).Error
	})
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
