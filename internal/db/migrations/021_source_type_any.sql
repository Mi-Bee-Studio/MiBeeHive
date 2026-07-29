-- 021_source_type_any.sql
-- Drop the closed CHECK constraint on projects.source_type.
--
-- The constraint (from 001_init / 017_consolidated) only allowed
-- 'github','go','hashicorp','grafana', but the app registers 7 crawlers
-- (also npm, pypi, crates) and will add more (e.g. 'rulesrc'). An npm/pypi/crates
-- project INSERT silently failed the CHECK. Source-type validation now belongs
-- to the Fetcher registry at runtime, not the schema.
--
-- SQLite cannot ALTER a CHECK in place, so rebuild the table (standard pattern).
-- This matches the FINAL projects schema (with the `enabled` column added in 004).

CREATE TABLE projects_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    source_type TEXT NOT NULL,
    source_url TEXT NOT NULL,
    config JSON NOT NULL DEFAULT '{}',
    latest_version TEXT DEFAULT '',
    last_crawled_at DATETIME,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO projects_new (id, name, display_name, source_type, source_url, config,
                          latest_version, last_crawled_at, enabled, created_at)
SELECT id, name, display_name, source_type, source_url, config,
       latest_version, last_crawled_at,
       CASE WHEN enabled IS NULL THEN 1 ELSE enabled END,
       created_at
FROM projects;

DROP TABLE projects;

ALTER TABLE projects_new RENAME TO projects;
