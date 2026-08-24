-- 027_backfill_files_source_type.sql
-- Backfill files.source_type from the owning project.
--
-- Issue #63: the column added in 023 was never populated by the crawler (the
-- INSERT omitted it), so every collected file showed an empty "来源" badge in
-- the file center and source_type filtering matched nothing. New rows are
-- written correctly going forward; this migration repairs existing rows.

UPDATE files
SET source_type = (SELECT p.source_type FROM projects p WHERE p.id = files.project_id)
WHERE (source_type IS NULL OR source_type = '')
  AND project_id IS NOT NULL;
