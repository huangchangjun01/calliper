package services

import (
	"context"
	"log"
	"sync"
	"time"
)

// BackfillProgress tracks the progress of a backfill operation.
type BackfillProgress struct {
	Symbol    string    `json:"symbol"`
	Status    string    `json:"status"` // pending, running, completed, failed
	Progress  int       `json:"progress"` // percentage 0-100
	Records   int       `json:"records"`
	Error     string    `json:"error,omitempty"`
	UpdatedAt time.Time `json:"updated_at"`
}

// HistoryBackfill handles historical data backfill tasks.
type HistoryBackfill struct {
	service   *MarketDataService
	workers   int
	mu        sync.Mutex
	progress  map[string]*BackfillProgress
	checkpoints map[string]time.Time // symbol -> last completed timestamp
}

// NewHistoryBackfill creates a new HistoryBackfill instance.
func NewHistoryBackfill(service *MarketDataService, workers int) *HistoryBackfill {
	if workers <= 0 {
		workers = 4
	}
	return &HistoryBackfill{
		service:     service,
		workers:     workers,
		progress:    make(map[string]*BackfillProgress),
		checkpoints: make(map[string]time.Time),
	}
}

// BackfillDailyData backfills daily kline data for the given symbols.
func (hb *HistoryBackfill) BackfillDailyData(ctx context.Context, symbols []string, years int) error {
	log.Printf("[HistoryBackfill] Starting daily data backfill for %d symbols, %d years", len(symbols), years)

	end := time.Now()
	start := end.AddDate(-years, 0, 0)

	return hb.backfill(ctx, symbols, start, end, "1d")
}

// BackfillMinuteData backfills minute kline data for the given symbols.
func (hb *HistoryBackfill) BackfillMinuteData(ctx context.Context, symbols []string, months int) error {
	log.Printf("[HistoryBackfill] Starting minute data backfill for %d symbols, %d months", len(symbols), months)

	end := time.Now()
	start := end.AddDate(0, -months, 0)

	return hb.backfill(ctx, symbols, start, end, "1m")
}

// backfill is the core backfill logic with goroutine pool and checkpoint support.
func (hb *HistoryBackfill) backfill(ctx context.Context, symbols []string, start, end time.Time, interval string) error {
	symbolsCh := make(chan string, len(symbols))
	errCh := make(chan error, len(symbols))
	var wg sync.WaitGroup

	// Initialize progress tracking
	for _, symbol := range symbols {
		hb.progress[symbol] = &BackfillProgress{
			Symbol:    symbol,
			Status:    "pending",
			Progress:  0,
			UpdatedAt: time.Now(),
		}
	}

	// Feed symbols into the channel
	for _, sym := range symbols {
		symbolsCh <- sym
	}
	close(symbolsCh)

	// Start worker pool
	for i := 0; i < hb.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for symbol := range symbolsCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				hb.updateProgress(symbol, "running", 0)

				// Checkpoint: resume from last completed timestamp
				hb.mu.Lock()
				checkpoint, hasCheckpoint := hb.checkpoints[symbol]
				hb.mu.Unlock()

				symbolStart := start
				if hasCheckpoint && checkpoint.After(start) {
					symbolStart = checkpoint
					log.Printf("[HistoryBackfill] Resuming %s from checkpoint: %s", symbol, symbolStart.Format("2006-01-02"))
				}

				// Determine which collector to use based on the symbol
				collector := hb.detectCollector(symbol)
				if collector == nil {
					hb.updateProgress(symbol, "failed", 0)
					errCh <- nil
					continue
				}

				data, err := collector.FetchHistoricalData(symbol, symbolStart, end, interval)
				if err != nil {
					log.Printf("[HistoryBackfill] Failed to backfill %s: %v", symbol, err)
					hb.updateProgress(symbol, "failed", 0)
					hb.mu.Lock()
					hb.progress[symbol].Error = err.Error()
					hb.mu.Unlock()
					errCh <- err
					continue
				}

				// Clean and publish data
				cleaned := hb.service.GetCleaner().CleanMarketData(data)
				kafkaProd := hb.service.GetKafkaProducer()
				if kafkaProd != nil {
					_ = kafkaProd.PublishMarketData(ctx, cleaned)
				}

				// Update checkpoint
				if len(data) > 0 {
					hb.mu.Lock()
					hb.checkpoints[symbol] = data[len(data)-1].Timestamp
					hb.mu.Unlock()
				}

				hb.updateProgress(symbol, "completed", len(data))
				errCh <- nil
			}
		}(i)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(errCh)
	}()

	// Collect errors
	var firstErr error
	for err := range errCh {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		log.Printf("[HistoryBackfill] Backfill completed with errors: %v", firstErr)
	} else {
		log.Printf("[HistoryBackfill] Backfill completed successfully for %d symbols", len(symbols))
	}

	return firstErr
}

// detectCollector determines which collector to use based on the symbol pattern.
func (hb *HistoryBackfill) detectCollector(symbol string) MarketDataCollector {
	collectors := hb.service.GetCollectors()

	// Chinese A-share symbols are numeric
	isNumeric := true
	for _, ch := range symbol {
		if ch < '0' || ch > '9' {
			isNumeric = false
			break
		}
	}
	if isNumeric && len(symbol) == 6 {
		return collectors["CN"]
	}

	// HK stocks end with .HK
	if len(symbol) > 3 && symbol[len(symbol)-3:] == ".HK" {
		if c, ok := collectors["HK"]; ok {
			return c
		}
		// Fallback to US collector
		return collectors["US"]
	}

	// Default to US collector
	return collectors["US"]
}

// updateProgress updates the progress of a backfill operation.
func (hb *HistoryBackfill) updateProgress(symbol, status string, records int) {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	if p, ok := hb.progress[symbol]; ok {
		p.Status = status
		p.Records = records
		if status == "completed" {
			p.Progress = 100
		}
		p.UpdatedAt = time.Now()
	}
}

// GetProgress returns the current backfill progress for all symbols.
func (hb *HistoryBackfill) GetProgress() map[string]*BackfillProgress {
	hb.mu.Lock()
	defer hb.mu.Unlock()

	result := make(map[string]*BackfillProgress, len(hb.progress))
	for k, v := range hb.progress {
		copy := *v
		result[k] = &copy
	}
	return result
}

// ClearCheckpoints clears all backfill checkpoints.
func (hb *HistoryBackfill) ClearCheckpoints() {
	hb.mu.Lock()
	defer hb.mu.Unlock()
	hb.checkpoints = make(map[string]time.Time)
	log.Println("[HistoryBackfill] Checkpoints cleared")
}