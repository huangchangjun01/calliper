package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// AKShareCollector implements MarketDataCollector using Sina Finance API.
// Supports A-share (CN), US stocks, and HK stocks.
type AKShareCollector struct {
	marketCode   string
	mlServiceURL string
	httpClient   *http.Client
}

// NewAKShareCollector creates a new AKShare collector for the given market.
func NewAKShareCollector(mlServiceURL string, marketCode string) *AKShareCollector {
	return &AKShareCollector{
		marketCode:   marketCode,
		mlServiceURL: mlServiceURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
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
func (c *AKShareCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[AKShare] Fetching real-time data for %d symbols from Sina Finance (market=%s)", len(symbols), c.marketCode)

	var sinaCodes []string
	for _, s := range symbols {
		sinaCodes = append(sinaCodes, c.sinaSymbol(s))
	}

	url := fmt.Sprintf("https://hq.sinajs.cn/list=%s", strings.Join(sinaCodes, ","))
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[AKShare] HTTP error: %v, falling back to empty data", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	text := string(body)
	lines := strings.Split(text, "\n")

	var result []MarketData
	now := time.Now()

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse Sina Finance format: var hq_str_XXXX="data,fields,..."
		idx := strings.Index(line, "\"")
		if idx < 0 {
			continue
		}
		dataStr := line[idx+1:]
		endIdx := strings.LastIndex(dataStr, "\"")
		if endIdx < 0 {
			continue
		}
		dataStr = dataStr[:endIdx]

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
			continue
		}

		if md.Price.IsZero() {
			continue
		}

		result = append(result, md)
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

	price := decimal.NewFromFloat(parseFloatSafe(parts[3]))
	open := decimal.NewFromFloat(parseFloatSafe(parts[1]))
	prevClose := decimal.NewFromFloat(parseFloatSafe(parts[2]))
	high := decimal.NewFromFloat(parseFloatSafe(parts[4]))
	low := decimal.NewFromFloat(parseFloatSafe(parts[5]))
	volume := parseInt64Safe(parts[8])
	amount := parseFloatSafe(parts[9])

	change := price.Sub(prevClose)
	changePercent := float64(0)
	if !prevClose.IsZero() {
		changePercent, _ = change.Div(prevClose).Mul(decimal.NewFromInt(100)).Float64()
	}

	md := MarketData{
		Symbol:        symbol,
		Name:          parts[0],
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

	// Parse bid/ask if available
	if len(parts) > 29 {
		bidPrices := make([]decimal.Decimal, 5)
		bidVolumes := make([]int64, 5)
		askPrices := make([]decimal.Decimal, 5)
		askVolumes := make([]int64, 5)

		for j := 0; j < 5; j++ {
			bidPrices[j] = decimal.NewFromFloat(parseFloatSafe(parts[11+j*2]))
			bidVolumes[j] = parseInt64Safe(parts[10+j*2])
			askPrices[j] = decimal.NewFromFloat(parseFloatSafe(parts[21+j*2]))
			askVolumes[j] = parseInt64Safe(parts[20+j*2])
		}
		md.BidPrices = bidPrices
		md.BidVolumes = bidVolumes
		md.AskPrices = askPrices
		md.AskVolumes = askVolumes
	}

	return md
}

// parseUSData parses US stock market data from Sina response.
// Format: name(0), price(1), change_percent(2), change(3), ...
func (c *AKShareCollector) parseUSData(parts []string, symbol string, now time.Time) MarketData {
	name := parts[0]
	price := decimal.NewFromFloat(parseFloatSafe(parts[1]))
	changePercent := parseFloatSafe(parts[2])
	change := decimal.NewFromFloat(parseFloatSafe(parts[3]))

	md := MarketData{
		Symbol:        symbol,
		Name:          name,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Timestamp:     now,
		MarketCode:    c.marketCode,
	}
	return md
}

// parseHKData parses HK stock market data from Sina response.
// Format: name(0), open(1), prev_close(2), price(3), high(4), low(5), ...
func (c *AKShareCollector) parseHKData(parts []string, symbol string, now time.Time) MarketData {
	if len(parts) < 10 {
		return MarketData{}
	}

	name := parts[1]
	price := decimal.NewFromFloat(parseFloatSafe(parts[4]))
	open := decimal.NewFromFloat(parseFloatSafe(parts[2]))
	prevClose := decimal.NewFromFloat(parseFloatSafe(parts[3]))
	high := decimal.NewFromFloat(parseFloatSafe(parts[5]))
	low := decimal.NewFromFloat(parseFloatSafe(parts[6]))
	change := decimal.NewFromFloat(parseFloatSafe(parts[7]))
	changePercent := parseFloatSafe(parts[8])

	md := MarketData{
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
	return md
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
		log.Printf("[AKShare] JSON parse error: %v, body: %s", err, string(body[:min(len(body), 200)]))
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

		open := decimal.NewFromFloat(parseFloatSafe(item.Open))
		high := decimal.NewFromFloat(parseFloatSafe(item.High))
		low := decimal.NewFromFloat(parseFloatSafe(item.Low))
		close := decimal.NewFromFloat(parseFloatSafe(item.Close))
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

		if !md.PreClose.IsZero() {
			md.Change = md.Price.Sub(md.PreClose)
			md.ChangePercent, _ = md.Change.Div(md.PreClose).Mul(decimal.NewFromInt(100)).Float64()
		}

		result = append(result, md)
	}

	log.Printf("[AKShare] Fetched %d historical K-line records for %s", len(result), symbol)
	return result, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
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