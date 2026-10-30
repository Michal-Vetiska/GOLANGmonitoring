// Package models defines the core domain types shared across PulseWatch.
package models

import "time"

// MonitorStatus represents the derived health of a monitored endpoint.
type MonitorStatus string

const (
	StatusUp      MonitorStatus = "UP"
	StatusDown    MonitorStatus = "DOWN"
	StatusUnknown MonitorStatus = "UNKNOWN"
)

// Monitor is a user-defined HTTP endpoint to check periodically.
type Monitor struct {
	ID              string        `json:"id"`
	Name            string        `json:"name"`
	URL             string        `json:"url"`
	IntervalSeconds int           `json:"interval_seconds"`
	Enabled         bool          `json:"enabled"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	// Latest check summary (joined / denormalized for API responses).
	Status          MonitorStatus `json:"status"`
	LastStatusCode  *int          `json:"last_status_code,omitempty"`
	LastResponseMs  *int64        `json:"last_response_time_ms,omitempty"`
	LastCheckedAt   *time.Time    `json:"last_checked_at,omitempty"`
	LastError       *string       `json:"last_error,omitempty"`
	LastIsUp        *bool         `json:"last_is_up,omitempty"`
}

// CheckResult is one observation produced by the monitoring engine.
type CheckResult struct {
	ID             int64     `json:"id"`
	MonitorID      string    `json:"monitor_id"`
	CheckedAt      time.Time `json:"checked_at"`
	ResponseTimeMs *int64    `json:"response_time_ms,omitempty"`
	StatusCode     *int      `json:"status_code,omitempty"`
	IsUp           bool      `json:"is_up"`
	ErrorMessage   *string   `json:"error_message,omitempty"`
}

// CreateMonitorRequest is the JSON body for POST /api/monitors.
type CreateMonitorRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	IntervalSeconds *int   `json:"interval_seconds,omitempty"`
	Enabled         *bool  `json:"enabled,omitempty"`
}

// UpdateMonitorRequest is the JSON body for PATCH /api/monitors/{id}.
// Pointer fields allow partial updates (nil = leave unchanged).
type UpdateMonitorRequest struct {
	Name            *string `json:"name,omitempty"`
	URL             *string `json:"url,omitempty"`
	IntervalSeconds *int    `json:"interval_seconds,omitempty"`
	Enabled         *bool   `json:"enabled,omitempty"`
}

// DashboardStats aggregates high-level metrics for the UI summary cards.
type DashboardStats struct {
	TotalMonitors    int     `json:"total_monitors"`
	OnlineMonitors   int     `json:"online_monitors"`
	OfflineMonitors  int     `json:"offline_monitors"`
	UnknownMonitors  int     `json:"unknown_monitors"`
	AvgResponseTime  float64 `json:"avg_response_time_ms"`
	OverallUptimePct float64 `json:"overall_uptime_pct"`
}

// MonitorStats holds detail-page statistics for a single monitor.
type MonitorStats struct {
	UptimePct       float64 `json:"uptime_pct"`
	AvgResponseMs   float64 `json:"avg_response_time_ms"`
	MinResponseMs   *int64  `json:"min_response_time_ms,omitempty"`
	MaxResponseMs   *int64  `json:"max_response_time_ms,omitempty"`
	TotalChecks     int     `json:"total_checks"`
	SuccessfulChecks int    `json:"successful_checks"`
}

// SSEEvent is broadcast to connected Server-Sent Events clients.
type SSEEvent struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

const (
	EventCheckResult   = "check_result"
	EventMonitorUpdate = "monitor_update"
	EventMonitorDelete = "monitor_delete"
)
