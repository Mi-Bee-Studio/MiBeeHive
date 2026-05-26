-- 019_storage_paths.sql
-- Storage migration tracking table for moving files between storage paths.

CREATE TABLE IF NOT EXISTS storage_migrations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    module TEXT NOT NULL,
    old_path TEXT NOT NULL,
    new_path TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','completed','failed','cancelled')),
    progress INTEGER DEFAULT 0,
    total_files INTEGER DEFAULT 0,
    migrated_files INTEGER DEFAULT 0,
    total_bytes INTEGER DEFAULT 0,
    migrated_bytes INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_storage_migrations_status ON storage_migrations(status);
