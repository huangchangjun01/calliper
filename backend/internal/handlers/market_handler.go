package handlers

import (
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"

	"github.com/quant-trading/backend/internal/services"
)

// MarketHandler handles market data HTTP requests.
type MarketHandler struct {
	marketService *services.MarketDataService
	backfill      *services.HistoryBackfill
}

// NewMarketHandler creates a new MarketHandler.
func NewMarketHandler(marketService *services.MarketDataService) *MarketHandler {
	return &MarketHandler{
		marketService: marketService,
		backfill:      services.NewHistoryBackfill(marketService, 4),
	}
}

// GetRealtime returns real-time market data for a single symbol.
// GET /api/v1/market/realtime/:symbol
func (h *MarketHandler) GetRealtime(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	// Determine market code from symbol
	marketCode := h.detectMarketCode(symbol)

	data, err := h.marketService.CollectMarketDataForSymbols(c.Request.Context(), marketCode, []string{symbol})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data: " + err.Error()})
		return
	}

	if len(data) > 0 {
		success(c, gin.H{
			"data": data[0],
		})
		return
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "symbol not found"})
}

// BatchRealtimeRequest represents a batch real-time data request.
type BatchRealtimeRequest struct {
	Symbols []string `json:"symbols" binding:"required"`
}

// GetRealtimeBatch returns real-time market data for multiple symbols.
// POST /api/v1/market/realtime/batch
func (h *MarketHandler) GetRealtimeBatch(c *gin.Context) {
	var req BatchRealtimeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Limit to 50 symbols to avoid excessive API calls
	if len(req.Symbols) > 50 {
		req.Symbols = req.Symbols[:50]
	}

	// Group symbols by market code
	marketGroups := make(map[string][]string)
	for _, symbol := range req.Symbols {
		code := h.detectMarketCode(symbol)
		marketGroups[code] = append(marketGroups[code], symbol)
	}

	var allData []services.MarketData
	for code, symbols := range marketGroups {
		data, err := h.marketService.CollectMarketDataForSymbols(c.Request.Context(), code, symbols)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data for " + code + ": " + err.Error()})
			return
		}
		allData = append(allData, data...)
	}

	success(c, gin.H{
		"data":  allData,
		"count": len(allData),
	})
}

// GetKline returns kline (candlestick) data for a symbol.
// GET /api/v1/market/kline/:symbol?interval=1m|5m|15m|30m|60m|1d&from=&to=
func (h *MarketHandler) GetKline(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	interval := c.DefaultQuery("interval", "1d")
	fromStr := c.DefaultQuery("from", "")
	toStr := c.DefaultQuery("to", "")

	// Validate interval
	validIntervals := map[string]bool{
		"1m": true, "5m": true, "15m": true, "30m": true, "60m": true, "1h": true, "1d": true,
	}
	if !validIntervals[interval] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid interval, supported: 1m,5m,15m,30m,60m,1d"})
		return
	}

	// Parse time range
	from := time.Now().AddDate(0, -1, 0) // Default: 1 month ago
	to := time.Now()

	if fromStr != "" {
		parsed, err := time.Parse("2006-01-02", fromStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid from date format, use YYYY-MM-DD"})
			return
		}
		from = parsed
	}
	if toStr != "" {
		parsed, err := time.Parse("2006-01-02", toStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid to date format, use YYYY-MM-DD"})
			return
		}
		to = parsed
	}

	// Determine collector
	collectors := h.marketService.GetCollectors()
	marketCode := h.detectMarketCode(symbol)
	collector, ok := collectors[marketCode]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported market"})
		return
	}

	data, err := collector.FetchHistoricalData(symbol, from, to, interval)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch kline data: " + err.Error()})
		return
	}

	// Clean the data
	cleaned := h.marketService.GetCleaner().CleanMarketData(data)

	success(c, gin.H{
		"symbol":   symbol,
		"interval": interval,
		"from":     from.Format("2006-01-02"),
		"to":       to.Format("2006-01-02"),
		"data":     cleaned,
		"count":    len(cleaned),
	})
}

// GetDepth returns market depth (order book) data for a symbol.
// GET /api/v1/market/depth/:symbol
func (h *MarketHandler) GetDepth(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "symbol is required"})
		return
	}

	marketCode := h.detectMarketCode(symbol)
	data, err := h.marketService.CollectMarketData(c.Request.Context(), marketCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch depth data: " + err.Error()})
		return
	}

	for _, md := range data {
		if md.Symbol == symbol {
			success(c, gin.H{
				"symbol":      md.Symbol,
				"bid_prices":  md.BidPrices,
				"bid_volumes": md.BidVolumes,
				"ask_prices":  md.AskPrices,
				"ask_volumes": md.AskVolumes,
				"timestamp":   md.Timestamp,
			})
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "symbol not found"})
}

// BackfillRequest represents a backfill request.
type BackfillRequest struct {
	Symbols  []string `json:"symbols" binding:"required"`
	DataType string   `json:"data_type"` // "daily" or "minute"
	Years    int      `json:"years"`     // for daily data
	Months   int      `json:"months"`    // for minute data
}

// TriggerBackfill triggers a historical data backfill task.
// POST /api/v1/market/backfill
func (h *MarketHandler) TriggerBackfill(c *gin.Context) {
	var req BackfillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	if req.DataType == "" {
		req.DataType = "daily"
	}
	if req.Years <= 0 && req.Months <= 0 {
		req.Years = 3
		req.Months = 6
	}

	// Run backfill in background
	go func() {
		var err error
		switch req.DataType {
		case "daily":
			err = h.backfill.BackfillDailyData(c.Request.Context(), req.Symbols, req.Years)
		case "minute":
			err = h.backfill.BackfillMinuteData(c.Request.Context(), req.Symbols, req.Months)
		default:
			err = h.backfill.BackfillDailyData(c.Request.Context(), req.Symbols, req.Years)
		}
		if err != nil {
			// Error already logged in backfill
		}
	}()

	c.JSON(http.StatusAccepted, gin.H{
		"message":   "backfill task started",
		"data_type": req.DataType,
		"symbols":   len(req.Symbols),
	})
}

// GetBackfillProgress returns the progress of backfill tasks.
// GET /api/v1/market/backfill/progress
func (h *MarketHandler) GetBackfillProgress(c *gin.Context) {
	progress := h.backfill.GetProgress()
	c.JSON(http.StatusOK, gin.H{
		"progress": progress,
	})
}

// detectMarketCode determines the market code from a symbol.
func (h *MarketHandler) detectMarketCode(symbol string) string {
	upper := strings.ToUpper(symbol)

	// Suffix-based detection (case-insensitive)
	if strings.HasSuffix(upper, ".SH") || strings.HasSuffix(upper, ".SZ") || strings.HasSuffix(upper, ".BJ") {
		return "CN"
	}
	if strings.HasSuffix(upper, ".HK") {
		return "HK"
	}
	if strings.HasSuffix(upper, ".T") {
		return "JP"
	}
	if strings.HasSuffix(upper, ".L") {
		return "UK"
	}
	if strings.HasSuffix(upper, ".PA") || strings.HasSuffix(upper, ".DE") || strings.HasSuffix(upper, ".AS") {
		return "EU"
	}

	// Pure numeric codes
	isNumeric := true
	for _, ch := range symbol {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric {
		if len(symbol) == 6 {
			return "CN" // A-share
		}
		if len(symbol) == 5 {
			return "HK" // HK 5-digit
		}
	}

	// Pure alphabetic → US
	return "US"
}

// ──────────────────────────────────────────────────────────────
// Market indices (real Sina Finance API)
// ──────────────────────────────────────────────────────────────

// indexDef maps a display symbol to a Sina Finance index code and parser type.
type indexDef struct {
	DisplaySymbol string // e.g. "000001.SH"
	SinaCode      string // e.g. "sh000001"
	Type          string // "CN", "HK", "US", "EU"
}

var defaultIndices = []indexDef{
	{"000001.SH", "sh000001", "CN"},
	{"399001.SZ", "sz399001", "CN"},
	{"399006.SZ", "sz399006", "CN"},
	{"HSI", "rt_hkHSI", "HK"},
	{"DJI", "int_dji", "US"},
	{"IXIC", "int_nasdaq", "US"},
	{"SPX", "int_sp500", "US"},
	{"DAX", "b_DAX", "EU"},
	{"CAC", "b_CAC", "EU"},
}

// IndexData is the JSON-serialisable index quote returned by the API.
type IndexData struct {
	Symbol        string  `json:"symbol"`
	Name          string  `json:"name"`
	Price         float64 `json:"price"`
	Change        float64 `json:"change"`
	ChangePercent float64 `json:"changePercent"`
	Volume        int64   `json:"volume"`
	Timestamp     int64   `json:"timestamp"`
}

// GetIndices returns real-time market index data from Sina Finance.
// GET /api/v1/market/indices
func (h *MarketHandler) GetIndices(c *gin.Context) {
	// Determine which indices to fetch
	indices := defaultIndices
	if symParam := c.Query("symbols"); symParam != "" {
		requested := strings.Split(symParam, ",")
		var filtered []indexDef
		for _, req := range requested {
			req = strings.TrimSpace(req)
			for _, def := range defaultIndices {
				if strings.EqualFold(def.DisplaySymbol, req) {
					filtered = append(filtered, def)
					break
				}
			}
		}
		if len(filtered) > 0 {
			indices = filtered
		}
	}

	result := h.fetchSinaIndices(indices)
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// fetchSinaIndices calls the Sina Finance realtime API once for all indices.
func (h *MarketHandler) fetchSinaIndices(indices []indexDef) []IndexData {
	if len(indices) == 0 {
		return []IndexData{}
	}

	// Build the Sina symbol list
	var sinaCodes []string
	for _, idx := range indices {
		sinaCodes = append(sinaCodes, idx.SinaCode)
	}
	url := "https://hq.sinajs.cn/list=" + strings.Join(sinaCodes, ",")

	client := &http.Client{Timeout: 8 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Printf("[GetIndices] failed to create request: %v", err)
		return []IndexData{}
	}
	req.Header.Set("Referer", "https://finance.sina.com.cn")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[GetIndices] failed to call Sina API: %v", err)
		return []IndexData{}
	}
	defer resp.Body.Close()

	// Sina returns GBK-encoded content
	reader := transform.NewReader(resp.Body, simplifiedchinese.GBK.NewDecoder())
	body, err := io.ReadAll(reader)
	if err != nil {
		log.Printf("[GetIndices] failed to read response: %v", err)
		return []IndexData{}
	}

	lines := strings.Split(string(body), "\n")
	now := time.Now().Unix()

	var result []IndexData
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Format: var hq_str_sh000001="...";
		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			continue
		}
		sinaKey := line[:eqIdx]                         // var hq_str_sh000001
		sinaKey = strings.TrimPrefix(sinaKey, "var hq_str_") // sh000001
		quoteStart := strings.Index(line, "\"")
		quoteEnd := strings.LastIndex(line, "\"")
		if quoteStart < 0 || quoteEnd <= quoteStart {
			continue
		}
		content := line[quoteStart+1 : quoteEnd]
		if content == "" {
			continue
		}

		// Find the matching index definition
		var def *indexDef
		for i := range indices {
			if indices[i].SinaCode == sinaKey {
				def = &indices[i]
				break
			}
		}
		if def == nil {
			continue
		}

		if idx, ok := parseSinaIndexLine(def, content, now); ok {
			result = append(result, idx)
		}
	}

	return result
}

// parseSinaIndexLine parses a single Sina index line based on its type.
func parseSinaIndexLine(def *indexDef, content string, nowTs int64) (IndexData, bool) {
	parts := strings.Split(content, ",")
	if len(parts) < 4 {
		return IndexData{}, false
	}

	var name string
	var price, change, changePercent float64
	var volume int64

	switch def.Type {
	case "CN":
		// 名称(0),今开(1),昨收(2),最新价(3),最高(4),最低(5),买(6),卖(7),成交量(8),...
		name = parts[0]
		price = parseFloatSafe(parts[3])
		prevClose := parseFloatSafe(parts[2])
		if prevClose > 0 {
			change = price - prevClose
			changePercent = change / prevClose * 100
		}
		if len(parts) > 8 {
			volume = parseInt64Safe(parts[8])
		}
	case "HK":
		// 代码(0),名称(1),今开(2),昨收(3),最新价(4),最高(5),最低(6),涨跌额(7),涨跌幅(8),...
		if len(parts) < 9 {
			return IndexData{}, false
		}
		name = parts[1]
		price = parseFloatSafe(parts[4])
		change = parseFloatSafe(parts[7])
		changePercent = parseFloatSafe(parts[8])
	case "US":
		// 名称(0),最新价(1),涨跌额(2),涨跌幅(3)
		name = parts[0]
		price = parseFloatSafe(parts[1])
		change = parseFloatSafe(parts[2])
		changePercent = parseFloatSafe(parts[3])
	case "EU":
		// 名称(0),最新价(1),涨跌额(2),涨跌幅(3),...
		name = parts[0]
		price = parseFloatSafe(parts[1])
		change = parseFloatSafe(parts[2])
		changePercent = parseFloatSafe(parts[3])
	default:
		return IndexData{}, false
	}

	if price <= 0 {
		return IndexData{}, false
	}

	return IndexData{
		Symbol:        def.DisplaySymbol,
		Name:          name,
		Price:         price,
		Change:        change,
		ChangePercent: changePercent,
		Volume:        volume,
		Timestamp:     nowTs,
	}, true
}

// GetMarketStatistics returns aggregate market statistics.
// GET /api/v1/market/statistics
func (h *MarketHandler) GetMarketStatistics(c *gin.Context) {
	success(c, gin.H{
		"limitUpCount":   0,
		"limitDownCount": 0,
		"upCount":        0,
		"downCount":      0,
		"flatCount":      0,
		"totalAmount":    0,
	})
}

// GetFundamentals returns fundamental data for a stock.
// GET /api/v1/market/fundamentals/:symbol
func (h *MarketHandler) GetFundamentals(c *gin.Context) {
	success(c, gin.H{
		"marketCap":     0,
		"pe":            0,
		"pb":            0,
		"roe":           0,
		"debtRatio":     0,
		"currentRatio":  0,
		"eps":           0,
		"dividendYield": 0,
	})
}

// parseFloatSafe parses a string to float64, returning 0 on error.
func parseFloatSafe(s string) float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return v
}

// parseInt64Safe parses a string to int64, returning 0 on error.
func parseInt64Safe(s string) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return v
}