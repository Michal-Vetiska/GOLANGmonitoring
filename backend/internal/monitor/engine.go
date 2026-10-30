package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/models"
)

const (
	DefaultInterval = 30
	DefaultWorkers  = 5
	MinInterval     = 10
	MaxInterval     = 3600
)

// ResultHandler is called after each check completes (persist + broadcast).
// It may return an enriched result (e.g. with a database ID).
type ResultHandler func(result models.CheckResult) models.CheckResult

// MonitorSource supplies the current set of enabled monitors to the scheduler.
type MonitorSource interface {
	ListEnabled(ctx context.Context) ([]models.Monitor, error)
}

// HTTPChecker performs a single endpoint check. *Checker implements this in production;
// tests can supply a lighter implementation that talks to httptest.
type HTTPChecker interface {
	Check(ctx context.Context, monitorID, rawURL string) models.CheckResult
}

// Engine is the concurrent monitoring scheduler + bounded worker pool.
type Engine struct {
	source   MonitorSource
	checker  HTTPChecker
	onResult ResultHandler
	workers  int
	logger   *slog.Logger

	jobs     chan checkJob
	wg       sync.WaitGroup
	inflight sync.Map // monitorID -> struct{} — prevents duplicate concurrent checks
}

type checkJob struct {
	monitorID string
	url       string
}

// NewEngine constructs a monitoring engine. Call Run to start workers + scheduler.
func NewEngine(source MonitorSource, checker HTTPChecker, onResult ResultHandler, workers int, logger *slog.Logger) *Engine {
	if workers <= 0 {
		workers = DefaultWorkers
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		source:   source,
		checker:  checker,
		onResult: onResult,
		workers:  workers,
		logger:   logger,
		jobs:     make(chan checkJob, workers*4),
	}
}

// Run starts worker goroutines and the scheduler. It blocks until ctx is cancelled,
// then drains in-flight work and returns.
func (e *Engine) Run(ctx context.Context) {
	e.logger.Info("monitoring engine starting", "workers", e.workers)

	for i := 0; i < e.workers; i++ {
		e.wg.Add(1)
		go e.worker(ctx, i)
	}

	e.scheduler(ctx)

	close(e.jobs)
	e.wg.Wait()
	e.logger.Info("monitoring engine stopped")
}

func (e *Engine) worker(ctx context.Context, id int) {
	defer e.wg.Done()
	e.logger.Debug("worker started", "id", id)
	for job := range e.jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		checkCtx, cancel := context.WithTimeout(ctx, DefaultHTTPTimeout+time.Second)
		result := e.checker.Check(checkCtx, job.monitorID, job.url)
		cancel()

		if e.onResult != nil {
			_ = e.onResult(result)
		}
		e.inflight.Delete(job.monitorID)
	}
}

// scheduler ticks every second and enqueues due monitors.
func (e *Engine) scheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	lastQueued := make(map[string]time.Time)

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			monitors, err := e.source.ListEnabled(ctx)
			if err != nil {
				e.logger.Error("failed to list enabled monitors", "err", err)
				continue
			}
			for _, m := range monitors {
				interval := m.IntervalSeconds
				if interval < MinInterval {
					interval = DefaultInterval
				}
				last, ok := lastQueued[m.ID]
				if ok && now.Sub(last) < time.Duration(interval)*time.Second {
					continue
				}
				if !e.enqueue(m.ID, m.URL) {
					continue
				}
				lastQueued[m.ID] = now
			}
			active := make(map[string]struct{}, len(monitors))
			for _, m := range monitors {
				active[m.ID] = struct{}{}
			}
			for id := range lastQueued {
				if _, ok := active[id]; !ok {
					delete(lastQueued, id)
				}
			}
		}
	}
}

// enqueue submits a check job if one is not already in flight. Returns false if skipped.
func (e *Engine) enqueue(monitorID, url string) bool {
	if _, loaded := e.inflight.LoadOrStore(monitorID, struct{}{}); loaded {
		return false
	}
	job := checkJob{monitorID: monitorID, url: url}
	select {
	case e.jobs <- job:
		return true
	default:
		e.inflight.Delete(monitorID)
		e.logger.Warn("worker pool saturated, skipping check", "monitor_id", monitorID)
		return false
	}
}

// TriggerCheck enqueues an immediate manual check. Returns false if already in flight
// or the pool is saturated.
func (e *Engine) TriggerCheck(monitorID, url string) bool {
	return e.enqueue(monitorID, url)
}

// TriggerCheckSync runs a check on the calling goroutine (used by HTTP manual-check
// endpoint so the caller can receive the result immediately).
func (e *Engine) TriggerCheckSync(ctx context.Context, monitorID, url string) models.CheckResult {
	_, loaded := e.inflight.LoadOrStore(monitorID, struct{}{})
	if !loaded {
		defer e.inflight.Delete(monitorID)
	}
	checkCtx, cancel := context.WithTimeout(ctx, DefaultHTTPTimeout+time.Second)
	defer cancel()
	result := e.checker.Check(checkCtx, monitorID, url)
	if e.onResult != nil {
		result = e.onResult(result)
	}
	return result
}
