-- 024: Performance indexes for file center list queries (#57).
-- The default sort ORDER BY created_at DESC on 2729+ rows was forcing a
-- full table scan + sort on every request (~900ms p50 on ARM64 NAS).
-- These composite indexes let SQLite serve the most common queries
-- (status=complete + created_at sort, project_id + created_at sort)
-- directly from the index without a filesort.

CREATE INDEX IF NOT EXISTS idx_files_status_created ON files(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_files_project_created ON files(project_id, created_at DESC);
