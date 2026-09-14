package handlers

import (
	"net/http"

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
	status := h.simTradeService.GetStatus(c.Request.Context())
	success(c, status)
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

// TriggerSimTrading handles POST /api/v1/trading/sim/trigger
// 手动触发一轮决策周期（便于演示与联调）。
func (h *SimTradeHandler) TriggerSimTrading(c *gin.Context) {
	if !h.simTradeService.TriggerDecisionCycle(c.Request.Context()) {
		success(c, gin.H{"message": "非交易时段，已跳过本轮模拟交易决策"})
		return
	}
	success(c, gin.H{"message": "已触发一轮模拟交易决策"})
}

// GetDecisions handles GET /api/v1/trading/sim/decisions
func (h *SimTradeHandler) GetDecisions(c *gin.Context) {
	limit, _, _ := parsePageLimit(c.DefaultQuery("limit", "50"), c.DefaultQuery("offset", "0"), 50)

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
	limit, offset, _ := parsePageLimit(c.DefaultQuery("limit", "20"), c.DefaultQuery("offset", "0"), 20)

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

// GetHistoryDates handles GET /api/v1/trading/sim/history/dates
// 返回存在模拟交易记录的日期列表（YYYY-MM-DD，倒序），用于历史决策弹框选择。
func (h *SimTradeHandler) GetHistoryDates(c *gin.Context) {
	dates, err := h.simTradeService.GetDecisionDates(c.Request.Context())
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"dates": dates,
		"total": len(dates),
	})
}

// GetHistoryByDate handles GET /api/v1/trading/sim/history?date=YYYY-MM-DD
// 返回指定日期的模拟交易决策记录（复用 tradeToView 视图转换）。
func (h *SimTradeHandler) GetHistoryByDate(c *gin.Context) {
	date := c.Query("date")
	if date == "" {
		fail(c, http.StatusBadRequest, 40001, "date 参数不能为空（格式 YYYY-MM-DD）")
		return
	}

	trades, err := h.simTradeService.GetTradesByDate(c.Request.Context(), date)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	records := make([]services.SimRecordView, 0, len(trades))
	for _, t := range trades {
		_, record := h.simTradeService.ToRecordView(t)
		records = append(records, record)
	}

	success(c, gin.H{
		"date":    date,
		"records": records,
		"total":   len(records),
	})
}
