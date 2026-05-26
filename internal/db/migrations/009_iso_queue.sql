-- 009_iso_queue.sql
-- Add download_status column for queue tracking
ALTER TABLE iso_catalog ADD COLUMN download_status TEXT NOT NULL DEFAULT '';
