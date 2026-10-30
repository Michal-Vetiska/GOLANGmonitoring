package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/pulsewatch/pulsewatch/backend/internal/api"
	"github.com/pulsewatch/pulsewatch/backend/internal/events"
	"github.com/pulsewatch/pulsewatch/backend/internal/models"
	"github.com/pulsewatch/pulsewatch/backend/internal/monitor"
	"github.com/pulsewatch/pulsewatch/backend/internal/storage"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg := loadConfig()

	if dir := filepath.Dir(cfg.DBPath); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			logger.Error("failed to create data directory", "dir", dir, "err", err)
			os.Exit(1)
		}
	}

	store, err := storage.Open(cfg.DBPath)
	if err != nil {
		logger.Error("failed to open database", "err", err)
		os.Exit(1)
	}
	defer store.Close()

	hub := events.NewHub(logger)
	checker := monitor.NewChecker(cfg.HTTPTimeout)

	onResult := func(result models.CheckResult) models.CheckResult {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		saved, err := store.SaveCheckResult(ctx, result)
		if err != nil {
			logger.Error("failed to save check result", "err", err, "monitor_id", result.MonitorID)
			return result
		}
		hub.Publish(models.SSEEvent{Type: models.EventCheckResult, Payload: saved})
		return saved
	}

	engine := monitor.NewEngine(store, checker, onResult, cfg.Workers, logger)
	server := api.NewServer(store, engine, hub, logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go engine.Run(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      0, // SSE needs long-lived writes; per-request ctx still applies
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("PulseWatch API listening", "addr", cfg.Addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server error", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", "err", err)
	}
	logger.Info("PulseWatch stopped")
}

type config struct {
	Addr        string
	DBPath      string
	Workers     int
	HTTPTimeout time.Duration
}

func loadConfig() config {
	addr := envOr("PULSEWATCH_ADDR", ":8080")
	dbPath := envOr("PULSEWATCH_DB", "./data/pulsewatch.db")
	workers, _ := strconv.Atoi(envOr("PULSEWATCH_WORKERS", "5"))
	timeoutSec, _ := strconv.Atoi(envOr("PULSEWATCH_HTTP_TIMEOUT_SEC", "5"))
	if workers <= 0 {
		workers = monitor.DefaultWorkers
	}
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	return config{
		Addr:        addr,
		DBPath:      dbPath,
		Workers:     workers,
		HTTPTimeout: time.Duration(timeoutSec) * time.Second,
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
