package services

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// MarketDataService orchestrates market data collection across multiple markets.
type MarketDataService struct {
	tsdb            *gorm.DB
	redis           *redis.Client
	kafkaProd       *KafkaProducer
	collectors      map[string]MarketDataCollector
	cleaner         *DataCleaner
	mu              sync.RWMutex
	cancelFuncs     map[string]context.CancelFunc
	onDataCollected func([]MarketData)
}

// MarketDataServiceConfig holds configuration for MarketDataService.
type MarketDataServiceConfig struct {
	TSDB         *gorm.DB
	Redis        *redis.Client
	KafkaBrokers []string
	MLServiceURL string
}

// NewMarketDataService creates a new MarketDataService.
func NewMarketDataService(cfg MarketDataServiceConfig) *MarketDataService {
	var kafkaProd *KafkaProducer
	if len(cfg.KafkaBrokers) > 0 {
		kafkaProd = NewKafkaProducer(KafkaProducerConfig{
			Brokers: cfg.KafkaBrokers,
		})
	} else {
		kafkaProd = NewKafkaProducerNoop()
	}

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
		tsdb:        cfg.TSDB,
		redis:       cfg.Redis,
		kafkaProd:   kafkaProd,
		collectors:  collectors,
		cleaner:     NewDataCleaner(),
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

		go s.runCollectionLoop(marketCtx, marketCode, collector)
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
	s.mu.RLock()
	collector, exists := s.collectors[marketCode]
	s.mu.RUnlock()

	if !exists {
		return nil, nil
	}

	defaultSymbols := s.getDefaultSymbols(marketCode)
	data, err := collector.FetchRealTimeData(defaultSymbols)
	if err != nil {
		log.Printf("[MarketDataService] Failed to fetch data for %s: %v", marketCode, err)
		return nil, err
	}

	// Clean the data
	cleaned := s.cleaner.CleanMarketData(data)

	// Publish to Kafka
	if s.kafkaProd != nil {
		if err := s.kafkaProd.PublishMarketData(ctx, cleaned); err != nil {
			log.Printf("[MarketDataService] Failed to publish to Kafka: %v", err)
		}
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

	// Collect immediately on start
	s.CollectMarketData(ctx, marketCode)

	ticker := time.NewTicker(s.getCollectionInterval(marketCode))
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CollectMarketData(ctx, marketCode)
		}
	}
}

// getCollectionInterval returns the collection interval for a market.
func (s *MarketDataService) getCollectionInterval(marketCode string) time.Duration {
	switch marketCode {
	case "CN":
		return 30 * time.Second // A-share snapshot every 30s
	default:
		return 30 * time.Second
	}
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

// getDefaultSymbols returns symbols for a market, pulling from database first
// with a hardcoded fallback list when the database is empty.
func (s *MarketDataService) getDefaultSymbols(marketCode string) []string {
	// Try to get symbols from database
	if s.tsdb != nil {
		var symbols []string
		err := s.tsdb.Model(&struct {
			Symbol string
		}{}).
			Table("stocks").
			Where("is_active = ?", true).
			Limit(5).
			Pluck("symbol", &symbols).Error
		if err == nil && len(symbols) > 0 {
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

// GetKafkaProducer returns the Kafka producer for external use.
func (s *MarketDataService) GetKafkaProducer() *KafkaProducer {
	return s.kafkaProd
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