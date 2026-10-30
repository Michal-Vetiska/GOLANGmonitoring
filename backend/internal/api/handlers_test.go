package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/events"
	"github.com/pulsewatch/pulsewatch/backend/internal/models"
	"github.com/pulsewatch/pulsewatch/backend/internal/monitor"
	"github.com/pulsewatch/pulsewatch/backend/internal/storage"
)

type fakeChecker struct {
	result models.CheckResult
}

func (f fakeChecker) Check(ctx context.Context, monitorID, rawURL string) models.CheckResult {
	r := f.result
	r.MonitorID = monitorID
	r.CheckedAt = time.Now().UTC()
	return r
}

func setupServer(t *testing.T) (*Server, *storage.Store) {
	t.Helper()
	dir := t.TempDir()
	store, err := storage.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	hub := events.NewHub(nil)
	code := 200
	ms := int64(12)
	engine := monitor.NewEngine(store, fakeChecker{result: models.CheckResult{
		IsUp: true, StatusCode: &code, ResponseTimeMs: &ms,
	}}, func(r models.CheckResult) models.CheckResult {
		saved, _ := store.SaveCheckResult(context.Background(), r)
		hub.Publish(models.SSEEvent{Type: models.EventCheckResult, Payload: saved})
		return saved
	}, 2, nil)

	return NewServer(store, engine, hub, nil), store
}

func TestHealth(t *testing.T) {
	srv, _ := setupServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestCreateMonitor_Validation(t *testing.T) {
	srv, _ := setupServer(t)

	body := `{"name":"Bad","url":"http://127.0.0.1/secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestMonitorCRUD(t *testing.T) {
	srv, _ := setupServer(t)

	body := `{"name":"Example","url":"https://example.com","interval_seconds":30}`
	req := httptest.NewRequest(http.MethodPost, "/api/monitors", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", rec.Code, rec.Body.String())
	}

	var created models.Monitor
	if err := json.NewDecoder(rec.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if created.ID == "" || created.Name != "Example" {
		t.Fatalf("unexpected monitor: %+v", created)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/monitors/"+created.ID, nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status=%d", rec.Code)
	}

	patch := `{"enabled":false}`
	req = httptest.NewRequest(http.MethodPatch, "/api/monitors/"+created.ID, bytes.NewBufferString(patch))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch status=%d body=%s", rec.Code, rec.Body.String())
	}
	var updated models.Monitor
	_ = json.NewDecoder(rec.Body).Decode(&updated)
	if updated.Enabled {
		t.Fatal("expected disabled")
	}

	req = httptest.NewRequest(http.MethodPost, "/api/monitors/"+created.ID+"/check", nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("check status=%d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodDelete, "/api/monitors/"+created.ID, nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/monitors/"+created.ID, nil)
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestListMonitorsEmpty(t *testing.T) {
	srv, _ := setupServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/monitors", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	var list []models.Monitor
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("expected empty array, got %#v", list)
	}
}
