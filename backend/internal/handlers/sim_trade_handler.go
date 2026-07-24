package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/services"
)

// SimTradeHandler handles HTTP requests for simulated trading operations.
type SimTradeHandler struct {
	simTradeService *services.SimTradeService
	accountService  *services.AccountService
	positionManager *services.PositionManager
}

// NewSimTradeHandler creates a new SimTradeHandler.
func NewSimTradeHandler(simTradeService *services.SimTradeService, accountService *services.AccountService, positionManager *services.PositionManager) *SimTradeHandler {
	return &SimTradeHandler{
		simTradeService: simTradeService,
		accountService:  accountService,
		positionManager: positionManager,
	}
}

// GetStatus handles GET /api/v1/trading/sim/status
func (h *SimTradeHandler) GetStatus(c *gin.Context) {
	isRunning := h.simTradeService.IsRunning()
	tradeCount := h.simTradeService.GetTodayTradeCount()

	account, err := h.accountService.GetAccount()
	var todayPnL float64
	if err == nil {
		todayPnL = account.TodayPnL
	}

	success(c, gin.H{
		"is_running":        isRunning,
		"today_trade_count": tradeCount,
		"today_pnl":         todayPnL,
	})
}

// StartSimTrading handles POST /api/v1/trading/sim/start
func (h *SimTradeHandler) StartSimTrading(c *gin.Context) {
	if h.simTradeService.IsRunning() {
		success(c, gin.H{"message": "模拟交易已在运行中"})
		return
	}

	h.simTradeService.StartScheduler(c.Request.Context())
	success(c, gin.H{"message": "模拟交易已启动"})
}

// StopSimTrading handles POST /api/v1/trading/sim/stop
func (h *SimTradeHandler) StopSimTrading(c *gin.Context) {
	if !h.simTradeService.IsRunning() {
		success(c, gin.H{"message": "模拟交易未在运行"})
		return
	}

	h.simTradeService.StopScheduler()
	success(c, gin.H{"message": "模拟交易已停止"})
}

// GetDecisions handles GET /api/v1/trading/sim/decisions
func (h *SimTradeHandler) GetDecisions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	trades, err := h.simTradeService.GetLatestDecisions(c.Request.Context(), limit)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"decisions": trades,
		"total":     len(trades),
	})
}

// GetAccount handles GET /api/v1/trading/sim/account
func (h *SimTradeHandler) GetAccount(c *gin.Context) {
	account, err := h.accountService.GetAccount()
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, account)
}

// GetPositions handles GET /api/v1/trading/sim/positions
func (h *SimTradeHandler) GetPositions(c *gin.Context) {
	positions, err := h.positionManager.GetPositions()
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"positions": positions,
		"total":     len(positions),
	})
}

// GetTrades handles GET /api/v1/trading/sim/trades
func (h *SimTradeHandler) GetTrades(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	trades, total, err := h.simTradeService.GetSimTrades(c.Request.Context(), limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"trades": trades,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}