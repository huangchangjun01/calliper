package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/services"
)

// ──────────────────────────────────────────────────────────────
// EvaluationHandler — 评估 HTTP 处理器
// ──────────────────────────────────────────────────────────────

// EvaluationHandler handles HTTP requests for prediction evaluation.
type EvaluationHandler struct {
	evalService *services.EvaluationService
	scheduler   *services.EvaluationScheduler
}

// NewEvaluationHandler creates a new EvaluationHandler.
func NewEvaluationHandler(evalService *services.EvaluationService, scheduler *services.EvaluationScheduler) *EvaluationHandler {
	return &EvaluationHandler{
		evalService: evalService,
		scheduler:   scheduler,
	}
}

// ──────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────

// GetAccuracy handles GET /api/v1/evaluation/accuracy/:symbol
// Returns basic prediction accuracy for a single stock.
func (h *EvaluationHandler) GetAccuracy(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	stats, err := h.evalService.GetAccuracyStats(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, stats)
}

// GetAccuracyStats handles GET /api/v1/evaluation/accuracy/:symbol/stats
// Returns detailed accuracy statistics (7d/30d/cumulative, by period).
func (h *EvaluationHandler) GetAccuracyStats(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	stats, err := h.evalService.GetAccuracyStats(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, stats)
}

// GetRanking handles GET /api/v1/evaluation/ranking
// Returns accuracy ranking across stocks.
// Query params: period (short/medium/long/all), limit (default 20).
func (h *EvaluationHandler) GetRanking(c *gin.Context) {
	period := c.DefaultQuery("period", "all")
	limit := 20
	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	rankings, err := h.evalService.GetAccuracyRanking(period, limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, rankings)
}

// GetMetrics handles GET /api/v1/evaluation/metrics/:symbol
// Returns composite evaluation metrics (excess return, Sharpe, max drawdown).
func (h *EvaluationHandler) GetMetrics(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	metrics, err := h.evalService.GetMetrics(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, metrics)
}

// GetFailureAnalysis handles GET /api/v1/evaluation/failure/:symbol
// Returns failure attribution analysis for a failed prediction.
// Query params: prediction_id (required).
func (h *EvaluationHandler) GetFailureAnalysis(c *gin.Context) {
	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	predIDStr := c.DefaultQuery("prediction_id", "0")
	predID, err := strconv.ParseUint(predIDStr, 10, 64)
	if err != nil || predID == 0 {
		fail(c, http.StatusBadRequest, 40001, "valid prediction_id is required")
		return
	}

	analysis, err := h.evalService.AnalyzeFailure(symbol, uint(predID))
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, analysis)
}

// RunEvaluation handles POST /api/v1/evaluation/run
// Manually triggers a daily evaluation run (admin only).
func (h *EvaluationHandler) RunEvaluation(c *gin.Context) {
	if h.scheduler == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "evaluation scheduler not available")
		return
	}

	if err := h.scheduler.RunEvaluation(); err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"status":  "success",
		"message": "Daily evaluation triggered successfully",
	})
}