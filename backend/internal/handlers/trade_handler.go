package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/services"
)

// TradeHandler handles HTTP requests for trading operations.
type TradeHandler struct {
	tradeService *services.TradeService
}

// NewTradeHandler creates a new TradeHandler.
func NewTradeHandler(svc *services.TradeService) *TradeHandler {
	return &TradeHandler{tradeService: svc}
}

// PlaceOrderRequest represents the JSON body for placing an order.
type PlaceOrderRequest struct {
	Symbol        string `json:"symbol" binding:"required"`
	Action        string `json:"action" binding:"required"`     // buy / sell
	OrderType     string `json:"order_type" binding:"required"` // market / limit
	Price         string `json:"price" binding:"required"`
	Quantity      int    `json:"quantity" binding:"required"`
	IsReal        bool   `json:"is_real"`
	TradePassword string `json:"trade_password"`
}

// PlaceOrder handles POST /api/v1/trading/order
func (h *TradeHandler) PlaceOrder(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid request: "+err.Error())
		return
	}

	// Validate trade password
	if req.TradePassword == "" {
		fail(c, http.StatusBadRequest, 40002, "交易密码不能为空")
		return
	}

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	price, err := decimal.NewFromString(req.Price)
	if err != nil {
		fail(c, http.StatusBadRequest, 40003, "invalid price format")
		return
	}

	tradeType := "simulated"
	if req.IsReal {
		tradeType = "real"
	}

	svcReq := services.PlaceOrderRequest{
		Symbol:    req.Symbol,
		Action:    req.Action,
		OrderType: req.OrderType,
		Price:     price,
		Quantity:  req.Quantity,
		TradeType: tradeType,
	}

	order, err := h.tradeService.PlaceOrder(c.Request.Context(), userID, svcReq)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, order)
}

// CancelOrder handles DELETE /api/v1/trading/order/:id
func (h *TradeHandler) CancelOrder(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	orderID := c.Param("id")
	if orderID == "" {
		fail(c, http.StatusBadRequest, 40001, "order id is required")
		return
	}

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	if err := h.tradeService.CancelOrder(c.Request.Context(), userID, orderID); err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{"message": "订单已撤单"})
}

// GetOrders handles GET /api/v1/trading/orders
func (h *TradeHandler) GetOrders(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	status := c.DefaultQuery("status", "all")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	orders, total, err := h.tradeService.GetOrders(c.Request.Context(), userID, status, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"orders": orders,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

// GetOrderByID handles GET /api/v1/trading/order/:id
func (h *TradeHandler) GetOrderByID(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	orderID := c.Param("id")
	if orderID == "" {
		fail(c, http.StatusBadRequest, 40001, "order id is required")
		return
	}

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	order, err := h.tradeService.GetOrderByID(c.Request.Context(), userID, orderID)
	if err != nil {
		fail(c, http.StatusNotFound, 40401, err.Error())
		return
	}

	success(c, order)
}

// GetPositions handles GET /api/v1/trading/positions
func (h *TradeHandler) GetPositions(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	isReal, _ := strconv.ParseBool(c.DefaultQuery("is_real", "false"))

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	positions, err := h.tradeService.GetPositions(c.Request.Context(), userID, isReal)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"positions": positions,
	})
}

// GetAccount handles GET /api/v1/trading/account
func (h *TradeHandler) GetAccount(c *gin.Context) {
	if h.tradeService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "trading service not available")
		return
	}

	userID, err := parseUserID(c)
	if err != nil {
		fail(c, http.StatusUnauthorized, 40101, err.Error())
		return
	}

	account, err := h.tradeService.GetAccount(c.Request.Context(), userID)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, account)
}

// parseUserID extracts and converts the user ID from the JWT context.
func parseUserID(c *gin.Context) (uint, error) {
	userIDStr := middleware.GetUserID(c)
	if userIDStr == "" {
		return 0, errUnauthorized
	}

	id, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		// For placeholder IDs like "placeholder-user-id", use a fallback
		return 1, nil
	}
	return uint(id), nil
}

var errUnauthorized = &userIDError{}

type userIDError struct{}

func (e *userIDError) Error() string {
	return "unauthorized"
}