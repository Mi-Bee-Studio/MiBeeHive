-- 015_registry.sql
-- Container registry management and image sync tables.

CREATE TABLE IF NOT EXISTS registries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('dockerhub','ghcr','acr','tcr','quay')),
    username TEXT NOT NULL DEFAULT '',
    encrypted_password TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS registry_repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    registry_id INTEGER NOT NULL REFERENCES registries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    tag_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    last_synced DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(registry_id, name)
);

CREATE INDEX IF NOT EXISTS idx_registry_repos_registry ON registry_repos(registry_id, name);

CREATE TABLE IF NOT EXISTS sync_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_registry_id INTEGER NOT NULL REFERENCES registries(id),
    target_registry_id INTEGER NOT NULL REFERENCES registries(id),
    source_repo TEXT NOT NULL,
    source_tag TEXT NOT NULL,
    target_repo TEXT NOT NULL,
    target_tag TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','completed','failed')),
    progress_bytes INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_sync_tasks_status ON sync_tasks(status);

CREATE TABLE IF NOT EXISTS retention_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    registry_id INTEGER NOT NULL REFERENCES registries(id) ON DELETE CASCADE,
    repo_pattern TEXT NOT NULL DEFAULT '*',
    keep_days INTEGER DEFAULT 0,
    keep_count INTEGER DEFAULT 0,
    keep_pattern TEXT DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    last_executed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_retention CHECK(keep_days >= 0 AND keep_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_retention_policies_registry ON retention_policies(registry_id);
