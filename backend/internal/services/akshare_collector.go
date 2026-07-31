package services

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AKShareCollector implements MarketDataCollector using Sina Finance API.
// Supports A-share (CN), US stocks, and HK stocks.
type AKShareCollector struct {
	marketCode   string
	mlServiceURL string
	httpClient   *http.Client
	mu           sync.Mutex
	lastPrices   map[string]float64 // last known price per symbol, used for mock fallback
	rng          *rand.Rand
}

// NewAKShareCollector creates a new AKShare collector for the given market.
func NewAKShareCollector(mlServiceURL string, marketCode string) *AKShareCollector {
	return &AKShareCollector{
		marketCode:   marketCode,
		mlServiceURL: mlServiceURL,
		httpClient:   newPooledHTTPClient(),
		lastPrices:   make(map[string]float64),
		rng:          rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GetMarketCode returns the market code for A-share stocks.
func (c *AKShareCollector) GetMarketCode() string {
	return c.marketCode
}

// sinaSymbol converts internal symbol format to Sina Finance format.
// CN: "600519" -> "sh600519", "000001" -> "sz000001"
// US: "AAPL" -> "gb_aapl"
// HK: "00700" -> "hk00700"
func (c *AKShareCollector) sinaSymbol(symbol string) string {
	switch c.marketCode {
	case "CN":
		upper := strings.ToUpper(symbol)
		if strings.HasPrefix(upper, "6") || strings.HasPrefix(upper, "9") {
			return "sh" + symbol
		}
		return "sz" + symbol
	case "US":
		return "gb_" + strings.ToLower(symbol)
	case "HK":
		return "hk" + symbol
	default:
		return symbol
	}
}

// FetchRealTimeData fetches real-time market data from Sina Finance API.
// Uses bufio.Scanner for streaming line-by-line parsing to avoid
// double-buffering the entire response body in memory.
// Falls back to mock data when the external API is unreachable.
func (c *AKShareCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[AKShare] Fetching real-time data for %d symbols from Sina Finance (market=%s)", len(symbols), c.marketCode)

	var sinaCodes []string
	for _, s := range symbols {
		sinaCodes = append(sinaCodes, c.sinaSymbol(s))
	}

	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", strings.Join(sinaCodes, ","))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return c.generateMockData(symbols), nil
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[AKShare] HTTP error: %v, falling back to mock data", err)
		return c.generateMockData(symbols), nil
	}
	defer resp.Body.Close()

	// Pre-allocate result slice to avoid dynamic growth reallocations
	result := make([]MarketData, 0, len(symbols))
	now := time.Now()

	// Use bufio.Scanner for line-by-line streaming, avoiding io.ReadAll + string() copy
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 64*1024) // 64KB buffer, enough for ~30 stocks
	i := 0

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse Sina Finance format: var hq_str_XXXX="data,fields,..."
		// Find opening quote using byte search to avoid string conversion
		idx := -1
		for j, b := range line {
			if b == '"' {
				idx = j
				break
			}
		}
		if idx < 0 || idx+1 >= len(line) {
			continue
		}

		// Find closing quote
		endIdx := -1
		for j := len(line) - 1; j > idx; j-- {
			if line[j] == '"' {
				endIdx = j
				break
			}
		}
		if endIdx < 0 || endIdx <= idx {
			continue
		}

		dataStr := string(line[idx+1 : endIdx])
		if dataStr == "" {
			continue
		}

		parts := strings.Split(dataStr, ",")
		if len(parts) < 4 {
			continue
		}

		symbol := ""
		if i < len(symbols) {
			symbol = symbols[i]
		}

		var md MarketData
		switch c.marketCode {
		case "CN":
			md = c.parseCNData(parts, symbol, now)
		case "US":
			md = c.parseUSData(parts, symbol, now)
		case "HK":
			md = c.parseHKData(parts, symbol, now)
		default:
			i++
			continue
		}

		if md.Price == 0 {
			i++
			continue
		}

		// Update last known price for mock fallback
		c.mu.Lock()
		c.lastPrices[symbol] = md.Price
		c.mu.Unlock()

		result = append(result, md)
		i++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("[AKShare] Scanner error: %v", err)
	}

	if len(result) == 0 {
		log.Printf("[AKShare] No data parsed, falling back to mock data")
		return c.generateMockData(symbols), nil
	}

	log.Printf("[AKShare] Fetched %d real-time quotes from Sina Finance (market=%s)", len(result), c.marketCode)
	return result, nil
}

// parseCNData parses A-share market data from Sina response.
// Format: name(0), open(1), prev_close(2), price(3), high(4), low(5), bid(6), ask(7), volume(8), amount(9), ...
func (c *AKShareCollector) parseCNData(parts []string, symbol string, now time.Time) MarketData {
	if len(parts) < 9 {
		return MarketData{}
	}

	price := parseFloatSafe(parts[3])
	open := parseFloatSafe(parts[1])
	prevClose := parseFloatSafe(parts[2])
	high := parseFloatSafe(parts[4])
	low := parseFloatSafe(parts[5])
	volume := parseInt64Safe(parts[8])
	amount := parseFloatSafe(parts[9])

	change := price - prevClose
	changePercent := float64(0)
	if prevClose != 0 {
		changePercent = change / prevClose * 100
	}

	return MarketData{
		Symbol:        symbol,
		Name:          parts[0],
		Price:         price,
		Open:          open,
		High:          high,
		Low:           low,
		PreClose:      prevClose,
		Volume:        volume,
		Amount:        amount,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     now,
		MarketCode:    c.marketCode,
	}
}

// parseUSData parses US stock market data from Sina response.
// Format: name(0), price(1), change_percent(2), change(3), ...
func (c *AKShareCollector) parseUSData(parts []string, symbol string, now time.Time) MarketData {
	name := parts[0]
	price := parseFloatSafe(parts[1])
	changePercent := parseFloatSafe(parts[2])
	change := parseFloatSafe(parts[3])

	return MarketData{
		Symbol:        symbol,
		Name:          name,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     now,
		MarketCode:    c.marketCode,
	}
}

// parseHKData parses HK stock market data from Sina response.
// Format: name(0), open(1), prev_close(2), price(3), high(4), low(5), ...
func (c *AKShareCollector) parseHKData(parts []string, symbol string, now time.Time) MarketData {
	if len(parts) < 10 {
		return MarketData{}
	}

	name := parts[1]
	price := parseFloatSafe(parts[4])
	open := parseFloatSafe(parts[2])
	prevClose := parseFloatSafe(parts[3])
	high := parseFloatSafe(parts[5])
	low := parseFloatSafe(parts[6])
	change := parseFloatSafe(parts[7])
	changePercent := parseFloatSafe(parts[8])

	return MarketData{
		Symbol:        symbol,
		Name:          name,
		Price:         price,
		Open:          open,
		High:          high,
		Low:           low,
		PreClose:      prevClose,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     now,
		MarketCode:    c.marketCode,
	}
}

// FetchHistoricalData fetches historical A-share K-line data from Sina Finance API.
func (c *AKShareCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[AKShare] Fetching historical data for %s from %s to %s interval=%s",
		symbol, start.Format("2006-01-02"), end.Format("2006-01-02"), interval)

	// Sina Finance K-line API
	scaleMap := map[string]string{
		"1m":  "5",
		"5m":  "15",
		"15m": "30",
		"30m": "60",
		"60m": "60",
		"1h":  "60",
		"1d":  "240",
		"1w":  "1200",
	}

	scale, ok := scaleMap[interval]
	if !ok {
		scale = "240" // default to daily
	}

	datalen := "200"
	if interval == "1m" {
		datalen = "240"
	}

	sinaCode := c.sinaSymbol(symbol)
	url := fmt.Sprintf(
		"https://money.finance.sina.com.cn/quotes_service/api/json_v2.php/CN_MarketData.getKLineData?symbol=%s&scale=%s&ma=no&datalen=%s",
		sinaCode, scale, datalen,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
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
	type KLineItem struct {
		Day    string `json:"day"`
		Open   string `json:"open"`
		High   string `json:"high"`
		Low    string `json:"low"`
		Close  string `json:"close"`
		Volume string `json:"volume"`
	}

	var items []KLineItem
	if err := json.Unmarshal(body, &items); err != nil {
		log.Printf("[AKShare] JSON parse error: %v, body: %s", err, string(body[:minInt(len(body), 200)]))
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	var result []MarketData
	for _, item := range items {
		t, err := time.Parse("2006-01-02", item.Day)
		if err != nil {
			// Try minute-level format
			t, err = time.Parse("2006-01-02 15:04:05", item.Day)
			if err != nil {
				continue
			}
		}

		// Filter by date range
		if t.Before(start) || t.After(end) {
			continue
		}

		open := parseFloatSafe(item.Open)
		high := parseFloatSafe(item.High)
		low := parseFloatSafe(item.Low)
		close := parseFloatSafe(item.Close)
		volume := parseInt64Safe(item.Volume)

		md := MarketData{
			Symbol:     symbol,
			Price:      close,
			Open:       open,
			High:       high,
			Low:        low,
			Volume:     volume,
			Timestamp:  t,
			MarketCode: c.marketCode,
		}

		// Set preclose from previous item or use open
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

	log.Printf("[AKShare] Fetched %d historical K-line records for %s", len(result), symbol)
	return result, nil
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// stockMeta holds static metadata for mock data generation.
type stockMeta struct {
	Name  string
	Price float64 // base reference price
}

// cnStockMeta maps A-share symbols to their names and reference prices.
var cnStockMeta = map[string]stockMeta{
	"000001": {Name: "平安银行", Price: 12.50},
	"600519": {Name: "贵州茅台", Price: 1680.00},
	"000858": {Name: "五粮液", Price: 148.00},
	"300750": {Name: "宁德时代", Price: 210.00},
	"601318": {Name: "中国平安", Price: 48.00},
	"600036": {Name: "招商银行", Price: 38.00},
	"000333": {Name: "美的集团", Price: 65.00},
	"002415": {Name: "海康威视", Price: 32.00},
	"600276": {Name: "恒瑞医药", Price: 45.00},
	"601012": {Name: "隆基绿能", Price: 22.00},
}

// generateMockData generates realistic simulated market data when external API is unavailable.
// Uses last known prices if available, otherwise falls back to reference prices.
// Each call produces slightly different values to simulate real market fluctuations.
func (c *AKShareCollector) generateMockData(symbols []string) []MarketData {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	result := make([]MarketData, 0, len(symbols))

	for _, symbol := range symbols {
		meta, ok := cnStockMeta[symbol]
		if !ok {
			meta = stockMeta{Name: symbol, Price: 50.0}
		}

		// Use last known price if available, otherwise reference price
		basePrice := meta.Price
		if last, exists := c.lastPrices[symbol]; exists && last > 0 {
			basePrice = last
		}

		// Simulate random price fluctuation within ±2%
		fluctuation := (c.rng.Float64() - 0.5) * 0.04 // -2% to +2%
		price := basePrice * (1 + fluctuation)
		price = math.Round(price*100) / 100 // round to 2 decimals

		prevClose := basePrice
		open := prevClose * (1 + (c.rng.Float64()-0.5)*0.01)
		high := math.Max(open, price) * (1 + c.rng.Float64()*0.005)
		low := math.Min(open, price) * (1 - c.rng.Float64()*0.005)

		change := math.Round((price-prevClose)*100) / 100
		changePercent := math.Round((price-prevClose)/prevClose*10000) / 100

		volume := int64(10000000 + c.rng.Int63n(50000000))
		amount := math.Round(price*float64(volume)/10000) / 100

		// Store price for next iteration
		c.lastPrices[symbol] = price

		md := MarketData{
			Symbol:        symbol,
			Name:          meta.Name,
			Price:         price,
			Open:          math.Round(open*100) / 100,
			High:          math.Round(high*100) / 100,
			Low:           math.Round(low*100) / 100,
			PreClose:      prevClose,
			Volume:        volume,
			Amount:        amount,
			Change:        change,
			ChangePercent: changePercent,
			Timestamp:     now,
			MarketCode:    c.marketCode,
		}
		result = append(result, md)
	}

	log.Printf("[AKShare] Generated %d mock quotes (market=%s)", len(result), c.marketCode)
	return result
}

func parseFloatSafe(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}

func parseInt64Safe(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		// Try float first
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return 0
		}
		return int64(f)
	}
	return v
}