-- OS Install config enhancements: add os_type, config_name, enabled, updated_at
ALTER TABLE os_install_configs ADD COLUMN os_type TEXT;
ALTER TABLE os_install_configs ADD COLUMN config_name TEXT;
ALTER TABLE os_install_configs ADD COLUMN enabled BOOLEAN DEFAULT 1;
ALTER TABLE os_install_configs ADD COLUMN updated_at DATETIME;
