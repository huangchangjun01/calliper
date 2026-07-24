package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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

	data, err := h.marketService.CollectMarketData(c.Request.Context(), marketCode)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data: " + err.Error()})
		return
	}

	// Filter for the requested symbol
	for _, md := range data {
		if md.Symbol == symbol {
			c.JSON(http.StatusOK, gin.H{
				"data": md,
			})
			return
		}
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

	// Group symbols by market code
	marketGroups := make(map[string][]string)
	for _, symbol := range req.Symbols {
		code := h.detectMarketCode(symbol)
		marketGroups[code] = append(marketGroups[code], symbol)
	}

	var allData []services.MarketData
	for code := range marketGroups {
		data, err := h.marketService.CollectMarketData(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch market data for " + code + ": " + err.Error()})
			return
		}
		allData = append(allData, data...)
	}

	c.JSON(http.StatusOK, gin.H{
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

	c.JSON(http.StatusOK, gin.H{
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
			c.JSON(http.StatusOK, gin.H{
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
	// HK stocks
	if len(symbol) >= 5 && symbol[len(symbol)-3:] == ".HK" {
		return "HK"
	}

	// Check if purely numeric (A-share)
	isNumeric := true
	for _, ch := range symbol {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric && len(symbol) == 6 {
		return "CN"
	}

	// Default to US
	return "US"
}