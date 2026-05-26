-- 016_add_indexes.sql
-- Add indexes for common query patterns.

CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);
