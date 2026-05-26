-- Add enabled column to projects table for soft-delete
ALTER TABLE projects ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT 1;

-- Source credentials table for API tokens
CREATE TABLE IF NOT EXISTS source_credentials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,  -- 'github', 'hashicorp', 'grafana', 'go'
    token TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_type)
);
