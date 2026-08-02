-- 023_virtual_index.sql
-- Add virtual index tables for the supply layer's virtual directory trees,
-- plus metadata columns on files.
--
-- Issue #34: the supply layer needs to expose collected files through logical
-- directory trees (channels → views → nodes) rather than only the raw
-- project/filename layout. These tables materialize that tree so a single
-- lookup by full_path is fast, and rule folders can be populated
-- event-driven without scanning the whole files table on every request.
--
-- Foreign keys are handled by migrate.go (OFF during migration, ON after), so
-- no PRAGMA is emitted here.

-- channels: distribution channels (public, internal, token)
CREATE TABLE IF NOT EXISTS channels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    auth_mode TEXT NOT NULL DEFAULT 'anonymous',
    description TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- virtual_views: logical directory trees within a channel
CREATE TABLE IF NOT EXISTS virtual_views (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    channel_id INTEGER NOT NULL REFERENCES channels(id),
    mode TEXT NOT NULL DEFAULT 'curated' CHECK(mode IN ('rule','curated','hybrid')),
    writable BOOLEAN NOT NULL DEFAULT 0,
    sort_order INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- virtual_nodes: nodes in the virtual tree (folders, file references, rule folders)
CREATE TABLE IF NOT EXISTS virtual_nodes (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    view_id INTEGER NOT NULL REFERENCES virtual_views(id),
    parent_id INTEGER REFERENCES virtual_nodes(id),
    name TEXT NOT NULL,
    node_type TEXT NOT NULL DEFAULT 'folder' CHECK(node_type IN ('folder','file_ref','rule_folder')),
    file_id INTEGER REFERENCES files(id),
    rule_config TEXT,
    sort_order INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'visible' CHECK(status IN ('visible','hidden')),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- share_links: time-limited or count-limited share tokens
CREATE TABLE IF NOT EXISTS share_links (
    token TEXT PRIMARY KEY,
    file_id INTEGER NOT NULL REFERENCES files(id),
    expires_at DATETIME,
    max_downloads INTEGER,
    download_count INTEGER NOT NULL DEFAULT 0,
    note TEXT DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Materialized full-path index (hot path: single lookup by full_path)
CREATE TABLE IF NOT EXISTS virtual_node_paths (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    view_id INTEGER NOT NULL,
    node_id INTEGER NOT NULL REFERENCES virtual_nodes(id),
    full_path TEXT NOT NULL UNIQUE,
    UNIQUE(view_id, node_id)
);

-- Materialized rule-folder entries (event-driven)
CREATE TABLE IF NOT EXISTS virtual_rule_entries (
    rule_node_id INTEGER NOT NULL REFERENCES virtual_nodes(id),
    file_id INTEGER NOT NULL REFERENCES files(id),
    PRIMARY KEY (rule_node_id, file_id)
);

-- Extend files with supply-layer metadata. DEFAULT NULL keeps existing rows
-- valid; execMigration silently skips "duplicate column" errors, so these
-- ALTERs are idempotent.
ALTER TABLE files ADD COLUMN source_type TEXT DEFAULT NULL;
ALTER TABLE files ADD COLUMN category TEXT DEFAULT NULL;
ALTER TABLE files ADD COLUMN storage_subdir TEXT DEFAULT NULL;
ALTER TABLE files ADD COLUMN public_token TEXT DEFAULT NULL;

CREATE INDEX IF NOT EXISTS idx_virtual_nodes_view_parent ON virtual_nodes(view_id, parent_id);
CREATE INDEX IF NOT EXISTS idx_virtual_nodes_view ON virtual_nodes(view_id);
CREATE INDEX IF NOT EXISTS idx_share_links_file ON share_links(file_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_files_public_token ON files(public_token) WHERE public_token IS NOT NULL;