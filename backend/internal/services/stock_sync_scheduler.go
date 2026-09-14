package services

import (
	"context"
	"log"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/quant-trading/backend/internal/models"
	"github.com/quant-trading/backend/internal/util"
)

// ChineseMarketCodes are the A-share exchanges whose stock lists are synced automatically.
var ChineseMarketCodes = []string{"SSE", "SZSE", "BSE"}

// StockSyncScheduler periodically syncs the stock lists for Chinese markets.
// It auto-syncs on startup when the stocks table is empty, then re-syncs
// every configured interval (default 24h) to keep the list fresh.
type StockSyncScheduler struct {
	stockService *StockService
	interval     time.Duration

	mu        sync.Mutex
	isRunning bool
	stopCh    chan struct{}
	doneCh    chan struct{}
}

// NewStockSyncScheduler creates a scheduler that re-syncs stock lists every interval.
// It returns nil when the stock service is unavailable.
func NewStockSyncScheduler(stockService *StockService) *StockSyncScheduler {
	if stockService == nil {
		return nil
	}
	return &StockSyncScheduler{
		stockService: stockService,
		interval:     getStockSyncInterval(),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

// Start begins the scheduled stock sync loop. On first run it syncs immediately
// if the stocks table is empty, then re-syncs every interval.
func (s *StockSyncScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		log.Println("[StockSyncScheduler] Already running, skipping Start")
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	util.SafeGo(func() { s.loop(ctx) })
	log.Printf("[StockSyncScheduler] Started — auto sync on startup, re-sync every %s", s.interval)
}

// Stop gracefully stops the scheduler.
func (s *StockSyncScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}
	close(s.stopCh)
	<-s.doneCh
	s.isRunning = false
	log.Println("[StockSyncScheduler] Stopped")
}

// loop is the main scheduling loop.
func (s *StockSyncScheduler) loop(ctx context.Context) {
	defer close(s.doneCh)

	// Auto-sync on startup if the stocks table is empty.
	if s.needsInitialSync() {
		s.syncAll()
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.syncAll()
		}
	}
}

// needsInitialSync reports whether the stocks table currently has no rows.
func (s *StockSyncScheduler) needsInitialSync() bool {
	var count int64
	if err := s.stockService.db.Model(&models.Stock{}).Count(&count).Error; err != nil {
		log.Printf("[StockSyncScheduler] Failed to check stock count: %v", err)
		return false
	}
	return count == 0
}

// syncAll fetches and upserts the stock list for all Chinese markets.
func (s *StockSyncScheduler) syncAll() {
	log.Println("[StockSyncScheduler] Syncing stock lists for Chinese markets...")
	for _, code := range ChineseMarketCodes {
		if err := s.stockService.SyncStocksFromMarket(code); err != nil {
			log.Printf("[StockSyncScheduler] Sync failed for %s: %v", code, err)
		}
	}
	log.Println("[StockSyncScheduler] Stock list sync completed")
}

// getStockSyncInterval returns the sync interval, configurable via the
// STOCK_SYNC_INTERVAL_HOURS environment variable (default 24h).
func getStockSyncInterval() time.Duration {
	hours := 24
	if v := os.Getenv("STOCK_SYNC_INTERVAL_HOURS"); v != "" {
		if h, err := strconv.Atoi(v); err == nil && h > 0 {
			hours = h
		}
	}
	return time.Duration(hours) * time.Hour
}
