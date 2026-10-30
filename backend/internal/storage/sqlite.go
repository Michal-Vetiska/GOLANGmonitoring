package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsewatch/pulsewatch/backend/internal/models"

	_ "modernc.org/sqlite"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

const (
	// MaxHistoryPerMonitor caps stored check results per monitor to limit DB growth.
	MaxHistoryPerMonitor = 500
	timeLayout           = time.RFC3339Nano
)

// Store wraps the SQLite database and provides monitor / result persistence.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database and applies migrations.
func Open(path string) (*Store, error) {
	// modernc.org/sqlite driver name is "sqlite".
	// _pragma=foreign_keys(1) enables FK enforcement.
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1) // SQLite + simple writer model
	db.SetMaxIdleConns(1)

	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) migrate() error {
	data, err := migrationFS.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read migration: %w", err)
	}
	if _, err := s.db.Exec(string(data)); err != nil {
		return fmt.Errorf("apply migration: %w", err)
	}
	return nil
}

// Close closes the underlying database.
func (s *Store) Close() error {
	return s.db.Close()
}

// CreateMonitor inserts a new monitor and returns it.
func (s *Store) CreateMonitor(ctx context.Context, name, url string, interval int, enabled bool) (*models.Monitor, error) {
	now := time.Now().UTC()
	m := &models.Monitor{
		ID:              uuid.NewString(),
		Name:            name,
		URL:             url,
		IntervalSeconds: interval,
		Enabled:         enabled,
		CreatedAt:       now,
		UpdatedAt:       now,
		Status:          models.StatusUnknown,
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO monitors (id, name, url, interval_seconds, enabled, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.Name, m.URL, m.IntervalSeconds, boolToInt(m.Enabled),
		m.CreatedAt.Format(timeLayout), m.UpdatedAt.Format(timeLayout),
	)
	if err != nil {
		return nil, fmt.Errorf("insert monitor: %w", err)
	}
	return m, nil
}

// ListMonitors returns all monitors with their latest check summary.
func (s *Store) ListMonitors(ctx context.Context) ([]models.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.name, m.url, m.interval_seconds, m.enabled, m.created_at, m.updated_at,
		       r.checked_at, r.response_time_ms, r.status_code, r.is_up, r.error_message
		FROM monitors m
		LEFT JOIN check_results r ON r.id = (
			SELECT id FROM check_results WHERE monitor_id = m.id ORDER BY checked_at DESC LIMIT 1
		)
		ORDER BY m.created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("list monitors: %w", err)
	}
	defer rows.Close()

	var out []models.Monitor
	for rows.Next() {
		m, err := scanMonitorWithLatest(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// ListEnabled implements monitor.MonitorSource for the scheduler.
func (s *Store) ListEnabled(ctx context.Context) ([]models.Monitor, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, url, interval_seconds, enabled, created_at, updated_at
		FROM monitors WHERE enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("list enabled: %w", err)
	}
	defer rows.Close()

	var out []models.Monitor
	for rows.Next() {
		m, err := scanMonitor(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetMonitor returns one monitor by ID with latest check summary.
func (s *Store) GetMonitor(ctx context.Context, id string) (*models.Monitor, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT m.id, m.name, m.url, m.interval_seconds, m.enabled, m.created_at, m.updated_at,
		       r.checked_at, r.response_time_ms, r.status_code, r.is_up, r.error_message
		FROM monitors m
		LEFT JOIN check_results r ON r.id = (
			SELECT id FROM check_results WHERE monitor_id = m.id ORDER BY checked_at DESC LIMIT 1
		)
		WHERE m.id = ?`, id)

	m, err := scanMonitorWithLatest(row)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

// UpdateMonitor applies a partial update.
func (s *Store) UpdateMonitor(ctx context.Context, id string, name, url *string, interval *int, enabled *bool) (*models.Monitor, error) {
	existing, err := s.GetMonitor(ctx, id)
	if err != nil {
		return nil, err
	}
	if name != nil {
		existing.Name = *name
	}
	if url != nil {
		existing.URL = *url
	}
	if interval != nil {
		existing.IntervalSeconds = *interval
	}
	if enabled != nil {
		existing.Enabled = *enabled
	}
	existing.UpdatedAt = time.Now().UTC()

	_, err = s.db.ExecContext(ctx, `
		UPDATE monitors SET name=?, url=?, interval_seconds=?, enabled=?, updated_at=? WHERE id=?`,
		existing.Name, existing.URL, existing.IntervalSeconds, boolToInt(existing.Enabled),
		existing.UpdatedAt.Format(timeLayout), id,
	)
	if err != nil {
		return nil, fmt.Errorf("update monitor: %w", err)
	}
	return s.GetMonitor(ctx, id)
}

// DeleteMonitor removes a monitor (cascade deletes check_results).
func (s *Store) DeleteMonitor(ctx context.Context, id string) error {
	res, err := s.db.ExecContext(ctx, `DELETE FROM monitors WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete monitor: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// SaveCheckResult persists a result and prunes old history for that monitor.
func (s *Store) SaveCheckResult(ctx context.Context, r models.CheckResult) (models.CheckResult, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO check_results (monitor_id, checked_at, response_time_ms, status_code, is_up, error_message)
		VALUES (?, ?, ?, ?, ?, ?)`,
		r.MonitorID, r.CheckedAt.Format(timeLayout), r.ResponseTimeMs, r.StatusCode, boolToInt(r.IsUp), r.ErrorMessage,
	)
	if err != nil {
		return r, fmt.Errorf("insert check result: %w", err)
	}
	id, _ := res.LastInsertId()
	r.ID = id

	// Keep only the newest MaxHistoryPerMonitor rows for this monitor.
	_, _ = s.db.ExecContext(ctx, `
		DELETE FROM check_results
		WHERE monitor_id = ?
		  AND id NOT IN (
			SELECT id FROM check_results
			WHERE monitor_id = ?
			ORDER BY checked_at DESC
			LIMIT ?
		  )`, r.MonitorID, r.MonitorID, MaxHistoryPerMonitor)

	return r, nil
}

// GetHistory returns the most recent check results for a monitor.
func (s *Store) GetHistory(ctx context.Context, monitorID string, limit int) ([]models.CheckResult, error) {
	if limit <= 0 || limit > MaxHistoryPerMonitor {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, monitor_id, checked_at, response_time_ms, status_code, is_up, error_message
		FROM check_results
		WHERE monitor_id = ?
		ORDER BY checked_at DESC
		LIMIT ?`, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	defer rows.Close()

	var out []models.CheckResult
	for rows.Next() {
		r, err := scanResult(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetDashboardStats computes summary metrics across all monitors.
func (s *Store) GetDashboardStats(ctx context.Context) (*models.DashboardStats, error) {
	monitors, err := s.ListMonitors(ctx)
	if err != nil {
		return nil, err
	}
	stats := &models.DashboardStats{TotalMonitors: len(monitors)}
	var sumMs float64
	var countMs int
	var upChecks, totalChecks int

	for _, m := range monitors {
		switch m.Status {
		case models.StatusUp:
			stats.OnlineMonitors++
		case models.StatusDown:
			stats.OfflineMonitors++
		default:
			stats.UnknownMonitors++
		}
		if m.LastResponseMs != nil {
			sumMs += float64(*m.LastResponseMs)
			countMs++
		}
	}
	if countMs > 0 {
		stats.AvgResponseTime = sumMs / float64(countMs)
	}

	row := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN is_up = 1 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM check_results`)
	if err := row.Scan(&upChecks, &totalChecks); err != nil {
		return nil, err
	}
	if totalChecks > 0 {
		stats.OverallUptimePct = float64(upChecks) / float64(totalChecks) * 100
	}
	return stats, nil
}

// GetMonitorStats computes detail statistics for one monitor.
func (s *Store) GetMonitorStats(ctx context.Context, monitorID string) (*models.MonitorStats, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN is_up = 1 THEN 1 ELSE 0 END), 0),
			AVG(response_time_ms),
			MIN(response_time_ms),
			MAX(response_time_ms)
		FROM check_results
		WHERE monitor_id = ?`, monitorID)

	var total, success int
	var avg sql.NullFloat64
	var minMs, maxMs sql.NullInt64
	if err := row.Scan(&total, &success, &avg, &minMs, &maxMs); err != nil {
		return nil, err
	}
	st := &models.MonitorStats{
		TotalChecks:      total,
		SuccessfulChecks: success,
	}
	if total > 0 {
		st.UptimePct = float64(success) / float64(total) * 100
	}
	if avg.Valid {
		st.AvgResponseMs = avg.Float64
	}
	if minMs.Valid {
		v := minMs.Int64
		st.MinResponseMs = &v
	}
	if maxMs.Valid {
		v := maxMs.Int64
		st.MaxResponseMs = &v
	}
	return st, nil
}

// ErrNotFound indicates a missing resource.
var ErrNotFound = fmt.Errorf("not found")

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func parseTime(s string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

type scannable interface {
	Scan(dest ...any) error
}

func scanMonitor(row scannable) (models.Monitor, error) {
	var m models.Monitor
	var enabled int
	var created, updated string
	err := row.Scan(&m.ID, &m.Name, &m.URL, &m.IntervalSeconds, &enabled, &created, &updated)
	if err != nil {
		return m, err
	}
	m.Enabled = enabled == 1
	m.CreatedAt, err = parseTime(created)
	if err != nil {
		return m, err
	}
	m.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return m, err
	}
	m.Status = models.StatusUnknown
	return m, nil
}

func scanMonitorWithLatest(row scannable) (models.Monitor, error) {
	var m models.Monitor
	var enabled int
	var created, updated string
	var checkedAt sql.NullString
	var respMs sql.NullInt64
	var statusCode sql.NullInt64
	var isUp sql.NullInt64
	var errMsg sql.NullString

	err := row.Scan(
		&m.ID, &m.Name, &m.URL, &m.IntervalSeconds, &enabled, &created, &updated,
		&checkedAt, &respMs, &statusCode, &isUp, &errMsg,
	)
	if err != nil {
		return m, err
	}
	m.Enabled = enabled == 1
	m.CreatedAt, err = parseTime(created)
	if err != nil {
		return m, err
	}
	m.UpdatedAt, err = parseTime(updated)
	if err != nil {
		return m, err
	}
	m.Status = models.StatusUnknown
	if checkedAt.Valid {
		t, err := parseTime(checkedAt.String)
		if err != nil {
			return m, err
		}
		m.LastCheckedAt = &t
		if respMs.Valid {
			v := respMs.Int64
			m.LastResponseMs = &v
		}
		if statusCode.Valid {
			v := int(statusCode.Int64)
			m.LastStatusCode = &v
		}
		if isUp.Valid {
			up := isUp.Int64 == 1
			m.LastIsUp = &up
			if up {
				m.Status = models.StatusUp
			} else {
				m.Status = models.StatusDown
			}
		}
		if errMsg.Valid {
			msg := errMsg.String
			m.LastError = &msg
		}
	}
	return m, nil
}

func scanResult(row scannable) (models.CheckResult, error) {
	var r models.CheckResult
	var checked string
	var respMs sql.NullInt64
	var statusCode sql.NullInt64
	var isUp int
	var errMsg sql.NullString
	err := row.Scan(&r.ID, &r.MonitorID, &checked, &respMs, &statusCode, &isUp, &errMsg)
	if err != nil {
		return r, err
	}
	r.CheckedAt, err = parseTime(checked)
	if err != nil {
		return r, err
	}
	r.IsUp = isUp == 1
	if respMs.Valid {
		v := respMs.Int64
		r.ResponseTimeMs = &v
	}
	if statusCode.Valid {
		v := int(statusCode.Int64)
		r.StatusCode = &v
	}
	if errMsg.Valid {
		msg := errMsg.String
		r.ErrorMessage = &msg
	}
	return r, nil
}
