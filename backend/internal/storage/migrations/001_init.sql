-- PulseWatch initial schema
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS monitors (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    url              TEXT NOT NULL,
    interval_seconds INTEGER NOT NULL DEFAULT 30,
    enabled          INTEGER NOT NULL DEFAULT 1,
    created_at       TEXT NOT NULL,
    updated_at       TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS check_results (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    monitor_id        TEXT NOT NULL,
    checked_at        TEXT NOT NULL,
    response_time_ms  INTEGER,
    status_code       INTEGER,
    is_up             INTEGER NOT NULL DEFAULT 0,
    error_message     TEXT,
    FOREIGN KEY (monitor_id) REFERENCES monitors(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_check_results_monitor_checked
    ON check_results(monitor_id, checked_at DESC);

CREATE INDEX IF NOT EXISTS idx_monitors_enabled
    ON monitors(enabled);
