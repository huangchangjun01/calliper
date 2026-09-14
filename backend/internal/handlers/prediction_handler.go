package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/services"
)

// PredictionHandler handles HTTP requests for stock predictions.
type PredictionHandler struct {
	predictionService *services.PredictionService
	evalService       *services.EvaluationService
}

// NewPredictionHandler creates a new PredictionHandler.
func NewPredictionHandler(svc *services.PredictionService) *PredictionHandler {
	return &PredictionHandler{predictionService: svc}
}

// SetEvaluationService sets the evaluation service for accuracy-related endpoints.
func (h *PredictionHandler) SetEvaluationService(svc *services.EvaluationService) {
	h.evalService = svc
}

// ──────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────

// GetPrediction handles GET /api/v1/predictions/:symbol
func (h *PredictionHandler) GetPrediction(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	result, err := h.predictionService.GetPrediction(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, result)
}

// BatchPredict handles POST /api/v1/predictions/batch
func (h *PredictionHandler) BatchPredict(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	var req services.BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid request body: "+err.Error())
		return
	}

	req.Symbols = dedupSymbols(req.Symbols)
	if len(req.Symbols) == 0 {
		fail(c, http.StatusBadRequest, 40001, "symbols list is required")
		return
	}

	results, err := h.predictionService.GetBatchPredictions(req.Symbols)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, results)
}

// GetSymbolHistory handles GET /api/v1/predictions/:symbol/history
func (h *PredictionHandler) GetSymbolHistory(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	period := c.DefaultQuery("period", "short_term")
	limit, _, _ := parsePageLimit(c.DefaultQuery("limit", "10"), c.DefaultQuery("offset", "0"), 10)

	history, err := h.predictionService.GetPredictionHistory(symbol, period, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, history)
}

// GetPredictionAccuracy handles GET /api/v1/predictions/accuracy/:symbol
func (h *PredictionHandler) GetPredictionAccuracy(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	report, err := h.predictionService.GetPredictionAccuracy(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, report)
}

// TriggerPrediction handles POST /api/v1/predictions/run (admin)
func (h *PredictionHandler) TriggerPrediction(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	if err := h.predictionService.TriggerPrediction(); err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"status":  "success",
		"message": "Daily prediction task triggered",
	})
}

// GetModelStatus handles GET /api/v1/models/status (admin)
func (h *PredictionHandler) GetModelStatus(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	statuses, err := h.predictionService.GetModelStatus()
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, statuses)
}

// GeneratePredictionsRequest is the request body for generating predictions.
type GeneratePredictionsRequest struct {
	Symbols []string `json:"symbols"`
}

// dedupSymbols trims whitespace and removes duplicate symbols while preserving
// the original request order. Comparison is case-insensitive.
func dedupSymbols(symbols []string) []string {
	seen := make(map[string]struct{}, len(symbols))
	out := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

// GeneratePredictions handles POST /api/v1/predictions/generate
// Fetches predictions from the ML service and persists them to the database
// so the prediction page can display them.
func (h *PredictionHandler) GeneratePredictions(c *gin.Context) {
	if h.predictionService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "prediction service not available")
		return
	}

	var req GeneratePredictionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid request body: "+err.Error())
		return
	}
	req.Symbols = dedupSymbols(req.Symbols)
	if len(req.Symbols) == 0 {
		fail(c, http.StatusBadRequest, 40001, "symbols list is required")
		return
	}

	results, err := h.predictionService.GenerateAndPersist(req.Symbols)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	persisted := 0
	for _, r := range results {
		persisted += r.Count
	}

	success(c, gin.H{
		"status":    "success",
		"results":   results,
		"persisted": persisted,
	})
}

// ──────────────────────────────────────────────────────────────
// Aggregated prediction/evaluation endpoints
// ──────────────────────────────────────────────────────────────

// GetSummaries handles GET /api/v1/predictions/summaries
// Returns per-period aggregate counts computed from real prediction records.
func (h *PredictionHandler) GetSummaries(c *gin.Context) {
	if h.evalService == nil {
		success(c, []services.PredictionPeriodSummary{})
		return
	}
	success(c, h.evalService.GetPredictionSummaries())
}

// GetDetails handles GET /api/v1/predictions/details
// Returns real prediction records filtered and paginated.
// Query params: period, direction, confidence_min, expired(true|false|pending),
// offset (default 0), limit (default 20, max 100).
func (h *PredictionHandler) GetDetails(c *gin.Context) {
	if h.evalService == nil {
		success(c, gin.H{"items": []interface{}{}, "total": 0, "limit": 0, "offset": 0})
		return
	}

	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "20"), c.DefaultQuery("offset", "0"), 20)

	filter := services.PredictionFilter{
		Period:    c.Query("period"),
		Direction: c.Query("direction"),
		Expired:   c.Query("expired"),
	}
	if cm, err := strconv.ParseFloat(c.DefaultQuery("confidence_min", "0"), 64); err == nil && cm > 0 {
		filter.ConfidenceMin = cm
	}

	items, total, err := h.evalService.ListPredictions(filter, offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	success(c, gin.H{"items": items, "total": total, "limit": limit, "offset": offset})
}

// GetPredictionHistory handles GET /api/v1/predictions/history
// Returns the prediction history list joined with stock symbol/name.
// Query params: symbol, period, status (pending|correct|wrong),
// from/to (YYYY-MM-DD, inclusive), offset (default 0), limit (default 20, max 100).
func (h *PredictionHandler) GetPredictionHistory(c *gin.Context) {
	if h.evalService == nil {
		success(c, gin.H{"items": []interface{}{}, "total": 0, "limit": 0, "offset": 0})
		return
	}

	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "20"), c.DefaultQuery("offset", "0"), 20)

	filter := services.HistoryFilter{
		Symbol: c.Query("symbol"),
		Period: c.Query("period"),
		Status: c.Query("status"),
		From:   c.Query("from"),
		To:     c.Query("to"),
	}

	items, total, err := h.evalService.ListPredictionHistory(filter, offset, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	success(c, gin.H{"items": items, "total": total, "limit": limit, "offset": offset})
}

// GetPredictionStats handles GET /api/v1/predictions/stats
// Returns per-bucket (day/week/month) prediction counts and accuracy.
// Buckets without evaluated predictions carry a null accuracy (断点).
// Query params: bucket (day|week|month, default day), from/to (YYYY-MM-DD,
// default last 30 days).
func (h *PredictionHandler) GetPredictionStats(c *gin.Context) {
	if h.evalService == nil {
		success(c, []services.BucketStat{})
		return
	}

	bucket := c.DefaultQuery("bucket", "day")
	switch bucket {
	case "day", "week", "month":
	default:
		bucket = "day"
	}

	to := time.Now()
	from := to.AddDate(0, 0, -30)
	if v := c.Query("to"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			to = t
		}
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.ParseInLocation("2006-01-02", v, time.Local); err == nil {
			from = t
		}
	}

	stats, err := h.evalService.GetPredictionStats(bucket, from, to)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	success(c, stats)
}

// GetAccuracyTrend handles GET /api/v1/predictions/accuracy
// Returns a real accuracy time-series trend (date -> accuracy) computed from
// PredictionAccuracy. Every date in the range is returned; dates without
// evaluated predictions have accuracy: null (断点，JSON null).
// Query params: period (short|medium|long), days (default 30).
func (h *PredictionHandler) GetAccuracyTrend(c *gin.Context) {
	if h.evalService == nil {
		success(c, []services.AccuracyTrendItem{})
		return
	}

	period := c.DefaultQuery("period", "short")
	days := 30
	if d, err := strconv.Atoi(c.DefaultQuery("days", "30")); err == nil && d > 0 && d <= 365 {
		days = d
	}

	items, err := h.evalService.GetAccuracyTrendData(period, days)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	success(c, items)
}

// StockAccuracyItem represents per-stock accuracy data.
type StockAccuracyItem struct {
	Symbol           string  `json:"symbol"`
	Accuracy         float64 `json:"accuracy"`
	TotalPredictions int     `json:"total_predictions"`
}

// GetStockAccuracy handles GET /api/v1/predictions/stock-accuracy
// Returns accuracy ranking across all stocks.
func (h *PredictionHandler) GetStockAccuracy(c *gin.Context) {
	if h.evalService != nil {
		rankings, err := h.evalService.GetAccuracyRanking("all", 50)
		if err == nil {
			items := make([]StockAccuracyItem, 0, len(rankings))
			for _, r := range rankings {
				items = append(items, StockAccuracyItem{
					Symbol:           r.Symbol,
					Accuracy:         r.Accuracy,
					TotalPredictions: r.TotalPredictions,
				})
			}
			success(c, items)
			return
		}
	}

	success(c, []StockAccuracyItem{})
}

// GetFailures handles GET /api/v1/predictions/failures
// Returns recent wrong predictions (Success=false) joined with actual direction.
// Query params: limit (default 20).
func (h *PredictionHandler) GetFailures(c *gin.Context) {
	if h.evalService == nil {
		success(c, []services.FailureItem{})
		return
	}

	limit, _, _ := parsePageLimit(c.DefaultQuery("limit", "20"), c.DefaultQuery("offset", "0"), 20)

	items, err := h.evalService.ListFailures(limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	success(c, items)
}
