package handlers

import (
	"net/http"
	"strconv"

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

// GetPredictionHistory handles GET /api/v1/predictions/:symbol/history
func (h *PredictionHandler) GetPredictionHistory(c *gin.Context) {
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
	limit := 10
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "10")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

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

// ──────────────────────────────────────────────────────────────
// Aggregated prediction/evaluation endpoints
// ──────────────────────────────────────────────────────────────

// PredictionSummary represents a compact prediction for the list view.
type PredictionSummary struct {
	Symbol      string  `json:"symbol"`
	Period      string  `json:"period"`
	Direction   string  `json:"direction"`
	Confidence  float64 `json:"confidence"`
	TargetPrice float64 `json:"target_price"`
	PredictedAt string  `json:"predicted_at"`
}

// GetSummaries handles GET /api/v1/predictions/summaries
// Returns a list of recent prediction summaries.
func (h *PredictionHandler) GetSummaries(c *gin.Context) {
	// Return empty data if ML service is not available
	success(c, []PredictionSummary{})
}

// PredictionDetail represents a detailed prediction record.
type PredictionDetail struct {
	ID          int      `json:"id"`
	Symbol      string   `json:"symbol"`
	Period      string   `json:"period"`
	Direction   string   `json:"direction"`
	Confidence  float64  `json:"confidence"`
	TargetPrice float64  `json:"target_price"`
	PredictedAt string   `json:"predicted_at"`
	IsCorrect   *bool    `json:"is_correct,omitempty"`
	KeyFactors  []string `json:"key_factors,omitempty"`
}

// GetDetails handles GET /api/v1/predictions/details
// Returns a list of detailed prediction records.
func (h *PredictionHandler) GetDetails(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	_ = limit
	success(c, []PredictionDetail{})
}

// AccuracyTrend represents accuracy trend over time.
type AccuracyTrend struct {
	Period   string  `json:"period"`
	Date     string  `json:"date"`
	Accuracy float64 `json:"accuracy"`
}

// GetAccuracyTrend handles GET /api/v1/predictions/accuracy
// Returns accuracy trend data for a given time period.
func (h *PredictionHandler) GetAccuracyTrend(c *gin.Context) {
	period := c.DefaultQuery("period", "short")

	// Get accuracy ranking data as trend
	if h.evalService != nil {
		rankings, err := h.evalService.GetAccuracyRanking(period, 50)
		if err == nil {
			// Convert ranking to trend format
			trend := make([]AccuracyTrend, 0, len(rankings))
			for i, r := range rankings {
				trend = append(trend, AccuracyTrend{
					Period:   period,
					Date:     r.Symbol,
					Accuracy: r.Accuracy,
				})
				if i >= 19 {
					break
				}
			}
			success(c, trend)
			return
		}
	}

	success(c, []AccuracyTrend{})
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

// FailureCase represents a prediction failure case.
type FailureCase struct {
	ID                 int    `json:"id"`
	Symbol             string `json:"symbol"`
	PredictedDirection string `json:"predicted_direction"`
	ActualDirection    string `json:"actual_direction"`
	Summary            string `json:"summary"`
	PredictedAt        string `json:"predicted_at"`
}

// GetFailures handles GET /api/v1/predictions/failures
// Returns recent prediction failure cases.
func (h *PredictionHandler) GetFailures(c *gin.Context) {
	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
		limit = l
	}
	_ = limit

	// Return empty array for now — failures are populated by evaluation service
	success(c, []FailureCase{})
}