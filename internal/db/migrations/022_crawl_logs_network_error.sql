-- 022_crawl_logs_network_error.sql
-- Add 'network_error' to crawl_logs.status CHECK constraint.
--
-- Issue #23: transient fetch failures (timeout, connection reset, 5xx) used to
-- land in the generic 'error' bucket alongside genuine upstream/config errors,
-- so operators couldn't tell "the source is misconfigured" from "the network
-- blinked" in logs/metrics. A dedicated status lets them be distinguished. It
-- is set only after retries are exhausted, by internal/crawler retry.go.
--
-- SQLite cannot ALTER a CHECK in place, so rebuild the table (same pattern as
-- 021_source_type_any.sql). Final schema matches 017_consolidated + the new
-- status value.

CREATE TABLE crawl_logs_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    status TEXT NOT NULL CHECK(status IN ('running','success','error','rate_limited','network_error')),
    versions_found INTEGER DEFAULT 0,
    files_downloaded INTEGER DEFAULT 0,
    error_message TEXT DEFAULT ''
);

INSERT INTO crawl_logs_new (id, project_id, started_at, finished_at, status,
                            versions_found, files_downloaded, error_message)
SELECT id, project_id, started_at, finished_at, status,
       versions_found, files_downloaded, error_message
FROM crawl_logs;

DROP TABLE crawl_logs;

ALTER TABLE crawl_logs_new RENAME TO crawl_logs;

CREATE INDEX IF NOT EXISTS idx_crawl_logs_project_id ON crawl_logs(project_id);
