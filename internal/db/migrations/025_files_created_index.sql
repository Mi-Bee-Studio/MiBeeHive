-- 025: Standalone created_at index for unfiltered ORDER BY (#57).
-- Migration 024 added (status, created_at) and (project_id, created_at)
-- composites, but the file center default view has NO WHERE clause —
-- it sorts ALL rows by created_at DESC. Without a standalone index,
-- SQLite does a full scan + temp B-tree sort on 2730+ rows.

CREATE INDEX IF NOT EXISTS idx_files_created ON files(created_at DESC);
