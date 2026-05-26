-- 014_iso_checksum.sql
-- Add optional SHA256 checksum column for ISO download verification.

ALTER TABLE iso_catalog ADD COLUMN sha256 TEXT DEFAULT '';
