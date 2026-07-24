package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// AkshareClient fetches stock lists for Chinese exchanges (SSE/SZSE/BSE).
// Uses Sina Finance API for real stock list data.
type AkshareClient struct {
	BaseURL    string
	RetryCfg   RetryConfig
	httpClient *http.Client
}

// NewAkshareClient creates a new AkshareClient with default settings.
func NewAkshareClient() *AkshareClient {
	return &AkshareClient{
		BaseURL:  "http://ml-service:8000/api/akshare",
		RetryCfg: DefaultRetryConfig(),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// FetchStockList implements DataSource.
// Fetches real stock list from Sina Finance API for Chinese exchanges.
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
	// Try to reach Sina Finance API
	req, _ := http.NewRequest("GET", "https://hq.sinajs.cn/list=sh600519", nil)
	req.Header.Set("Referer", "https://finance.sina.com.cn")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sina finance unreachable: %w", err)
	}
	resp.Body.Close()
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

// doFetch fetches stock list from Sina Finance API.
func (c *AkshareClient) doFetch(marketCode string) ([]StockRaw, error) {
	// Map market code to Sina Finance node
	nodeMap := map[string]string{
		"SSE":  "sh_a",
		"SZSE": "sz_a",
		"BSE":  "bj_a",
	}

	node, ok := nodeMap[marketCode]
	if !ok {
		return nil, fmt.Errorf("unsupported market: %s", marketCode)
	}

	// Fetch up to 200 stocks per page
	url := fmt.Sprintf(
		"https://vip.stock.finance.sina.com.cn/quotes_service/api/json_v2.php/Market_Center.getHQNodeData?page=1&num=200&sort=symbol&asc=1&node=%s",
		node,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Parse JSON response
	type SinaStockItem struct {
		Symbol string `json:"symbol"`
		Name   string `json:"name"`
		Code   string `json:"code"`
	}

	var items []SinaStockItem
	if err := json.Unmarshal(body, &items); err != nil {
		log.Printf("[Akshare] JSON parse error for %s: %v", marketCode, err)
		// Fall back to default stock list if API fails
		return c.defaultStockList(marketCode), nil
	}

	if len(items) == 0 {
		return c.defaultStockList(marketCode), nil
	}

	var stocks []StockRaw
	for _, item := range items {
		code := item.Code
		if code == "" {
			code = item.Symbol
		}
		exchange := marketCode
		if marketCode == "SSE" {
			exchange = "SSE"
		} else if marketCode == "SZSE" {
			exchange = "SZSE"
		} else if marketCode == "BSE" {
			exchange = "BSE"
		}

		stocks = append(stocks, StockRaw{
			Symbol:   code,
			Name:     item.Name,
			NameCN:   item.Name,
			Exchange: exchange,
			Currency: "CNY",
			LotSize:  100,
			IsActive: true,
		})
	}

	log.Printf("akshare: fetched %d stocks for %s from Sina Finance", len(stocks), marketCode)
	return stocks, nil
}

// defaultStockList returns a known set of major stocks for each market as fallback.
func (c *AkshareClient) defaultStockList(marketCode string) []StockRaw {
	switch marketCode {
	case "SSE":
		return []StockRaw{
			{Symbol: "600000", Name: "浦发银行", NameCN: "浦发银行", Exchange: "SSE", Industry: "银行", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600519", Name: "贵州茅台", NameCN: "贵州茅台", Exchange: "SSE", Industry: "白酒", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "601318", Name: "中国平安", NameCN: "中国平安", Exchange: "SSE", Industry: "保险", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600036", Name: "招商银行", NameCN: "招商银行", Exchange: "SSE", Industry: "银行", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600276", Name: "恒瑞医药", NameCN: "恒瑞医药", Exchange: "SSE", Industry: "医药", Sector: "医疗", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600900", Name: "长江电力", NameCN: "长江电力", Exchange: "SSE", Industry: "电力", Sector: "公用事业", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "601012", Name: "隆基绿能", NameCN: "隆基绿能", Exchange: "SSE", Industry: "光伏", Sector: "新能源", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600030", Name: "中信证券", NameCN: "中信证券", Exchange: "SSE", Industry: "证券", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "601888", Name: "中国中免", NameCN: "中国中免", Exchange: "SSE", Industry: "旅游", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "600809", Name: "山西汾酒", NameCN: "山西汾酒", Exchange: "SSE", Industry: "白酒", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	case "SZSE":
		return []StockRaw{
			{Symbol: "000001", Name: "平安银行", NameCN: "平安银行", Exchange: "SZSE", Industry: "银行", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "000858", Name: "五粮液", NameCN: "五粮液", Exchange: "SZSE", Industry: "白酒", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "300750", Name: "宁德时代", NameCN: "宁德时代", Exchange: "SZSE", Industry: "电池", Sector: "新能源", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "002415", Name: "海康威视", NameCN: "海康威视", Exchange: "SZSE", Industry: "安防", Sector: "科技", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "000002", Name: "万科A", NameCN: "万科A", Exchange: "SZSE", Industry: "房地产", Sector: "地产", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "002594", Name: "比亚迪", NameCN: "比亚迪", Exchange: "SZSE", Industry: "汽车", Sector: "制造", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "300059", Name: "东方财富", NameCN: "东方财富", Exchange: "SZSE", Industry: "互联网", Sector: "金融", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "000333", Name: "美的集团", NameCN: "美的集团", Exchange: "SZSE", Industry: "家电", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "000651", Name: "格力电器", NameCN: "格力电器", Exchange: "SZSE", Industry: "家电", Sector: "消费", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "300760", Name: "迈瑞医疗", NameCN: "迈瑞医疗", Exchange: "SZSE", Industry: "医疗", Sector: "医疗", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	case "BSE":
		return []StockRaw{
			{Symbol: "830799", Name: "艾融软件", NameCN: "艾融软件", Exchange: "BSE", Industry: "软件", Sector: "科技", Currency: "CNY", LotSize: 100, IsActive: true},
			{Symbol: "831445", Name: "龙竹科技", NameCN: "龙竹科技", Exchange: "BSE", Industry: "制造", Sector: "工业", Currency: "CNY", LotSize: 100, IsActive: true},
		}
	default:
		return nil
	}
}