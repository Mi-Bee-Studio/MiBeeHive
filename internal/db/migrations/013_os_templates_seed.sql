-- 013_os_templates_seed.sql
-- Seed default OS installation configurations for additional Linux distributions.
-- Uses WHERE NOT EXISTS for idempotency — safe to re-run without duplicates.

-- Rocky Linux 9 Server (kickstart)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'Rocky Linux 9 Server', 'rocky-9-default', 'rocky',
       '{"hostname":"rocky-9-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'rocky-9-default');

-- AlmaLinux 9 Server (kickstart)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'AlmaLinux 9 Server', 'alma-9-default', 'alma',
       '{"hostname":"alma-9-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'alma-9-default');

-- Fedora Server 41 (kickstart)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'Fedora Server 41', 'fedora-41-default', 'fedora',
       '{"hostname":"fedora-41-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'fedora-41-default');

-- openSUSE Leap 15.6 (AutoYAST)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'openSUSE Leap 15.6', 'opensuse-leap-156-default', 'opensuse',
       '{"hostname":"opensuse-leap-156-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'opensuse-leap-156-default');
