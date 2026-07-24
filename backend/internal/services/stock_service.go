package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/models"
)

const (
	stockCachePrefix = "stock:list:"
	stockCacheTTL    = 24 * time.Hour
)

// StockService provides stock retrieval, search, and sync operations.
type StockService struct {
	db            *gorm.DB
	redis         *redis.Client
	akshareClient DataSource
	yahooClient   DataSource
}

// NewStockService creates a new StockService.
func NewStockService(db *gorm.DB, rdb *redis.Client) *StockService {
	return &StockService{
		db:            db,
		redis:         rdb,
		akshareClient: NewAkshareClient(),
		yahooClient:   NewYahooClient(),
	}
}

// SyncStocksFromMarket fetches stock list from the appropriate data source and
// upserts records into the database, then refreshes the Redis cache.
func (s *StockService) SyncStocksFromMarket(marketCode string) error {
	var stocks []StockRaw
	var err error

	if IsChineseMarket(marketCode) {
		stocks, err = s.akshareClient.FetchStockList(marketCode)
	} else if IsOverseasMarket(marketCode) {
		stocks, err = s.yahooClient.FetchStockList(marketCode)
	} else {
		return fmt.Errorf("unsupported market code: %s", marketCode)
	}
	if err != nil {
		return fmt.Errorf("fetch stock list for %s: %w", marketCode, err)
	}

	if len(stocks) == 0 {
		log.Printf("stock_service: no stocks returned for %s", marketCode)
		return nil
	}

	// Find or create the Market record
	var market models.Market
	err = s.db.Where("code = ?", marketCode).First(&market).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			market = models.Market{
				Code:     marketCode,
				Name:     marketCode,
				Country:  MarketCodeToCountry(marketCode),
				Currency: MarketCodeToCurrency(marketCode),
				Timezone: "UTC",
				Status:   "active",
			}
			if createErr := s.db.Create(&market).Error; createErr != nil {
				return fmt.Errorf("create market %s: %w", marketCode, createErr)
			}
		} else {
			return fmt.Errorf("lookup market %s: %w", marketCode, err)
		}
	}

	// Upsert each stock
	for _, raw := range stocks {
		var stock models.Stock
		result := s.db.Where("symbol = ? AND market_id = ?", raw.Symbol, market.ID).First(&stock)

		if result.Error == gorm.ErrRecordNotFound {
			stock = models.Stock{
				Symbol:    raw.Symbol,
				Name:      raw.Name,
				NameCN:    raw.NameCN,
				MarketID:  market.ID,
				Exchange:  raw.Exchange,
				Industry:  raw.Industry,
				Sector:    raw.Sector,
				MarketCap: raw.MarketCap,
				Currency:  raw.Currency,
				LotSize:   raw.LotSize,
				IsActive:  raw.IsActive,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}
			if createErr := s.db.Create(&stock).Error; createErr != nil {
				log.Printf("stock_service: create stock %s: %v", raw.Symbol, createErr)
				continue
			}
		} else if result.Error == nil {
			stock.Name = raw.Name
			stock.NameCN = raw.NameCN
			stock.Exchange = raw.Exchange
			stock.Industry = raw.Industry
			stock.Sector = raw.Sector
			stock.MarketCap = raw.MarketCap
			stock.Currency = raw.Currency
			stock.LotSize = raw.LotSize
			stock.IsActive = raw.IsActive
			stock.UpdatedAt = time.Now()
			if saveErr := s.db.Save(&stock).Error; saveErr != nil {
				log.Printf("stock_service: update stock %s: %v", raw.Symbol, saveErr)
			}
		} else {
			log.Printf("stock_service: lookup stock %s: %v", raw.Symbol, result.Error)
		}
	}

	log.Printf("stock_service: synced %d stocks for market %s", len(stocks), marketCode)

	// Refresh Redis cache
	if err := s.CacheStockList(marketCode); err != nil {
		log.Printf("stock_service: cache refresh failed for %s: %v", marketCode, err)
	}

	return nil
}

// SearchStocks performs a fuzzy search by symbol or name, optionally filtered by market.
// Results are paginated by limit and offset.
func (s *StockService) SearchStocks(query string, marketCode string, limit int, offset int) ([]models.Stock, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	db := s.db.Model(&models.Stock{}).Preload("Market").Where("is_active = ?", true)

	if query != "" {
		like := "%" + query + "%"
		db = db.Where("symbol ILIKE ? OR name ILIKE ? OR name_cn ILIKE ?", like, like, like)
	}

	if marketCode != "" {
		db = db.Joins("JOIN markets ON markets.id = stocks.market_id").
			Where("markets.code = ?", marketCode)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count stocks: %w", err)
	}

	var stocks []models.Stock
	if err := db.Order("symbol ASC").Limit(limit).Offset(offset).Find(&stocks).Error; err != nil {
		return nil, 0, fmt.Errorf("search stocks: %w", err)
	}

	return stocks, total, nil
}

// GetStocksByMarket returns all active stocks for a given market, paginated.
func (s *StockService) GetStocksByMarket(marketCode string, limit int, offset int) ([]models.Stock, int64, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	db := s.db.Model(&models.Stock{}).Preload("Market").
		Joins("JOIN markets ON markets.id = stocks.market_id").
		Where("markets.code = ? AND stocks.is_active = ?", marketCode, true)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count stocks by market: %w", err)
	}

	var stocks []models.Stock
	if err := db.Order("symbol ASC").Limit(limit).Offset(offset).Find(&stocks).Error; err != nil {
		return nil, 0, fmt.Errorf("get stocks by market: %w", err)
	}

	return stocks, total, nil
}

// GetStockBySymbol looks up a single stock by its symbol.
func (s *StockService) GetStockBySymbol(symbol string) (*models.Stock, error) {
	var stock models.Stock
	err := s.db.Preload("Market").Where("symbol = ? AND is_active = ?", symbol, true).First(&stock).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("get stock by symbol: %w", err)
	}
	return &stock, nil
}

// CacheStockList serializes the stock list for a market and stores it in Redis.
func (s *StockService) CacheStockList(marketCode string) error {
	if s.redis == nil {
		return nil
	}

	stocks, _, err := s.GetStocksByMarket(marketCode, 10000, 0)
	if err != nil {
		return err
	}

	data, err := json.Marshal(stocks)
	if err != nil {
		return fmt.Errorf("marshal stock list: %w", err)
	}

	key := stockCachePrefix + marketCode
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.redis.Set(ctx, key, data, stockCacheTTL).Err()
}

// GetStockListFromCache reads the stock list for a market from Redis.
func (s *StockService) GetStockListFromCache(marketCode string) ([]models.Stock, error) {
	if s.redis == nil {
		return nil, fmt.Errorf("redis not available")
	}

	key := stockCachePrefix + marketCode
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := s.redis.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}

	var stocks []models.Stock
	if err := json.Unmarshal(data, &stocks); err != nil {
		return nil, fmt.Errorf("unmarshal stock list: %w", err)
	}
	return stocks, nil
}

// HealthCheck returns the status of each data source.
func (s *StockService) HealthCheck() map[string]string {
	result := make(map[string]string)

	if err := s.akshareClient.HealthCheck(); err != nil {
		result["akshare"] = "unhealthy: " + err.Error()
	} else {
		result["akshare"] = "healthy"
	}

	if err := s.yahooClient.HealthCheck(); err != nil {
		result["yahoo"] = "unhealthy: " + err.Error()
	} else {
		result["yahoo"] = "healthy"
	}

	if s.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := s.redis.Ping(ctx).Err(); err != nil {
			result["redis"] = "unhealthy: " + err.Error()
		} else {
			result["redis"] = "healthy"
		}
	} else {
		result["redis"] = "not_configured"
	}

	if s.db != nil {
		sqlDB, err := s.db.DB()
		if err != nil || sqlDB.Ping() != nil {
			result["database"] = "unhealthy"
		} else {
			result["database"] = "healthy"
		}
	} else {
		result["database"] = "not_configured"
	}

	return result
}