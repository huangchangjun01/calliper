package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

// EastMoneyCollector implements MarketDataCollector using East Money (东方财富) API.
// This is the primary data source for A-share stocks.
// API: https://push2.eastmoney.com/api/qt/stock/get
type EastMoneyCollector struct {
	marketCode string
	httpClient *http.Client
}

// NewEastMoneyCollector creates a new East Money collector.
func NewEastMoneyCollector(marketCode string) *EastMoneyCollector {
	return &EastMoneyCollector{
		marketCode: marketCode,
		httpClient: newPooledHTTPClient(),
	}
}

// GetMarketCode returns the market code.
func (c *EastMoneyCollector) GetMarketCode() string {
	return c.marketCode
}

// eastMoneySecID converts internal symbol to East Money secid format.
// CN: "600519" -> "1.600519" (Shanghai), "000001" -> "0.000001" (Shenzhen)
func (c *EastMoneyCollector) eastMoneySecID(symbol string) string {
	upper := strings.ToUpper(symbol)
	if strings.HasPrefix(upper, "6") || strings.HasPrefix(upper, "9") {
		return "1." + symbol
	}
	return "0." + symbol
}

// eastMoneyFields are the field codes requested from the East Money API.
// f43=最新价, f57=名称, f58=代码, f60=昨收, f169=今开, f170=最高, f171=最低,
// f46=成交量, f47=成交额, f48=换手率, f162=市盈率, f167=市净率,
// f116=总市值, f117=流通市值, f168=涨跌额, f50=量比
const eastMoneyFields = "f43,f57,f58,f60,f169,f170,f171,f46,f47,f48,f162,f167,f116,f117,f168,f50"

// emResponse is the top-level response from East Money API.
type emResponse struct {
	RC   int           `json:"rc"`
	RT   int           `json:"rt"`
	Data emStockData   `json:"data"`
}

// emStockData is the per-stock data from East Money API.
// Fields are returned as raw values (numbers may be strings or floats).
type emStockData struct {
	F43  json.RawMessage `json:"f43"`  // 最新价
	F57  json.RawMessage `json:"f57"`  // 名称
	F58  json.RawMessage `json:"f58"`  // 代码
	F60  json.RawMessage `json:"f60"`  // 昨收
	F169 json.RawMessage `json:"f169"` // 今开
	F170 json.RawMessage `json:"f170"` // 最高
	F171 json.RawMessage `json:"f171"` // 最低
	F46  json.RawMessage `json:"f46"`  // 成交量
	F47  json.RawMessage `json:"f47"`  // 成交额
	F48  json.RawMessage `json:"f48"`  // 换手率
	F162 json.RawMessage `json:"f162"` // 市盈率
	F167 json.RawMessage `json:"f167"` // 市净率
	F116 json.RawMessage `json:"f116"` // 总市值
	F117 json.RawMessage `json:"f117"` // 流通市值
	F168 json.RawMessage `json:"f168"` // 涨跌额
	F50  json.RawMessage `json:"f50"`  // 量比
}

// FetchRealTimeData fetches real-time market data from East Money API.
// East Money returns JSON with UTF-8 encoding.
func (c *EastMoneyCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[EastMoney] Fetching real-time data for %d symbols (market=%s)", len(symbols), c.marketCode)

	if len(symbols) == 0 {
		return nil, nil
	}

	// East Money get API supports a single secid at a time.
	// Use concurrent requests for efficiency.
	result := make([]MarketData, 0, len(symbols))
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, len(symbols))

	for _, symbol := range symbols {
		wg.Add(1)
		go func(sym string) {
			defer wg.Done()
			md, err := c.fetchSingleStock(sym)
			if err != nil {
				errCh <- err
				return
			}
			if md.Price == 0 {
				return
			}
			mu.Lock()
			result = append(result, md)
			mu.Unlock()
		}(symbol)
	}

	wg.Wait()
	close(errCh)

	// Collect errors for logging but don't fail entirely
	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		log.Printf("[EastMoney] %d/%d requests failed, got %d results (market=%s)",
			len(errs), len(symbols), len(result), c.marketCode)
	}

	log.Printf("[EastMoney] Fetched %d real-time quotes (market=%s)", len(result), c.marketCode)
	return result, nil
}

// fetchSingleStock fetches data for a single stock from East Money.
func (c *EastMoneyCollector) fetchSingleStock(symbol string) (MarketData, error) {
	secID := c.eastMoneySecID(symbol)
	url := fmt.Sprintf("https://push2.eastmoney.com/api/qt/stock/get?secid=%s&fields=%s",
		secID, eastMoneyFields)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return MarketData{}, fmt.Errorf("create request for %s: %w", symbol, err)
	}
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return MarketData{}, fmt.Errorf("HTTP request for %s: %w", symbol, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return MarketData{}, fmt.Errorf("read body for %s: %w", symbol, err)
	}

	var emResp emResponse
	if err := json.Unmarshal(body, &emResp); err != nil {
		return MarketData{}, fmt.Errorf("parse JSON for %s: %w", symbol, err)
	}

	if emResp.RC != 0 {
		return MarketData{}, fmt.Errorf("API error for %s: rc=%d", symbol, emResp.RC)
	}

	md := c.parseData(emResp.Data, symbol)
	return md, nil
}

// parseData parses East Money stock data into MarketData.
func (c *EastMoneyCollector) parseData(d emStockData, symbol string) MarketData {
	now := time.Now()

	// East Money returns prices in original units for the get API.
	price := parseRawFloat(d.F43)
	prevClose := parseRawFloat(d.F60)
	open := parseRawFloat(d.F169)
	high := parseRawFloat(d.F170)
	low := parseRawFloat(d.F171)
	volume := parseRawInt64(d.F46)
	amount := parseRawFloat(d.F47)
	turnoverRate := parseRawFloat(d.F48)
	pe := parseRawFloat(d.F162)
	pb := parseRawFloat(d.F167)
	totalMarketCap := parseRawFloat(d.F116)
	floatMarketCap := parseRawFloat(d.F117)
	change := parseRawFloat(d.F168)

	changePercent := float64(0)
	if prevClose != 0 {
		changePercent = (price - prevClose) / prevClose * 100
	}

	name := parseRawString(d.F57)

	return MarketData{
		Symbol:         symbol,
		Name:           name,
		Price:          price,
		Open:           open,
		High:           high,
		Low:            low,
		PreClose:       prevClose,
		Volume:         volume,
		Amount:         amount,
		Change:         change,
		ChangePercent:  changePercent,
		TurnoverRate:   turnoverRate,
		PE:             pe,
		PB:             pb,
		TotalMarketCap: totalMarketCap,
		FloatMarketCap: floatMarketCap,
		Timestamp:      now,
		MarketCode:     c.marketCode,
	}
}

// FetchHistoricalData fetches historical K-line data from East Money.
func (c *EastMoneyCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[EastMoney] Fetching historical data for %s from %s to %s",
		symbol, start.Format("2006-01-02"), end.Format("2006-01-02"))

	secID := c.eastMoneySecID(symbol)

	// Map interval to East Money klt (K-line type) parameter
	kltMap := map[string]string{
		"1m":  "1",
		"5m":  "5",
		"15m": "15",
		"30m": "30",
		"60m": "60",
		"1h":  "60",
		"1d":  "101",
		"1w":  "102",
	}
	klt, ok := kltMap[interval]
	if !ok {
		klt = "101" // default to daily
	}

	url := fmt.Sprintf(
		"https://push2his.eastmoney.com/api/qt/stock/kline/get?secid=%s&klt=%s&fqt=1&"+
			"beg=%s&end=%s&fields=f51,f52,f53,f54,f55,f56,f57",
		secID, klt,
		start.Format("20060102"), end.Format("20060102"),
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	// Parse K-line response
	type klineItem struct {
		Klines []string `json:"klines"`
	}
	type klineData struct {
		Data klineItem `json:"data"`
	}
	type klineResp struct {
		RC   int       `json:"rc"`
		Data klineData `json:"data"`
	}

	var kr klineResp
	if err := json.Unmarshal(body, &kr); err != nil {
		log.Printf("[EastMoney] JSON parse error for %s: %v, body: %s",
			symbol, err, string(body[:minInt(len(body), 200)]))
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	if kr.RC != 0 {
		return nil, fmt.Errorf("API error: rc=%d", kr.RC)
	}

	// Each kline is a comma-separated string: date,open,close,high,low,volume,amount
	var result []MarketData
	for _, line := range kr.Data.Data.Klines {
		parts := strings.Split(line, ",")
		if len(parts) < 6 {
			continue
		}

		t, err := time.Parse("2006-01-02", parts[0])
		if err != nil {
			continue
		}

		open := parseFloatSafe(parts[1])
		closePrice := parseFloatSafe(parts[2])
		high := parseFloatSafe(parts[3])
		low := parseFloatSafe(parts[4])
		volume := parseInt64Safe(parts[5])
		amount := float64(0)
		if len(parts) >= 7 {
			amount = parseFloatSafe(parts[6])
		}

		md := MarketData{
			Symbol:     symbol,
			Price:      closePrice,
			Open:       open,
			High:       high,
			Low:        low,
			Volume:     volume,
			Amount:     amount,
			Timestamp:  t,
			MarketCode: c.marketCode,
		}

		if len(result) > 0 {
			md.PreClose = result[len(result)-1].Price
		} else {
			md.PreClose = open
		}

		if md.PreClose != 0 {
			md.Change = md.Price - md.PreClose
			md.ChangePercent = md.Change / md.PreClose * 100
		}

		result = append(result, md)
	}

	log.Printf("[EastMoney] Fetched %d historical K-line records for %s", len(result), symbol)
	return result, nil
}

// ---- Raw JSON parsing helpers ----

// parseRawFloat extracts a float64 from a json.RawMessage.
// Handles both string and number JSON values.
func parseRawFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return 0
	}
	// Try as string first (common in East Money API)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" || s == "-" {
			return 0
		}
		return parseFloatSafe(s)
	}
	// Try as number
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	return 0
}

// parseRawInt64 extracts an int64 from a json.RawMessage.
func parseRawInt64(raw json.RawMessage) int64 {
	if len(raw) == 0 || string(raw) == "null" || string(raw) == `""` {
		return 0
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" || s == "-" {
			return 0
		}
		return parseInt64Safe(s)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return int64(f)
	}
	return 0
}

// parseRawString extracts a string from a json.RawMessage.
func parseRawString(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// Fallback: use raw bytes directly (strip quotes)
	s = string(raw)
	s = strings.Trim(s, `"`)
	return s
}