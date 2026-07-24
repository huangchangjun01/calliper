package services

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// ──────────────────────────────────────────────────────────────
// EvaluationService — 预测成功率自评估服务
// ──────────────────────────────────────────────────────────────

// EvaluationService evaluates prediction accuracy and computes
// risk / return metrics for the quantitative trading system.
type EvaluationService struct {
	db   *gorm.DB
	tsdb *gorm.DB
}

// NewEvaluationService creates a new EvaluationService.
func NewEvaluationService(db, tsdb *gorm.DB) *EvaluationService {
	return &EvaluationService{db: db, tsdb: tsdb}
}

// ──────────────────────────────────────────────────────────────
// Response types
// ──────────────────────────────────────────────────────────────

// AccuracyStats holds multi-horizon accuracy statistics.
type AccuracyStats struct {
	Symbol             string                  `json:"symbol"`
	Accuracy7d         float64                 `json:"accuracy_7d"`
	Accuracy30d        float64                 `json:"accuracy_30d"`
	AccuracyTotal      float64                 `json:"accuracy_total"`
	TotalPredictions   int                     `json:"total_predictions"`
	ByPeriod           map[string]PeriodStats  `json:"by_period"`
}

// PeriodStats holds accuracy for a single prediction period.
type PeriodStats struct {
	Period         string  `json:"period"`
	Accuracy7d     float64 `json:"accuracy_7d"`
	Accuracy30d    float64 `json:"accuracy_30d"`
	AccuracyTotal  float64 `json:"accuracy_total"`
	CorrectCount   int     `json:"correct_count"`
	TotalCount     int     `json:"total_count"`
}

// StockAccuracy holds per-stock accuracy ranking data.
type StockAccuracy struct {
	Symbol         string  `json:"symbol"`
	Accuracy       float64 `json:"accuracy"`
	TotalPredictions int   `json:"total_predictions"`
}

// EvaluationMetrics holds composite risk / return metrics.
type EvaluationMetrics struct {
	Symbol        string  `json:"symbol"`
	ExcessReturn  float64 `json:"excess_return"`
	SharpeRatio   float64 `json:"sharpe_ratio"`
	MaxDrawdown   float64 `json:"max_drawdown"`
}

// FailureAnalysis holds attribution data for a failed prediction.
type FailureAnalysis struct {
	PredictionID    uint   `json:"prediction_id"`
	Symbol          string `json:"symbol"`
	PredictedDirection string `json:"predicted_direction"`
	ActualDirection string `json:"actual_direction"`
	IsEarningsSeason bool  `json:"is_earnings_season"`
	IndustryAnomaly bool   `json:"industry_anomaly"`
	VolumeAnomaly   bool   `json:"volume_anomaly"`
	Summary         string `json:"summary"`
}

// ──────────────────────────────────────────────────────────────
// Daily evaluation
// ──────────────────────────────────────────────────────────────

// EvaluateDaily evaluates all predictions made yesterday against actual
// market data and writes accuracy records.
func (s *EvaluationService) EvaluateDaily(ctx context.Context) error {
	yesterday := time.Now().AddDate(0, 0, -1)
	startOfDay := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	// 1. 获取昨日所有预测记录
	var predictions []models.Prediction
	if s.db != nil {
		if err := s.db.WithContext(ctx).
			Where("predicted_at >= ? AND predicted_at < ?", startOfDay, endOfDay).
			Find(&predictions).Error; err != nil {
			return fmt.Errorf("failed to query predictions: %w", err)
		}
	}

	if len(predictions) == 0 {
		log.Printf("[Evaluation] No predictions found for %s, skipping evaluation", yesterday.Format("2006-01-02"))
		return nil
	}

	// 2. 获取昨日实际行情数据
	for _, pred := range predictions {
		actualChange := s.getActualChange(ctx, pred.StockID, startOfDay, endOfDay)

		// 3. 对比预测方向与实际涨跌方向
		isCorrect := s.judgePrediction(pred.Direction, actualChange)

		actualDirection := s.directionFromChange(actualChange)

		// 4. 写入 prediction_accuracies 表
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

		// 5. 更新预测记录的 success 字段
		if s.db != nil {
			success := isCorrect
			_ = s.db.WithContext(ctx).Model(&pred).Update("success", success)
		}
	}

	return nil
}

// judgePrediction compares predicted direction with actual price change.
func (s *EvaluationService) judgePrediction(direction string, actualChange float64) bool {
	switch direction {
	case "bullish", "看涨", "up":
		return actualChange > 0
	case "bearish", "看跌", "down":
		return actualChange < 0
	case "neutral", "震荡", "flat":
		return math.Abs(actualChange) <= 1.0
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

// getActualChange retrieves yesterday's price change for a stock.
func (s *EvaluationService) getActualChange(ctx context.Context, stockID uint, start, end time.Time) float64 {
	if s.tsdb == nil {
		return 0
	}
	var daily models.StockPriceDaily
	err := s.tsdb.WithContext(ctx).
		Where("stock_id = ? AND time >= ? AND time < ?", stockID, start, end).
		Order("time DESC").
		First(&daily).Error
	if err == nil && daily.Close > 0 && daily.Open > 0 {
		return (daily.Close - daily.Open) / daily.Open * 100
	}
	return 0
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
			Period:        period,
			CorrectCount:  correctTotal,
			TotalCount:    totalTotal,
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

	var rankings []StockAccuracy
	now := time.Now()
	thirtyDaysAgo := now.AddDate(0, 0, -30)

	for _, stock := range stocks {
		var accuracies []models.PredictionAccuracy
		query := s.db.Where("stock_id = ?", stock.ID)
		if period != "" && period != "all" {
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
		accuracy := math.Round(float64(correct)/float64(len(accuracies))*10000) / 100
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
		ActualDirection:    "bearish",
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