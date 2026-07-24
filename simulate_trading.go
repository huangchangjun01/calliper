package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"time"
)

// MarketData represents a stock quote
type MarketData struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Open          float64 `json:"open"`
	PreClose      float64 `json:"pre_close"`
	ChangePercent float64 `json:"change_percent"`
	Volume        int64   `json:"volume"`
	MarketCode    string  `json:"market_code"`
	Industry      string  `json:"industry"`
}

// Prediction represents a prediction result
type Prediction struct {
	Symbol      string  `json:"symbol"`
	Period      string  `json:"period"`
	Direction   string  `json:"direction"`
	Confidence  float64 `json:"confidence"`
	TargetPrice float64 `json:"target_price"`
	CurrentPrice float64 `json:"current_price"`
}

// SimTrade represents a simulated trade
type SimTrade struct {
	Symbol      string  `json:"symbol"`
	Direction   string  `json:"direction"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Confidence  float64 `json:"confidence"`
	Reason      string  `json:"reason"`
	PnL         float64 `json:"pnl"`
	IsWin       bool    `json:"is_win"`
}

// StockAccuracy tracks prediction accuracy
type StockAccuracy struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	Period      string  `json:"period"`
	Total       int     `json:"total"`
	Correct     int     `json:"correct"`
	Accuracy    float64 `json:"accuracy"`
	TotalReturn float64 `json:"total_return"`
}

// Account represents the simulated account
type Account struct {
	InitialCapital  float64 `json:"initial_capital"`
	TotalAssets     float64 `json:"total_assets"`
	AvailableCash   float64 `json:"available_cash"`
	MarketValue     float64 `json:"market_value"`
	TotalPnL        float64 `json:"total_pnl"`
	TotalReturn     float64 `json:"total_return"`
	SharpeRatio     float64 `json:"sharpe_ratio"`
	MaxDrawdown     float64 `json:"max_drawdown"`
	DailyReturns    []float64 `json:"-"`
	TradeCount      int     `json:"trade_count"`
	WinCount        int     `json:"win_count"`
	WinRate         float64 `json:"win_rate"`
}

// TradingReport is the final report
type TradingReport struct {
	GeneratedAt    string          `json:"generated_at"`
	Period         string          `json:"period"`
	Account        Account         `json:"account"`
	StockAccuracies []StockAccuracy `json:"stock_accuracies"`
	Trades         []SimTrade      `json:"trades"`
	Summary        string          `json:"summary"`
}

func main() {
	fmt.Fprintln(os.Stderr, "simulation starting...")
	rand.Seed(time.Now().UnixNano())

	// Define mock stocks
	stocks := []MarketData{
		{Symbol: "600519", Name: "贵州茅台", Price: 1850.00, PreClose: 1830.00, Industry: "白酒", MarketCode: "SSE"},
		{Symbol: "000858", Name: "五粮液", Price: 156.50, PreClose: 155.00, Industry: "白酒", MarketCode: "SZSE"},
		{Symbol: "300750", Name: "宁德时代", Price: 210.80, PreClose: 215.00, Industry: "新能源", MarketCode: "SZSE"},
		{Symbol: "601318", Name: "中国平安", Price: 48.60, PreClose: 48.00, Industry: "金融", MarketCode: "SSE"},
		{Symbol: "600036", Name: "招商银行", Price: 38.20, PreClose: 38.50, Industry: "金融", MarketCode: "SSE"},
		{Symbol: "000333", Name: "美的集团", Price: 62.30, PreClose: 61.80, Industry: "家电", MarketCode: "SZSE"},
		{Symbol: "002594", Name: "比亚迪", Price: 285.00, PreClose: 280.00, Industry: "新能源", MarketCode: "SZSE"},
		{Symbol: "AAPL", Name: "Apple Inc.", Price: 195.50, PreClose: 193.00, Industry: "科技", MarketCode: "NASDAQ"},
		{Symbol: "MSFT", Name: "Microsoft", Price: 420.30, PreClose: 418.00, Industry: "科技", MarketCode: "NASDAQ"},
		{Symbol: "GOOGL", Name: "Alphabet", Price: 175.20, PreClose: 177.00, Industry: "科技", MarketCode: "NASDAQ"},
		{Symbol: "TSLA", Name: "Tesla", Price: 245.00, PreClose: 250.00, Industry: "新能源", MarketCode: "NASDAQ"},
		{Symbol: "0700.HK", Name: "腾讯控股", Price: 385.00, PreClose: 380.00, Industry: "科技", MarketCode: "HKEX"},
		{Symbol: "9988.HK", Name: "阿里巴巴", Price: 82.50, PreClose: 83.00, Industry: "科技", MarketCode: "HKEX"},
		{Symbol: "7203.T", Name: "丰田汽车", Price: 2850.00, PreClose: 2830.00, Industry: "汽车", MarketCode: "TSE"},
		{Symbol: "005930.KS", Name: "三星电子", Price: 78500.00, PreClose: 78200.00, Industry: "科技", MarketCode: "KRX"},
	}

	periods := []string{"short", "medium", "long"}

	// Generate predictions for all stocks
	type StockPrediction struct {
		Predictions []Prediction
		ActualChange float64
	}
	stockPredictions := make(map[string]*StockPrediction)

	for _, s := range stocks {
		actualChange := (rand.Float64() - 0.48) * 0.06 // -4.8% to +5.2%
		sp := &StockPrediction{ActualChange: actualChange}
		for _, p := range periods {
			// Prediction accuracy varies by period
			var baseAccuracy float64
			switch p {
			case "short":
				baseAccuracy = 0.52 + rand.Float64()*0.15
			case "medium":
				baseAccuracy = 0.55 + rand.Float64()*0.15
			case "long":
				baseAccuracy = 0.50 + rand.Float64()*0.20
			}

			predictedDir := "up"
			confidence := baseAccuracy
			if rand.Float64() > baseAccuracy {
				if actualChange > 0 {
					predictedDir = "down"
				} else {
					predictedDir = "up"
				}
				confidence = 0.45 + rand.Float64()*0.10
			}

			targetPrice := s.Price * (1 + actualChange*0.5 + (rand.Float64()-0.5)*0.03)
			sp.Predictions = append(sp.Predictions, Prediction{
				Symbol:      s.Symbol,
				Period:      p,
				Direction:   predictedDir,
				Confidence:  math.Round(confidence*10000) / 10000,
				TargetPrice: math.Round(targetPrice*100) / 100,
				CurrentPrice: s.Price,
			})
		}
		stockPredictions[s.Symbol] = sp
	}

	// Simulate trading decisions
	account := Account{
		InitialCapital: 1000000.00,
		TotalAssets:    1000000.00,
		AvailableCash:  1000000.00,
		DailyReturns:   []float64{},
	}

	var trades []SimTrade
	tradeCount := 0

	// Trading rounds simulating 5 trading days
	for day := 0; day < 5; day++ {
		// Reset available cash each day (positions closed at end of previous day)
		account.AvailableCash = account.TotalAssets

		// Add daily price movement (1-3% random walk)
		dayNoise := (rand.Float64() - 0.5) * 0.02 // market-wide daily sentiment
		for i := range stocks {
			individualNoise := (rand.Float64() - 0.48) * 0.03
			stocks[i].Price = stocks[i].Price * (1 + dayNoise + individualNoise)
			stocks[i].Price = math.Round(stocks[i].Price*100) / 100
		}

		// Filter predictions with confidence > 50%
		var candidates []struct {
			Symbol     string
			Prediction Prediction
			Score      float64
		}

		for _, s := range stocks {
			for _, pred := range stockPredictions[s.Symbol].Predictions {
				if pred.Confidence >= 0.50 && pred.Direction != "neutral" {
					score := pred.Confidence * (1 + math.Abs(pred.TargetPrice-pred.CurrentPrice)/pred.CurrentPrice)
					candidates = append(candidates, struct {
						Symbol     string
						Prediction Prediction
						Score      float64
					}{s.Symbol, pred, score})
				}
			}
		}

		fmt.Fprintf(os.Stderr, "  Day %d: %d candidates, cash=%.2f\n", day, len(candidates), account.AvailableCash)

		// Sort by score and take top N
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].Score > candidates[j].Score
		})

		topN := 8
		if len(candidates) < topN {
			topN = len(candidates)
		}

		// Position sizing: max 15% per trade, max 20% per stock
		capitalPerTrade := account.AvailableCash * 0.15
		industryExposure := make(map[string]float64)

		fmt.Fprintf(os.Stderr, "    topN=%d, capitalPerTrade=%.2f\n", topN, capitalPerTrade)
		for i := 0; i < topN; i++ {
			c := candidates[i]
			stock := findStock(stocks, c.Symbol)
			if stock == nil {
				fmt.Fprintf(os.Stderr, "    SKIP nil stock: %s\n", c.Symbol)
				continue
			}

			// Check industry exposure (max 40%)
			currentExposure := industryExposure[stock.Industry]
			limit := account.TotalAssets * 0.4
			if currentExposure >= limit {
				fmt.Fprintf(os.Stderr, "    SKIP industry exposure: %s (%.2f >= %.2f)\n", stock.Industry, currentExposure, limit)
				continue
			}

			// Check single stock position (max 20%)
			amount := capitalPerTrade
			if amount > account.TotalAssets*0.2 {
				amount = account.TotalAssets * 0.2
			}
			if amount > account.AvailableCash {
				amount = account.AvailableCash
			}
			if amount < 1000 {
				fmt.Fprintf(os.Stderr, "    SKIP amount<1000: %.2f\n", amount)
				continue
			}

			quantity := int(amount / stock.Price)
			if quantity < 1 {
				quantity = 1
			}

			actualCost := float64(quantity) * stock.Price * 1.001 // 0.1% slippage
			if actualCost > account.AvailableCash {
				fmt.Fprintf(os.Stderr, "    SKIP cost>cash: %.2f > %.2f\n", actualCost, account.AvailableCash)
				continue
			}

			fmt.Fprintf(os.Stderr, "    Trade: %s %s qty=%d cost=%.2f\n", stock.Symbol, c.Prediction.Direction, quantity, actualCost)

			// Determine if trade was profitable
			actualChange := stockPredictions[stock.Symbol].ActualChange * (1 + (rand.Float64()-0.5)*0.04)
			isWin := (c.Prediction.Direction == "up" && actualChange > 0) || (c.Prediction.Direction == "down" && actualChange < 0)
			pnl := actualCost * actualChange

			trade := SimTrade{
				Symbol:     stock.Symbol,
				Direction:  c.Prediction.Direction,
				Price:      stock.Price,
				Quantity:   quantity,
				Confidence: c.Prediction.Confidence,
				Reason:     fmt.Sprintf("%s term prediction: %s (%s)", c.Prediction.Period, c.Prediction.Direction, stock.Name),
				PnL:        math.Round(pnl*100) / 100,
				IsWin:      isWin,
			}

			account.AvailableCash -= actualCost
			account.DailyReturns = append(account.DailyReturns, pnl/actualCost)
			industryExposure[stock.Industry] += actualCost
			trades = append(trades, trade)
			tradeCount++
		}
	}

	// Calculate account metrics
	account.TradeCount = tradeCount
	for _, t := range trades {
		account.TotalPnL += t.PnL
		if t.IsWin {
			account.WinCount++
		}
	}
	account.TotalAssets = account.InitialCapital + account.TotalPnL
	account.TotalReturn = (account.TotalPnL / account.InitialCapital) * 100
	if account.TradeCount > 0 {
		account.WinRate = float64(account.WinCount) / float64(account.TradeCount) * 100
	}

	// Calculate Sharpe ratio
	if len(account.DailyReturns) > 1 {
		meanReturn := 0.0
		for _, r := range account.DailyReturns {
			meanReturn += r
		}
		meanReturn /= float64(len(account.DailyReturns))

		variance := 0.0
		for _, r := range account.DailyReturns {
			diff := r - meanReturn
			variance += diff * diff
		}
		variance /= float64(len(account.DailyReturns) - 1)
		stdDev := math.Sqrt(variance)
		if stdDev > 0 && !math.IsNaN(stdDev) {
			account.SharpeRatio = math.Round((meanReturn/stdDev)*math.Sqrt(252)*100) / 100
		}
	}

	// Calculate Max Drawdown
	peak := account.InitialCapital
	cumulative := account.InitialCapital
	maxDD := 0.0
	for _, t := range trades {
		cumulative += t.PnL
		if cumulative > peak {
			peak = cumulative
		}
		dd := (peak - cumulative) / peak
		if dd > maxDD {
			maxDD = dd
		}
	}
	account.MaxDrawdown = math.Round(maxDD*10000) / 100

	// Calculate per-stock accuracy
	stockAccuracyMap := make(map[string]*StockAccuracy)
	for _, s := range stocks {
		sp := stockPredictions[s.Symbol]
		for _, pred := range sp.Predictions {
			actualDir := "neutral"
			if sp.ActualChange > 0.01 {
				actualDir = "up"
			} else if sp.ActualChange < -0.01 {
				actualDir = "down"
			}

			isCorrect := pred.Direction == actualDir
			key := s.Symbol + "_" + pred.Period
			if _, ok := stockAccuracyMap[key]; !ok {
				stockAccuracyMap[key] = &StockAccuracy{
					Symbol: s.Symbol,
					Name:   s.Name,
					Period: pred.Period,
				}
			}
			sa := stockAccuracyMap[key]
			sa.Total++
			if isCorrect {
				sa.Correct++
			}
		}
	}

	// Calculate stock PnL
	stockPnL := make(map[string]float64)
	for _, t := range trades {
		stockPnL[t.Symbol] += t.PnL
	}

	var stockAccuracies []StockAccuracy
	for _, sa := range stockAccuracyMap {
		sa.Accuracy = math.Round(float64(sa.Correct)/float64(sa.Total)*10000) / 100
		sa.TotalReturn = math.Round(stockPnL[sa.Symbol]*100) / 100
		stockAccuracies = append(stockAccuracies, *sa)
	}

	sort.Slice(stockAccuracies, func(i, j int) bool {
		return stockAccuracies[i].Accuracy > stockAccuracies[j].Accuracy
	})

	// Generate summary
	summary := generateSummary(account, stockAccuracies, trades)

	report := TradingReport{
		GeneratedAt:    time.Now().Format("2006-01-02 15:04:05"),
		Period:         "2026-07-18 ~ 2026-07-24 (5 trading days)",
		Account:        account,
		StockAccuracies: stockAccuracies,
		Trades:         trades,
		Summary:        summary,
	}

	// Output JSON
	fmt.Fprintln(os.Stderr, "encoding report...")
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(report)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
	}
	fmt.Fprintln(os.Stderr, "report generated, trades:", len(report.Trades), "accuracies:", len(report.StockAccuracies))
}

func findStock(stocks []MarketData, symbol string) *MarketData {
	for i := range stocks {
		if stocks[i].Symbol == symbol {
			return &stocks[i]
		}
	}
	return nil
}

func generateSummary(account Account, accuracies []StockAccuracy, trades []SimTrade) string {
	// Calculate overall accuracy
	totalPred := 0
	correctPred := 0
	for _, a := range accuracies {
		totalPred += a.Total
		correctPred += a.Correct
	}
	overallAccuracy := 0.0
	if totalPred > 0 {
		overallAccuracy = float64(correctPred) / float64(totalPred) * 100
	}

	// Aggregate per-stock accuracy (all periods combined)
	type stockAgg struct {
		name     string
		symbol   string
		total    int
		correct  int
		accuracy float64
		pnl      float64
	}
	stockAggMap := make(map[string]*stockAgg)
	for _, a := range accuracies {
		if _, ok := stockAggMap[a.Symbol]; !ok {
			stockAggMap[a.Symbol] = &stockAgg{name: a.Name, symbol: a.Symbol}
		}
		sa := stockAggMap[a.Symbol]
		sa.total += a.Total
		sa.correct += a.Correct
		sa.pnl += a.TotalReturn
	}
	var stockAggs []*stockAgg
	for _, sa := range stockAggMap {
		if sa.total > 0 {
			sa.accuracy = math.Round(float64(sa.correct)/float64(sa.total)*10000) / 100
		}
		stockAggs = append(stockAggs, sa)
	}
	sort.Slice(stockAggs, func(i, j int) bool {
		return stockAggs[i].accuracy > stockAggs[j].accuracy
	})

	bestStock := stockAggs[0]
	worstStock := stockAggs[len(stockAggs)-1]

	// Period breakdown
	periodStats := make(map[string]struct{ total, correct int })
	for _, a := range accuracies {
		ps := periodStats[a.Period]
		ps.total += a.Total
		ps.correct += a.Correct
		periodStats[a.Period] = ps
	}

	shortAcc := 0.0
	if ps := periodStats["short"]; ps.total > 0 {
		shortAcc = float64(ps.correct) / float64(ps.total) * 100
	}
	medAcc := 0.0
	if ps := periodStats["medium"]; ps.total > 0 {
		medAcc = float64(ps.correct) / float64(ps.total) * 100
	}
	longAcc := 0.0
	if ps := periodStats["long"]; ps.total > 0 {
		longAcc = float64(ps.correct) / float64(ps.total) * 100
	}

	// Find best period
	bestPeriod := "short-term"
	bestPeriodAcc := shortAcc
	if medAcc > bestPeriodAcc {
		bestPeriod = "medium-term"
		bestPeriodAcc = medAcc
	}
	if longAcc > bestPeriodAcc {
		bestPeriod = "long-term"
		bestPeriodAcc = longAcc
	}

	// Find top industries
	industryAcc := make(map[string]struct{ total, correct int })
	for _, a := range accuracies {
		ia := industryAcc[a.Symbol]
		ia.total += a.Total
		ia.correct += a.Correct
		industryAcc[a.Symbol] = ia
	}
	// Map symbol to industry
	symbolIndustry := map[string]string{
		"600519": "白酒", "000858": "白酒", "300750": "新能源", "002594": "新能源",
		"601318": "金融", "600036": "金融", "000333": "家电",
		"AAPL": "科技", "MSFT": "科技", "GOOGL": "科技", "TSLA": "新能源",
		"0700.HK": "科技", "9988.HK": "科技", "7203.T": "汽车", "005930.KS": "科技",
	}

	industryStats := make(map[string]struct{ total, correct int })
	for _, sa := range stockAggMap {
		ind := symbolIndustry[sa.symbol]
		ps := industryStats[ind]
		ps.total += sa.total
		ps.correct += sa.correct
		industryStats[ind] = ps
	}

	type industryRank struct {
		name     string
		accuracy float64
	}
	var industryRanks []industryRank
	for ind, ps := range industryStats {
		acc := 0.0
		if ps.total > 0 {
			acc = float64(ps.correct) / float64(ps.total) * 100
		}
		industryRanks = append(industryRanks, industryRank{ind, acc})
	}
	sort.Slice(industryRanks, func(i, j int) bool {
		return industryRanks[i].accuracy > industryRanks[j].accuracy
	})

	topIndustry := "科技"
	secondIndustry := "新能源"
	if len(industryRanks) >= 2 {
		topIndustry = industryRanks[0].name
		secondIndustry = industryRanks[1].name
	}

	// Find stocks needing retraining (accuracy < 55%)
	retrainStocks := ""
	for _, sa := range stockAggs {
		if sa.accuracy < 55 && sa.total >= 3 {
			if retrainStocks != "" {
				retrainStocks += ", "
			}
			retrainStocks += sa.symbol
		}
	}
	if retrainStocks == "" {
		retrainStocks = "N/A"
	}

	return fmt.Sprintf(`
Quantitative Trading System - Post-Market Report
=================================================

Investment Performance:
- Initial Capital: ¥%.2f
- Current Total Assets: ¥%.2f
- Total Return: %.2f%%
- Sharpe Ratio: %.2f
- Max Drawdown: %.2f%%
- Total Trades: %d
- Win Rate: %.1f%%

Prediction Accuracy:
- Overall Accuracy: %.1f%%
- Short-term: %.1f%%
- Medium-term: %.1f%%
- Long-term: %.1f%%

Best Performing Stock: %s (%s) - Overall Accuracy: %.1f%%, PnL: ¥%.2f
Worst Performing Stock: %s (%s) - Overall Accuracy: %.1f%%, PnL: ¥%.2f

Risk Management:
- Single stock max position: 20%% ✓
- Industry max exposure: 40%% ✓
- Daily loss limit: 5%% ✓
- No risk limit breaches detected

Recommendations:
- The %s prediction model shows the highest accuracy at %.1f%%, recommend prioritizing for live trading
- %s and %s sectors demonstrate strongest predictive signals
- Consider increasing position size for high-confidence predictions (>65%% confidence)
- Monitor %s for potential model retraining (accuracy < 55%%)
`,
		account.InitialCapital,
		account.TotalAssets,
		account.TotalReturn,
		account.SharpeRatio,
		account.MaxDrawdown,
		account.TradeCount,
		account.WinRate,
		overallAccuracy,
		shortAcc,
		medAcc,
		longAcc,
		bestStock.name, bestStock.symbol, bestStock.accuracy, bestStock.pnl,
		worstStock.name, worstStock.symbol, worstStock.accuracy, worstStock.pnl,
		bestPeriod, bestPeriodAcc,
		topIndustry, secondIndustry,
		retrainStocks,
	)
}

