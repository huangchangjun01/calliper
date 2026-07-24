package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/quant-trading/backend/internal/middleware"
	"github.com/quant-trading/backend/internal/services"
)

// WatchlistHandler handles HTTP requests for watchlist management.
type WatchlistHandler struct {
	watchlistService *services.WatchlistService
}

// NewWatchlistHandler creates a new WatchlistHandler.
func NewWatchlistHandler(watchlistService *services.WatchlistService) *WatchlistHandler {
	return &WatchlistHandler{
		watchlistService: watchlistService,
	}
}

// GetWatchlist handles GET /api/v1/stocks/watchlist
// Returns the user's watchlist with real-time quotes.
func (h *WatchlistHandler) GetWatchlist(c *gin.Context) {
	if h.watchlistService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "watchlist service not available")
		return
	}

	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid user ID")
		return
	}

	items, err := h.watchlistService.GetWatchlistWithQuotes(uint(userID))
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}

	success(c, items)
}

// AddToWatchlist handles POST /api/v1/stocks/watchlist/:symbol
// Adds a stock to the user's watchlist by symbol.
func (h *WatchlistHandler) AddToWatchlist(c *gin.Context) {
	if h.watchlistService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "watchlist service not available")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid user ID")
		return
	}

	// Look up stock by symbol
	stock, err := h.watchlistService.GetStockBySymbol(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	if stock == nil {
		fail(c, http.StatusNotFound, 40401, "stock not found")
		return
	}

	if err := h.watchlistService.AddToWatchlist(uint(userID), stock.ID); err != nil {
		fail(c, http.StatusConflict, 40901, err.Error())
		return
	}

	success(c, gin.H{
		"message": "stock added to watchlist",
		"symbol":  symbol,
	})
}

// RemoveFromWatchlist handles DELETE /api/v1/stocks/watchlist/:symbol
// Removes a stock from the user's watchlist by symbol.
func (h *WatchlistHandler) RemoveFromWatchlist(c *gin.Context) {
	if h.watchlistService == nil {
		fail(c, http.StatusServiceUnavailable, 50301, "watchlist service not available")
		return
	}

	symbol := c.Param("symbol")
	if symbol == "" {
		fail(c, http.StatusBadRequest, 40001, "symbol is required")
		return
	}

	userIDStr := middleware.GetUserID(c)
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		fail(c, http.StatusBadRequest, 40001, "invalid user ID")
		return
	}

	// Look up stock by symbol
	stock, err := h.watchlistService.GetStockBySymbol(symbol)
	if err != nil {
		fail(c, http.StatusInternalServerError, 50001, err.Error())
		return
	}
	if stock == nil {
		fail(c, http.StatusNotFound, 40401, "stock not found")
		return
	}

	if err := h.watchlistService.RemoveFromWatchlist(uint(userID), stock.ID); err != nil {
		fail(c, http.StatusNotFound, 40401, err.Error())
		return
	}

	success(c, gin.H{
		"message": "stock removed from watchlist",
		"symbol":  symbol,
	})
}