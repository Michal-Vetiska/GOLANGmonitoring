package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/models"
)

type staticSource struct {
	monitors []models.Monitor
}

func (s staticSource) ListEnabled(ctx context.Context) ([]models.Monitor, error) {
	return s.monitors, nil
}

// rawChecker talks to httptest without SSRF dial restrictions (tests only).
type rawChecker struct {
	inner *Checker
}

func (r rawChecker) Check(ctx context.Context, monitorID, rawURL string) models.CheckResult {
	return r.inner.CheckRaw(ctx, monitorID, rawURL)
}

func TestEngine_EnqueueRejectsDuplicateInflight(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got atomic.Int64
	engine := NewEngine(staticSource{}, rawChecker{NewTestChecker(2 * time.Second)}, func(r models.CheckResult) models.CheckResult {
		got.Add(1)
		return r
	}, 2, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine.wg.Add(1)
	go func() {
		defer engine.wg.Done()
		for job := range engine.jobs {
			res := engine.checker.Check(ctx, job.monitorID, job.url)
			if engine.onResult != nil {
				_ = engine.onResult(res)
			}
			engine.inflight.Delete(job.monitorID)
		}
	}()

	if !engine.enqueue("m1", srv.URL) {
		t.Fatal("first enqueue should succeed")
	}
	if engine.enqueue("m1", srv.URL) {
		t.Fatal("duplicate inflight enqueue should be rejected")
	}

	waitUntil(t, 2*time.Second, func() bool { return got.Load() >= 1 })

	cancel()
	close(engine.jobs)
	engine.wg.Wait()

	if hits.Load() < 1 {
		t.Fatal("expected at least one HTTP hit")
	}
}

func TestEngine_TriggerCheckSync(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var got atomic.Int64
	engine := NewEngine(staticSource{}, rawChecker{NewTestChecker(2 * time.Second)}, func(r models.CheckResult) models.CheckResult {
		got.Add(1)
		return r
	}, 1, nil)

	result := engine.TriggerCheckSync(context.Background(), "m1", srv.URL)
	if !result.IsUp {
		t.Fatalf("expected UP, err=%v", result.ErrorMessage)
	}
	if got.Load() != 1 {
		t.Fatalf("onResult calls = %d", got.Load())
	}
}

func TestEngine_GracefulCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	src := staticSource{monitors: []models.Monitor{{
		ID: "m1", URL: srv.URL, IntervalSeconds: 10, Enabled: true,
	}}}
	engine := NewEngine(src, rawChecker{NewTestChecker(2 * time.Second)}, nil, 2, nil)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	time.Sleep(1200 * time.Millisecond) // allow at least one scheduler tick
	cancel()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("engine did not stop after cancellation")
	}
}

func waitUntil(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}
