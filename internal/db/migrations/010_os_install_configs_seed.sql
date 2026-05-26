-- 010_os_install_configs_seed.sql
-- Seed default OS installation configurations for mainstream Linux distributions.
-- Uses WHERE NOT EXISTS for idempotency — safe to re-run without duplicates.

-- Ubuntu 22.04 LTS Server (autoinstall)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'Ubuntu 22.04 LTS Server', 'ubuntu-2204-default', 'ubuntu',
       '{"hostname":"ubuntu-2204-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'ubuntu-2204-default');

-- Ubuntu 24.04 LTS Server (autoinstall)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'Ubuntu 24.04 LTS Server', 'ubuntu-2404-default', 'ubuntu',
       '{"hostname":"ubuntu-2404-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'ubuntu-2404-default');

-- Debian 12 Server (preseed)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'Debian 12 Server', 'debian-12-default', 'debian',
       '{"hostname":"debian-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'debian-12-default');

-- CentOS Stream 9 Server (kickstart)
INSERT INTO os_install_configs (name, config_name, os_type, config, enabled, created_at, updated_at)
SELECT 'CentOS Stream 9 Server', 'centos-stream9-default', 'centos',
       '{"hostname":"centos-server","timezone":"Asia/Shanghai","language":"en_US","keyboard_layout":"us","disk":"/dev/sda","partition_scheme":"whole_disk"}',
       1, datetime('now'), datetime('now')
WHERE NOT EXISTS (SELECT 1 FROM os_install_configs WHERE config_name = 'centos-stream9-default');
