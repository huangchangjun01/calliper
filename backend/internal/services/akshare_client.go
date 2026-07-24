package services

import (
	"fmt"
	"log"
	"time"
)

// AkshareClient fetches stock lists for Chinese exchanges (SSE/SZSE/BSE).
// Currently uses mock data; it will be wired to AKShare HTTP API or the ml-service REST API.
type AkshareClient struct {
	BaseURL    string
	RetryCfg   RetryConfig
}

// NewAkshareClient creates a new AkshareClient with default settings.
func NewAkshareClient() *AkshareClient {
	return &AkshareClient{
		BaseURL:  "http://ml-service:8000/api/akshare",
		RetryCfg: DefaultRetryConfig(),
	}
}

// FetchStockList implements DataSource.
func (c *AkshareClient) FetchStockList(marketCode string) ([]StockRaw, error) {
	switch marketCode {
	case "SSE", "SZSE", "BSE":
		return c.fetchWithRetry(marketCode)
	default:
		return nil, fmt.Errorf("akshare: unsupported market %s", marketCode)
	}
}

// HealthCheck implements DataSource.
func (c *AkshareClient) HealthCheck() error {
	// TODO: Replace with actual HTTP health check to ml-service
	return nil
}

func (c *AkshareClient) fetchWithRetry(marketCode string) ([]StockRaw, error) {
	var lastErr error
	for attempt := 0; attempt < c.RetryCfg.MaxRetries; attempt++ {
		stocks, err := c.doFetch(marketCode)
		if err == nil {
			return stocks, nil
		}
		lastErr = err
		log.Printf("akshare: attempt %d/%d for %s failed: %v", attempt+1, c.RetryCfg.MaxRetries, marketCode, err)
		time.Sleep(c.RetryCfg.BaseDelay * time.Duration(attempt+1))
	}
	return nil, fmt.Errorf("akshare: all %d attempts for %s failed: %w", c.RetryCfg.MaxRetries, marketCode, lastErr)
}

// doFetch performs the actual data retrieval.
// TODO: Replace with real HTTP call to ml-service or AKShare REST API.
func (c *AkshareClient) doFetch(marketCode string) ([]StockRaw, error) {
	// Mock data — always returns empty when the real API is not wired.
	// Remove this block and uncomment the HTTP call when ready.

	stocks := c.mockStockList(marketCode)
	log.Printf("akshare: fetched %d stocks for %s (mock)", len(stocks), marketCode)
	return stocks, nil

	/*
		// Real implementation (requires ml-service):
		ctx, cancel := context.WithTimeout(context.Background(), c.RetryCfg.Timeout)
		defer cancel()

		url := fmt.Sprintf("%s/stock_list?market=%s", c.BaseURL, marketCode)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("akshare: unexpected status %d", resp.StatusCode)
		}

		var stocks []StockRaw
		if err := json.NewDecoder(resp.Body).Decode(&stocks); err != nil {
			return nil, err
		}
		return stocks, nil
	*/
}

// mockStockList returns a small set of mock stocks for development.
func (c *AkshareClient) mockStockList(marketCode string) []StockRaw {
	switch marketCode {
	case "SSE":
		return []StockRaw{
			{Symbol: "600000", Name: "Pudong Development Bank", NameCN: "浦发银行", Exchange: "SSE", Industry: "Banking", Sector: "Financials", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600519", Name: "Kweichow Moutai", NameCN: "贵州茅台", Exchange: "SSE", Industry: "Beverages", Sector: "Consumer Staples", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "601318", Name: "China Ping An", NameCN: "中国平安", Exchange: "SSE", Industry: "Insurance", Sector: "Financials", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600036", Name: "China Merchants Bank", NameCN: "招商银行", Exchange: "SSE", Industry: "Banking", Sector: "Financials", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	case "SZSE":
		return []StockRaw{
			{Symbol: "000001", Name: "Ping An Bank", NameCN: "平安银行", Exchange: "SZSE", Industry: "Banking", Sector: "Financials", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "000858", Name: "Wuliangye", NameCN: "五粮液", Exchange: "SZSE", Industry: "Beverages", Sector: "Consumer Staples", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "300750", Name: "CATL", NameCN: "宁德时代", Exchange: "SZSE", Industry: "Batteries", Sector: "Industrials", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	case "BSE":
		return []StockRaw{
			{Symbol: "830799", Name: "Aikang Technology", NameCN: "艾康科技", Exchange: "BSE", Industry: "Technology", Sector: "Information Technology", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "831445", Name: "Longzhu Technology", NameCN: "龙竹科技", Exchange: "BSE", Industry: "Manufacturing", Sector: "Industrials", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	default:
		return nil
	}
}