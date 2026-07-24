package services

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// YahooClient fetches stock lists for overseas exchanges via Yahoo Finance.
// Currently uses mock data; it will be wired to yfinance or Yahoo Finance API.
type YahooClient struct {
	RetryCfg   RetryConfig

	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

// NewYahooClient creates a new YahooClient with rate limiting defaults.
func NewYahooClient() *YahooClient {
	return &YahooClient{
		RetryCfg:    DefaultRetryConfig(),
		minInterval: 200 * time.Millisecond,
	}
}

// rateLimit blocks until the minimum interval since the last request has elapsed.
func (c *YahooClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	elapsed := time.Since(c.lastRequest)
	if elapsed < c.minInterval {
		time.Sleep(c.minInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// FetchStockList implements DataSource.
func (c *YahooClient) FetchStockList(marketCode string) ([]StockRaw, error) {
	if !IsOverseasMarket(marketCode) {
		return nil, fmt.Errorf("yahoo: unsupported market %s", marketCode)
	}
	c.rateLimit()
	return c.fetchWithRetry(marketCode)
}

// HealthCheck implements DataSource.
func (c *YahooClient) HealthCheck() error {
	// TODO: Replace with actual health check against the data source.
	return nil
}

func (c *YahooClient) fetchWithRetry(marketCode string) ([]StockRaw, error) {
	var lastErr error
	for attempt := 0; attempt < c.RetryCfg.MaxRetries; attempt++ {
		c.rateLimit()
		stocks, err := c.doFetch(marketCode)
		if err == nil {
			return stocks, nil
		}
		lastErr = err
		log.Printf("yahoo: attempt %d/%d for %s failed: %v", attempt+1, c.RetryCfg.MaxRetries, marketCode, err)
		time.Sleep(c.RetryCfg.BaseDelay * time.Duration(attempt+1))
	}
	return nil, fmt.Errorf("yahoo: all %d attempts for %s failed: %w", c.RetryCfg.MaxRetries, marketCode, lastErr)
}

// doFetch performs the actual fetch.
// TODO: Replace with real HTTP call to Yahoo Finance API or yfinance.
func (c *YahooClient) doFetch(marketCode string) ([]StockRaw, error) {
	// Mock data — always returns empty when the real API is not wired.
	stocks := c.mockStockList(marketCode)
	log.Printf("yahoo: fetched %d stocks for %s (mock)", len(stocks), marketCode)
	return stocks, nil

	/*
		// Real implementation (requires yfinance or Yahoo Finance API):
		ctx, cancel := context.WithTimeout(context.Background(), c.RetryCfg.Timeout)
		defer cancel()

		url := fmt.Sprintf("https://query1.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=%s&count=250", marketCode)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("yahoo: unexpected status %d", resp.StatusCode)
		}

		var stocks []StockRaw
		if err := json.NewDecoder(resp.Body).Decode(&stocks); err != nil {
			return nil, err
		}
		return stocks, nil
	*/
}

// mockStockList returns a small set of mock overseas stocks for development.
func (c *YahooClient) mockStockList(marketCode string) []StockRaw {
	switch marketCode {
	case "HKEX":
		return []StockRaw{
			{Symbol: "0700.HK", Name: "Tencent Holdings", NameCN: "腾讯控股", Exchange: "HKEX", Industry: "Internet", Sector: "Communication Services", Currency: "HKD", LotSize: 100, IsActive: true},
			{Symbol: "9988.HK", Name: "Alibaba Group", NameCN: "阿里巴巴", Exchange: "HKEX", Industry: "E-Commerce", Sector: "Consumer Discretionary", Currency: "HKD", LotSize: 100, IsActive: true},
			{Symbol: "0941.HK", Name: "China Mobile", NameCN: "中国移动", Exchange: "HKEX", Industry: "Telecom", Sector: "Communication Services", Currency: "HKD", LotSize: 500, IsActive: true},
		}
	case "NYSE":
		return []StockRaw{
			{Symbol: "BRK.B", Name: "Berkshire Hathaway", Exchange: "NYSE", Industry: "Conglomerate", Sector: "Financials", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "JPM", Name: "JPMorgan Chase", Exchange: "NYSE", Industry: "Banking", Sector: "Financials", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "WMT", Name: "Walmart", Exchange: "NYSE", Industry: "Retail", Sector: "Consumer Staples", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "NASDAQ":
		return []StockRaw{
			{Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ", Industry: "Technology", Sector: "Information Technology", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "MSFT", Name: "Microsoft Corporation", Exchange: "NASDAQ", Industry: "Technology", Sector: "Information Technology", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "GOOGL", Name: "Alphabet Inc.", Exchange: "NASDAQ", Industry: "Internet", Sector: "Communication Services", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "AMZN", Name: "Amazon.com", Exchange: "NASDAQ", Industry: "E-Commerce", Sector: "Consumer Discretionary", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "AMEX":
		return []StockRaw{
			{Symbol: "SPY", Name: "SPDR S&P 500 ETF", Exchange: "AMEX", Industry: "ETF", Sector: "Financials", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "TSE":
		return []StockRaw{
			{Symbol: "7203.T", Name: "Toyota Motor", Exchange: "TSE", Industry: "Automotive", Sector: "Consumer Discretionary", Currency: "JPY", LotSize: 100, IsActive: true},
			{Symbol: "6758.T", Name: "Sony Group", Exchange: "TSE", Industry: "Electronics", Sector: "Information Technology", Currency: "JPY", LotSize: 100, IsActive: true},
		}
	case "LSE":
		return []StockRaw{
			{Symbol: "HSBA.L", Name: "HSBC Holdings", Exchange: "LSE", Industry: "Banking", Sector: "Financials", Currency: "GBP", LotSize: 1, IsActive: true},
			{Symbol: "BP.L", Name: "BP plc", Exchange: "LSE", Industry: "Oil & Gas", Sector: "Energy", Currency: "GBP", LotSize: 1, IsActive: true},
		}
	case "Euronext":
		return []StockRaw{
			{Symbol: "MC.PA", Name: "LVMH Moët Hennessy", Exchange: "Euronext", Industry: "Luxury", Sector: "Consumer Discretionary", Currency: "EUR", LotSize: 1, IsActive: true},
		}
	case "Xetra":
		return []StockRaw{
			{Symbol: "SAP.DE", Name: "SAP SE", Exchange: "Xetra", Industry: "Technology", Sector: "Information Technology", Currency: "EUR", LotSize: 1, IsActive: true},
		}
	case "ASX":
		return []StockRaw{
			{Symbol: "BHP.AX", Name: "BHP Group", Exchange: "ASX", Industry: "Mining", Sector: "Materials", Currency: "AUD", LotSize: 1, IsActive: true},
		}
	case "TSX":
		return []StockRaw{
			{Symbol: "RY.TO", Name: "Royal Bank of Canada", Exchange: "TSX", Industry: "Banking", Sector: "Financials", Currency: "CAD", LotSize: 1, IsActive: true},
		}
	case "KRX":
		return []StockRaw{
			{Symbol: "005930.KS", Name: "Samsung Electronics", Exchange: "KRX", Industry: "Electronics", Sector: "Information Technology", Currency: "KRW", LotSize: 1, IsActive: true},
		}
	default:
		return nil
	}
}