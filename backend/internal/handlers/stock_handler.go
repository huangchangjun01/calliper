package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/services"
)

// StockHandler handles HTTP requests for stock search and sync.
type StockHandler struct {
	stockService *services.StockService
}

// NewStockHandler creates a new StockHandler.
func NewStockHandler(svc *services.StockService) *StockHandler {
	return &StockHandler{stockService: svc}
}

// ──────────────────────────────────────────────────────────────
// Response helpers
// ──────────────────────────────────────────────────────────────

// APIResponse is the unified API response envelope.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

func success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{
		Code:    0,
		Message: "success",
		Data:    data,
	})
}

func fail(c *gin.Context, httpStatus int, code int, message string) {
	c.JSON(httpStatus, APIResponse{
		Code:    code,
		Message: message,
		Data:    nil,
	})
}

// SearchStockResponse wraps search results with pagination metadata.
type SearchStockResponse struct {
	Stocks interface{} `json:"stocks"`
	Total  int64       `json:"total"`
	Limit  int         `json:"limit"`
	Offset int         `json:"offset"`
}

// ──────────────────────────────────────────────────────────────
// Handlers
// ──────────────────────────────────────────────────────────────

// SearchStocks handles GET /api/v1/stocks/search?q=...&market=...&limit=...&offset=...
func (h *StockHandler) SearchStocks(c *gin.Context) {
	if h.stockService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "stock service not available")
		return
	}

	query := c.Query("q")
	marketCode := c.Query("market")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	stocks, total, err := h.stockService.SearchStocks(query, marketCode, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, SearchStockResponse{
		Stocks: stocks,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetStocksByMarket handles GET /api/v1/stocks/market/:code?limit=...&offset=...
func (h *StockHandler) GetStocksByMarket(c *gin.Context) {
	if h.stockService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "stock service not available")
		return
	}

	marketCode := c.Param("code")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	stocks, total, err := h.stockService.GetStocksByMarket(marketCode, limit, offset)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, SearchStockResponse{
		Stocks: stocks,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	})
}

// GetStockBySymbol handles GET /api/v1/stocks/:symbol
func (h *StockHandler) GetStockBySymbol(c *gin.Context) {
	if h.stockService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "stock service not available")
		return
	}

	symbol := c.Param("symbol")

	stock, err := h.stockService.GetStockBySymbol(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	if stock == nil {
		fail(c, http.StatusNotFound, 40401, "stock not found")
		return
	}

	success(c, stock)
}

// SyncMarket handles POST /api/v1/stocks/sync/:market
func (h *StockHandler) SyncMarket(c *gin.Context) {
	if h.stockService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "stock service not available")
		return
	}

	marketCode := c.Param("market")

	if err := h.stockService.SyncStocksFromMarket(marketCode); err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, gin.H{
		"market":  marketCode,
		"message": "sync completed successfully",
	})
}

// HealthCheck handles GET /api/v1/stocks/health
func (h *StockHandler) HealthCheck(c *gin.Context) {
	if h.stockService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "stock service not available")
		return
	}

	status := h.stockService.HealthCheck()
	success(c, status)
}