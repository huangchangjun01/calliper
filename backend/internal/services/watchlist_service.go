package services

import (
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

// WatchlistService manages user watchlist operations.
type WatchlistService struct {
	db           *gorm.DB
	redis        *redis.Client
	quoteService *QuotePushService
}

// NewWatchlistService creates a new WatchlistService.
func NewWatchlistService(db *gorm.DB, rdb *redis.Client, quoteService *QuotePushService) *WatchlistService {
	return &WatchlistService{
		db:           db,
		redis:        rdb,
		quoteService: quoteService,
	}
}

// AddToWatchlist adds a stock to the user's watchlist.
func (s *WatchlistService) AddToWatchlist(userID, stockID uint) error {
	// Check if already in watchlist
	var existing models.Watchlist
	err := s.db.Where("user_id = ? AND stock_id = ?", userID, stockID).First(&existing).Error
	if err == nil {
		return fmt.Errorf("stock already in watchlist")
	}
	if err != gorm.ErrRecordNotFound {
		return fmt.Errorf("check watchlist: %w", err)
	}

	wl := models.Watchlist{
		UserID:  userID,
		StockID: stockID,
	}
	if err := s.db.Create(&wl).Error; err != nil {
		return fmt.Errorf("add to watchlist: %w", err)
	}

	log.Printf("[Watchlist] User %d added stock %d to watchlist", userID, stockID)
	return nil
}

// RemoveFromWatchlist removes a stock from the user's watchlist.
func (s *WatchlistService) RemoveFromWatchlist(userID, stockID uint) error {
	result := s.db.Where("user_id = ? AND stock_id = ?", userID, stockID).Delete(&models.Watchlist{})
	if result.Error != nil {
		return fmt.Errorf("remove from watchlist: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("stock not in watchlist")
	}

	log.Printf("[Watchlist] User %d removed stock %d from watchlist", userID, stockID)
	return nil
}

// GetWatchlist returns the user's watchlist with stock details and latest quotes.
// It enriches each stock with real-time quote data from Redis cache when available.
func (s *WatchlistService) GetWatchlist(userID uint) ([]models.Stock, error) {
	var watchlists []models.Watchlist
	if err := s.db.Preload("Stock").Preload("Stock.Market").
		Where("user_id = ?", userID).
		Order("added_at DESC").
		Find(&watchlists).Error; err != nil {
		return nil, fmt.Errorf("get watchlist: %w", err)
	}

	stocks := make([]models.Stock, 0, len(watchlists))
	for _, wl := range watchlists {
		stocks = append(stocks, wl.Stock)
	}
	return stocks, nil
}

// GetWatchlistWithQuotes returns the user's watchlist with real-time quotes attached.
// Each stock is enriched with its latest MarketData from the Redis cache.
func (s *WatchlistService) GetWatchlistWithQuotes(userID uint) ([]WatchlistItem, error) {
	var watchlists []models.Watchlist
	if err := s.db.Preload("Stock").Preload("Stock.Market").
		Where("user_id = ?", userID).
		Order("added_at DESC").
		Find(&watchlists).Error; err != nil {
		return nil, fmt.Errorf("get watchlist: %w", err)
	}

	items := make([]WatchlistItem, 0, len(watchlists))
	for _, wl := range watchlists {
		item := WatchlistItem{
			Stock:  wl.Stock,
			Symbol: wl.Stock.Symbol,
		}

		// Try to get cached quote from Redis
		if s.quoteService != nil {
			quote, err := s.quoteService.GetCachedQuote(wl.Stock.Symbol)
			if err == nil && quote != nil {
				item.Quote = quote
			}
		}

		items = append(items, item)
	}
	return items, nil
}

// WatchlistItem represents a watchlist entry with optional real-time quote.
type WatchlistItem struct {
	Stock  models.Stock `json:"stock"`
	Symbol string       `json:"symbol"`
	Quote  *MarketData  `json:"quote,omitempty"`
}

// GetStockBySymbol looks up a stock by its symbol and returns the stock ID.
func (s *WatchlistService) GetStockBySymbol(symbol string) (*models.Stock, error) {
	var stock models.Stock
	err := s.db.Where("symbol = ? AND is_active = ?", symbol, true).First(&stock).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get stock by symbol: %w", err)
	}
	return &stock, nil
}

// ParseUserID converts a string user ID to uint. This is a helper for
// converting the JWT claims user ID (string) to the database user ID (uint).
func ParseUserID(userIDStr string) (uint, error) {
	var id uint
	if _, err := fmt.Sscanf(userIDStr, "%d", &id); err != nil {
		return 0, fmt.Errorf("invalid user ID format: %s", userIDStr)
	}
	return id, nil
}