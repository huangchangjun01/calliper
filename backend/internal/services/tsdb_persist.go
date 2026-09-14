package services

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/quant-trading/backend/internal/models"
)

// stockIDCacheTTL 控制 stocks 表 symbol->id 映射的全量刷新周期，
// 以缓和新股票同步导致的缓存过期问题。
const stockIDCacheTTL = 10 * time.Minute

// ──────────────────────────────────────────────────────────────
// 时序库落库助手：将行情快照 / 历史日线写入 TimescaleDB。
// 行情采集与回填数据直接入库，不依赖消息队列中转。
// ──────────────────────────────────────────────────────────────

// TSDBPersist provides direct persistence of market data into TimescaleDB,
// with symbol -> stock_id resolution and idempotent upserts.
type TSDBPersist struct {
	db   *gorm.DB // 主库（stocks 表）
	tsdb *gorm.DB // 时序库

	idMu     sync.RWMutex
	stockIDs map[string]uint // 小写 symbol -> stock id 的内存缓存
	lastLoad time.Time       // 上次全量加载时间
}

// NewTSDBPersist creates a TSDBPersist helper. Either handle may be nil;
// write calls are no-ops (with a log) when tsdb is nil.
func NewTSDBPersist(db, tsdb *gorm.DB) *TSDBPersist {
	return &TSDBPersist{db: db, tsdb: tsdb, stockIDs: make(map[string]uint)}
}

// PersistTicks writes real-time snapshots into stock_prices_tick.
// Unknown symbols are skipped with a log line.
func (p *TSDBPersist) PersistTicks(ctx context.Context, data []MarketData) {
	if p.tsdb == nil || len(data) == 0 {
		return
	}

	records := make([]models.StockPriceTick, 0, len(data))
	for _, md := range data {
		stockID, ok := p.resolveStockID(ctx, md.Symbol)
		if !ok {
			log.Printf("[TSDBPersist] Skip tick for unknown symbol %s", md.Symbol)
			continue
		}
		records = append(records, models.StockPriceTick{
			Time:      md.Timestamp,
			StockID:   stockID,
			Price:     md.Price,
			Volume:    md.Volume,
			Direction: directionFromChangePtr(md.Change),
		})
	}

	if len(records) == 0 {
		return
	}

	// Idempotent: upsert on (time, stock_id)
	if err := p.tsdb.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "time"}, {Name: "stock_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"price", "volume", "direction"}),
	}).CreateInBatches(records, 200).Error; err != nil {
		log.Printf("[TSDBPersist] Failed to persist ticks: %v", err)
		return
	}
	log.Printf("[TSDBPersist] Persisted %d tick records", len(records))
}

// PersistDaily writes historical daily OHLCV records into stock_prices_daily.
// Unknown symbols are skipped; upsert keeps repeat backfills idempotent.
func (p *TSDBPersist) PersistDaily(ctx context.Context, data []MarketData) {
	if p.tsdb == nil || len(data) == 0 {
		return
	}

	records := make([]models.StockPriceDaily, 0, len(data))
	for _, md := range data {
		stockID, ok := p.resolveStockID(ctx, md.Symbol)
		if !ok {
			log.Printf("[TSDBPersist] Skip daily for unknown symbol %s", md.Symbol)
			continue
		}
		if md.Price <= 0 {
			continue // 无收盘价的行无意义
		}
		records = append(records, models.StockPriceDaily{
			Time:           md.Timestamp,
			StockID:        stockID,
			Open:           md.Open,
			High:           md.High,
			Low:            md.Low,
			Close:          md.Price,
			Volume:         md.Volume,
			Amount:         md.Amount,
			TurnoverRate:   md.TurnoverRate,
			PERatio:        md.PE,
			PBRatio:        md.PB,
			TotalMarketCap: md.TotalMarketCap,
			FloatMarketCap: md.FloatMarketCap,
		})
	}

	if len(records) == 0 {
		return
	}

	// Idempotent: upsert on (time, stock_id)
	if err := p.tsdb.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "time"}, {Name: "stock_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"open", "high", "low", "close", "volume", "amount"}),
	}).CreateInBatches(records, 100).Error; err != nil {
		log.Printf("[TSDBPersist] Failed to persist daily records: %v", err)
		return
	}
	log.Printf("[TSDBPersist] Persisted %d daily records", len(records))
}

// resolveStockID maps a symbol to its stock ID from the main database.
// Uses an in-memory cache (lowercase symbol -> id) loaded in bulk from the
// stocks table; on cache miss it queries the DB and backfills the cache.
func (p *TSDBPersist) resolveStockID(ctx context.Context, symbol string) (uint, bool) {
	if p.db == nil {
		return 0, false
	}
	key := strings.ToLower(symbol)

	// 快路径：缓存新鲜且命中时仅需读锁。
	p.idMu.RLock()
	fresh := p.stockIDs != nil && time.Since(p.lastLoad) < stockIDCacheTTL
	id, ok := p.stockIDs[key]
	p.idMu.RUnlock()
	if ok {
		return id, true
	}
	if !fresh {
		// 缓存过期/未加载：全量刷新后重读。
		p.ensureCacheLoaded(ctx)
		p.idMu.RLock()
		id, ok = p.stockIDs[key]
		p.idMu.RUnlock()
		if ok {
			return id, true
		}
	}

	// 失命中：回查主库并回填缓存。
	var stock models.Stock
	if err := p.db.WithContext(ctx).Where("LOWER(symbol) = ?", key).First(&stock).Error; err != nil {
		return 0, false
	}
	p.idMu.Lock()
	p.stockIDs[key] = stock.ID
	p.idMu.Unlock()
	return stock.ID, true
}

// ensureCacheLoaded 在首次访问或缓存超过 TTL 时，从 stocks 表批量加载 symbol->id 映射。
// 过期后全量刷新，兼顾与 stocks 表变化的一致性，逻辑保持简单。
func (p *TSDBPersist) ensureCacheLoaded(ctx context.Context) {
	p.idMu.Lock()
	defer p.idMu.Unlock()

	if p.stockIDs != nil && time.Since(p.lastLoad) < stockIDCacheTTL {
		return
	}

	fresh := make(map[string]uint, len(p.stockIDs)+256)
	var stocks []models.Stock
	if err := p.db.WithContext(ctx).Select("id", "symbol").Find(&stocks).Error; err != nil {
		log.Printf("[TSDBPersist] Failed to load stock id cache: %v", err)
		// 加载失败时保留旧缓存，避免空映射导致落库中断。
		return
	}
	for _, s := range stocks {
		fresh[strings.ToLower(s.Symbol)] = s.ID
	}
	p.stockIDs = fresh
	p.lastLoad = time.Now()
}

// directionFromChangePtr returns "up"/"down"/"flat" for tick direction annotation.
func directionFromChangePtr(change float64) string {
	if change > 0 {
		return "up"
	}
	if change < 0 {
		return "down"
	}
	return "flat"
}
