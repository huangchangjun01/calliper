package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/shopspring/decimal"
)

// YahooCollector implements MarketDataCollector for overseas stocks via Yahoo Finance.
type YahooCollector struct {
	marketCode   string
	mlServiceURL string
	httpClient   *http.Client
	mu           sync.Mutex
	lastRequest  time.Time
	minInterval  time.Duration
}

// NewYahooCollector creates a new Yahoo Finance collector.
func NewYahooCollector(mlServiceURL string) *YahooCollector {
	return &YahooCollector{
		marketCode:   "US",
		mlServiceURL: mlServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		minInterval: 200 * time.Millisecond,
	}
}

// GetMarketCode returns the market code for overseas stocks.
func (c *YahooCollector) GetMarketCode() string {
	return c.marketCode
}

// rateLimit blocks until the minimum interval since the last request has elapsed.
func (c *YahooCollector) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.lastRequest)
	if elapsed < c.minInterval {
		time.Sleep(c.minInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// yahooChartResponse represents the Yahoo Finance v8 chart API response.
type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol             string  `json:"symbol"`
				RegularMarketPrice float64 `json:"regularMarketPrice"`
				PreviousClose      float64 `json:"previousClose"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Open   []float64 `json:"open"`
					High   []float64 `json:"high"`
					Low    []float64 `json:"low"`
					Close  []float64 `json:"close"`
					Volume []int64   `json:"volume"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error interface{} `json:"error"`
	} `json:"chart"`
}

// FetchRealTimeData fetches real-time overseas market data from Yahoo Finance.
func (c *YahooCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[Yahoo] Fetching real-time data for %d symbols from Yahoo Finance", len(symbols))

	var result []MarketData
	now := time.Now()

	for _, symbol := range symbols {
		c.rateLimit()

		url := fmt.Sprintf("https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=1m&range=1d", symbol)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("[Yahoo] Request error for %s: %v", symbol, err)
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			log.Printf("[Yahoo] HTTP error for %s: %v", symbol, err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Printf("[Yahoo] Read error for %s: %v", symbol, err)
			continue
		}

		var chart yahooChartResponse
		if err := json.Unmarshal(body, &chart); err != nil {
			log.Printf("[Yahoo] JSON parse error for %s: %v", symbol, err)
			continue
		}

		if chart.Chart.Error != nil || len(chart.Chart.Result) == 0 {
			log.Printf("[Yahoo] No data for %s", symbol)
			continue
		}

		r := chart.Chart.Result[0]
		meta := r.Meta

		price := decimal.NewFromFloat(meta.RegularMarketPrice)
		prevClose := decimal.NewFromFloat(meta.PreviousClose)

		change := price.Sub(prevClose)
		changePercent := float64(0)
		if !prevClose.IsZero() {
			changePercent, _ = change.Div(prevClose).Mul(decimal.NewFromInt(100)).Float64()
		}

		// Get latest quote data
		var open, high, low decimal.Decimal
		var volume int64
		var amount float64

		if len(r.Indicators.Quote) > 0 {
			q := r.Indicators.Quote[0]
			n := len(q.Close)
			if n > 0 {
				open = decimal.NewFromFloat(q.Open[n-1])
				high = decimal.NewFromFloat(q.High[n-1])
				low = decimal.NewFromFloat(q.Low[n-1])
				volume = q.Volume[n-1]
				amount = q.Close[n-1] * float64(q.Volume[n-1])
			}
		}

		md := MarketData{
			Symbol:        symbol,
			Name:          meta.Symbol,
			Price:         price,
			Open:          open,
			High:          high,
			Low:           low,
			PreClose:      prevClose,
			Volume:        volume,
			Amount:        decimal.NewFromFloat(amount),
			Change:        change,
			ChangePercent: changePercent,
			Timestamp:     now,
			MarketCode:    c.marketCode,
		}

		result = append(result, md)
	}

	log.Printf("[Yahoo] Fetched %d real-time quotes from Yahoo Finance", len(result))
	return result, nil
}

// FetchHistoricalData fetches historical overseas market data from Yahoo Finance.
func (c *YahooCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[Yahoo] Fetching historical data for %s from %s to %s interval=%s",
		symbol, start.Format("2006-01-02"), end.Format("2006-01-02"), interval)

	c.rateLimit()

	// Yahoo Finance interval mapping
	intervalMap := map[string]string{
		"1m":  "1m",
		"5m":  "5m",
		"15m": "15m",
		"30m": "30m",
		"60m": "1h",
		"1h":  "1h",
		"1d":  "1d",
		"1w":  "1wk",
	}

	yahooInterval, ok := intervalMap[interval]
	if !ok {
		yahooInterval = "1d"
	}

	period1 := start.Unix()
	period2 := end.Unix()

	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?period1=%d&period2=%d&interval=%s",
		symbol, period1, period2, yahooInterval,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	var chart yahooChartResponse
	if err := json.Unmarshal(body, &chart); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if chart.Chart.Error != nil || len(chart.Chart.Result) == 0 {
		return nil, fmt.Errorf("no data returned for %s", symbol)
	}

	r := chart.Chart.Result[0]
	timestamps := r.Timestamp

	if len(r.Indicators.Quote) == 0 {
		return nil, nil
	}

	q := r.Indicators.Quote[0]
	n := len(timestamps)

	var result []MarketData
	for i := 0; i < n && i < len(q.Close); i++ {
		t := time.Unix(timestamps[i], 0)

		open := decimal.NewFromFloat(q.Open[i])
		high := decimal.NewFromFloat(q.High[i])
		low := decimal.NewFromFloat(q.Low[i])
		close := decimal.NewFromFloat(q.Close[i])
		volume := q.Volume[i]
		amount := q.Close[i] * float64(volume)

		md := MarketData{
			Symbol:     symbol,
			Price:      close,
			Open:       open,
			High:       high,
			Low:        low,
			Volume:     volume,
			Amount:     decimal.NewFromFloat(amount),
			Timestamp:  t,
			MarketCode: c.marketCode,
		}

		// Set preclose from previous item
		if len(result) > 0 {
			md.PreClose = result[len(result)-1].Price
		} else {
			md.PreClose = open
		}

		if !md.PreClose.IsZero() {
			md.Change = md.Price.Sub(md.PreClose)
			md.ChangePercent, _ = md.Change.Div(md.PreClose).Mul(decimal.NewFromInt(100)).Float64()
		}

		result = append(result, md)
	}

	log.Printf("[Yahoo] Fetched %d historical records for %s", len(result), symbol)
	return result, nil
}