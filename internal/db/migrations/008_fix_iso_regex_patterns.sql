-- 008_fix_iso_regex_patterns.sql
-- Fix regex patterns to anchor at end-of-string, preventing matches on .iso.manifest, .iso.zsync, .iso.sha512, etc.

UPDATE iso_catalog SET filename_pattern = filename_pattern || '$' WHERE filename_pattern NOT LIKE '%$';
