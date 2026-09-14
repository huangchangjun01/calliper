package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/quant-trading/backend/internal/models"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────────────────────
// EvaluationService — 预测成功率自评估服务
// ──────────────────────────────────────────────────────────────

// EvaluationService evaluates prediction accuracy and computes
// risk / return metrics for the quantitative trading system.
type EvaluationService struct {
	db   *gorm.DB
	tsdb *gorm.DB

	// Evaluation / decision-support thresholds (go/no-go gates)
	suspendThreshold        int
	retrainThreshold        int
	highConfidenceThreshold float64
}

// NewEvaluationService creates a new EvaluationService.
func NewEvaluationService(db, tsdb *gorm.DB) *EvaluationService {
	return &EvaluationService{
		db:                      db,
		tsdb:                    tsdb,
		suspendThreshold:        45,
		retrainThreshold:        60,
		highConfidenceThreshold: 60,
	}
}

// SetThresholds configures the go/no-go evaluation thresholds (override env defaults).
func (s *EvaluationService) SetThresholds(suspendThreshold, retrainThreshold int) {
	s.suspendThreshold = suspendThreshold
	s.retrainThreshold = retrainThreshold
}

// SetHighConfidenceThreshold configures the dashboard high-confidence cutoff.
func (s *EvaluationService) SetHighConfidenceThreshold(v float64) {
	s.highConfidenceThreshold = v
}

// GetHighConfidenceThreshold returns the current high-confidence cutoff.
func (s *EvaluationService) GetHighConfidenceThreshold() float64 {
	return s.highConfidenceThreshold
}

// ──────────────────────────────────────────────────────────────
// Response types
// ──────────────────────────────────────────────────────────────

// AccuracyStats holds multi-horizon accuracy statistics.
type AccuracyStats struct {
	Symbol           string                 `json:"symbol"`
	Accuracy7d       float64                `json:"accuracy_7d"`
	Accuracy30d      float64                `json:"accuracy_30d"`
	AccuracyTotal    float64                `json:"accuracy_total"`
	TotalPredictions int                    `json:"total_predictions"`
	ByPeriod         map[string]PeriodStats `json:"by_period"`
}

// PeriodStats holds accuracy for a single prediction period.
type PeriodStats struct {
	Period        string  `json:"period"`
	Accuracy7d    float64 `json:"accuracy_7d"`
	Accuracy30d   float64 `json:"accuracy_30d"`
	AccuracyTotal float64 `json:"accuracy_total"`
	CorrectCount  int     `json:"correct_count"`
	TotalCount    int     `json:"total_count"`
}

// StockAccuracy holds per-stock accuracy ranking data.
type StockAccuracy struct {
	Symbol           string  `json:"symbol"`
	Accuracy         float64 `json:"accuracy"`
	TotalPredictions int     `json:"total_predictions"`
}

// EvaluationMetrics holds composite risk / return metrics.
type EvaluationMetrics struct {
	Symbol       string  `json:"symbol"`
	ExcessReturn float64 `json:"excess_return"`
	SharpeRatio  float64 `json:"sharpe_ratio"`
	MaxDrawdown  float64 `json:"max_drawdown"`
}

// FailureAnalysis holds attribution data for a failed prediction.
type FailureAnalysis struct {
	PredictionID       uint   `json:"prediction_id"`
	Symbol             string `json:"symbol"`
	PredictedDirection string `json:"predicted_direction"`
	ActualDirection    string `json:"actual_direction"`
	IsEarningsSeason   bool   `json:"is_earnings_season"`
	IndustryAnomaly    bool   `json:"industry_anomaly"`
	VolumeAnomaly      bool   `json:"volume_anomaly"`
	Summary            string `json:"summary"`
}

// ──────────────────────────────────────────────────────────────
// Daily evaluation
// ──────────────────────────────────────────────────────────────

// EvaluateDaily evaluates expired but unverified predictions (待验证) against
// actual market data and writes accuracy records. Predictions without real
// market data are skipped (无行情顺延) and remain待验证 (Success = NULL).
func (s *EvaluationService) EvaluateDaily(ctx context.Context) error {
	now := time.Now()

	// 1. 获取所有已到期但未验证的预测（success IS NULL 且 valid_until 已过）
	var predictions []models.Prediction
	if s.db != nil {
		if err := s.db.WithContext(ctx).
			Where("success IS NULL AND valid_until IS NOT NULL AND valid_until <= ?", now).
			Find(&predictions).Error; err != nil {
			return fmt.Errorf("failed to query expired unverified predictions: %w", err)
		}
	}

	if len(predictions) == 0 {
		log.Printf("[Evaluation] No expired unverified predictions (valid_until <= now), skipping evaluation")
		return nil
	}

	log.Printf("[Evaluation] Found %d expired unverified prediction(s) to evaluate", len(predictions))

	// 2. 逐一获取实际行情并判定结果
	for _, pred := range predictions {
		start := pred.PredictedAt
		end := pred.ValidUntil
		if end.IsZero() {
			end = now
		}

		actualChange, ok := s.getActualChange(ctx, pred.StockID, start, end)

		// 3. 无行情：无行情顺延，保持 Success = NULL（待验证），不写准确率记录
		if !ok {
			log.Printf("[Evaluation] Prediction #%d (stock=%d, dir=%s) expired but no market data found in window [%s, %s], SKIPPED (无行情顺延, remains 待验证)",
				pred.ID, pred.StockID, pred.Direction, start.Format("2006-01-02"), end.Format("2006-01-02"))
			continue
		}

		// 4. 对比预测方向与实际涨跌方向
		isCorrect := s.judgePrediction(pred.Direction, actualChange, pred.Period)
		actualDirection := s.directionFromChange(actualChange)

		// 5. 写入 prediction_accuracies 表
		accuracy := models.PredictionAccuracy{
			StockID:            pred.StockID,
			PredictionID:       pred.ID,
			PredictedDirection: pred.Direction,
			ActualDirection:    actualDirection,
			IsCorrect:          isCorrect,
			Period:             pred.Period,
			EvaluatedAt:        time.Now(),
		}
		if s.db != nil {
			_ = s.db.WithContext(ctx).Create(&accuracy)
		}

		// 6. 更新预测记录的 success 字段（success := isCorrect）
		if s.db != nil {
			success := isCorrect
			_ = s.db.WithContext(ctx).Model(&pred).Update("success", success)
		}
		log.Printf("[Evaluation] Prediction #%d updated: dir=%s actualChange=%.2f%% isCorrect=%v",
			pred.ID, pred.Direction, actualChange, isCorrect)
	}

	return nil
}

// judgePrediction compares predicted direction with actual price change.
func (s *EvaluationService) judgePrediction(direction string, actualChange float64, period string) bool {
	switch direction {
	case "bullish", "看涨", "up":
		return actualChange > 0
	case "bearish", "看跌", "down":
		return actualChange < 0
	case "neutral", "震荡", "flat":
		var threshold float64
		switch period {
		case "medium_term", "medium":
			threshold = 2.0
		case "long_term", "long":
			threshold = 5.0
		default:
			threshold = 0.5
		}
		return math.Abs(actualChange) <= threshold
	default:
		return false
	}
}

// directionFromChange converts a price change percentage to a direction label.
func (s *EvaluationService) directionFromChange(change float64) string {
	if change > 0.5 {
		return "bullish"
	} else if change < -0.5 {
		return "bearish"
	}
	return "neutral"
}

// getActualChange retrieves the cumulative price change for a stock over the given window.
// It returns ok=false when no real market data exists in the range (tsdb nil or
// no StockPriceDaily row found with valid prices), so the caller can skip.
func (s *EvaluationService) getActualChange(ctx context.Context, stockID uint, start, end time.Time) (change float64, ok bool) {
	if s.tsdb == nil {
		return 0, false
	}
	// 第一条日线（time ASC）
	var first models.StockPriceDaily
	err := s.tsdb.WithContext(ctx).
		Where("stock_id = ? AND time >= ? AND time < ?", stockID, start, end).
		Order("time ASC").
		First(&first).Error
	if err != nil || first.Close <= 0 {
		return 0, false
	}
	// 最后一条日线（time DESC）
	var last models.StockPriceDaily
	errLast := s.tsdb.WithContext(ctx).
		Where("stock_id = ? AND time >= ? AND time < ?", stockID, start, end).
		Order("time DESC").
		First(&last).Error
	if errLast != nil || last.Close <= 0 {
		return 0, false
	}
	if !last.Time.After(first.Time) {
		return 0, false
	}
	return (last.Close - first.Close) / first.Close * 100, true
}

// ──────────────────────────────────────────────────────────────
// Accuracy statistics
// ──────────────────────────────────────────────────────────────

// GetAccuracyStats computes 7-day, 30-day and cumulative accuracy for a symbol.
func (s *EvaluationService) GetAccuracyStats(symbol string) (*AccuracyStats, error) {
	var stock models.Stock
	if s.db != nil {
		if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
			return nil, fmt.Errorf("stock not found: %s", symbol)
		}
	}

	now := time.Now()
	sevenDaysAgo := now.AddDate(0, 0, -7)
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	stats := &AccuracyStats{
		Symbol:   symbol,
		ByPeriod: make(map[string]PeriodStats),
	}

	periods := []string{"short", "medium", "long"}
	for _, period := range periods {
		correct7d, total7d := s.countAccuracy(stock.ID, period, sevenDaysAgo, now)
		correct30d, total30d := s.countAccuracy(stock.ID, period, thirtyDaysAgo, now)
		correctTotal, totalTotal := s.countAccuracy(stock.ID, period, time.Time{}, now)

		ps := PeriodStats{
			Period:       period,
			CorrectCount: correctTotal,
			TotalCount:   totalTotal,
		}
		if total7d > 0 {
			ps.Accuracy7d = math.Round(float64(correct7d)/float64(total7d)*10000) / 100
		}
		if total30d > 0 {
			ps.Accuracy30d = math.Round(float64(correct30d)/float64(total30d)*10000) / 100
		}
		if totalTotal > 0 {
			ps.AccuracyTotal = math.Round(float64(correctTotal)/float64(totalTotal)*10000) / 100
		}
		stats.ByPeriod[period] = ps
		stats.TotalPredictions += totalTotal
	}

	// Aggregate stats
	var aggCorrect7d, aggTotal7d, aggCorrect30d, aggTotal30d, aggCorrectTotal, aggTotalTotal int
	if s.db != nil {
		for _, period := range periods {
			c7, t7 := s.countAccuracy(stock.ID, period, sevenDaysAgo, now)
			aggCorrect7d += c7
			aggTotal7d += t7
			c30, t30 := s.countAccuracy(stock.ID, period, thirtyDaysAgo, now)
			aggCorrect30d += c30
			aggTotal30d += t30
			cTotal, tTotal := s.countAccuracy(stock.ID, period, time.Time{}, now)
			aggCorrectTotal += cTotal
			aggTotalTotal += tTotal
		}
	}

	if aggTotal7d > 0 {
		stats.Accuracy7d = math.Round(float64(aggCorrect7d)/float64(aggTotal7d)*10000) / 100
	}
	if aggTotal30d > 0 {
		stats.Accuracy30d = math.Round(float64(aggCorrect30d)/float64(aggTotal30d)*10000) / 100
	}
	if aggTotalTotal > 0 {
		stats.AccuracyTotal = math.Round(float64(aggCorrectTotal)/float64(aggTotalTotal)*10000) / 100
	}

	return stats, nil
}

// countAccuracy counts correct and total predictions for a stock/period/time range.
func (s *EvaluationService) countAccuracy(stockID uint, period string, since, until time.Time) (correct, total int) {
	if s.db == nil {
		return 0, 0
	}

	var accuracies []models.PredictionAccuracy
	query := s.db.Where("stock_id = ? AND period = ?", stockID, period)
	if !since.IsZero() {
		query = query.Where("evaluated_at >= ?", since)
	}
	query = query.Where("evaluated_at <= ?", until)
	if err := query.Find(&accuracies).Error; err != nil {
		return 0, 0
	}

	for _, a := range accuracies {
		total++
		if a.IsCorrect {
			correct++
		}
	}
	return
}

// ──────────────────────────────────────────────────────────────
// Accuracy ranking
// ──────────────────────────────────────────────────────────────

// GetAccuracyRanking returns stocks ranked by prediction accuracy.
func (s *EvaluationService) GetAccuracyRanking(period string, limit int) ([]StockAccuracy, error) {
	if s.db == nil {
		return nil, fmt.Errorf("database not available")
	}

	var stocks []models.Stock
	if err := s.db.Where("is_active = ?", true).Find(&stocks).Error; err != nil {
		return nil, fmt.Errorf("failed to query stocks: %w", err)
	}

	// 无数据时返回空数组（非 nil），符合前端文档预期。
	rankings := make([]StockAccuracy, 0)
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	for _, stock := range stocks {
		var accuracies []models.PredictionAccuracy
		query := s.db.Where("stock_id = ?", stock.ID)
		// 仅 short/medium/long 视为真实周期过滤；""、all 及其它非法值一律不过滤。
		switch period {
		case "short", "medium", "long":
			query = query.Where("period = ?", period)
		}
		query = query.Where("evaluated_at >= ?", thirtyDaysAgo)
		query.Find(&accuracies)

		if len(accuracies) == 0 {
			continue
		}

		correct := 0
		for _, a := range accuracies {
			if a.IsCorrect {
				correct++
			}
		}
		// roundPercent 返回百分制（0-100），前端按 0-1 比例展示，统一为分数
		accuracy := roundPercent(float64(correct), float64(len(accuracies))) / 100.0
		rankings = append(rankings, StockAccuracy{
			Symbol:           stock.Symbol,
			Accuracy:         accuracy,
			TotalPredictions: len(accuracies),
		})
	}

	sort.Slice(rankings, func(i, j int) bool {
		return rankings[i].Accuracy > rankings[j].Accuracy
	})

	if limit > 0 && len(rankings) > limit {
		rankings = rankings[:limit]
	}

	return rankings, nil
}

// ──────────────────────────────────────────────────────────────
// Excess return
// ──────────────────────────────────────────────────────────────

// CalculateExcessReturn computes the excess return of a stock's simulated
// trading portfolio over a benchmark (market index).
func (s *EvaluationService) CalculateExcessReturn(symbol string, period string) (float64, error) {
	if s.db == nil {
		return 0, fmt.Errorf("database not available")
	}

	// Use simulated trades to compute cumulative return
	trades, err := s.getSimulatedTrades(symbol)
	if err != nil {
		return 0, fmt.Errorf("failed to get simulated trades: %w", err)
	}

	portfolioReturn := s.computePortfolioReturn(trades)
	benchmarkReturn := s.getBenchmarkReturn(period)

	return math.Round((portfolioReturn-benchmarkReturn)*10000) / 100, nil
}

// computePortfolioReturn estimates total return from simulated trades.
func (s *EvaluationService) computePortfolioReturn(trades []models.SimulatedTrade) float64 {
	if len(trades) == 0 {
		return 0
	}
	totalPL := 0.0
	totalCost := 0.0
	for _, t := range trades {
		cost := t.Price * float64(t.Quantity)
		totalCost += cost
		// Calculate P&L based on current price from DB
		currentPrice := s.getCurrentPriceFromDB(t.StockID)
		if currentPrice > 0 {
			if t.TradeType == "buy" {
				totalPL += (currentPrice - t.Price) * float64(t.Quantity)
			} else if t.TradeType == "sell" {
				totalPL += (t.Price - currentPrice) * float64(t.Quantity)
			}
		}
	}
	if totalCost == 0 {
		return 0
	}
	return totalPL / totalCost * 100
}

// getCurrentPriceFromDB gets the latest price for a stock from the database.
func (s *EvaluationService) getCurrentPriceFromDB(stockID uint) float64 {
	if s.tsdb == nil {
		return 0
	}
	var daily models.StockPriceDaily
	if err := s.tsdb.Where("stock_id = ?", stockID).
		Order("time DESC").
		First(&daily).Error; err != nil {
		return 0
	}
	return daily.Close
}

// getBenchmarkReturn returns benchmark index return for the period.
func (s *EvaluationService) getBenchmarkReturn(period string) float64 {
	// Compute benchmark return from actual market index data
	if s.tsdb == nil {
		return 0
	}

	now := time.Now()
	var daysBack int
	switch period {
	case "short":
		daysBack = 7
	case "medium":
		daysBack = 30
	case "long":
		daysBack = 90
	default:
		daysBack = 7
	}

	start := now.AddDate(0, 0, -daysBack)

	// Try to get SSE Composite Index (000001) as benchmark
	var daily models.StockPriceDaily
	err := s.tsdb.Where("stock_id = (SELECT id FROM stocks WHERE symbol = '000001' LIMIT 1)").
		Where("time >= ? AND time <= ?", start, now).
		Order("time ASC").
		First(&daily).Error
	if err != nil {
		return 0
	}

	var latest models.StockPriceDaily
	err = s.tsdb.Where("stock_id = (SELECT id FROM stocks WHERE symbol = '000001' LIMIT 1)").
		Where("time >= ? AND time <= ?", start, now).
		Order("time DESC").
		First(&latest).Error
	if err != nil || daily.Close == 0 {
		return 0
	}

	return (latest.Close - daily.Close) / daily.Close * 100
}

// getSimulatedTrades fetches simulated trades for a symbol.
func (s *EvaluationService) getSimulatedTrades(symbol string) ([]models.SimulatedTrade, error) {
	var stock models.Stock
	if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return nil, err
	}
	var trades []models.SimulatedTrade
	if err := s.db.Where("stock_id = ?", stock.ID).Find(&trades).Error; err != nil {
		return nil, err
	}
	return trades, nil
}

// ──────────────────────────────────────────────────────────────
// Sharpe ratio
// ──────────────────────────────────────────────────────────────

// CalculateSharpeRatio computes the Sharpe ratio based on daily returns
// from the simulated trading portfolio.
func (s *EvaluationService) CalculateSharpeRatio(symbol string) (float64, error) {
	dailyReturns := s.getDailyReturns(symbol)

	if len(dailyReturns) < 2 {
		return 0, nil
	}

	meanReturn := s.mean(dailyReturns)
	stdDev := s.stdDev(dailyReturns, meanReturn)

	if stdDev == 0 {
		return 0, nil
	}

	// Risk-free rate ~ 2.5% annually, daily ≈ 0.01%
	riskFreeDaily := 0.025 / 252
	sharpe := (meanReturn - riskFreeDaily) / stdDev * math.Sqrt(252)

	return math.Round(sharpe*100) / 100, nil
}

// getDailyReturns returns daily portfolio returns.
func (s *EvaluationService) getDailyReturns(symbol string) []float64 {
	if s.db == nil || s.tsdb == nil {
		return nil
	}

	var stock models.Stock
	if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return nil
	}

	var dailies []models.StockPriceDaily
	_ = s.tsdb.Where("stock_id = ?", stock.ID).
		Order("time ASC").
		Limit(252).
		Find(&dailies)

	if len(dailies) < 2 {
		return nil
	}

	var returns []float64
	for i := 1; i < len(dailies); i++ {
		if dailies[i-1].Close > 0 {
			r := (dailies[i].Close - dailies[i-1].Close) / dailies[i-1].Close
			returns = append(returns, r)
		}
	}
	return returns
}

// ──────────────────────────────────────────────────────────────
// Max drawdown
// ──────────────────────────────────────────────────────────────

// CalculateMaxDrawdown computes the maximum drawdown from simulated trading.
func (s *EvaluationService) CalculateMaxDrawdown(symbol string) (float64, error) {
	dailyReturns := s.getDailyReturns(symbol)
	if len(dailyReturns) < 2 {
		return 0, nil
	}

	// Build cumulative equity curve
	equity := 1.0
	peak := 1.0
	maxDD := 0.0

	for _, r := range dailyReturns {
		equity *= (1 + r)
		if equity > peak {
			peak = equity
		}
		dd := (peak - equity) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}

	return math.Round(maxDD*10000) / 100, nil
}

// ──────────────────────────────────────────────────────────────
// Failure analysis
// ──────────────────────────────────────────────────────────────

// AnalyzeFailure performs attribution analysis on a failed prediction.
func (s *EvaluationService) AnalyzeFailure(symbol string, predictionID uint) (*FailureAnalysis, error) {
	var stock models.Stock
	if s.db != nil {
		if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
			return nil, fmt.Errorf("stock not found: %s", symbol)
		}
	}

	var prediction models.Prediction
	if s.db != nil {
		if err := s.db.First(&prediction, predictionID).Error; err != nil {
			return nil, fmt.Errorf("prediction not found: %d", predictionID)
		}
	} else {
		prediction = models.Prediction{
			ID:        predictionID,
			Direction: "bullish",
			Period:    "short",
		}
	}

	analysis := &FailureAnalysis{
		PredictionID:       predictionID,
		Symbol:             symbol,
		PredictedDirection: prediction.Direction,
		ActualDirection:    s.actualDirectionFromPrices(prediction.StockID),
	}

	// 检测是否在财报发布日附近
	analysis.IsEarningsSeason = s.isNearEarningsDate(symbol)

	// 检测当日是否有大幅行业异动（行业指数涨跌 > 3%）
	analysis.IndustryAnomaly = s.hasIndustryAnomaly(symbol)

	// 检测是否有突发事件（成交量异常放大 > 3倍均值）
	analysis.VolumeAnomaly = s.hasVolumeAnomaly(symbol)

	// Build summary
	var reasons []string
	if analysis.IsEarningsSeason {
		reasons = append(reasons, "财报季节波动")
	}
	if analysis.IndustryAnomaly {
		reasons = append(reasons, "行业板块大幅异动")
	}
	if analysis.VolumeAnomaly {
		reasons = append(reasons, "成交量异常放大")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "模型预测偏差")
	}
	analysis.Summary = fmt.Sprintf("预测失败归因: %s", joinStrings(reasons, "；"))

	return analysis, nil
}

// actualDirectionFromPrices derives the actual market direction from real stock
// prices by comparing the two most recent closing prices in TSDB. Returns:
//   - "up"   when the latest close rose more than +1% vs the previous close
//   - "down" when the latest close fell more than -1% vs the previous close
//   - "flat" when the price moved within +/-1%
//   - ""     when no price data is available (never fabricates a direction)
func (s *EvaluationService) actualDirectionFromPrices(stockID uint) string {
	if s.tsdb == nil {
		return ""
	}

	var prices []models.StockPriceDaily
	if err := s.tsdb.Where("stock_id = ?", stockID).
		Order("time DESC").
		Limit(2).
		Find(&prices).Error; err != nil {
		return ""
	}

	if len(prices) < 2 {
		return ""
	}

	// prices[0] is the latest close, prices[1] is the previous close
	prev := prices[1].Close
	if prev == 0 {
		return ""
	}
	change := (prices[0].Close - prev) / prev * 100 // percent change
	switch {
	case change > 1.0:
		return "up"
	case change < -1.0:
		return "down"
	default:
		return "flat"
	}
}

// isNearEarningsDate checks if the date is near earnings season.
func (s *EvaluationService) isNearEarningsDate(symbol string) bool {
	// Check if within typical earnings season months
	now := time.Now()
	month := now.Month()
	// A-share earnings seasons: Jan-Apr (annual report), Jul-Aug (semi-annual), Oct (Q3)
	return month >= time.January && month <= time.April ||
		month >= time.July && month <= time.August ||
		month == time.October
}

// hasIndustryAnomaly checks if the industry index moved > 3%.
func (s *EvaluationService) hasIndustryAnomaly(symbol string) bool {
	if s.tsdb == nil {
		return false
	}

	var stock models.Stock
	if s.db == nil {
		return false
	}
	if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return false
	}

	// Check if the industry index has moved > 3% recently
	now := time.Now()
	start := now.AddDate(0, 0, -1)

	var daily models.StockPriceDaily
	if err := s.tsdb.Where("stock_id = ?", stock.ID).
		Where("time >= ? AND time <= ?", start, now).
		Order("time DESC").
		First(&daily).Error; err != nil {
		return false
	}

	if daily.Open > 0 {
		change := math.Abs((daily.Close - daily.Open) / daily.Open * 100)
		return change > 3.0
	}
	return false
}

// hasVolumeAnomaly checks if volume is > 3x average.
func (s *EvaluationService) hasVolumeAnomaly(symbol string) bool {
	if s.tsdb == nil {
		return false
	}

	var stock models.Stock
	if s.db == nil {
		return false
	}
	if err := s.db.Where("symbol = ?", symbol).First(&stock).Error; err != nil {
		return false
	}

	// Get average volume over last 20 days
	var avgVolume float64
	_ = s.tsdb.Model(&models.StockPriceDaily{}).
		Where("stock_id = ?", stock.ID).
		Select("AVG(volume)").
		Order("time DESC").
		Limit(20).
		Scan(&avgVolume)

	// Get latest volume
	var latest models.StockPriceDaily
	if err := s.tsdb.Where("stock_id = ?", stock.ID).
		Order("time DESC").First(&latest).Error; err != nil {
		return false
	}

	if avgVolume > 0 && float64(latest.Volume) > avgVolume*3 {
		return true
	}
	return false
}

// ──────────────────────────────────────────────────────────────
// Evaluation metrics (composite)
// ──────────────────────────────────────────────────────────────

// GetMetrics returns all evaluation metrics for a symbol.
func (s *EvaluationService) GetMetrics(symbol string) (*EvaluationMetrics, error) {
	excessReturn, err := s.CalculateExcessReturn(symbol, "short")
	if err != nil {
		excessReturn = 0
	}

	sharpe, err := s.CalculateSharpeRatio(symbol)
	if err != nil {
		sharpe = 0
	}

	maxDD, err := s.CalculateMaxDrawdown(symbol)
	if err != nil {
		maxDD = 0
	}

	return &EvaluationMetrics{
		Symbol:       symbol,
		ExcessReturn: excessReturn,
		SharpeRatio:  sharpe,
		MaxDrawdown:  maxDD,
	}, nil
}

// ──────────────────────────────────────────────────────────────
// Math helpers
// ──────────────────────────────────────────────────────────────

func (s *EvaluationService) mean(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func (s *EvaluationService) stdDev(values []float64, mean float64) float64 {
	if len(values) < 2 {
		return 0
	}
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	return math.Sqrt(sumSq / float64(len(values)-1))
}

// joinStrings joins strings with a separator.
func joinStrings(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += sep + parts[i]
	}
	return result
}

// ──────────────────────────────────────────────────────────────
// Decision-support: prediction list, failures, accuracy trend
// ──────────────────────────────────────────────────────────────

// PredictionFilter describes the filterable fields for listing predictions.
type PredictionFilter struct {
	Period        string  // exact period match
	Direction     string  // exact direction match
	ConfidenceMin float64 // confidence >= threshold
	Expired       string  // "", "true", "false", "pending"
}

// PredictionDetailItem is a single prediction row for the list view.
type PredictionDetailItem struct {
	ID           uint      `json:"id"`
	Symbol       string    `json:"symbol"`
	Name         string    `json:"name"`
	Period       string    `json:"period"`
	Direction    string    `json:"direction"`
	Confidence   float64   `json:"confidence"`
	TargetPrice  float64   `json:"target_price"`
	ActualPrice  *float64  `json:"actual_price,omitempty"` // 最近收盘价（TSDB），用于与目标价对比
	PredictedAt  time.Time `json:"predicted_at"`
	ValidUntil   time.Time `json:"valid_until"`
	Expired      bool      `json:"expired"`
	Status       string    `json:"status"` // "pending" | "correct" | "wrong"
	IsCorrect    *bool     `json:"is_correct,omitempty"`
	ModelVersion string    `json:"model_version"`
	KeyFactors   []string  `json:"key_factors,omitempty"`
}

// ListPredictions returns real prediction records ordered by predicted_at desc,
// supports filters (period / direction / confidence_min / expired) and pagination.
func (s *EvaluationService) ListPredictions(filter PredictionFilter, offset, limit int) ([]PredictionDetailItem, int64, error) {
	if s.db == nil {
		return nil, 0, nil
	}

	q := s.db.Model(&models.Prediction{})
	if filter.Period != "" {
		q = q.Where("period = ?", filter.Period)
	}
	if filter.Direction != "" {
		q = q.Where("direction = ?", filter.Direction)
	}
	if filter.ConfidenceMin > 0 {
		q = q.Where("confidence >= ?", filter.ConfidenceMin)
	}
	switch strings.ToLower(filter.Expired) {
	case "true":
		q = q.Where("success IS NOT NULL")
	case "false", "pending":
		q = q.Where("success IS NULL")
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var preds []models.Prediction
	if err := q.Preload("Stock").
		Order("predicted_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&preds).Error; err != nil {
		return nil, 0, err
	}

	items := make([]PredictionDetailItem, 0, len(preds))
	closes := s.latestCloseByStock(predStockIDs(preds))
	for _, p := range preds {
		item := buildPredictionDetailItem(p)
		if v, ok := closes[p.StockID]; ok && v > 0 {
			item.ActualPrice = &v
		}
		items = append(items, item)
	}
	return items, total, nil
}

// predStockIDs collects unique stock ids from predictions.
func predStockIDs(preds []models.Prediction) []uint {
	ids := make([]uint, 0, len(preds))
	for _, p := range preds {
		ids = append(ids, p.StockID)
	}
	return ids
}

// latestCloseByStock returns the latest daily close price per stock from TSDB.
func (s *EvaluationService) latestCloseByStock(ids []uint) map[uint]float64 {
	out := make(map[uint]float64, len(ids))
	if s.tsdb == nil || len(ids) == 0 {
		return out
	}
	type row struct {
		StockID uint
		Close   float64
	}
	var rows []row
	if err := s.tsdb.Raw(
		`SELECT DISTINCT ON (stock_id) stock_id, close FROM stock_prices_daily WHERE stock_id IN ? ORDER BY stock_id, time DESC`,
		ids,
	).Scan(&rows).Error; err == nil {
		for _, r := range rows {
			out[r.StockID] = r.Close
		}
	}
	return out
}

func buildPredictionDetailItem(p models.Prediction) PredictionDetailItem {
	status := "pending"
	var isCorrect *bool
	if p.Success != nil {
		isCorrect = p.Success
		if *p.Success {
			status = "correct"
		} else {
			status = "wrong"
		}
	}
	item := PredictionDetailItem{
		ID:           p.ID,
		Period:       p.Period,
		Direction:    p.Direction,
		Confidence:   p.Confidence,
		TargetPrice:  p.TargetPrice,
		PredictedAt:  p.PredictedAt,
		ValidUntil:   p.ValidUntil,
		Expired:      status != "pending",
		Status:       status,
		IsCorrect:    isCorrect,
		ModelVersion: p.ModelVersion,
		KeyFactors:   parseFactors(p.Factors),
	}
	if p.Stock.Symbol != "" {
		item.Symbol = p.Stock.Symbol
		item.Name = stockDisplayName(p.Stock)
	}
	return item
}

// ──────────────────────────────────────────────────────────────
// Prediction history (预测历史记录)
// ──────────────────────────────────────────────────────────────

// HistoryFilter describes the filterable fields for prediction history.
type HistoryFilter struct {
	Symbol string // exact symbol match
	Period string // exact period match
	Status string // "pending" | "correct" | "wrong"
	From   string // YYYY-MM-DD, inclusive lower bound on predicted_at
	To     string // YYYY-MM-DD, inclusive upper bound on predicted_at
}

// HistoryItem is a single prediction history record joined with stock info.
type HistoryItem struct {
	ID          uint      `json:"id"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Period      string    `json:"period"`
	Direction   string    `json:"direction"`
	Confidence  float64   `json:"confidence"`
	TargetPrice float64   `json:"target_price"`
	ActualPrice *float64  `json:"actual_price,omitempty"` // 最近收盘价（TSDB），用于与目标价对比
	PredictedAt time.Time `json:"predicted_at"`
	ValidUntil  time.Time `json:"valid_until"`
	Status      string    `json:"status"` // "pending" | "correct" | "wrong"
}

// ListPredictionHistory returns prediction history records joined with stock
// symbol/name, ordered by predicted_at desc. Status is derived from Success:
// nil → pending, true → correct, false → wrong. Supports symbol / period /
// status filters and an inclusive date range on predicted_at.
func (s *EvaluationService) ListPredictionHistory(filter HistoryFilter, offset, limit int) ([]HistoryItem, int64, error) {
	if s.db == nil {
		return nil, 0, nil
	}

	q := s.db.Model(&models.Prediction{})
	if filter.Symbol != "" {
		q = q.Where("stock_id IN (SELECT id FROM stocks WHERE symbol = ?)", filter.Symbol)
	}
	if filter.Period != "" {
		q = q.Where("period = ?", filter.Period)
	}
	switch strings.ToLower(filter.Status) {
	case "pending":
		q = q.Where("success IS NULL")
	case "correct":
		q = q.Where("success = ?", true)
	case "wrong":
		q = q.Where("success = ?", false)
	}
	if filter.From != "" {
		if from, err := time.ParseInLocation("2006-01-02", filter.From, time.Local); err == nil {
			q = q.Where("predicted_at >= ?", from)
		}
	}
	if filter.To != "" {
		if to, err := time.ParseInLocation("2006-01-02", filter.To, time.Local); err == nil {
			q = q.Where("predicted_at < ?", to.AddDate(0, 0, 1))
		}
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var preds []models.Prediction
	if err := q.Preload("Stock").
		Order("predicted_at DESC").
		Offset(offset).
		Limit(limit).
		Find(&preds).Error; err != nil {
		return nil, 0, err
	}

	items := make([]HistoryItem, 0, len(preds))
	closes := s.latestCloseByStock(predStockIDs(preds))
	for _, p := range preds {
		status := "pending"
		if p.Success != nil {
			if *p.Success {
				status = "correct"
			} else {
				status = "wrong"
			}
		}
		item := HistoryItem{
			ID:          p.ID,
			Period:      p.Period,
			Direction:   p.Direction,
			Confidence:  p.Confidence,
			TargetPrice: p.TargetPrice,
			PredictedAt: p.PredictedAt,
			ValidUntil:  p.ValidUntil,
			Status:      status,
		}
		if v, ok := closes[p.StockID]; ok && v > 0 {
			item.ActualPrice = &v
		}
		if p.Stock.Symbol != "" {
			item.Symbol = p.Stock.Symbol
			item.Name = stockDisplayName(p.Stock)
		}
		items = append(items, item)
	}
	return items, total, nil
}

// ──────────────────────────────────────────────────────────────
// Bucket statistics (分时段统计，含断点)
// ──────────────────────────────────────────────────────────────

// BucketStat holds prediction statistics for one time bucket. Accuracy is
// null when the bucket has no evaluated predictions (correct+wrong == 0),
// so the frontend renders a breakpoint instead of 0.
type BucketStat struct {
	Bucket   string   `json:"bucket"` // 2026-08-26 / 2026-W35 / 2026-08
	Total    int      `json:"total"`
	Correct  int      `json:"correct"`
	Wrong    int      `json:"wrong"`
	Pending  int      `json:"pending"`
	Accuracy *float64 `json:"accuracy"`
}

// GetPredictionStats aggregates predictions per day/week/month bucket over
// [from, to]. EVERY bucket in the range is returned (total may be 0); buckets
// without evaluated predictions carry a null accuracy (断点). Accuracy is
// correct/(correct+wrong) as a percentage.
func (s *EvaluationService) GetPredictionStats(bucket string, from, to time.Time) ([]BucketStat, error) {
	if s.db == nil {
		return nil, nil
	}

	unit := "day"
	switch bucket {
	case "day", "week", "month":
		unit = bucket
	}

	// SQL aggregation grouped by date_trunc bucket, keyed by a formatted
	// string so DB-generated keys match the Go-generated bucket list.
	var keyExpr string
	switch unit {
	case "week":
		keyExpr = `TO_CHAR(date_trunc('week', predicted_at), 'IYYY-"W"IW')`
	case "month":
		keyExpr = `TO_CHAR(date_trunc('month', predicted_at), 'YYYY-MM')`
	default:
		keyExpr = `TO_CHAR(predicted_at, 'YYYY-MM-DD')`
	}

	fromStart := bucketStart(unit, from)
	toEnd := startOfNextDay(to)

	var rows []struct {
		Bucket  string
		Total   int
		Correct int
		Wrong   int
		Pending int
	}
	err := s.db.Model(&models.Prediction{}).
		Select(keyExpr+" AS bucket, COUNT(*) AS total, "+
			"SUM(CASE WHEN success THEN 1 ELSE 0 END) AS correct, "+
			"SUM(CASE WHEN success = false THEN 1 ELSE 0 END) AS wrong, "+
			"SUM(CASE WHEN success IS NULL THEN 1 ELSE 0 END) AS pending").
		Where("predicted_at >= ? AND predicted_at < ?", fromStart, toEnd).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Build the full bucket list so missing buckets appear as null breakpoints.
	stats := make([]BucketStat, 0)
	index := make(map[string]int)
	for cur := bucketStart(unit, from); !cur.After(bucketStart(unit, to)); cur = nextBucketStart(unit, cur) {
		key := formatBucketKey(unit, cur)
		index[key] = len(stats)
		stats = append(stats, BucketStat{Bucket: key})
	}

	for _, r := range rows {
		i, ok := index[r.Bucket]
		if !ok {
			continue
		}
		stats[i].Total = r.Total
		stats[i].Correct = r.Correct
		stats[i].Wrong = r.Wrong
		stats[i].Pending = r.Pending
		if evaluated := r.Correct + r.Wrong; evaluated > 0 {
			acc := roundPercent(float64(r.Correct), float64(evaluated))
			stats[i].Accuracy = &acc
		}
	}

	return stats, nil
}

// bucketStart truncates t to the start of its day/week(Monday)/month bucket.
func bucketStart(unit string, t time.Time) time.Time {
	switch unit {
	case "week":
		wd := int(t.Weekday()) // Sunday = 0
		if wd == 0 {
			wd = 7
		}
		d := t.AddDate(0, 0, -(wd - 1))
		return time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, t.Location())
	case "month":
		return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, t.Location())
	default:
		return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	}
}

// nextBucketStart advances t to the start of the next day/week/month bucket.
func nextBucketStart(unit string, t time.Time) time.Time {
	switch unit {
	case "week":
		return t.AddDate(0, 0, 7)
	case "month":
		return t.AddDate(0, 1, 0)
	default:
		return t.AddDate(0, 0, 1)
	}
}

// formatBucketKey renders a bucket start as 2026-08-26 / 2026-W35 / 2026-08.
func formatBucketKey(unit string, t time.Time) string {
	switch unit {
	case "week":
		year, week := t.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	case "month":
		return t.Format("2006-01")
	default:
		return t.Format("2006-01-02")
	}
}

// startOfNextDay returns 00:00 of the day after t (inclusive end-of-day bound).
func startOfNextDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location()).AddDate(0, 0, 1)
}

// FailureItem is a recent wrong prediction (Success=false) for the failures list.
type FailureItem struct {
	ID                 uint      `json:"id"`
	Symbol             string    `json:"symbol"`
	Name               string    `json:"name"`
	Period             string    `json:"period"`
	PredictedDirection string    `json:"predicted_direction"`
	ActualDirection    string    `json:"actual_direction"`
	PredictedAt        time.Time `json:"predicted_at"`
	Summary            string    `json:"summary"`
}

// ListFailures returns recent predictions where Success=false, joined with the
// actual direction from the matching prediction accuracy record.
func (s *EvaluationService) ListFailures(limit int) ([]FailureItem, error) {
	if s.db == nil {
		return nil, nil
	}

	var preds []models.Prediction
	q := s.db.Where("success = ?", false).Preload("Stock").Order("predicted_at DESC")
	if limit > 0 {
		q = q.Limit(limit)
	}
	if err := q.Find(&preds).Error; err != nil {
		return nil, err
	}

	items := make([]FailureItem, 0, len(preds))
	for _, p := range preds {
		actual := ""
		var acc models.PredictionAccuracy
		if err := s.db.Where("prediction_id = ?", p.ID).Order("evaluated_at DESC").First(&acc).Error; err == nil {
			// 归一化为 up|down|flat，避免 neutral 等历史写法破坏前端查表
			actual = normalizeDirection(acc.ActualDirection)
		}
		if actual == "" {
			actual = "unknown"
		}

		name := stockDisplayName(p.Stock)
		symbol := p.Stock.Symbol
		summary := fmt.Sprintf("%s %s(%s) 预测方向 %s，实际方向 %s，预测失败",
			directionLabel(p.Direction), name, symbol, directionLabel(p.Direction), directionLabel(actual))

		items = append(items, FailureItem{
			ID:                 p.ID,
			Symbol:             symbol,
			Name:               name,
			Period:             p.Period,
			PredictedDirection: p.Direction,
			ActualDirection:    actual,
			PredictedAt:        p.PredictedAt,
			Summary:            summary,
		})
	}
	return items, nil
}

// AccuracyTrendItem is one point in the time-series accuracy trend.
// Accuracy is null for dates without evaluated predictions (断点).
type AccuracyTrendItem struct {
	Date     string   `json:"date"`
	Accuracy *float64 `json:"accuracy"`
}

// GetAccuracyTrendData computes an accuracy time-series (date -> accuracy)
// from PredictionAccuracy over the last N days. EVERY date in the range is
// returned; dates without evaluated predictions carry a null accuracy so the
// frontend can render gaps. period uses the DB convention short/medium/long;
// anything else applies no period filter.
func (s *EvaluationService) GetAccuracyTrendData(period string, days int) ([]AccuracyTrendItem, error) {
	if s.db == nil {
		return nil, nil
	}
	if days <= 0 {
		days = 30
	}
	now := time.Now()
	since := now.AddDate(0, 0, -days)

	q := s.db.Model(&models.PredictionAccuracy{}).
		Select("TO_CHAR(evaluated_at, 'YYYY-MM-DD') AS day, COUNT(*) AS total, SUM(CASE WHEN is_correct THEN 1 ELSE 0 END) AS correct").
		Where("evaluated_at >= ?", since)
	switch period {
	case "short":
		q = q.Where("period = ?", "short")
	case "medium":
		q = q.Where("period = ?", "medium")
	case "long":
		q = q.Where("period = ?", "long")
	}

	var rows []struct {
		Day     string
		Total   int
		Correct int
	}
	if err := q.Group("day").Order("day ASC").Scan(&rows).Error; err != nil {
		return nil, err
	}

	byDay := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Total > 0 {
			// roundPercent 返回百分制（0-100），前端按 0-1 比例展示，统一为分数
			byDay[r.Day] = roundPercent(float64(r.Correct), float64(r.Total)) / 100.0
		}
	}

	// Emit every date in [since, now]; missing dates become null breakpoints.
	items := make([]AccuracyTrendItem, 0, days+1)
	for d := since; !d.After(now); d = d.AddDate(0, 0, 1) {
		key := d.Format("2006-01-02")
		item := AccuracyTrendItem{Date: key}
		if acc, ok := byDay[key]; ok {
			item.Accuracy = &acc
		}
		items = append(items, item)
	}
	return items, nil
}

// ──────────────────────────────────────────────────────────────
// Dashboard aggregation
// ──────────────────────────────────────────────────────────────

// HighConfidenceItem is a top high-confidence prediction.
type HighConfidenceItem struct {
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	Period      string    `json:"period"`
	Direction   string    `json:"direction"`
	Confidence  float64   `json:"confidence"`
	TargetPrice float64   `json:"target_price"`
	PredictedAt time.Time `json:"predicted_at"`
}

// PredictionPeriodSummary aggregates prediction direction counts per period.
// JSON 字段保持 camelCase，与前端 PredictionSummary 类型一致。
type PredictionPeriodSummary struct {
	Period      string `json:"period"`
	PeriodLabel string `json:"periodLabel"`
	Total       int64  `json:"total"`
	UpCount     int64  `json:"upCount"`
	DownCount   int64  `json:"downCount"`
	FlatCount   int64  `json:"flatCount"`
}

var summaryPeriodLabels = map[string]string{
	"short":  "短期预测",
	"medium": "中短期预测",
	"long":   "长期预测",
}

// GetPredictionSummaries returns per-period direction aggregates over stored
// predictions (真实数据统计，含 pending/correct/wrong 全部记录)。
func (s *EvaluationService) GetPredictionSummaries() []PredictionPeriodSummary {
	out := make([]PredictionPeriodSummary, 0, 3)
	if s.db == nil {
		return out
	}
	type row struct {
		Period    string
		Direction string
		Cnt       int64
	}
	var rows []row
	if err := s.db.Model(&models.Prediction{}).
		Select("period, direction, count(*) as cnt").
		Group("period, direction").Scan(&rows).Error; err != nil {
		return out
	}

	agg := make(map[string]*PredictionPeriodSummary, 3)
	for _, p := range []string{"short", "medium", "long"} {
		agg[p] = &PredictionPeriodSummary{Period: p, PeriodLabel: summaryPeriodLabels[p]}
	}
	for _, r := range rows {
		a := agg[r.Period]
		if a == nil {
			continue
		}
		a.Total += r.Cnt
		switch r.Direction {
		case "up":
			a.UpCount += r.Cnt
		case "down":
			a.DownCount += r.Cnt
		case "flat":
			a.FlatCount += r.Cnt
		}
	}
	for _, p := range []string{"short", "medium", "long"} {
		out = append(out, *agg[p])
	}
	return out
}

// GetHighConfidencePredictions returns the top N pending predictions with
// confidence >= threshold (0-1 比例), ordered by confidence desc.
func (s *EvaluationService) GetHighConfidencePredictions(threshold float64, n int) ([]HighConfidenceItem, error) {
	if s.db == nil {
		return nil, nil
	}
	if n <= 0 {
		n = 10
	}

	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	// 先按置信度预取更多候选项（便于过滤后仍能取满 n 条）。
	prefetch := n * 4
	if prefetch < n {
		prefetch = n
	}

	var preds []models.Prediction
	if err := s.db.Where("success IS NULL AND confidence >= ?", threshold).
		Preload("Stock").
		Order("confidence DESC").
		Limit(prefetch).
		Find(&preds).Error; err != nil {
		return nil, err
	}

	items := make([]HighConfidenceItem, 0, n)
	for _, p := range preds {
		if len(items) >= n {
			break
		}
		// spec：个股最近 30 日准确率 < 45%（0.45）的股票不应出现在高置信度推荐。
		// 无 30 日评估数据（total==0，新股票）保留，不被误杀。
		correct, total := s.countAccuracy(p.StockID, p.Period, thirtyDaysAgo, now)
		if total > 0 && float64(correct)/float64(total) < 0.45 {
			continue
		}
		item := HighConfidenceItem{
			Period:      p.Period,
			Direction:   p.Direction,
			Confidence:  p.Confidence,
			TargetPrice: p.TargetPrice,
			PredictedAt: p.PredictedAt,
		}
		if p.Stock.Symbol != "" {
			item.Symbol = p.Stock.Symbol
			item.Name = stockDisplayName(p.Stock)
		}
		items = append(items, item)
	}
	return items, nil
}

// AccuracySummary holds overall accuracy across all stocks.
type AccuracySummary struct {
	Accuracy7d     float64 `json:"accuracy_7d"`
	Accuracy30d    float64 `json:"accuracy_30d"`
	AccuracyTotal  float64 `json:"accuracy_total"`
	TotalEvaluated int     `json:"total_evaluated"`
}

// GetAccuracySummary computes overall 7d / 30d / total accuracy across all stocks.
func (s *EvaluationService) GetAccuracySummary() (*AccuracySummary, error) {
	sum := &AccuracySummary{}
	if s.db == nil {
		return sum, nil
	}

	now := time.Now()
	// roundPercent 返回百分制（0-100），前端按 0-1 比例展示，统一为分数（同本文件其它 accuracy 字段口径）
	if c7, t7 := s.countAllAccuracy(now.AddDate(0, 0, -7), now); t7 > 0 {
		sum.Accuracy7d = roundPercent(float64(c7), float64(t7)) / 100.0
	}
	if c30, t30 := s.countAllAccuracy(now.AddDate(0, 0, -30), now); t30 > 0 {
		sum.Accuracy30d = roundPercent(float64(c30), float64(t30)) / 100.0
	}
	if cTotal, tTotal := s.countAllAccuracy(time.Time{}, now); tTotal > 0 {
		sum.AccuracyTotal = roundPercent(float64(cTotal), float64(tTotal)) / 100.0
		sum.TotalEvaluated = tTotal
	}
	return sum, nil
}

// countAllAccuracy counts correct/total across all stocks for a time window.
func (s *EvaluationService) countAllAccuracy(since, until time.Time) (correct, total int) {
	q := s.db.Model(&models.PredictionAccuracy{})
	if !since.IsZero() {
		q = q.Where("evaluated_at >= ?", since)
	}
	q = q.Where("evaluated_at <= ?", until)

	var row struct {
		Total   int
		Correct int
	}
	if err := q.Select("COUNT(*) AS total, SUM(CASE WHEN is_correct THEN 1 ELSE 0 END) AS correct").
		Scan(&row).Error; err != nil {
		return 0, 0
	}
	return row.Correct, row.Total
}

// RiskAlert is a single risk signal for the dashboard.
type RiskAlert struct {
	Type    string    `json:"type"`
	Symbol  string    `json:"symbol,omitempty"`
	Message string    `json:"message"`
	Level   string    `json:"level"`
	Time    time.Time `json:"time,omitempty"`
}

// GetRiskAlerts returns recent risk signals: stocks with >=3 consecutive wrong
// predictions, plus recent simulated-trading risk events (last 7 days).
func (s *EvaluationService) GetRiskAlerts() ([]RiskAlert, error) {
	alerts := []RiskAlert{}
	if s.db == nil {
		return alerts, nil
	}

	// Continuous failures: >=3 consecutive wrong predictions per stock.
	var stocks []models.Stock
	_ = s.db.Where("is_active = ?", true).Find(&stocks).Error
	for _, st := range stocks {
		var accs []models.PredictionAccuracy
		if err := s.db.Where("stock_id = ?", st.ID).Order("evaluated_at DESC").Find(&accs).Error; err != nil {
			continue
		}
		consec := 0
		for _, a := range accs {
			if !a.IsCorrect {
				consec++
			} else {
				break
			}
		}
		if consec >= 3 {
			alerts = append(alerts, RiskAlert{
				Type:    "consecutive_failures",
				Symbol:  st.Symbol,
				Level:   "high",
				Message: fmt.Sprintf("%s(%s) 连续 %d 次预测失败", stockDisplayName(st), st.Symbol, consec),
			})
		}
	}

	// Recent sim risk events (last 7 days).
	sevenDaysAgo := time.Now().AddDate(0, 0, -7)
	var events []models.RiskEvent
	if err := s.db.Where("created_at >= ?", sevenDaysAgo).
		Order("created_at DESC").Limit(10).Find(&events).Error; err == nil {
		for _, ev := range events {
			alerts = append(alerts, RiskAlert{
				Type:    "risk_event",
				Message: ev.Message,
				Level:   "medium",
				Time:    ev.CreatedAt,
			})
		}
	}

	return alerts, nil
}

// ──────────────────────────────────────────────────────────────
// Thresholds & model health
// ──────────────────────────────────────────────────────────────

// EvalThresholds exposes the go/no-go thresholds to the dashboard.
type EvalThresholds struct {
	Suspend        int     `json:"suspend"`
	Retrain        int     `json:"retrain"`
	HighConfidence float64 `json:"high_confidence"`
}

// GetThresholds returns the current evaluation thresholds.
func (s *EvaluationService) GetThresholds() EvalThresholds {
	return EvalThresholds{
		Suspend:        s.suspendThreshold,
		Retrain:        s.retrainThreshold,
		HighConfidence: s.highConfidenceThreshold,
	}
}

// ModelHealth reports whether predictions should be suspended and whether a
// retrain is warranted, based on the configured thresholds.
type ModelHealth struct {
	Suspend                   bool   `json:"suspend"`
	Reason                    string `json:"reason"`
	ConsecutiveBelowThreshold int    `json:"consecutive_below_threshold"`
}

// GetModelHealth computes model health from 30d accuracy (suspend gate) and the
// number of consecutive days below the retrain threshold.
func (s *EvaluationService) GetModelHealth() *ModelHealth {
	h := &ModelHealth{}

	summary, _ := s.GetAccuracySummary()
	if summary != nil && summary.TotalEvaluated > 0 {
		if summary.Accuracy30d < float64(s.suspendThreshold) {
			h.Suspend = true
			h.Reason = fmt.Sprintf("30天准确率 %.1f%% 低于暂停阈值 %d%%，建议暂停信号展示",
				summary.Accuracy30d, s.suspendThreshold)
		}
	}

	trend, _ := s.GetAccuracyTrendData("all", 30)
	consec := 0
	for i := len(trend) - 1; i >= 0; i-- {
		if trend[i].Accuracy == nil {
			// 断点：当日无已评估预测，跳过（不中断连续计数）
			continue
		}
		if *trend[i].Accuracy < float64(s.retrainThreshold) {
			consec++
		} else {
			break
		}
	}
	h.ConsecutiveBelowThreshold = consec
	if consec >= 5 {
		reason := fmt.Sprintf("连续 %d 天准确率低于重训阈值 %d%%，建议触发重新训练", consec, s.retrainThreshold)
		if h.Reason != "" {
			h.Reason += "；" + reason
		} else {
			h.Reason = reason
		}
	}
	if h.Reason == "" {
		h.Reason = "模型健康"
	}
	return h
}

// LatestStockSync returns a heuristic for the latest stock sync timestamp
// (latest stocks.updated_at, falling back to sim_accounts.updated_at).
func (s *EvaluationService) LatestStockSync() *time.Time {
	if s.db == nil {
		return nil
	}
	var stock models.Stock
	if err := s.db.Order("updated_at DESC").First(&stock).Error; err == nil && !stock.UpdatedAt.IsZero() {
		t := stock.UpdatedAt
		return &t
	}
	var acc models.SimAccount
	if err := s.db.Order("updated_at DESC").First(&acc).Error; err == nil && !acc.UpdatedAt.IsZero() {
		t := acc.UpdatedAt
		return &t
	}
	return nil
}

// ──────────────────────────────────────────────────────────────
// Helpers
// ──────────────────────────────────────────────────────────────

func stockDisplayName(s models.Stock) string {
	if s.NameCN != "" {
		return s.NameCN
	}
	return s.Name
}

func directionLabel(d string) string {
	switch d {
	case "bullish", "up", "看涨":
		return "看涨"
	case "bearish", "down", "看跌":
		return "看跌"
	case "neutral", "flat", "震荡":
		return "震荡"
	default:
		if d == "" {
			return "未知"
		}
		return d
	}
}

func roundPercent(correct, total float64) float64 {
	return math.Round(correct/total*100*100) / 100
}

func parseFactors(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return nil
	}
	var items []FactorItem
	if err := json.Unmarshal(raw, &items); err != nil || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		label := it.Name
		if it.Value != 0 {
			label += fmt.Sprintf(": %.2f", it.Value)
		}
		if it.Description != "" {
			label += " (" + it.Description + ")"
		}
		out = append(out, label)
	}
	return out
}
