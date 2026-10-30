package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestCheckRaw_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := NewTestChecker(2 * time.Second)
	result := c.CheckRaw(context.Background(), "mon-1", srv.URL)
	if !result.IsUp {
		t.Fatalf("expected UP, err=%v", deref(result.ErrorMessage))
	}
	if result.StatusCode == nil || *result.StatusCode != 200 {
		t.Fatalf("status = %v", result.StatusCode)
	}
	if result.ResponseTimeMs == nil {
		t.Fatal("expected response time")
	}
}

func TestCheckRaw_DownStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewTestChecker(2 * time.Second)
	result := c.CheckRaw(context.Background(), "mon-1", srv.URL)
	if result.IsUp {
		t.Fatal("expected DOWN for 500")
	}
	if result.StatusCode == nil || *result.StatusCode != 500 {
		t.Fatalf("status = %v", result.StatusCode)
	}
}

func TestCheckRaw_RedirectCountsAsUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.com", http.StatusFound)
	}))
	defer srv.Close()

	c := NewTestChecker(2 * time.Second)
	result := c.CheckRaw(context.Background(), "mon-1", srv.URL)
	if !result.IsUp {
		t.Fatalf("3xx should count as UP, err=%v", deref(result.ErrorMessage))
	}
	if result.StatusCode == nil || *result.StatusCode != 302 {
		t.Fatalf("status = %v", result.StatusCode)
	}
}

func TestCheckRaw_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewTestChecker(50 * time.Millisecond)
	result := c.CheckRaw(context.Background(), "mon-1", srv.URL)
	if result.IsUp {
		t.Fatal("expected timeout failure")
	}
	if result.ErrorMessage == nil {
		t.Fatal("expected error message")
	}
}

func TestCheck_RejectsPrivateURL(t *testing.T) {
	c := NewChecker(2 * time.Second)
	result := c.Check(context.Background(), "mon-1", "http://127.0.0.1:9/")
	if result.IsUp {
		t.Fatal("localhost must be rejected")
	}
	if result.ErrorMessage == nil {
		t.Fatal("expected error")
	}
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
