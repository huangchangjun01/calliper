package handlers

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/services"
)

// ──────────────────────────────────────────────────────────────
// DashboardHandler — 决策支持大盘（degradable per-card sections）
// ──────────────────────────────────────────────────────────────

// DashboardHandler aggregates decision-support data into independent sections
// so the frontend can degrade per-card without failing the whole request.
type DashboardHandler struct {
	evalService       *services.EvaluationService
	predictionService *services.PredictionService
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(eval *services.EvaluationService, pred *services.PredictionService) *DashboardHandler {
	return &DashboardHandler{evalService: eval, predictionService: pred}
}

// GetDashboard handles GET /api/v1/dashboard (protected).
// Each section is computed independently; a failure in one section sets it to
// an "unavailable" marker while still returning HTTP 200.
func (h *DashboardHandler) GetDashboard(c *gin.Context) {
	threshold := float64(60)
	if h.evalService != nil {
		threshold = h.evalService.GetHighConfidenceThreshold()
	}
	if t, err := strconv.ParseFloat(c.DefaultQuery("threshold", "0"), 64); err == nil && t > 0 {
		threshold = t
	}

	success(c, gin.H{
		"high_confidence": h.section(func() (interface{}, error) {
			// GetHighConfidenceThreshold 为百分数（如 60），而库内 confidence 为 0~1 小数（如 0.61），统一换算后比较。
			return h.evalService.GetHighConfidencePredictions(threshold/100.0, 10)
		}),
		"accuracy_summary": h.section(func() (interface{}, error) {
			return h.evalService.GetAccuracySummary()
		}),
		"risk_alerts": h.section(func() (interface{}, error) {
			return h.evalService.GetRiskAlerts()
		}),
		"system_status": h.buildSystemStatus(),
	})
}

// section runs a dashboard section and converts errors into an unavailable marker.
func (h *DashboardHandler) section(fn func() (interface{}, error)) interface{} {
	if h.evalService == nil {
		return gin.H{"status": "unavailable", "error": "evaluation service not available"}
	}
	v, err := fn()
	if err != nil {
		return gin.H{"status": "unavailable", "error": err.Error()}
	}
	return v
}

// buildSystemStatus assembles the system_status section: last sync heuristic,
// ML model status, thresholds and model health, plus a degraded flag.
func (h *DashboardHandler) buildSystemStatus() interface{} {
	status := gin.H{"degraded": false}

	// Latest stock sync timestamp heuristic.
	if h.evalService != nil {
		if ls := h.evalService.LatestStockSync(); ls != nil {
			status["last_sync"] = ls.Format("2006-01-02 15:04:05")
			if time.Since(*ls) > 24*time.Hour {
				status["stale"] = true
				status["degraded"] = true
			}
		} else {
			status["last_sync"] = nil
			status["stale"] = true
			status["degraded"] = true
		}
	} else {
		status["last_sync"] = nil
		status["stale"] = true
		status["degraded"] = true
	}

	// ML model status (ignore errors).
	var models []services.ModelStatus
	if h.predictionService != nil {
		if ms, err := h.predictionService.GetModelStatus(); err == nil {
			models = ms
		} else {
			status["degraded"] = true
		}
	} else {
		status["degraded"] = true
	}
	status["models"] = models

	// Thresholds & model health.
	if h.evalService != nil {
		status["thresholds"] = h.evalService.GetThresholds()
		status["model_health"] = h.evalService.GetModelHealth()
	} else {
		status["thresholds"] = nil
		status["model_health"] = gin.H{"status": "unavailable"}
		status["degraded"] = true
	}

	return status
}
