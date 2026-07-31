package services

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

// TencentCollector implements MarketDataCollector using Tencent Finance API.
// This is the primary data source for A-share stocks.
// API: https://qt.gtimg.cn/q=sh600519,sz000001
type TencentCollector struct {
	marketCode string
	httpClient *http.Client
}

// NewTencentCollector creates a new Tencent Finance collector.
func NewTencentCollector(marketCode string) *TencentCollector {
	return &TencentCollector{
		marketCode: marketCode,
		httpClient: newPooledHTTPClient(),
	}
}

// GetMarketCode returns the market code.
func (c *TencentCollector) GetMarketCode() string {
	return c.marketCode
}

// tencentSymbol converts internal symbol to Tencent Finance format.
// CN: "600519" -> "sh600519", "000001" -> "sz000001"
func (c *TencentCollector) tencentSymbol(symbol string) string {
	upper := strings.ToUpper(symbol)
	if strings.HasPrefix(upper, "6") || strings.HasPrefix(upper, "9") {
		return "sh" + symbol
	}
	return "sz" + symbol
}

// FetchRealTimeData fetches real-time market data from Tencent Finance.
// Tencent returns GBK-encoded semi-colon-delimited lines.
//
// Response format per line:
//
//	v_sh600519="1~名称~代码~最新价~昨收~今开~成交量(手)~外盘~内盘~...~31涨跌额~32涨跌幅%~33最高~34最低~...~36成交量~37成交额(万)~38换手率~39市盈率~...~44流通市值~45总市值~46市净率";
func (c *TencentCollector) FetchRealTimeData(symbols []string) ([]MarketData, error) {
	log.Printf("[Tencent] Fetching real-time data for %d symbols (market=%s)", len(symbols), c.marketCode)

	var codes []string
	for _, s := range symbols {
		codes = append(codes, c.tencentSymbol(s))
	}

	url := "https://qt.gtimg.cn/q=" + strings.Join(codes, ",")
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Tencent returns GBK-encoded content
	gbkReader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(gbkReader)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	text := string(body)
	lines := strings.Split(text, "\n")

	result := make([]MarketData, 0, len(symbols))
	now := time.Now()

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.Contains(line, "=\"") {
			continue
		}

		// Extract data between quotes
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

		parts := strings.Split(dataStr, "~")
		if len(parts) < 40 {
			continue
		}

		symbol := ""
		if i < len(symbols) {
			symbol = symbols[i]
		}

		md := c.parseLine(parts, symbol, now)
		if md.Price == 0 {
			continue
		}

		result = append(result, md)
	}

	log.Printf("[Tencent] Fetched %d real-time quotes (market=%s)", len(result), c.marketCode)
	return result, nil
}

// parseLine parses a single Tencent Finance response line.
// Fields (0-indexed): 1=名称, 2=代码, 3=最新价, 4=昨收, 5=今开, 33=最高, 34=最低,
// 31=涨跌额, 32=涨跌幅%, 6/36=成交量, 37=成交额(万), 38=换手率, 39=市盈率, 46=市净率,
// 44=流通市值, 45=总市值
func (c *TencentCollector) parseLine(parts []string, symbol string, now time.Time) MarketData {
	price := parseFloatSafe(parts[3])
	prevClose := parseFloatSafe(parts[4])
	open := parseFloatSafe(parts[5])
	high := parseFloatSafe(parts[33])
	low := parseFloatSafe(parts[34])
	change := parseFloatSafe(parts[31])
	changePercent := parseFloatSafe(parts[32])
	volume := parseInt64Safe(parts[6]) // 手
	amount := parseFloatSafe(parts[37]) * 10000 // 万元 → 元
	turnoverRate := parseFloatSafe(parts[38])
	pe := parseFloatSafe(parts[39])
	pb := parseFloatSafe(parts[46])
	totalMarketCap := parseFloatSafe(parts[45])
	floatMarketCap := parseFloatSafe(parts[44])

	// If price fields are zero, compute from other fields
	if price == 0 && prevClose != 0 && change != 0 {
		price = prevClose + change
	}
	if changePercent == 0 && prevClose != 0 && price != 0 {
		changePercent = (price - prevClose) / prevClose * 100
	}

	return MarketData{
		Symbol:         symbol,
		Name:           parts[1],
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

// FetchHistoricalData fetches historical K-line data.
// Tencent doesn't have a simple historical API, so we delegate to Sina.
func (c *TencentCollector) FetchHistoricalData(symbol string, start, end time.Time, interval string) ([]MarketData, error) {
	log.Printf("[Tencent] Historical data not supported, delegating to Sina for %s", symbol)
	return nil, fmt.Errorf("tencent collector does not support historical data, use sina collector")
}

