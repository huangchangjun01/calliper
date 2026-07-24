package services

import (
	"context"
	"log"
	"sync"
	"time"
)

// ──────────────────────────────────────────────────────────────
// EvaluationScheduler — 评估任务定时调度器
// ──────────────────────────────────────────────────────────────

// EvaluationScheduler manages the scheduled execution of daily
// prediction evaluation tasks.
type EvaluationScheduler struct {
	evalService *EvaluationService

	mu       sync.Mutex
	isRunning bool
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// NewEvaluationScheduler creates a new EvaluationScheduler.
func NewEvaluationScheduler(evalService *EvaluationService) *EvaluationScheduler {
	return &EvaluationScheduler{
		evalService: evalService,
		stopCh:      make(chan struct{}),
		doneCh:      make(chan struct{}),
	}
}

// Start begins the scheduled evaluation loop.
// It runs evaluation daily after market close:
//   - A股 (Asia/Shanghai): 15:30 CST
//   - 美股 (US/Eastern): 次日 5:00 EST → 17:00 CST
func (s *EvaluationScheduler) Start(ctx context.Context) {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		log.Println("[EvaluationScheduler] Already running, skipping Start")
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	go s.loop(ctx)
	log.Println("[EvaluationScheduler] Started — daily evaluation scheduled at 15:30 CST")
}

// Stop gracefully stops the scheduler.
func (s *EvaluationScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isRunning {
		return
	}

	close(s.stopCh)
	<-s.doneCh
	s.isRunning = false
	log.Println("[EvaluationScheduler] Stopped")
}

// RunEvaluation triggers an immediate evaluation run.
func (s *EvaluationScheduler) RunEvaluation() error {
	log.Println("[EvaluationScheduler] Manual evaluation triggered")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	return s.evalService.EvaluateDaily(ctx)
}

// loop is the main scheduling loop.
func (s *EvaluationScheduler) loop(ctx context.Context) {
	defer close(s.doneCh)

	// Calculate next run time: today at 15:30 CST
	nextRun := s.nextEvaluationTime()
	log.Printf("[EvaluationScheduler] Next evaluation scheduled at %s", nextRun.Format("2006-01-02 15:04:05"))

	for {
		select {
		case <-s.stopCh:
			return
		case <-ctx.Done():
			return
		case <-time.After(time.Until(nextRun)):
			log.Println("[EvaluationScheduler] Running daily evaluation...")
			evalCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
			if err := s.evalService.EvaluateDaily(evalCtx); err != nil {
				log.Printf("[EvaluationScheduler] Evaluation failed: %v", err)
			} else {
				log.Println("[EvaluationScheduler] Daily evaluation completed successfully")
			}
			cancel()

			// Schedule next run
			nextRun = s.nextEvaluationTime()
			log.Printf("[EvaluationScheduler] Next evaluation scheduled at %s", nextRun.Format("2006-01-02 15:04:05"))
		}
	}
}

// nextEvaluationTime calculates the next 15:30 CST evaluation time.
// If current time is before 15:30 today, returns today at 15:30.
// Otherwise returns tomorrow at 15:30.
func (s *EvaluationScheduler) nextEvaluationTime() time.Time {
	cst := time.FixedZone("CST", 8*60*60)
	now := time.Now().In(cst)

	// Today at 15:30 CST
	today := time.Date(now.Year(), now.Month(), now.Day(), 15, 30, 0, 0, cst)

	if now.Before(today) {
		return today
	}
	// Tomorrow at 15:30 CST
	return today.Add(24 * time.Hour)
}