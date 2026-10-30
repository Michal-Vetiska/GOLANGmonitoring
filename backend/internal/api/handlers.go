package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/events"
	"github.com/pulsewatch/pulsewatch/backend/internal/models"
	"github.com/pulsewatch/pulsewatch/backend/internal/monitor"
	"github.com/pulsewatch/pulsewatch/backend/internal/storage"
)

// Server wires HTTP handlers to storage, engine, and the SSE hub.
type Server struct {
	store  *storage.Store
	engine *monitor.Engine
	hub    *events.Hub
	log    *slog.Logger
}

// NewServer constructs the API server.
func NewServer(store *storage.Store, engine *monitor.Engine, hub *events.Hub, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{store: store, engine: engine, hub: hub, log: logger}
}

// Handler returns the root http.Handler with all routes registered.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/monitors", s.handleListMonitors)
	mux.HandleFunc("POST /api/monitors", s.handleCreateMonitor)
	mux.HandleFunc("GET /api/monitors/{id}", s.handleGetMonitor)
	mux.HandleFunc("PATCH /api/monitors/{id}", s.handleUpdateMonitor)
	mux.HandleFunc("DELETE /api/monitors/{id}", s.handleDeleteMonitor)
	mux.HandleFunc("POST /api/monitors/{id}/check", s.handleCheckMonitor)
	mux.HandleFunc("GET /api/monitors/{id}/history", s.handleHistory)
	mux.HandleFunc("GET /api/monitors/{id}/stats", s.handleMonitorStats)
	mux.HandleFunc("GET /api/stats", s.handleDashboardStats)
	mux.HandleFunc("GET /api/events", s.handleSSE)

	return corsMiddleware(loggingMiddleware(s.log, mux))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"service": "pulsewatch",
		"time":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleListMonitors(w http.ResponseWriter, r *http.Request) {
	monitors, err := s.store.ListMonitors(r.Context())
	if err != nil {
		s.log.Error("list monitors", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list monitors")
		return
	}
	if monitors == nil {
		monitors = []models.Monitor{}
	}
	writeJSON(w, http.StatusOK, monitors)
}

func (s *Server) handleCreateMonitor(w http.ResponseWriter, r *http.Request) {
	var req models.CreateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	name, url, interval, enabled, err := validateCreate(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := s.store.CreateMonitor(r.Context(), name, url, interval, enabled)
	if err != nil {
		s.log.Error("create monitor", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create monitor")
		return
	}
	s.hub.Publish(models.SSEEvent{Type: models.EventMonitorUpdate, Payload: m})
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) handleGetMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		s.log.Error("get monitor", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to get monitor")
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleUpdateMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req models.UpdateMonitorRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if err := validateUpdate(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	m, err := s.store.UpdateMonitor(r.Context(), id, req.Name, req.URL, req.IntervalSeconds, req.Enabled)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		s.log.Error("update monitor", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to update monitor")
		return
	}
	s.hub.Publish(models.SSEEvent{Type: models.EventMonitorUpdate, Payload: m})
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) handleDeleteMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteMonitor(r.Context(), id); errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		s.log.Error("delete monitor", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to delete monitor")
		return
	}
	s.hub.Publish(models.SSEEvent{Type: models.EventMonitorDelete, Payload: map[string]string{"id": id}})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleCheckMonitor(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	m, err := s.store.GetMonitor(r.Context(), id)
	if errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get monitor")
		return
	}
	result := s.engine.TriggerCheckSync(r.Context(), m.ID, m.URL)
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetMonitor(r.Context(), id); errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get monitor")
		return
	}
	limit := 100
	if q := r.URL.Query().Get("limit"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			limit = n
		}
	}
	history, err := s.store.GetHistory(r.Context(), id, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get history")
		return
	}
	if history == nil {
		history = []models.CheckResult{}
	}
	writeJSON(w, http.StatusOK, history)
}

func (s *Server) handleMonitorStats(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetMonitor(r.Context(), id); errors.Is(err, storage.ErrNotFound) {
		writeError(w, http.StatusNotFound, "monitor not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get monitor")
		return
	}
	stats, err := s.store.GetMonitorStats(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleDashboardStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.store.GetDashboardStats(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := s.hub.Subscribe()
	defer s.hub.Unsubscribe(ch)

	// Initial comment keeps intermediaries from buffering.
	_, _ = w.Write([]byte(": connected\n\n"))
	flusher.Flush()

	heartbeat := time.NewTicker(25 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case payload, ok := <-ch:
			if !ok {
				return
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(payload)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if r.URL.Path != "/api/events" {
			log.Info("http",
				"method", r.Method,
				"path", r.URL.Path,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}
	})
}
