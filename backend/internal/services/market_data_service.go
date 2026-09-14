package services

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/quant-trading/backend/internal/util"
)

// MarketDataService orchestrates market data collection across multiple markets.
type MarketDataService struct {
	db              *gorm.DB
	tsdb            *gorm.DB
	redis           *redis.Client
	collectors      map[string]MarketDataCollector
	cleaner         *DataCleaner
	persist         *TSDBPersist
	mu              sync.RWMutex
	cancelFuncs     map[string]context.CancelFunc
	onDataCollected func([]MarketData)
}

// MarketDataServiceConfig holds configuration for MarketDataService.
type MarketDataServiceConfig struct {
	DB           *gorm.DB
	TSDB         *gorm.DB
	Redis        *redis.Client
	MLServiceURL string
}

// NewMarketDataService creates a new MarketDataService.
func NewMarketDataService(cfg MarketDataServiceConfig) *MarketDataService {
	// CN market: Tencent Finance as primary (East Money available as fallback
	// when the environment has access to push2.eastmoney.com).
	// In sandbox environments where East Money is blocked, Tencent is used directly.
	cnPrimary := NewTencentCollector("CN")
	cnFallback := NewEastMoneyCollector("CN")
	cnCollector := NewFallbackCollector(cnPrimary, cnFallback)

	collectors := map[string]MarketDataCollector{
		"CN": cnCollector,
	}

	return &MarketDataService{
		db:          cfg.DB,
		tsdb:        cfg.TSDB,
		redis:       cfg.Redis,
		collectors:  collectors,
		cleaner:     NewDataCleaner(),
		persist:     NewTSDBPersist(cfg.DB, cfg.TSDB),
		cancelFuncs: make(map[string]context.CancelFunc),
	}
}

// StartCollection starts market data collection for all configured markets.
// Each market runs on its own schedule based on trading hours.
func (s *MarketDataService) StartCollection(ctx context.Context) {
	log.Println("[MarketDataService] Starting market data collection...")

	for marketCode, collector := range s.collectors {
		marketCtx, cancel := context.WithCancel(ctx)
		s.mu.Lock()
		s.cancelFuncs[marketCode] = cancel
		s.mu.Unlock()

		util.SafeGo(func() { s.runCollectionLoop(marketCtx, marketCode, collector) })
	}

	log.Printf("[MarketDataService] Started collection for %d markets", len(s.collectors))
}

// StopCollection stops all market data collection loops.
func (s *MarketDataService) StopCollection() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for marketCode, cancel := range s.cancelFuncs {
		log.Printf("[MarketDataService] Stopping collection for market: %s", marketCode)
		cancel()
	}
	s.cancelFuncs = make(map[string]context.CancelFunc)
	log.Println("[MarketDataService] All collection loops stopped")
}

// CollectMarketData collects real-time market data for a specific market.
func (s *MarketDataService) CollectMarketData(ctx context.Context, marketCode string) ([]MarketData, error) {
	defaultSymbols := s.getDefaultSymbols(marketCode)
	return s.CollectMarketDataForSymbols(ctx, marketCode, defaultSymbols)
}

// CollectMarketDataForSymbols collects real-time market data for specific symbols in a market.
func (s *MarketDataService) CollectMarketDataForSymbols(ctx context.Context, marketCode string, symbols []string) ([]MarketData, error) {
	s.mu.RLock()
	collector, exists := s.collectors[marketCode]
	s.mu.RUnlock()

	if !exists || len(symbols) == 0 {
		return nil, nil
	}

	data, err := collector.FetchRealTimeData(symbols)
	if err != nil {
		log.Printf("[MarketDataService] Failed to fetch data for %s: %v", marketCode, err)
		return nil, err
	}

	// Clean the data
	cleaned := s.cleaner.CleanMarketData(data)

	// Persist directly to TimescaleDB.
	if s.persist != nil {
		s.persist.PersistTicks(ctx, cleaned)
	}

	// Cache latest data in Redis
	s.cacheMarketData(ctx, marketCode, cleaned)

	// Notify callback (used by QuotePushService for WebSocket broadcasting)
	s.mu.RLock()
	cb := s.onDataCollected
	s.mu.RUnlock()
	if cb != nil {
		cb(cleaned)
	}

	return cleaned, nil
}

// runCollectionLoop runs the periodic collection loop for a single market.
func (s *MarketDataService) runCollectionLoop(ctx context.Context, marketCode string, collector MarketDataCollector) {
	log.Printf("[MarketDataService] Collection loop started for market: %s", marketCode)

	// Collect immediately on start (盘外节流：非交易时段跳过)。
	s.collectOnce(ctx, marketCode)

	ticker := time.NewTicker(s.getCollectionInterval(marketCode))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectOnce(ctx, marketCode)
		}
	}
}

// collectOnce performs a single collection for a market, throttling when the
// market is outside trading hours (skip polling to avoid wasteful requests).
func (s *MarketDataService) collectOnce(ctx context.Context, marketCode string) {
	if !s.isTradingHours(marketCode) {
		// 盘外：行情基本不变化，跳过本轮采集以节流。
		return
	}
	s.CollectMarketData(ctx, marketCode)
}

// getCollectionInterval returns the collection interval for a market.
// The interval is configurable via the MARKET_COLLECT_INTERVAL_SEC
// environment variable (default 5s).
func (s *MarketDataService) getCollectionInterval(marketCode string) time.Duration {
	if v := os.Getenv("MARKET_COLLECT_INTERVAL_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			return time.Duration(sec) * time.Second
		}
	}
	return 5 * time.Second
}

// IsTradingHours exposes the trading-hour check for external callers
// (e.g. admin health判定在盘外豁免数据陈旧告警).
func (s *MarketDataService) IsTradingHours(marketCode string) bool {
	return s.isTradingHours(marketCode)
}

// isTradingHours checks if the given market is currently in trading hours.
// Uses market-specific timezones for accurate trading hour calculation.
func (s *MarketDataService) isTradingHours(marketCode string) bool {
	switch marketCode {
	case "CN":
		return s.isCNTradingHours()
	case "HK":
		return s.isHKTradingHours()
	case "US":
		return s.isUSTradingHours()
	default:
		return true
	}
}

// isCNTradingHours checks A-share trading hours: 9:30-11:30, 13:00-15:00 CST.
func (s *MarketDataService) isCNTradingHours() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return true // fallback: always collect if timezone lookup fails
	}
	now := time.Now().In(loc)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	h, m := now.Hour(), now.Minute()
	morning := (h == 9 && m >= 30) || (h == 10) || (h == 11 && m <= 30)
	afternoon := (h >= 13 && h < 15)
	return morning || afternoon
}

// isHKTradingHours checks HK trading hours: 9:30-12:00, 13:00-16:00 HKT.
func (s *MarketDataService) isHKTradingHours() bool {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return true
	}
	now := time.Now().In(loc)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	h, m := now.Hour(), now.Minute()
	morning := (h == 9 && m >= 30) || (h == 10 || h == 11) || (h == 12 && m == 0)
	afternoon := (h >= 13 && h < 16)
	return morning || afternoon
}

// isUSTradingHours checks US trading hours: 9:30-16:00 EST.
func (s *MarketDataService) isUSTradingHours() bool {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return true
	}
	now := time.Now().In(loc)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	h, m := now.Hour(), now.Minute()
	return (h == 9 && m >= 30) || (h >= 10 && h < 16)
}

// getDefaultSymbols returns symbols for a market, pulling the full set from the
// stocks table (main DB) in batches with a hard cap, with a hardcoded fallback
// list when the database is empty.
func (s *MarketDataService) getDefaultSymbols(marketCode string) []string {
	// Try to get symbols from database (stocks live in the main DB, not the TSDB)
	if s.db != nil {
		const batch = 500
		const maxSymbols = 2000 // 上限保护，避免单轮采集过载
		var symbols []string
		for offset := 0; offset < maxSymbols; offset += batch {
			var batchSymbols []string
			err := s.db.Model(&struct {
				Symbol string
			}{}).
				Table("stocks").
				Where("is_active = ?", true).
				Order("id ASC").
				Limit(batch).Offset(offset).
				Pluck("symbol", &batchSymbols).Error
			if err != nil {
				break
			}
			symbols = append(symbols, batchSymbols...)
			if len(batchSymbols) < batch {
				break // 已取完
			}
		}
		if len(symbols) > maxSymbols {
			symbols = symbols[:maxSymbols]
		}
		if len(symbols) > 0 {
			return symbols
		}
	}

	// Fallback: well-known major stocks (limited to 5)
	switch marketCode {
	case "CN":
		return []string{
			"000001", "600519", "000858", "300750", "601318",
		}
	case "US":
		return []string{
			"AAPL", "MSFT", "GOOGL", "TSLA", "NVDA",
		}
	case "HK":
		return []string{
			"00700", "09988", "00388", "02318", "00005",
		}
	default:
		return nil
	}
}

// cacheMarketData caches the latest market data in Redis.
func (s *MarketDataService) cacheMarketData(ctx context.Context, marketCode string, data []MarketData) {
	if s.redis == nil {
		return
	}

	for _, md := range data {
		key := "market:realtime:" + md.Symbol
		// Use Redis to cache the latest snapshot with a TTL
		_ = s.redis.Set(ctx, key, strconv.FormatFloat(md.Price, 'f', 2, 64), 30*time.Second).Err()
	}
}

// GetCollectors returns all registered collectors.
func (s *MarketDataService) GetCollectors() map[string]MarketDataCollector {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]MarketDataCollector, len(s.collectors))
	for k, v := range s.collectors {
		result[k] = v
	}
	return result
}

// SetDataCallback registers a callback that is invoked after market data is collected and cleaned.
// This is used by the QuotePushService to broadcast data to WebSocket clients.
func (s *MarketDataService) SetDataCallback(cb func([]MarketData)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDataCollected = cb
}

// GetCleaner returns the data cleaner instance.
func (s *MarketDataService) GetCleaner() *DataCleaner {
	return s.cleaner
}
