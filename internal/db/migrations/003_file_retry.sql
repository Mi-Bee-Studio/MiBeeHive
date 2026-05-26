-- Add retry tracking columns to files table
-- SQLite cannot alter CHECK constraints, so we recreate the table

CREATE TABLE IF NOT EXISTS files_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    version TEXT NOT NULL,
    filename TEXT NOT NULL,
    os TEXT DEFAULT '',
    arch TEXT DEFAULT '',
    ext TEXT DEFAULT '',
    size_bytes INTEGER DEFAULT 0,
    download_url TEXT NOT NULL,
    local_path TEXT NOT NULL,
    checksum TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','downloading','complete','error','imported','failed_permanent')),
    error_message TEXT DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO files_new (id, project_id, version, filename, os, arch, ext,
    size_bytes, download_url, local_path, checksum, status, error_message, created_at,
    retry_count, last_attempt_at)
SELECT id, project_id, version, filename, os, arch, ext,
    size_bytes, download_url, local_path, checksum, status, error_message, COALESCE(created_at, CURRENT_TIMESTAMP),
    0, NULL
FROM files;

DROP TABLE files;

ALTER TABLE files_new RENAME TO files;

CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);
CREATE INDEX IF NOT EXISTS idx_files_filename ON files(filename);
