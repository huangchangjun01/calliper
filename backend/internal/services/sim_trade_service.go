package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// SimTradeDecision represents a single simulated trading decision.
type SimTradeDecision struct {
	Symbol       string  `json:"symbol"`
	StockID      uint    `json:"stock_id"`
	Direction    string  `json:"direction"` // buy / sell / hold
	Price        float64 `json:"price"`
	Quantity     int     `json:"quantity"`
	Confidence   float64 `json:"confidence"`
	TargetPrice  float64 `json:"target_price"`
	ExpectedRet  float64 `json:"expected_ret"`
	Reason       string  `json:"reason"`
}

// SimTradeService orchestrates the simulated trading engine.
type SimTradeService struct {
	db                *gorm.DB
	predictionService *PredictionService
	redis             *redis.Client
	positionManager   *PositionManager
	accountService    *AccountService

	mu       sync.RWMutex
	isRunning bool
	cancelFn context.CancelFunc

	todayTradeCount int
	todayTradeDate  string
}

// NewSimTradeService creates a new SimTradeService.
func NewSimTradeService(db *gorm.DB, predictionService *PredictionService, redis *redis.Client, positionManager *PositionManager, accountService *AccountService) *SimTradeService {
	return &SimTradeService{
		db:                db,
		predictionService: predictionService,
		redis:             redis,
		positionManager:   positionManager,
		accountService:    accountService,
		isRunning:         false,
	}
}

// ──────────────────────────────────────────────────────────────
// Decision Engine
// ──────────────────────────────────────────────────────────────

// MockPrediction holds a mock prediction for simulated trading.
type MockPrediction struct {
	Symbol      string
	StockID     uint
	Direction   string
	Confidence  float64
	TargetPrice float64
	ExpectedRet float64
}

// MakeDecision generates trading decisions based on mock predictions.
func (s *SimTradeService) MakeDecision(ctx context.Context) ([]SimTradeDecision, error) {
	// 1. Get all active stocks
	var stocks []models.Stock
	if err := s.db.Where("is_active = ?", true).Preload("Market").Find(&stocks).Error; err != nil {
		return nil, fmt.Errorf("获取活跃股票失败: %w", err)
	}

	if len(stocks) == 0 {
		return nil, fmt.Errorf("没有活跃股票")
	}

	// 2. Generate mock predictions for all active stocks
	predictions := s.generateMockPredictions(stocks)

	// 3. Filter by confidence >= 55%
	var filtered []MockPrediction
	for _, p := range predictions {
		if p.Confidence >= 55.0 {
			filtered = append(filtered, p)
		}
	}

	// 4. Sort by expected return descending, take top 20
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ExpectedRet > filtered[j].ExpectedRet
	})

	topN := 20
	if len(filtered) < topN {
		topN = len(filtered)
	}
	topPicks := filtered[:topN]

	// 5. Get account info for position sizing
	account, err := s.accountService.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	totalCapital := account.TotalAssets
	maxSinglePosition := totalCapital * 0.20 // 单票最大 20%
	maxIndustryExposure := totalCapital * 0.40 // 行业最大 40%

	// 6. Get current positions and industry exposure
	currentPositions, err := s.positionManager.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

	// Build current position map by symbol
	positionMap := make(map[string]*models.Position)
	for _, pos := range currentPositions {
		if pos.Stock.Symbol != "" {
			positionMap[pos.Stock.Symbol] = &pos
		}
	}

	industryExposure, err := s.positionManager.GetIndustryExposure()
	if err != nil {
		industryExposure = make(map[string]float64)
	}

	// 7. Generate decisions
	var decisions []SimTradeDecision
	for _, pick := range topPicks {
		stock := s.findStockBySymbol(stocks, pick.Symbol)
		if stock == nil {
			continue
		}

		// Determine quantity based on position sizing
		currentPrice := pick.TargetPrice * (1 + (rand.Float64()-0.5)*0.02) // mock current price near target
		lotSize := 100
		if stock.LotSize > 0 {
			lotSize = stock.LotSize
		}

		decision := s.buildDecision(pick, currentPrice, lotSize, maxSinglePosition, maxIndustryExposure, positionMap, industryExposure, account.AvailableCash)

		if decision.Quantity > 0 {
			decisions = append(decisions, decision)
		}
	}

	return decisions, nil
}

// generateMockPredictions creates mock predictions for simulated trading.
func (s *SimTradeService) generateMockPredictions(stocks []models.Stock) []MockPrediction {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	var predictions []MockPrediction

	for _, stock := range stocks {
		// Generate mock confidence between 40% and 90%
		confidence := 40.0 + rng.Float64()*50.0

		// Generate mock expected return between -10% and +15%
		expectedRet := (rng.Float64()*25.0 - 10.0)

		// Determine direction
		direction := "hold"
		if expectedRet > 2.0 {
			direction = "up"
		} else if expectedRet < -2.0 {
			direction = "down"
		}

		// Base price (mock)
		basePrice := 10.0 + rng.Float64()*490.0
		targetPrice := basePrice * (1 + expectedRet/100.0)

		predictions = append(predictions, MockPrediction{
			Symbol:      stock.Symbol,
			StockID:     stock.ID,
			Direction:   direction,
			Confidence:  confidence,
			TargetPrice: math.Round(targetPrice*100) / 100,
			ExpectedRet: math.Round(expectedRet*100) / 100,
		})
	}

	return predictions
}

// buildDecision determines the action (buy/sell/hold) and quantity for a given prediction.
func (s *SimTradeService) buildDecision(
	pick MockPrediction,
	currentPrice float64,
	lotSize int,
	maxSinglePosition float64,
	maxIndustryExposure float64,
	positionMap map[string]*models.Position,
	industryExposure map[string]float64,
	availableCash float64,
) SimTradeDecision {
	decision := SimTradeDecision{
		Symbol:      pick.Symbol,
		StockID:     pick.StockID,
		Direction:   "hold",
		Price:       currentPrice,
		Confidence:  pick.Confidence,
		TargetPrice: pick.TargetPrice,
		ExpectedRet: pick.ExpectedRet,
	}

	existingPos := positionMap[pick.Symbol]

	if pick.Direction == "up" && pick.Confidence >= 60.0 {
		// Buy signal
		decision.Direction = "buy"

		// Calculate how many shares we can afford
		affordableQty := int(math.Floor(availableCash / currentPrice / float64(lotSize))) * lotSize
		if affordableQty <= 0 {
			decision.Direction = "hold"
			decision.Reason = "资金不足"
			return decision
		}

		// Respect single position limit
		maxQtyByPosition := int(math.Floor(maxSinglePosition / currentPrice / float64(lotSize))) * lotSize

		// If already holding, deduct existing position value
		if existingPos != nil {
			existingValue := float64(existingPos.Quantity) * currentPrice
			remainingAllowance := maxSinglePosition - existingValue
			if remainingAllowance <= 0 {
				decision.Direction = "hold"
				decision.Reason = "已达单票仓位上限"
				return decision
			}
			maxQtyByPosition = int(math.Floor(remainingAllowance / currentPrice / float64(lotSize))) * lotSize
		}

		// Take the minimum of affordable and max allowed
		qty := affordableQty
		if maxQtyByPosition < qty {
			qty = maxQtyByPosition
		}

		if qty <= 0 {
			decision.Direction = "hold"
			decision.Reason = "计算仓位为0"
			return decision
		}

		decision.Quantity = qty
		decision.Reason = fmt.Sprintf("预测上涨 %.2f%%, 置信度 %.1f%%, 买入 %d 股", pick.ExpectedRet, pick.Confidence, qty)

	} else if pick.Direction == "down" && pick.Confidence >= 60.0 && existingPos != nil && existingPos.Quantity > 0 {
		// Sell signal
		decision.Direction = "sell"

		// Sell up to 50% of existing position
		sellQty := existingPos.Quantity / 2
		sellQty = (sellQty / lotSize) * lotSize
		if sellQty <= 0 {
			sellQty = lotSize
		}
		if sellQty > existingPos.Quantity {
			sellQty = existingPos.Quantity
		}

		decision.Quantity = sellQty
		decision.Reason = fmt.Sprintf("预测下跌 %.2f%%, 置信度 %.1f%%, 卖出 %d 股", pick.ExpectedRet, pick.Confidence, sellQty)
	}

	return decision
}

// findStockBySymbol finds a stock in a slice by symbol.
func (s *SimTradeService) findStockBySymbol(stocks []models.Stock, symbol string) *models.Stock {
	for i := range stocks {
		if stocks[i].Symbol == symbol {
			return &stocks[i]
		}
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// Executor
// ──────────────────────────────────────────────────────────────

// ExecuteTrades executes a list of simulated trading decisions.
func (s *SimTradeService) ExecuteTrades(ctx context.Context, decisions []SimTradeDecision) error {
	if len(decisions) == 0 {
		return nil
	}

	for _, decision := range decisions {
		if decision.Direction == "hold" {
			continue
		}

		// Apply 0.1% slippage
		execPrice := decision.Price
		if decision.Direction == "buy" {
			execPrice = decision.Price * 1.001 // Buy: price slightly higher
		} else {
			execPrice = decision.Price * 0.999 // Sell: price slightly lower
		}
		execPrice = math.Round(execPrice*100) / 100

		// Calculate trade amount
		tradeAmount := execPrice * float64(decision.Quantity)

		if decision.Direction == "buy" {
			// Check available cash
			account, err := s.accountService.GetAccount()
			if err != nil {
				log.Printf("获取账户失败: %v", err)
				continue
			}
			if account.AvailableCash < tradeAmount {
				log.Printf("资金不足: 需要 %.2f, 可用 %.2f", tradeAmount, account.AvailableCash)
				continue
			}

			// Freeze funds
			if err := s.accountService.FreezeFunds(decimal.NewFromFloat(tradeAmount)); err != nil {
				log.Printf("冻结资金失败: %v", err)
				continue
			}

			// Update position
			if err := s.positionManager.UpdatePosition(decision.Symbol, decision.Quantity, decimal.NewFromFloat(execPrice)); err != nil {
				// Unfreeze on failure
				_ = s.accountService.UnfreezeFunds(decimal.NewFromFloat(tradeAmount))
				log.Printf("更新持仓失败: %v", err)
				continue
			}

			// Deduct from available cash (unfreeze + deduct)
			_ = s.accountService.UnfreezeFunds(decimal.NewFromFloat(tradeAmount))
			if err := s.accountService.UpdateBalance(decimal.NewFromFloat(-tradeAmount)); err != nil {
				log.Printf("更新余额失败: %v", err)
				continue
			}

		} else if decision.Direction == "sell" {
			// Update position (reduce)
			negQty := -decision.Quantity
			if err := s.positionManager.UpdatePosition(decision.Symbol, negQty, decimal.NewFromFloat(execPrice)); err != nil {
				log.Printf("更新持仓失败: %v", err)
				continue
			}

			// Add to available cash
			if err := s.accountService.UpdateBalance(decimal.NewFromFloat(tradeAmount)); err != nil {
				log.Printf("更新余额失败: %v", err)
				continue
			}
		}

		// Record simulated trade
		trade := models.SimulatedTrade{
			StockID:    decision.StockID,
			TradeType:  decision.Direction,
			Price:      execPrice,
			Quantity:   decision.Quantity,
			Confidence: decision.Confidence,
			Reason:     decision.Reason,
			ExecutedAt: time.Now(),
		}

		if err := s.db.Create(&trade).Error; err != nil {
			log.Printf("记录模拟交易失败: %v", err)
		}

		// Update trade count
		today := time.Now().Format("2006-01-02")
		s.mu.Lock()
		if s.todayTradeDate != today {
			s.todayTradeDate = today
			s.todayTradeCount = 0
		}
		s.todayTradeCount++
		s.mu.Unlock()
	}

	// Recalculate account after all trades
	if err := s.recalculateAccount(); err != nil {
		log.Printf("重新计算账户失败: %v", err)
	}

	return nil
}

// recalculateAccount updates account totals based on current positions and cash.
func (s *SimTradeService) recalculateAccount() error {
	positions, err := s.positionManager.GetPositions()
	if err != nil {
		return err
	}

	account, err := s.accountService.GetAccount()
	if err != nil {
		return err
	}

	var totalMarketValue float64
	for _, pos := range positions {
		totalMarketValue += pos.CurrentValue
	}

	account.MarketValue = totalMarketValue
	account.TotalAssets = account.AvailableCash + account.FrozenCash + totalMarketValue

	// Update in database
	return s.db.Model(&models.SimAccount{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
		"total_assets": account.TotalAssets,
		"market_value": account.MarketValue,
	}).Error
}

// ──────────────────────────────────────────────────────────────
// Risk Control
// ──────────────────────────────────────────────────────────────

// CheckRiskLimits performs risk control checks.
func (s *SimTradeService) CheckRiskLimits(ctx context.Context) error {
	account, err := s.accountService.GetAccount()
	if err != nil {
		return fmt.Errorf("获取账户信息失败: %w", err)
	}

	// 1. Check daily loss limit (5%)
	today := time.Now().Format("2006-01-02")
	todayPnL, err := s.getTodayPnLFromRedis(ctx, today)
	if err != nil {
		log.Printf("获取今日盈亏失败: %v", err)
	} else {
		totalAssets := account.TotalAssets
		if totalAssets > 0 {
			dailyLossPct := math.Abs(todayPnL) / totalAssets * 100
			if todayPnL < 0 && dailyLossPct > 5.0 {
				s.recordRiskEvent("daily_loss_limit", fmt.Sprintf("单日亏损超过5%%: %.2f%% (%.2f)", dailyLossPct, todayPnL), "")
				return fmt.Errorf("单日亏损超过5%%, 暂停当日交易")
			}
		}
	}

	// 2. Check single stock position limit (20%)
	positions, err := s.positionManager.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	totalAssets := account.TotalAssets
	for _, pos := range positions {
		positionPct := pos.CurrentValue / totalAssets * 100
		if positionPct > 20.0 {
			s.recordRiskEvent("single_position_limit",
				fmt.Sprintf("单票持仓超限: %s 占比 %.2f%%", pos.Stock.Symbol, positionPct), "")
		}
	}

	// 3. Check industry exposure limit (40%)
	industryExposure, err := s.positionManager.GetIndustryExposure()
	if err != nil {
		log.Printf("获取行业分布失败: %v", err)
	} else {
		for industry, exposure := range industryExposure {
			industryPct := exposure / totalAssets * 100
			if industryPct > 40.0 {
				s.recordRiskEvent("industry_exposure_limit",
					fmt.Sprintf("行业仓位超限: %s 占比 %.2f%%", industry, industryPct), "")
			}
		}
	}

	return nil
}

// getTodayPnLFromRedis reads today's PnL from Redis.
func (s *SimTradeService) getTodayPnLFromRedis(ctx context.Context, date string) (float64, error) {
	if s.redis == nil {
		return 0, nil
	}
	key := fmt.Sprintf("sim:daily_pnl:%s", date)
	val, err := s.redis.Get(ctx, key).Float64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

// recordRiskEvent logs a risk event to the database.
func (s *SimTradeService) recordRiskEvent(eventType, message, details string) {
	event := models.RiskEvent{
		EventType: eventType,
		Message:   message,
		Details:   details,
	}
	if err := s.db.Create(&event).Error; err != nil {
		log.Printf("记录风险事件失败: %v", err)
	}
}

// ──────────────────────────────────────────────────────────────
// Scheduler
// ──────────────────────────────────────────────────────────────

// StartScheduler starts the simulated trading scheduler.
func (s *SimTradeService) StartScheduler(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		log.Println("模拟交易调度器已在运行")
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	ctx, cancel := context.WithCancel(ctx)
	s.mu.Lock()
	s.cancelFn = cancel
	s.mu.Unlock()

	// Update account running status
	s.db.Model(&models.SimAccount{}).Where("id = ?", 1).Update("is_running", true)

	log.Println("模拟交易调度器已启动")

	go func() {
		defer func() {
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()
			s.db.Model(&models.SimAccount{}).Where("id = ?", 1).Update("is_running", false)
			log.Println("模拟交易调度器已停止")
		}()

		// Trading session check ticker: every 5 minutes
		checkTicker := time.NewTicker(5 * time.Minute)
		defer checkTicker.Stop()

		// Decision ticker: every 30 minutes during trading hours
		decisionTicker := time.NewTicker(30 * time.Minute)
		defer decisionTicker.Stop()

		// Settlement check: every 15 minutes
		settlementTicker := time.NewTicker(15 * time.Minute)
		defer settlementTicker.Stop()

		lastDecisionTime := time.Time{}
		lastSettledDate := ""

		for {
			select {
			case <-ctx.Done():
				return

			case <-checkTicker.C:
				// Periodic check: verify account is still running
				var account models.SimAccount
				if err := s.db.First(&account, 1).Error; err == nil {
					if !account.IsRunning {
						return
					}
				}

			case <-decisionTicker.C:
				now := time.Now()
				today := now.Format("2006-01-02")

				// Check if within trading hours (A-share: 9:30-15:00)
				if s.isInTradingHours(now) {
					// Avoid making decisions too frequently (at least 25 min apart)
					if now.Sub(lastDecisionTime) < 25*time.Minute {
						continue
					}

					log.Println("执行模拟交易决策...")
					s.runDecisionCycle(ctx)
					lastDecisionTime = now
				}

				// Settlement check after market close
				if s.isAfterHours(now) && lastSettledDate != today {
					log.Println("执行盘后结算...")
					if err := s.SettleDaily(ctx); err != nil {
						log.Printf("盘后结算失败: %v", err)
					} else {
						lastSettledDate = today
					}
				}

			case <-settlementTicker.C:
				now := time.Now()
				today := now.Format("2006-01-02")

				// Extra settlement check
				if s.isAfterHours(now) && lastSettledDate != today {
					log.Println("执行盘后结算(补充检查)...")
					if err := s.SettleDaily(ctx); err != nil {
						log.Printf("盘后结算失败: %v", err)
					} else {
						lastSettledDate = today
					}
				}
			}
		}
	}()
}

// runDecisionCycle executes one full decision cycle: check risk, make decisions, execute.
func (s *SimTradeService) runDecisionCycle(ctx context.Context) {
	// Check risk limits first
	if err := s.CheckRiskLimits(ctx); err != nil {
		log.Printf("风险控制: %v", err)
		// If daily loss limit hit, stop trading for the day
		s.recordRiskEvent("trading_halted", err.Error(), "")
		return
	}

	// Make decisions
	decisions, err := s.MakeDecision(ctx)
	if err != nil {
		log.Printf("生成决策失败: %v", err)
		return
	}

	log.Printf("生成 %d 条交易决策", len(decisions))

	// Execute trades
	if err := s.ExecuteTrades(ctx, decisions); err != nil {
		log.Printf("执行交易失败: %v", err)
	}
}

// StopScheduler stops the simulated trading scheduler.
func (s *SimTradeService) StopScheduler() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	if s.cancelFn != nil {
		s.cancelFn()
	}
	s.isRunning = false

	s.db.Model(&models.SimAccount{}).Where("id = ?", 1).Update("is_running", false)
	log.Println("模拟交易调度器已手动停止")
}

// IsRunning returns whether the scheduler is currently running.
func (s *SimTradeService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isRunning
}

// GetTodayTradeCount returns the number of trades executed today.
func (s *SimTradeService) GetTodayTradeCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	today := time.Now().Format("2006-01-02")
	if s.todayTradeDate != today {
		return 0
	}
	return s.todayTradeCount
}

// isInTradingHours checks if current time is within A-share trading hours.
func (s *SimTradeService) isInTradingHours(now time.Time) bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := now.In(loc)

	// Weekday only
	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}

	// Morning session: 9:30 - 11:30
	morningStart := time.Date(t.Year(), t.Month(), t.Day(), 9, 30, 0, 0, loc)
	morningEnd := time.Date(t.Year(), t.Month(), t.Day(), 11, 30, 0, 0, loc)
	if t.After(morningStart) && t.Before(morningEnd) {
		return true
	}

	// Afternoon session: 13:00 - 15:00
	afternoonStart := time.Date(t.Year(), t.Month(), t.Day(), 13, 0, 0, 0, loc)
	afternoonEnd := time.Date(t.Year(), t.Month(), t.Day(), 15, 0, 0, 0, loc)
	if t.After(afternoonStart) && t.Before(afternoonEnd) {
		return true
	}

	return false
}

// isAfterHours checks if current time is after the market close.
func (s *SimTradeService) isAfterHours(now time.Time) bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	t := now.In(loc)

	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}

	closeTime := time.Date(t.Year(), t.Month(), t.Day(), 15, 0, 0, 0, loc)
	return t.After(closeTime)
}

// ──────────────────────────────────────────────────────────────
// Settlement
// ──────────────────────────────────────────────────────────────

// SettleDaily performs end-of-day settlement.
func (s *SimTradeService) SettleDaily(ctx context.Context) error {
	today := time.Now().Format("2006-01-02")

	// 1. Calculate PnL for all positions
	positions, err := s.positionManager.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	var totalDailyPnL float64
	for _, pos := range positions {
		// Get current price from Redis or use last known price
		currentPrice := s.getCurrentPrice(ctx, pos.Stock.Symbol)
		if currentPrice > 0 {
			pnl, err := s.positionManager.CalculateUnrealizedPnL(pos.Stock.Symbol, decimal.NewFromFloat(currentPrice))
			if err == nil {
				pnlF, _ := pnl.Float64()
				totalDailyPnL += pnlF
			}
		}
	}

	// 2. Update account
	account, err := s.accountService.GetAccount()
	if err != nil {
		return fmt.Errorf("获取账户失败: %w", err)
	}

	// Calculate total market value
	var totalMarketValue float64
	for _, pos := range positions {
		currentPrice := s.getCurrentPrice(ctx, pos.Stock.Symbol)
		if currentPrice > 0 {
			pos.CurrentValue = currentPrice * float64(pos.Quantity)
		}
		totalMarketValue += pos.CurrentValue
		// Update position current value in DB
		s.db.Model(&models.Position{}).Where("id = ?", pos.ID).Update("current_value", pos.CurrentValue)
	}

	account.MarketValue = totalMarketValue
	account.TotalAssets = account.AvailableCash + account.FrozenCash + totalMarketValue
	account.TodayPnL = totalDailyPnL

	if account.TotalAssets > 0 {
		account.TodayReturn = totalDailyPnL / (account.TotalAssets - totalDailyPnL) * 100
	}

	// Save to database
	if err := s.db.Model(&models.SimAccount{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
		"total_assets": account.TotalAssets,
		"market_value": account.MarketValue,
		"today_pnl":    account.TodayPnL,
		"today_return": account.TodayReturn,
	}).Error; err != nil {
		return fmt.Errorf("更新账户失败: %w", err)
	}

	// 3. Record daily PnL in Redis
	if err := s.accountService.RecordDailyPnL(today, decimal.NewFromFloat(totalDailyPnL)); err != nil {
		log.Printf("记录每日盈亏到Redis失败: %v", err)
	}

	log.Printf("日结完成: 日期=%s, 总资产=%.2f, 今日盈亏=%.2f, 今日收益率=%.4f%%",
		today, account.TotalAssets, totalDailyPnL, account.TodayReturn)

	return nil
}

// getCurrentPrice gets the current price of a symbol from Redis or falls back to mock.
func (s *SimTradeService) getCurrentPrice(ctx context.Context, symbol string) float64 {
	if s.redis != nil {
		key := fmt.Sprintf("quote:%s", symbol)
		val, err := s.redis.Get(ctx, key).Float64()
		if err == nil && val > 0 {
			return val
		}
	}

	// Fallback: generate a mock price based on symbol hash
	var hash float64
	for _, c := range symbol {
		hash = hash*31 + float64(c)
	}
	return 10.0 + math.Mod(hash, 490.0)
}

// GetLatestDecisions retrieves the latest simulated trade decisions.
func (s *SimTradeService) GetLatestDecisions(ctx context.Context, limit int) ([]models.SimulatedTrade, error) {
	var trades []models.SimulatedTrade
	if err := s.db.Preload("Stock").Order("executed_at DESC").Limit(limit).Find(&trades).Error; err != nil {
		return nil, fmt.Errorf("查询模拟交易记录失败: %w", err)
	}
	return trades, nil
}

// GetSimTrades retrieves paginated simulated trade records.
func (s *SimTradeService) GetSimTrades(ctx context.Context, limit, offset int) ([]models.SimulatedTrade, int64, error) {
	var trades []models.SimulatedTrade
	var total int64

	if err := s.db.Model(&models.SimulatedTrade{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("查询模拟交易总数失败: %w", err)
	}

	if err := s.db.Preload("Stock").Order("executed_at DESC").Limit(limit).Offset(offset).Find(&trades).Error; err != nil {
		return nil, 0, fmt.Errorf("查询模拟交易记录失败: %w", err)
	}

	return trades, total, nil
}