-- 026: Virtual index audit log (#58).
-- Tracks who changed virtual_channels/views/nodes and what changed.
CREATE TABLE IF NOT EXISTS virtual_audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    admin_user   TEXT    NOT NULL,
    action       TEXT    NOT NULL,  -- create, update, delete
    entity_type  TEXT    NOT NULL,  -- channel, view, node
    entity_id    INTEGER NOT NULL,
    entity_name  TEXT    NOT NULL,
    diff_json    TEXT    NOT NULL DEFAULT '{}',
    created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON virtual_audit_log(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_created ON virtual_audit_log(created_at DESC);
