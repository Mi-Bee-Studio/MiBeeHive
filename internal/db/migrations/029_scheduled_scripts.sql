-- 029_scheduled_scripts.sql
-- Scheduled script execution (#70): user-managed shell scripts run on 5-field
-- cron schedules, with run history (stdout/stderr, exit code) kept in SQLite.
--
-- scheduled_scripts: one row per task. script_path is relative to the scripts
-- directory derived from the data dir (see service.ScriptService).
-- script_runs: one row per execution attempt; finished_at NULL while running.
-- (trigger_type, not trigger: TRIGGER is a SQLite keyword.)

CREATE TABLE IF NOT EXISTS scheduled_scripts (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    name           TEXT NOT NULL UNIQUE,
    schedule       TEXT NOT NULL,
    script_path    TEXT NOT NULL,
    enabled        INTEGER NOT NULL DEFAULT 1,
    timeout_seconds INTEGER NOT NULL DEFAULT 300,
    last_run_at    DATETIME,
    last_status    TEXT,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS script_runs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    script_id   INTEGER NOT NULL REFERENCES scheduled_scripts(id) ON DELETE CASCADE,
    started_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    finished_at DATETIME,
    exit_code   INTEGER,
    trigger_type TEXT NOT NULL DEFAULT 'cron',
    stdout      TEXT,
    stderr      TEXT,
    error       TEXT
);

CREATE INDEX IF NOT EXISTS idx_script_runs_script ON script_runs(script_id, id DESC);
