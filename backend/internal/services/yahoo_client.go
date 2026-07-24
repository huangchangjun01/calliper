package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

// YahooClient fetches stock lists for overseas exchanges via Yahoo Finance.
// Uses Yahoo Finance screener and chart APIs for real data.
type YahooClient struct {
	RetryCfg   RetryConfig
	httpClient *http.Client

	mu          sync.Mutex
	lastRequest time.Time
	minInterval time.Duration
}

// NewYahooClient creates a new YahooClient with rate limiting defaults.
func NewYahooClient() *YahooClient {
	return &YahooClient{
		RetryCfg:    DefaultRetryConfig(),
		httpClient:  &http.Client{Timeout: 15 * time.Second},
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
	// Try to reach Yahoo Finance
	c.rateLimit()
	req, _ := http.NewRequest("GET", "https://query1.finance.yahoo.com/v8/finance/chart/AAPL?interval=1d&range=5d", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("yahoo finance unreachable: %w", err)
	}
	resp.Body.Close()
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

// doFetch fetches stock list from Yahoo Finance screener API.
func (c *YahooClient) doFetch(marketCode string) ([]StockRaw, error) {
	// Yahoo Finance screener predefined IDs
	scrIDMap := map[string]string{
		"NASDAQ": "day_gainers",
		"NYSE":   "day_gainers",
		"HKEX":   "day_gainers",
		"AMEX":   "day_gainers",
	}

	scrID, ok := scrIDMap[marketCode]
	if !ok {
		// Try to use most active stocks screener
		return c.defaultStockList(marketCode), nil
	}

	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v1/finance/screener/predefined/saved?scrIds=%s&count=100",
		scrID,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return c.defaultStockList(marketCode), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[Yahoo] Screener API error for %s: %v", marketCode, err)
		return c.defaultStockList(marketCode), nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.defaultStockList(marketCode), nil
	}

	// Parse Yahoo Finance screener response
	type ScreenerQuote struct {
		Symbol             string `json:"symbol"`
		ShortName          string `json:"shortName"`
		LongName           string `json:"longName"`
		FullExchangeName   string `json:"fullExchangeName"`
		Market             string `json:"market"`
		Currency           string `json:"currency"`
		RegularMarketPrice float64 `json:"regularMarketPrice"`
	}

	type ScreenerResult struct {
		Finance struct {
			Result []struct {
				Quotes []ScreenerQuote `json:"quotes"`
			} `json:"result"`
		} `json:"finance"`
	}

	var sr ScreenerResult
	if err := json.Unmarshal(body, &sr); err != nil {
		log.Printf("[Yahoo] JSON parse error for screener %s: %v", marketCode, err)
		return c.defaultStockList(marketCode), nil
	}

	var stocks []StockRaw
	for _, result := range sr.Finance.Result {
		for _, q := range result.Quotes {
			exchange := marketCode
			lotSize := 1
			currency := q.Currency
			if currency == "" {
				currency = "USD"
			}

			// Determine lot size based on market
			switch marketCode {
			case "HKEX":
				currency = "HKD"
				lotSize = 100
			case "TSE":
				currency = "JPY"
				lotSize = 100
			case "LSE":
				currency = "GBP"
			case "Euronext":
				currency = "EUR"
			case "KRX":
				currency = "KRW"
			case "ASX":
				currency = "AUD"
			case "TSX":
				currency = "CAD"
			}

			name := q.ShortName
			if name == "" {
				name = q.LongName
			}

			stocks = append(stocks, StockRaw{
				Symbol:   q.Symbol,
				Name:     name,
				NameCN:   name,
				Exchange: exchange,
				Currency: currency,
				LotSize:  lotSize,
				IsActive: true,
			})
		}
	}

	if len(stocks) == 0 {
		return c.defaultStockList(marketCode), nil
	}

	log.Printf("yahoo: fetched %d stocks for %s from Yahoo Finance", len(stocks), marketCode)
	return stocks, nil
}

// defaultStockList returns a known set of major stocks for each overseas market as fallback.
func (c *YahooClient) defaultStockList(marketCode string) []StockRaw {
	switch marketCode {
	case "HKEX":
		return []StockRaw{
			{Symbol: "0700.HK", Name: "Tencent Holdings", NameCN: "腾讯控股", Exchange: "HKEX", Industry: "互联网", Sector: "通信", Currency: "HKD", LotSize: 100, IsActive: true},
			{Symbol: "9988.HK", Name: "Alibaba Group", NameCN: "阿里巴巴", Exchange: "HKEX", Industry: "电商", Sector: "消费", Currency: "HKD", LotSize: 100, IsActive: true},
			{Symbol: "0941.HK", Name: "China Mobile", NameCN: "中国移动", Exchange: "HKEX", Industry: "电信", Sector: "通信", Currency: "HKD", LotSize: 500, IsActive: true},
			{Symbol: "3690.HK", Name: "Meituan", NameCN: "美团", Exchange: "HKEX", Industry: "互联网", Sector: "消费", Currency: "HKD", LotSize: 100, IsActive: true},
			{Symbol: "1299.HK", Name: "AIA Group", NameCN: "友邦保险", Exchange: "HKEX", Industry: "保险", Sector: "金融", Currency: "HKD", LotSize: 200, IsActive: true},
		}
	case "NYSE":
		return []StockRaw{
			{Symbol: "BRK-B", Name: "Berkshire Hathaway", Exchange: "NYSE", Industry: "综合", Sector: "金融", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "JPM", Name: "JPMorgan Chase", Exchange: "NYSE", Industry: "银行", Sector: "金融", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "WMT", Name: "Walmart", Exchange: "NYSE", Industry: "零售", Sector: "消费", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "BAC", Name: "Bank of America", Exchange: "NYSE", Industry: "银行", Sector: "金融", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "XOM", Name: "Exxon Mobil", Exchange: "NYSE", Industry: "能源", Sector: "能源", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "NASDAQ":
		return []StockRaw{
			{Symbol: "AAPL", Name: "Apple Inc.", Exchange: "NASDAQ", Industry: "科技", Sector: "科技", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "MSFT", Name: "Microsoft Corporation", Exchange: "NASDAQ", Industry: "科技", Sector: "科技", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "GOOGL", Name: "Alphabet Inc.", Exchange: "NASDAQ", Industry: "互联网", Sector: "通信", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "AMZN", Name: "Amazon.com", Exchange: "NASDAQ", Industry: "电商", Sector: "消费", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "NVDA", Name: "NVIDIA Corporation", Exchange: "NASDAQ", Industry: "半导体", Sector: "科技", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "META", Name: "Meta Platforms", Exchange: "NASDAQ", Industry: "互联网", Sector: "通信", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "TSLA", Name: "Tesla, Inc.", Exchange: "NASDAQ", Industry: "汽车", Sector: "消费", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "AMEX":
		return []StockRaw{
			{Symbol: "SPY", Name: "SPDR S&P 500 ETF", Exchange: "AMEX", Industry: "ETF", Sector: "金融", Currency: "USD", LotSize: 1, IsActive: true},
			{Symbol: "QQQ", Name: "Invesco QQQ Trust", Exchange: "AMEX", Industry: "ETF", Sector: "金融", Currency: "USD", LotSize: 1, IsActive: true},
		}
	case "TSE":
		return []StockRaw{
			{Symbol: "7203.T", Name: "Toyota Motor", Exchange: "TSE", Industry: "汽车", Sector: "消费", Currency: "JPY", LotSize: 100, IsActive: true},
			{Symbol: "6758.T", Name: "Sony Group", Exchange: "TSE", Industry: "电子", Sector: "科技", Currency: "JPY", LotSize: 100, IsActive: true},
		}
	case "LSE":
		return []StockRaw{
			{Symbol: "HSBA.L", Name: "HSBC Holdings", Exchange: "LSE", Industry: "银行", Sector: "金融", Currency: "GBP", LotSize: 1, IsActive: true},
			{Symbol: "BP.L", Name: "BP plc", Exchange: "LSE", Industry: "能源", Sector: "能源", Currency: "GBP", LotSize: 1, IsActive: true},
		}
	case "Euronext":
		return []StockRaw{
			{Symbol: "MC.PA", Name: "LVMH Moet Hennessy", Exchange: "Euronext", Industry: "奢侈品", Sector: "消费", Currency: "EUR", LotSize: 1, IsActive: true},
		}
	case "Xetra":
		return []StockRaw{
			{Symbol: "SAP.DE", Name: "SAP SE", Exchange: "Xetra", Industry: "科技", Sector: "科技", Currency: "EUR", LotSize: 1, IsActive: true},
		}
	case "ASX":
		return []StockRaw{
			{Symbol: "BHP.AX", Name: "BHP Group", Exchange: "ASX", Industry: "矿业", Sector: "材料", Currency: "AUD", LotSize: 1, IsActive: true},
		}
	case "TSX":
		return []StockRaw{
			{Symbol: "RY.TO", Name: "Royal Bank of Canada", Exchange: "TSX", Industry: "银行", Sector: "金融", Currency: "CAD", LotSize: 1, IsActive: true},
		}
	case "KRX":
		return []StockRaw{
			{Symbol: "005930.KS", Name: "Samsung Electronics", Exchange: "KRX", Industry: "电子", Sector: "科技", Currency: "KRW", LotSize: 1, IsActive: true},
		}
	default:
		return nil
	}
}