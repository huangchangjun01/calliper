package services

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/shopspring/decimal"
)

// YahooCollector implements MarketDataCollector for overseas stocks via Yahoo Finance.
type YahooCollector struct {
	marketCode   string
	mlServiceURL string
}

// NewYahooCollector creates a new Yahoo Finance collector.
func NewYahooCollector(mlServiceURL string) *YahooCollector {
	return &YahooCollector{
		marketCode:   "US",
		mlServiceURL: mlServiceURL,
	}
}

// GetMarketCode returns the market code for overseas stocks.
func (c *YahooCollector) GetMarketCode() string {
	return c.marketCode
}

// FetchRealTimeData fetches real-time overseas market data.
// Uses mock data when ml-service is unavailable.
func (c *YahooCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[Yahoo] Fetching real-time data for %d symbols (mock mode)", len(symbols))

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
			Volume:         rand.Int63n(80000000) + 2000000,
			Amount:         price.Mul(decimal.NewFromInt(rand.Int63n(800000000) + 20000000)),
			Change:         change,
			ChangePercent:  changePercent,
			TurnoverRate:   rand.Float64() * 3,
			PE:             15 + rand.Float64()*50,
			PB:             2 + rand.Float64()*15,
			TotalMarketCap: price.Mul(decimal.NewFromInt(rand.Int63n(50000000000) + 500000000)),
			FloatMarketCap: price.Mul(decimal.NewFromInt(rand.Int63n(25000000000) + 250000000)),
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

// FetchHistoricalData fetches historical overseas market data.
// Uses mock data when ml-service is unavailable.
func (c *YahooCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[Yahoo] Fetching historical data for %s from %s to %s interval=%s (mock mode)",
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
			Volume:         rand.Int63n(50000000) + 1000000,
			Amount:         close.Mul(decimal.NewFromInt(rand.Int63n(500000000) + 10000000)),
			Change:         change,
			ChangePercent:  changePercent,
			TurnoverRate:   rand.Float64() * 3,
			PE:             15 + rand.Float64()*50,
			PB:             2 + rand.Float64()*15,
			TotalMarketCap: close.Mul(decimal.NewFromInt(rand.Int63n(50000000000) + 500000000)),
			FloatMarketCap: close.Mul(decimal.NewFromInt(rand.Int63n(25000000000) + 250000000)),
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

func (c *YahooCollector) mockBasePrice(symbol string) decimal.Decimal {
	h := 0
	for _, ch := range symbol {
		h = h*31 + int(ch)
	}
	base := 50.0 + float64(h%3000)/10.0
	return decimal.NewFromFloat(base)
}

func (c *YahooCollector) mockName(symbol string) string {
	nameMap := map[string]string{
		"AAPL":  "Apple Inc.",
		"GOOGL": "Alphabet Inc.",
		"MSFT":  "Microsoft Corporation",
		"AMZN":  "Amazon.com, Inc.",
		"TSLA":  "Tesla, Inc.",
		"META":  "Meta Platforms, Inc.",
		"NVDA":  "NVIDIA Corporation",
		"JPM":   "JPMorgan Chase & Co.",
		"V":     "Visa Inc.",
		"JNJ":   "Johnson & Johnson",
		"BABA":  "Alibaba Group",
		"0700.HK": "Tencent Holdings",
		"9988.HK": "Alibaba Group Holding",
	}
	if name, ok := nameMap[symbol]; ok {
		return name
	}
	return fmt.Sprintf("Stock %s", symbol)
}

func (c *YahooCollector) mockBidPrices(price decimal.Decimal) []decimal.Decimal {
	bidPrices := make([]decimal.Decimal, 5)
	for i := 0; i < 5; i++ {
		offset := decimal.NewFromFloat(float64(i+1) * 0.05)
		bidPrices[i] = price.Sub(offset)
	}
	return bidPrices
}

func (c *YahooCollector) mockAskPrices(price decimal.Decimal) []decimal.Decimal {
	askPrices := make([]decimal.Decimal, 5)
	for i := 0; i < 5; i++ {
		offset := decimal.NewFromFloat(float64(i+1) * 0.05)
		askPrices[i] = price.Add(offset)
	}
	return askPrices
}

func (c *YahooCollector) mockVolumes() []int64 {
	volumes := make([]int64, 5)
	for i := 0; i < 5; i++ {
		volumes[i] = rand.Int63n(50000) + 500
	}
	return volumes
}

func (c *YahooCollector) intervalStep(interval string) time.Duration {
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