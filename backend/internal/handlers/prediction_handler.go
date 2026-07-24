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
}

// NewPredictionHandler creates a new PredictionHandler.
func NewPredictionHandler(svc *services.PredictionService) *PredictionHandler {
	return &PredictionHandler{predictionService: svc}
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