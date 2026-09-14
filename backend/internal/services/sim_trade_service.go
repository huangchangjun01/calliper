package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quant-trading/backend/internal/models"
	"github.com/quant-trading/backend/internal/util"
)

// SimTradeDecision represents a single simulated trading decision.
type SimTradeDecision struct {
	Symbol       string  `json:"symbol"`
	StockID      uint    `json:"stock_id"`
	PredictionID *uint   `json:"prediction_id,omitempty"`
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
	marketDataSvc     *MarketDataService

	mu        sync.RWMutex
	isRunning bool
	cancelFn  context.CancelFunc

	todayTradeCount int
	todayTradeDate  string
}

// NewSimTradeService creates a new SimTradeService.
func NewSimTradeService(db *gorm.DB, predictionService *PredictionService, redis *redis.Client, positionManager *PositionManager, accountService *AccountService, marketDataSvc *MarketDataService) *SimTradeService {
	return &SimTradeService{
		db:                db,
		predictionService: predictionService,
		redis:             redis,
		positionManager:   positionManager,
		accountService:    accountService,
		marketDataSvc:     marketDataSvc,
		isRunning:         false,
	}
}

// ──────────────────────────────────────────────────────────────
// Decision Engine
// ──────────────────────────────────────────────────────────────

// PredictionInfo holds prediction data from ML service.
type PredictionInfo struct {
	Symbol       string
	StockID      uint
	PredictionID *uint
	Industry     string
	Direction    string
	Confidence   float64
	TargetPrice  float64
	ExpectedRet  float64
}

// MakeDecision generates trading decisions based on real ML predictions.
func (s *SimTradeService) MakeDecision(ctx context.Context) ([]SimTradeDecision, error) {
	// 1. Get active stocks (全量活跃股票，保证所有有效预测的标的都在决策空间内)
	var stocks []models.Stock
	if err := s.db.Where("is_active = ?", true).Preload("Market").Find(&stocks).Error; err != nil {
		return nil, fmt.Errorf("获取活跃股票失败: %w", err)
	}

	if len(stocks) == 0 {
		return nil, fmt.Errorf("没有活跃股票")
	}

	// 2. Get predictions from local predictions table or prediction service
	predictions, err := s.getPredictions(ctx, stocks)
	if err != nil {
		log.Printf("[SimTrade] Failed to get predictions: %v, using empty predictions", err)
		return nil, nil
	}

	if len(predictions) == 0 {
		log.Printf("[SimTrade] 无有效预测结果，本轮不生成交易决策")
		return nil, nil
	}

	// 3. 获取实时价格，计算每只股票的预期收益率（(目标价/现价 - 1)），并补齐行业信息
	priceMap := s.getRealTimePrices(ctx, predictions, stocks)
	for i := range predictions {
		pick := &predictions[i]
		if stock := findStockBySymbol(stocks, pick.Symbol); stock != nil {
			pick.Industry = stock.Industry
		}
		currentPrice := pick.TargetPrice
		if realPrice, ok := priceMap[pick.Symbol]; ok && realPrice > 0 {
			currentPrice = realPrice
		}
		if currentPrice > 0 && pick.TargetPrice > 0 {
			pick.ExpectedRet = (pick.TargetPrice/currentPrice - 1) * 100
		}
	}

	// 4. Filter by confidence >= 55% (confidence 存 0~1 小数，55% = 0.55)
	var filtered []PredictionInfo
	for _, p := range predictions {
		if p.Confidence >= 0.55 {
			filtered = append(filtered, p)
		}
	}

	// 5. Sort by expected return descending, take top 20
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].ExpectedRet > filtered[j].ExpectedRet
	})

	topN := 20
	if len(filtered) < topN {
		topN = len(filtered)
	}
	topPicks := filtered[:topN]

	// 6. Get account info for position sizing
	account, err := s.accountService.GetAccount()
	if err != nil {
		return nil, fmt.Errorf("获取账户信息失败: %w", err)
	}

	totalCapital := account.TotalAssets
	maxSinglePosition := totalCapital * 0.20   // 单票最大 20%
	maxIndustryExposure := totalCapital * 0.40 // 行业最大 40%

	// 7. Get current positions and industry exposure
	currentPositions, err := s.positionManager.GetPositions()
	if err != nil {
		return nil, fmt.Errorf("获取持仓失败: %w", err)
	}

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

	// 8. Generate decisions
	var decisions []SimTradeDecision
	for _, pick := range topPicks {
		stock := findStockBySymbol(stocks, pick.Symbol)
		if stock == nil {
			continue
		}

		// Use real price if available, otherwise use target price
		currentPrice := pick.TargetPrice
		if realPrice, ok := priceMap[pick.Symbol]; ok && realPrice > 0 {
			currentPrice = realPrice
		}

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

// getPredictions fetches predictions for simulated trading decisions.
// 优先使用本地 predictions 表中仍有效的预测结果（模拟交易策略按预测分析结果执行），
// 本地无有效数据时降级调用 ML 预测服务。
func (s *SimTradeService) getPredictions(ctx context.Context, stocks []models.Stock) ([]PredictionInfo, error) {
	// 1. 本地 predictions 表（有效期内、未验证），每只股票取最新一次预测，同批次优先 short > medium > long
	if s.db != nil {
		type predRow struct {
			ID          uint
			StockID     uint
			Symbol      string
			Period      string
			Direction   string
			Confidence  float64
			TargetPrice float64
		}
		var rows []predRow
		err := s.db.Raw(`
			SELECT t.id, t.stock_id, t.period, t.direction, t.confidence, t.target_price, st.symbol
			FROM (
				SELECT p.id, p.stock_id, p.period, p.direction, p.confidence, p.target_price,
					row_number() OVER (
						PARTITION BY p.stock_id
						ORDER BY p.predicted_at DESC,
							CASE p.period WHEN 'short' THEN 0 WHEN 'medium' THEN 1 ELSE 2 END
					) AS rn
				FROM predictions p
				WHERE p.valid_until > now() AND p.success IS NULL
			) t
			JOIN stocks st ON st.id = t.stock_id
			WHERE t.rn = 1 AND st.is_active = true
		`).Scan(&rows).Error
		if err == nil && len(rows) > 0 {
			var predictions []PredictionInfo
			for _, row := range rows {
				// 关联本地下游待验证的预测记录，作为决策依据
				id := row.ID
				direction := "flat"
				if row.Direction == "up" {
					direction = "up"
				} else if row.Direction == "down" {
					direction = "down"
				}
				predictions = append(predictions, PredictionInfo{
					Symbol:       row.Symbol,
					StockID:      row.StockID,
					PredictionID: &id,
					Industry:     "",
					Direction:    direction,
					Confidence:   row.Confidence,
					TargetPrice:  row.TargetPrice,
				})
			}
			if len(predictions) > 0 {
				return predictions, nil
			}
		}
	}

	// 2. 降级：调用 ML 预测服务
	if s.predictionService != nil {
		var predictions []PredictionInfo
		consecutiveErrors := 0
		const maxConsecutiveErrors = 5 // circuit breaker

		for _, stock := range stocks {
			pred, err := s.predictionService.GetPrediction(stock.Symbol)
			if err != nil {
				consecutiveErrors++
				if consecutiveErrors >= maxConsecutiveErrors {
					log.Printf("[SimTrade] Prediction service failed %d times consecutively, stopping (circuit breaker)", consecutiveErrors)
					break
				}
				continue
			}
			consecutiveErrors = 0 // reset on success

			if pred != nil {
				direction := "hold"
				if pred.Direction == "up" || pred.Direction == "上涨" {
					direction = "up"
				} else if pred.Direction == "down" || pred.Direction == "下跌" {
					direction = "down"
				}

				// 关联本地下游待验证的预测记录，作为决策依据
				var predictionID *uint
				if s.db != nil {
					var latestPred models.Prediction
					if err := s.db.Where("stock_id = ? AND success IS NULL", stock.ID).
						Order("valid_until DESC").
						First(&latestPred).Error; err == nil {
						id := latestPred.ID
						predictionID = &id
					}
				}

				predictions = append(predictions, PredictionInfo{
					Symbol:       stock.Symbol,
					StockID:      stock.ID,
					PredictionID: predictionID,
					Industry:     stock.Industry,
					Direction:    direction,
					Confidence:   pred.Confidence,
					TargetPrice:  pred.TargetPrice,
					ExpectedRet:  0,
				})
			}
		}
		if len(predictions) > 0 {
			return predictions, nil
		}
	}

	// If no predictions available, return empty
	return nil, nil
}

// getRealTimePrices fetches real-time prices from market data or Redis.
func (s *SimTradeService) getRealTimePrices(ctx context.Context, picks []PredictionInfo, stocks []models.Stock) map[string]float64 {
	priceMap := make(map[string]float64)

	// Try Redis first
	if s.redis != nil {
		for _, pick := range picks {
			key := fmt.Sprintf("quote:%s", pick.Symbol)
			val, err := s.redis.Get(ctx, key).Float64()
			if err == nil && val > 0 {
				priceMap[pick.Symbol] = val
			}
		}
	}

	// Try market data service
	if s.marketDataSvc != nil && len(priceMap) == 0 {
		collectors := s.marketDataSvc.GetCollectors()
		for _, collector := range collectors {
			var symbols []string
			for _, pick := range picks {
				if _, ok := priceMap[pick.Symbol]; !ok {
					symbols = append(symbols, pick.Symbol)
				}
			}
			if len(symbols) == 0 {
				break
			}
			data, err := collector.FetchRealTimeData(symbols)
			if err == nil {
				for _, md := range data {
					f := md.Price
					if f > 0 {
						priceMap[md.Symbol] = f
					}
				}
			}
		}
	}

	return priceMap
}

// buildDecision determines the action (buy/sell/hold) and quantity for a given prediction.
func (s *SimTradeService) buildDecision(
	pick PredictionInfo,
	currentPrice float64,
	lotSize int,
	maxSinglePosition float64,
	maxIndustryExposure float64,
	positionMap map[string]*models.Position,
	industryExposure map[string]float64,
	availableCash float64,
) SimTradeDecision {
	decision := SimTradeDecision{
		Symbol:       pick.Symbol,
		StockID:      pick.StockID,
		PredictionID: pick.PredictionID,
		Direction:    "hold",
		Price:        currentPrice,
		Confidence:   pick.Confidence,
		TargetPrice:  pick.TargetPrice,
		ExpectedRet:  pick.ExpectedRet,
	}

	existingPos := positionMap[pick.Symbol]

	if currentPrice <= 0 {
		// 实时价不可得时（Redis 空 + 采集失败），不能用非正价格核算仓位
		decision.Direction = "hold"
		decision.Reason = "无可用实时价格"
		return decision
	}

	if pick.Direction == "up" && pick.Confidence >= 0.60 {
		// Buy signal
		decision.Direction = "buy"

		affordableQty := int(math.Floor(availableCash/currentPrice/float64(lotSize))) * lotSize
		if affordableQty <= 0 {
			decision.Direction = "hold"
			decision.Reason = "资金不足"
			return decision
		}

		maxQtyByPosition := int(math.Floor(maxSinglePosition/currentPrice/float64(lotSize))) * lotSize

		if existingPos != nil {
			existingValue := float64(existingPos.Quantity) * currentPrice
			remainingAllowance := maxSinglePosition - existingValue
			if remainingAllowance <= 0 {
				decision.Direction = "hold"
				decision.Reason = "已达单票仓位上限"
				return decision
			}
			maxQtyByPosition = int(math.Floor(remainingAllowance/currentPrice/float64(lotSize))) * lotSize
		}

		qty := affordableQty
		if maxQtyByPosition < qty {
			qty = maxQtyByPosition
		}

		// 行业仓位约束：当前行业已占用 + 本次买入 <= 40%
		industry := pick.Industry
		if industry == "" {
			industry = "其他"
		}
		industryRemaining := maxIndustryExposure - industryExposure[industry]
		maxQtyByIndustry := int(math.Floor(industryRemaining/currentPrice/float64(lotSize))) * lotSize
		if maxQtyByIndustry > 0 && maxQtyByIndustry < qty {
			qty = maxQtyByIndustry
		}

		if qty <= 0 {
			decision.Direction = "hold"
			decision.Reason = "计算仓位为0"
			return decision
		}

		decision.Quantity = qty
		decision.Reason = fmt.Sprintf("预测上涨 %.2f%%, 置信度 %.1f%%, 目标价 %.2f, 买入 %d 股", pick.ExpectedRet, pick.Confidence, pick.TargetPrice, qty)

	} else if pick.Direction == "down" && pick.Confidence >= 0.60 && existingPos != nil && existingPos.Quantity > 0 {
		decision.Direction = "sell"

		sellQty := existingPos.Quantity / 2
		sellQty = (sellQty / lotSize) * lotSize
		if sellQty <= 0 {
			sellQty = lotSize
		}
		if sellQty > existingPos.Quantity {
			sellQty = existingPos.Quantity
		}

		decision.Quantity = sellQty
		decision.Reason = fmt.Sprintf("预测下跌 %.2f%%, 置信度 %.1f%%, 目标价 %.2f, 卖出 %d 股", pick.ExpectedRet, pick.Confidence, pick.TargetPrice, sellQty)
	}

	return decision
}

// findStockBySymbol finds a stock in a slice by symbol.
func findStockBySymbol(stocks []models.Stock, symbol string) *models.Stock {
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
			execPrice = decision.Price * 1.001
		} else {
			execPrice = decision.Price * 0.999
		}
		execPrice = math.Round(execPrice*100) / 100

		tradeAmount := execPrice * float64(decision.Quantity)

		if decision.Direction == "buy" {
			// 买入在单个事务内完成：锁账户校验资金→扣减余额→持仓 upsert→写流水。
			// 不再依赖 Freeze/Unfreeze 多轮往返（那双接口仅用于账户视图快照展示）。
			if err := s.executeBuy(ctx, decision, execPrice, tradeAmount); err != nil {
				log.Printf("买入失败: %v", err)
				continue
			}
		} else if decision.Direction == "sell" {
			// 卖出前先获取当前持仓均价，用于计算本次卖出的已实现盈亏
			var pos models.Position
			posErr := s.db.Where("stock_id = ? AND user_id = ? AND is_real = ?", decision.StockID, 1, false).First(&pos).Error
			var profit float64
			var profitRate float64
			if posErr == nil && pos.AvgCost > 0 {
				profit = (execPrice - pos.AvgCost) * float64(decision.Quantity)
				profitRate = (execPrice/pos.AvgCost - 1) * 100
			}

			negQty := -decision.Quantity
			if err := s.positionManager.UpdatePosition(decision.Symbol, negQty, decimal.NewFromFloat(execPrice)); err != nil {
				log.Printf("更新持仓失败: %v", err)
				continue
			}

			if err := s.accountService.UpdateBalance(decimal.NewFromFloat(tradeAmount)); err != nil {
				log.Printf("更新余额失败: %v", err)
				continue
			}

			trade := models.SimulatedTrade{
				StockID:      decision.StockID,
				TradeType:    decision.Direction,
				Price:        execPrice,
				Quantity:     decision.Quantity,
				Confidence:   decision.Confidence,
				PredictionID: decision.PredictionID,
				Reason:       decision.Reason,
				Profit:       profit,
				ProfitRate:   profitRate,
				ExecutedAt:   time.Now(),
			}

			if err := s.db.Create(&trade).Error; err != nil {
				log.Printf("记录模拟交易失败: %v", err)
			}
		}

		today := time.Now().Format("2006-01-02")
		s.mu.Lock()
		if s.todayTradeDate != today {
			s.todayTradeDate = today
			s.todayTradeCount = 0
		}
		s.todayTradeCount++
		s.mu.Unlock()
	}

	if err := s.recalculateAccount(); err != nil {
		log.Printf("重新计算账户失败: %v", err)
	}

	return nil
}

// executeBuy performs a buy in a single transaction: lock the account row and
// validate funds, deduct available cash, upsert the position with row lock, and
// write the simulated_trades record. This removes the freeze/unfreeze round-trips
// from the ordering path so funds and positions stay consistent under concurrency.
func (s *SimTradeService) executeBuy(ctx context.Context, decision SimTradeDecision, execPrice, tradeAmount float64) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		// 1) 锁定账户校验资金
		var account models.SimAccount
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&account, 1).Error; err != nil {
			return fmt.Errorf("查询模拟账户失败: %w", err)
		}
		if account.AvailableCash < tradeAmount {
			return fmt.Errorf("资金不足: 需要 %.2f, 可用 %.2f", tradeAmount, account.AvailableCash)
		}

		// 2) 扣减可用余额
		newBalance := account.AvailableCash - tradeAmount
		if err := tx.Model(&account).Update("available_cash", newBalance).Error; err != nil {
			return fmt.Errorf("更新余额失败: %w", err)
		}

		// 3) 持仓 upsert（行锁）
		stockID := decision.StockID
		if stockID == 0 {
			var stock models.Stock
			if err := tx.Where("symbol = ?", decision.Symbol).First(&stock).Error; err != nil {
				return fmt.Errorf("股票不存在: %s", decision.Symbol)
			}
			stockID = stock.ID
		}

		var existing models.Position
		err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("stock_id = ? AND user_id = ? AND is_real = ?", stockID, 1, false).
			First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			position := models.Position{
				UserID:        1,
				StockID:       stockID,
				Quantity:      decision.Quantity,
				AvgCost:       execPrice,
				CurrentValue:  execPrice * float64(decision.Quantity),
				UnrealizedPnL: 0,
				RealizedPnL:   0,
				IsReal:        false,
			}
			if err := tx.Create(&position).Error; err != nil {
				return fmt.Errorf("创建持仓失败: %w", err)
			}
		} else if err != nil {
			return fmt.Errorf("查询持仓失败: %w", err)
		} else {
			newQty := existing.Quantity + decision.Quantity
			if newQty < 0 {
				return fmt.Errorf("买入后持仓不能为负: 当前 %d 股, 买入 %d 股", existing.Quantity, decision.Quantity)
			}
			newAvg := (existing.AvgCost*float64(existing.Quantity) + execPrice*float64(decision.Quantity)) / float64(newQty)
			newAvg = math.Round(newAvg*10000) / 10000
			currentValue := execPrice * float64(newQty)
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"quantity":       newQty,
				"avg_cost":       newAvg,
				"current_value":  currentValue,
				"unrealized_pnl": (execPrice - newAvg) * float64(newQty),
			}).Error; err != nil {
				return fmt.Errorf("更新持仓失败: %w", err)
			}
		}

		// 4) 写 simulated_trades 流水
		trade := models.SimulatedTrade{
			StockID:      stockID,
			TradeType:    "buy",
			Price:        execPrice,
			Quantity:     decision.Quantity,
			Confidence:   decision.Confidence,
			PredictionID: decision.PredictionID,
			Reason:       decision.Reason,
			ExecutedAt:   time.Now(),
		}
		if err := tx.Create(&trade).Error; err != nil {
			return fmt.Errorf("记录模拟交易失败: %w", err)
		}
		return nil
	})
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

	// 调度器生命周期由自身持有的 background 根 context 决定，不依赖调用方传入 ctx（避免 HTTP request ctx 结束后 goroutine 退出）
	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx // 保留派生自 background 的 ctx，供下方首轮决策与 select 循环使用
	s.mu.Lock()
	s.cancelFn = cancel
	s.mu.Unlock()

	s.db.Model(&models.SimAccount{}).Where("id = ?", 1).Update("is_running", true)

	log.Println("模拟交易调度器已启动")

	util.SafeGo(func() {
		defer func() {
			s.mu.Lock()
			s.isRunning = false
			s.mu.Unlock()
			s.db.Model(&models.SimAccount{}).Where("id = ?", 1).Update("is_running", false)
			log.Println("模拟交易调度器已停止")
		}()

		decisionTicker := time.NewTicker(15 * time.Minute)
		defer decisionTicker.Stop()

		settlementTicker := time.NewTicker(15 * time.Minute)
		defer settlementTicker.Stop()

		lastSettledDate := ""

		// 启动即自动执行一轮决策，确保模拟交易在交易时段立即按预测结果开跑；
		// 盘外启动则不立即成交（等开盘后 ticker 触发）。
		if s.isInTradingHours(time.Now()) {
			log.Println("启动后立即执行首轮模拟交易决策...")
			s.runDecisionCycle(ctx)
		} else {
			log.Println("非交易时段启动，等待开盘后执行首轮模拟交易决策...")
		}

		for {
			select {
			case <-ctx.Done():
				return

			case <-decisionTicker.C:
				now := time.Now()
				today := now.Format("2006-01-02")

				if s.isInTradingHours(now) {
					log.Println("执行模拟交易决策...")
					s.runDecisionCycle(ctx)
				}

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
	})
}

// runDecisionCycle executes one full decision cycle.
func (s *SimTradeService) runDecisionCycle(ctx context.Context) {
	if err := s.CheckRiskLimits(ctx); err != nil {
		log.Printf("风险控制: %v", err)
		s.recordRiskEvent("trading_halted", err.Error(), "")
		return
	}

	decisions, err := s.MakeDecision(ctx)
	if err != nil {
		log.Printf("生成决策失败: %v", err)
		return
	}

	log.Printf("生成 %d 条交易决策", len(decisions))

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

	if t.Weekday() == time.Saturday || t.Weekday() == time.Sunday {
		return false
	}

	morningStart := time.Date(t.Year(), t.Month(), t.Day(), 9, 30, 0, 0, loc)
	morningEnd := time.Date(t.Year(), t.Month(), t.Day(), 11, 30, 0, 0, loc)
	if t.After(morningStart) && t.Before(morningEnd) {
		return true
	}

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

	positions, err := s.positionManager.GetPositions()
	if err != nil {
		return fmt.Errorf("获取持仓失败: %w", err)
	}

	var totalDailyPnL float64
	for _, pos := range positions {
		currentPrice := s.getCurrentPrice(ctx, pos.Stock.Symbol)
		if currentPrice > 0 {
			pnl, err := s.positionManager.CalculateUnrealizedPnL(pos.Stock.Symbol, decimal.NewFromFloat(currentPrice))
			if err == nil {
				pnlF, _ := pnl.Float64()
				totalDailyPnL += pnlF
			}
		}
	}

	account, err := s.accountService.GetAccount()
	if err != nil {
		return fmt.Errorf("获取账户失败: %w", err)
	}

	var totalMarketValue float64
	for _, pos := range positions {
		currentPrice := s.getCurrentPrice(ctx, pos.Stock.Symbol)
		if currentPrice > 0 {
			pos.CurrentValue = currentPrice * float64(pos.Quantity)
		}
		totalMarketValue += pos.CurrentValue
		s.db.Model(&models.Position{}).Where("id = ?", pos.ID).Update("current_value", pos.CurrentValue)
	}

	account.MarketValue = totalMarketValue
	account.TotalAssets = account.AvailableCash + account.FrozenCash + totalMarketValue
	account.TodayPnL = totalDailyPnL

	if account.TotalAssets > 0 {
		account.TodayReturn = totalDailyPnL / (account.TotalAssets - totalDailyPnL) * 100
	}

	if err := s.db.Model(&models.SimAccount{}).Where("id = ?", account.ID).Updates(map[string]interface{}{
		"total_assets": account.TotalAssets,
		"market_value": account.MarketValue,
		"today_pnl":    account.TodayPnL,
		"today_return": account.TodayReturn,
	}).Error; err != nil {
		return fmt.Errorf("更新账户失败: %w", err)
	}

	if err := s.accountService.RecordDailyPnL(today, decimal.NewFromFloat(totalDailyPnL)); err != nil {
		log.Printf("记录每日盈亏到Redis失败: %v", err)
	}

	log.Printf("日结完成: 日期=%s, 总资产=%.2f, 今日盈亏=%.2f, 今日收益率=%.4f%%",
		today, account.TotalAssets, totalDailyPnL, account.TodayReturn)

	return nil
}

// getCurrentPrice gets the current price of a symbol from Redis or market data.
func (s *SimTradeService) getCurrentPrice(ctx context.Context, symbol string) float64 {
	// Try Redis first
	if s.redis != nil {
		key := fmt.Sprintf("quote:%s", symbol)
		val, err := s.redis.Get(ctx, key).Float64()
		if err == nil && val > 0 {
			return val
		}
	}

	// Try Redis market realtime cache
	if s.redis != nil {
		key := fmt.Sprintf("market:realtime:%s", symbol)
		val, err := s.redis.Get(ctx, key).Float64()
		if err == nil && val > 0 {
			return val
		}
	}

	// Try market data service
	if s.marketDataSvc != nil {
		collectors := s.marketDataSvc.GetCollectors()
		for _, collector := range collectors {
			data, err := collector.FetchRealTimeData([]string{symbol})
			if err == nil && len(data) > 0 {
				f := data[0].Price
				if f > 0 {
					return f
				}
			}
		}
	}

	return 0
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

// GetDecisionDates retrieves the distinct list of dates (YYYY-MM-DD) that have
// simulated trade records, newest first. Used to populate the history picker.
func (s *SimTradeService) GetDecisionDates(ctx context.Context) ([]string, error) {
	dates := []string{}
	if err := s.db.Model(&models.SimulatedTrade{}).
		Select("DISTINCT TO_CHAR(executed_at, 'YYYY-MM-DD') AS date").
		Order("date DESC").
		Scan(&dates).Error; err != nil {
		return nil, fmt.Errorf("查询模拟交易日期列表失败: %w", err)
	}
	return dates, nil
}

// GetTradesByDate retrieves simulated trade records for a given date (YYYY-MM-DD).
func (s *SimTradeService) GetTradesByDate(ctx context.Context, date string) ([]models.SimulatedTrade, error) {
	start, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("日期格式无效: %s", date)
	}
	end := start.AddDate(0, 0, 1)

	var trades []models.SimulatedTrade
	if err := s.db.Preload("Stock").
		Where("executed_at >= ? AND executed_at < ?", start, end).
		Order("executed_at DESC").
		Find(&trades).Error; err != nil {
		return nil, fmt.Errorf("查询模拟交易记录失败: %w", err)
	}
	return trades, nil
}

// ──────────────────────────────────────────────────────────────
// Status View
// ──────────────────────────────────────────────────────────────

// SimAccountView is the account view for the simulated trading panel.
type SimAccountView struct {
	TotalAsset         float64 `json:"total_asset"`
	AvailableCash      float64 `json:"available_cash"`
	MarketValue        float64 `json:"market_value"`
	TodayProfit        float64 `json:"today_profit"`
	TodayProfitPercent float64 `json:"today_profit_percent"`
	TotalProfit        float64 `json:"total_profit"`
	TotalProfitPercent float64 `json:"total_profit_percent"`
	InitialCapital     float64 `json:"initial_capital"`
	StartDate          string  `json:"start_date"`
	IsRunning          bool    `json:"is_running"`
}

// SimDecisionView is a single simulated trading decision view.
type SimDecisionView struct {
	ID         uint      `json:"id"`
	Symbol     string    `json:"symbol"`
	Name       string    `json:"name"`
	Side       string    `json:"side"`
	Price      float64   `json:"price"`
	Quantity   int       `json:"quantity"`
	Confidence float64   `json:"confidence"`
	Reason     string    `json:"reason"`
	CreatedAt  time.Time `json:"created_at"`
}

// SimRecordView is a single simulated trade record view.
type SimRecordView struct {
	ID            uint      `json:"id"`
	Symbol        string    `json:"symbol"`
	Name          string    `json:"name"`
	Side          string    `json:"side"`
	Price         float64   `json:"price"`
	Quantity      int       `json:"quantity"`
	Profit        float64   `json:"profit"`
	ProfitPercent float64   `json:"profit_percent"`
	CreatedAt     time.Time `json:"created_at"`
}

// SimRiskControlView is the risk control view for the simulated trading panel.
type SimRiskControlView struct {
	MaxDailyLoss         float64 `json:"max_daily_loss"`
	CurrentDailyLoss     float64 `json:"current_daily_loss"`
	MaxPositionRatio     float64 `json:"max_position_ratio"`
	CurrentPositionRatio float64 `json:"current_position_ratio"`
	MaxSingleStockRatio  float64 `json:"max_single_stock_ratio"`
	Status               string  `json:"status"`
}

// SimStatusView is the full simulated trading status returned to the panel.
type SimStatusView struct {
	Running     bool               `json:"running"`
	Account     SimAccountView     `json:"account"`
	Decisions   []SimDecisionView  `json:"decisions"`
	Records     []SimRecordView    `json:"records"`
	RiskControl SimRiskControlView `json:"risk_control"`
}

func (s *SimTradeService) tradeToView(t models.SimulatedTrade) (SimDecisionView, SimRecordView) {
	decision := SimDecisionView{
		ID:         t.ID,
		Symbol:     t.Stock.Symbol,
		Name:       t.Stock.Name,
		Side:       t.TradeType,
		Price:      t.Price,
		Quantity:   t.Quantity,
		Confidence: t.Confidence,
		Reason:     t.Reason,
		CreatedAt:  t.ExecutedAt,
	}
	// 盈亏率兜底：历史记录无 profit_rate 时按卖出价估算
	profitRate := t.ProfitRate
	if profitRate == 0 && t.Quantity > 0 && t.Price > 0 {
		profitRate = t.Profit / (t.Price * float64(t.Quantity)) * 100
	}
	record := SimRecordView{
		ID:            t.ID,
		Symbol:        t.Stock.Symbol,
		Name:          t.Stock.Name,
		Side:          t.TradeType,
		Price:         t.Price,
		Quantity:      t.Quantity,
		Profit:        t.Profit,
		ProfitPercent: profitRate,
		CreatedAt:     t.ExecutedAt,
	}
	return decision, record
}

// ToRecordView converts a simulated trade record into its record view (exported for handlers).
func (s *SimTradeService) ToRecordView(t models.SimulatedTrade) (SimDecisionView, SimRecordView) {
	return s.tradeToView(t)
}

// GetStatus returns the full simulated trading status for the panel.
func (s *SimTradeService) GetStatus(ctx context.Context) *SimStatusView {
	view := &SimStatusView{
		Running:   s.IsRunning(),
		Decisions: []SimDecisionView{},
		Records:   []SimRecordView{},
	}

	// 账户
	account, err := s.accountService.GetAccount()
	if err == nil {
		totalProfit := account.TotalAssets - account.InitialCapital
		totalProfitPercent := 0.0
		if account.InitialCapital > 0 {
			totalProfitPercent = totalProfit / account.InitialCapital * 100
		}
		view.Account = SimAccountView{
			TotalAsset:         account.TotalAssets,
			AvailableCash:      account.AvailableCash,
			MarketValue:        account.MarketValue,
			TodayProfit:        account.TodayPnL,
			TodayProfitPercent: account.TodayReturn,
			TotalProfit:        totalProfit,
			TotalProfitPercent: totalProfitPercent,
			InitialCapital:     account.InitialCapital,
			StartDate:          account.StartDate,
			IsRunning:          account.IsRunning,
		}

		// 风险控制
		maxDailyLoss := account.TotalAssets * 0.05
		currentPositionRatio := 0.0
		if account.TotalAssets > 0 {
			currentPositionRatio = account.MarketValue / account.TotalAssets
		}
		status := "normal"
		if account.TodayPnL < 0 && -account.TodayPnL >= maxDailyLoss {
			status = "danger"
		} else if (account.TodayPnL < 0 && -account.TodayPnL >= maxDailyLoss*0.5) || currentPositionRatio >= 0.9 {
			status = "warning"
		}
		view.RiskControl = SimRiskControlView{
			MaxDailyLoss:         maxDailyLoss,
			CurrentDailyLoss:     account.TodayPnL,
			MaxPositionRatio:     1.0,
			CurrentPositionRatio: currentPositionRatio,
			MaxSingleStockRatio:  0.2,
			Status:               status,
		}
	}

	// 今日决策（当日已执行的交易视为已做出的决策）
	if trades, total, err := s.GetSimTrades(ctx, 10, 0); err == nil && total > 0 {
		for _, t := range trades {
			decision, _ := s.tradeToView(t)
			view.Decisions = append(view.Decisions, decision)
		}
	}

	// 交易记录
	if trades, _, err := s.GetSimTrades(ctx, 30, 0); err == nil {
		for _, t := range trades {
			_, record := s.tradeToView(t)
			view.Records = append(view.Records, record)
		}
	}

	return view
}

// TriggerDecisionCycle manually triggers one decision cycle.
// 返回是否实际执行：非交易时段（手动联调触发）不成交，返回 false。
func (s *SimTradeService) TriggerDecisionCycle(ctx context.Context) bool {
	if !s.isInTradingHours(time.Now()) {
		log.Println("非交易时段，跳过手动触发决策")
		return false
	}
	s.runDecisionCycle(ctx)
	return true
}
