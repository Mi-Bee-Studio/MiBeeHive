-- 020_fix_iso_catalog.sql
-- Fix Alpine seed data: point to stable branch pattern
-- Reset transient download errors so queue will retry them

-- Fix Alpine entries: use direct stable branch URL instead of version directory listing
-- Only update entries that haven't been successfully used (no current_url or error status)
UPDATE iso_catalog
SET base_url = 'https://dl-cdn.alpinelinux.org/alpine/',
    version_dir_pattern = 'v\d+\.\d+',
    iso_path_template = '{version}/releases/{arch}/',
    status = 'available',
    last_error = '',
    download_status = ''
WHERE distro = 'alpine'
  AND (status = 'error' OR status = 'available')
  AND last_error LIKE '%404%';

-- Reset transient download errors so queue will retry them
UPDATE iso_catalog
SET download_status = '',
    status = 'available',
    last_error = ''
WHERE download_status = 'error'
  AND (last_error LIKE '%TLS handshake%'
       OR last_error LIKE '%deadline exceeded%'
       OR last_error LIKE '%timeout%'
       OR last_error LIKE '%HTTP 403%');
