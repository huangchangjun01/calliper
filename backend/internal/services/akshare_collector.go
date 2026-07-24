package services

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/shopspring/decimal"
)

// AKShareCollector implements MarketDataCollector for A-share (Chinese) stocks.
type AKShareCollector struct {
	marketCode string
	mlServiceURL string
}

// NewAKShareCollector creates a new AKShare collector.
func NewAKShareCollector(mlServiceURL string) *AKShareCollector {
	return &AKShareCollector{
		marketCode:    "CN",
		mlServiceURL: mlServiceURL,
	}
}

// GetMarketCode returns the market code for A-share stocks.
func (c *AKShareCollector) GetMarketCode() string {
	return c.marketCode
}

// FetchRealTimeData fetches real-time A-share market data.
// Uses mock data when ml-service is unavailable.
func (c *AKShareCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[AKShare] Fetching real-time data for %d symbols (mock mode)", len(symbols))

	var result []MarketData
	now := time.Now()

	for _, symbol := range symbols {
		basePrice := c.mockBasePrice(symbol)
		price := basePrice.Mul(decimal.NewFromFloat(1 + (rand.Float64()-0.5)*0.06))
		preClose := basePrice
		change := price.Sub(preClose)
		changePercent, _ := price.Sub(preClose).Div(preClose).Mul(decimal.NewFromInt(100)).Float64()

		md := MarketData{
			Symbol:         symbol,
			Name:           c.mockName(symbol),
			Price:          price,
			Open:           basePrice.Mul(decimal.NewFromFloat(1 + (rand.Float64()-0.5)*0.03)),
			High:           price.Mul(decimal.NewFromFloat(1.02)),
			Low:            price.Mul(decimal.NewFromFloat(0.98)),
			PreClose:       preClose,
			Volume:         rand.Int63n(50000000) + 1000000,
			Amount:         price.Mul(decimal.NewFromInt(rand.Int63n(500000000) + 10000000)),
			Change:         change,
			ChangePercent:  changePercent,
			TurnoverRate:   rand.Float64() * 5,
			PE:             10 + rand.Float64()*40,
			PB:             1 + rand.Float64()*10,
			TotalMarketCap: price.Mul(decimal.NewFromInt(rand.Int63n(10000000000) + 100000000)),
			FloatMarketCap: price.Mul(decimal.NewFromInt(rand.Int63n(5000000000) + 50000000)),
			BidPrices:      c.mockBidPrices(price),
			BidVolumes:     c.mockVolumes(),
			AskPrices:      c.mockAskPrices(price),
			AskVolumes:     c.mockVolumes(),
			Timestamp:      now,
			MarketCode:     c.marketCode,
		}

		result = append(result, md)
	}

	return result, nil
}

// FetchHistoricalData fetches historical A-share data.
// Uses mock data when ml-service is unavailable.
func (c *AKShareCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[AKShare] Fetching historical data for %s from %s to %s interval=%s (mock mode)",
		symbol, start.Format("2006-01-02"), end.Format("2006-01-02"), interval)

	var result []MarketData
	basePrice := c.mockBasePrice(symbol)

	current := start
	for current.Before(end) || current.Equal(end) {
		step := c.intervalStep(interval)
		// Skip weekends for daily data
		if interval == "1d" && (current.Weekday() == time.Saturday || current.Weekday() == time.Sunday) {
			current = current.Add(step)
			continue
		}

		fluctuation := (rand.Float64() - 0.5) * 0.06
		open := basePrice
		close := basePrice.Mul(decimal.NewFromFloat(1 + fluctuation))
		high := open
		low := close
		if close.GreaterThan(open) {
			high = close.Mul(decimal.NewFromFloat(1.01))
			low = open.Mul(decimal.NewFromFloat(0.99))
		} else {
			high = open.Mul(decimal.NewFromFloat(1.01))
			low = close.Mul(decimal.NewFromFloat(0.99))
		}

		preClose := basePrice
		change := close.Sub(preClose)
		changePercent, _ := change.Div(preClose).Mul(decimal.NewFromInt(100)).Float64()

		md := MarketData{
			Symbol:         symbol,
			Name:           c.mockName(symbol),
			Price:          close,
			Open:           open,
			High:           high,
			Low:            low,
			PreClose:       preClose,
			Volume:         rand.Int63n(30000000) + 500000,
			Amount:         close.Mul(decimal.NewFromInt(rand.Int63n(300000000) + 5000000)),
			Change:         change,
			ChangePercent:  changePercent,
			TurnoverRate:   rand.Float64() * 5,
			PE:             10 + rand.Float64()*40,
			PB:             1 + rand.Float64()*10,
			TotalMarketCap: close.Mul(decimal.NewFromInt(rand.Int63n(10000000000) + 100000000)),
			FloatMarketCap: close.Mul(decimal.NewFromInt(rand.Int63n(5000000000) + 50000000)),
			BidPrices:      c.mockBidPrices(close),
			BidVolumes:     c.mockVolumes(),
			AskPrices:      c.mockAskPrices(close),
			AskVolumes:     c.mockVolumes(),
			Timestamp:      current,
			MarketCode:     c.marketCode,
		}

		result = append(result, md)
		basePrice = close
		current = current.Add(step)
	}

	return result, nil
}

func (c *AKShareCollector) mockBasePrice(symbol string) decimal.Decimal {
	// Deterministic base price based on symbol hash
	h := 0
	for _, ch := range symbol {
		h = h*31 + int(ch)
	}
	base := 10.0 + float64(h%1000)/10.0
	return decimal.NewFromFloat(base)
}

func (c *AKShareCollector) mockName(symbol string) string {
	nameMap := map[string]string{
		"000001": "平安银行",
		"000002": "万科A",
		"600000": "浦发银行",
		"600036": "招商银行",
		"600519": "贵州茅台",
		"000858": "五粮液",
		"300750": "宁德时代",
		"002415": "海康威视",
		"601318": "中国平安",
		"600276": "恒瑞医药",
	}
	if name, ok := nameMap[symbol]; ok {
		return name
	}
	return fmt.Sprintf("股票%s", symbol)
}

func (c *AKShareCollector) mockBidPrices(price decimal.Decimal) []decimal.Decimal {
	bidPrices := make([]decimal.Decimal, 5)
	for i := 0; i < 5; i++ {
		offset := decimal.NewFromFloat(float64(i+1) * 0.01)
		bidPrices[i] = price.Sub(offset)
	}
	return bidPrices
}

func (c *AKShareCollector) mockAskPrices(price decimal.Decimal) []decimal.Decimal {
	askPrices := make([]decimal.Decimal, 5)
	for i := 0; i < 5; i++ {
		offset := decimal.NewFromFloat(float64(i+1) * 0.01)
		askPrices[i] = price.Add(offset)
	}
	return askPrices
}

func (c *AKShareCollector) mockVolumes() []int64 {
	volumes := make([]int64, 5)
	for i := 0; i < 5; i++ {
		volumes[i] = rand.Int63n(100000) + 1000
	}
	return volumes
}

func (c *AKShareCollector) intervalStep(interval string) time.Duration {
	switch interval {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "15m":
		return 15 * time.Minute
	case "30m":
		return 30 * time.Minute
	case "60m", "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}