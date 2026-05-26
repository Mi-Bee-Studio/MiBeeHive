-- 017_consolidated.sql
-- Consolidated schema: idempotent recreation of all tables, indexes, and seed data
-- from migrations 001-016. This is a no-op on databases with 001-016 already applied.
-- For fresh databases, 001-016 create the schema first; 017 merely confirms it.

-- ============================================================
-- Tables (final state after migrations 001-016)
-- ============================================================

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    display_name TEXT NOT NULL,
    source_type TEXT NOT NULL CHECK(source_type IN ('github','go','hashicorp','grafana')),
    source_url TEXT NOT NULL,
    config JSON NOT NULL DEFAULT '{}',
    latest_version TEXT DEFAULT '',
    last_crawled_at DATETIME,
    enabled BOOLEAN NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS files (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    version TEXT NOT NULL,
    filename TEXT NOT NULL,
    os TEXT DEFAULT '',
    arch TEXT DEFAULT '',
    ext TEXT DEFAULT '',
    size_bytes INTEGER DEFAULT 0,
    download_url TEXT NOT NULL,
    local_path TEXT NOT NULL,
    checksum TEXT DEFAULT '',
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','downloading','complete','error','imported','failed_permanent')),
    error_message TEXT DEFAULT '',
    retry_count INTEGER NOT NULL DEFAULT 0,
    last_attempt_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS crawl_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id INTEGER NOT NULL REFERENCES projects(id),
    started_at DATETIME NOT NULL,
    finished_at DATETIME,
    status TEXT NOT NULL CHECK(status IN ('running','success','error','rate_limited')),
    versions_found INTEGER DEFAULT 0,
    files_downloaded INTEGER DEFAULT 0,
    error_message TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS os_install_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    config JSON NOT NULL DEFAULT '{}',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    os_type TEXT,
    config_name TEXT,
    enabled BOOLEAN DEFAULT 1,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS system_stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    sampled_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    cpu_usage_percent REAL NOT NULL,
    memory_total_bytes INTEGER NOT NULL,
    memory_used_bytes INTEGER NOT NULL,
    memory_usage_percent REAL NOT NULL,
    network_rx_bytes INTEGER NOT NULL,
    network_tx_bytes INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS source_credentials (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    token TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_type)
);

CREATE TABLE IF NOT EXISTS iso_catalog (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    distro TEXT NOT NULL,
    variant TEXT NOT NULL DEFAULT '',
    arch TEXT NOT NULL DEFAULT 'amd64',
    check_url TEXT NOT NULL,
    filename_pattern TEXT NOT NULL,
    current_url TEXT DEFAULT '',
    auto_update INTEGER DEFAULT 0,
    check_interval_hours INTEGER DEFAULT 24,
    last_checked DATETIME,
    last_error TEXT DEFAULT '',
    status TEXT DEFAULT 'available',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    download_status TEXT NOT NULL DEFAULT '',
    sha256 TEXT DEFAULT ''
);

CREATE TABLE IF NOT EXISTS container_apps (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    image TEXT NOT NULL,
    command TEXT DEFAULT '',
    env TEXT DEFAULT '{}',
    ports TEXT DEFAULT '[]',
    volumes TEXT DEFAULT '[]',
    networks TEXT DEFAULT '[]',
    restart_policy TEXT DEFAULT 'unless-stopped',
    memory_limit TEXT DEFAULT '',
    cpu_limit REAL DEFAULT 0,
    status TEXT DEFAULT 'stopped',
    container_id TEXT DEFAULT '',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS app_templates (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT UNIQUE NOT NULL,
    description TEXT DEFAULT '',
    image TEXT NOT NULL,
    command TEXT DEFAULT '',
    env TEXT DEFAULT '{}',
    ports TEXT DEFAULT '[]',
    volumes TEXT DEFAULT '[]',
    networks TEXT DEFAULT '[]',
    restart_policy TEXT DEFAULT 'unless-stopped',
    category TEXT DEFAULT 'general',
    icon TEXT DEFAULT '',
    enabled INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS registries (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    url TEXT NOT NULL,
    type TEXT NOT NULL CHECK(type IN ('dockerhub','ghcr','acr','tcr','quay')),
    username TEXT NOT NULL DEFAULT '',
    encrypted_password TEXT NOT NULL DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS registry_repos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    registry_id INTEGER NOT NULL REFERENCES registries(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    tag_count INTEGER NOT NULL DEFAULT 0,
    total_size INTEGER NOT NULL DEFAULT 0,
    last_synced DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(registry_id, name)
);

CREATE TABLE IF NOT EXISTS sync_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_registry_id INTEGER NOT NULL REFERENCES registries(id),
    target_registry_id INTEGER NOT NULL REFERENCES registries(id),
    source_repo TEXT NOT NULL,
    source_tag TEXT NOT NULL,
    target_repo TEXT NOT NULL,
    target_tag TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','completed','failed')),
    progress_bytes INTEGER NOT NULL DEFAULT 0,
    total_bytes INTEGER NOT NULL DEFAULT 0,
    error_message TEXT NOT NULL DEFAULT '',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS retention_policies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    registry_id INTEGER NOT NULL REFERENCES registries(id) ON DELETE CASCADE,
    repo_pattern TEXT NOT NULL DEFAULT '*',
    keep_days INTEGER DEFAULT 0,
    keep_count INTEGER DEFAULT 0,
    keep_pattern TEXT DEFAULT '',
    enabled INTEGER NOT NULL DEFAULT 1,
    last_executed_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT chk_retention CHECK(keep_days >= 0 AND keep_count >= 0)
);

-- ============================================================
-- Indexes (deduplicated — idx_files_project_id appears once)
-- ============================================================

CREATE INDEX IF NOT EXISTS idx_files_project_id ON files(project_id);
CREATE INDEX IF NOT EXISTS idx_files_filename ON files(filename);
CREATE INDEX IF NOT EXISTS idx_files_status ON files(status);
CREATE INDEX IF NOT EXISTS idx_crawl_logs_project_id ON crawl_logs(project_id);
CREATE INDEX IF NOT EXISTS idx_system_stats_sampled ON system_stats(sampled_at);
CREATE INDEX IF NOT EXISTS idx_registry_repos_registry ON registry_repos(registry_id, name);
CREATE INDEX IF NOT EXISTS idx_sync_tasks_status ON sync_tasks(status);
CREATE INDEX IF NOT EXISTS idx_retention_policies_registry ON retention_policies(registry_id);

-- ============================================================
-- Seed data: ISO catalog (16 entries, from 007 + 008 regex fix)
-- ============================================================

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Ubuntu Server 22.04 LTS (amd64)', 'ubuntu', 'server', 'amd64', 'https://releases.ubuntu.com/22.04/', 'ubuntu-22\.04\.\d+-live-server-amd64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Ubuntu Server 22.04 LTS (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Ubuntu Server 24.04 LTS (amd64)', 'ubuntu', 'server', 'amd64', 'https://releases.ubuntu.com/24.04/', 'ubuntu-24\.04\.\d+-live-server-amd64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Ubuntu Server 24.04 LTS (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Ubuntu Server 22.04 LTS (arm64)', 'ubuntu', 'server', 'arm64', 'https://cdimage.ubuntu.com/releases/22.04/release/', 'ubuntu-22\.04\.\d+-live-server-arm64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Ubuntu Server 22.04 LTS (arm64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Ubuntu Server 24.04 LTS (arm64)', 'ubuntu', 'server', 'arm64', 'https://cdimage.ubuntu.com/releases/24.04/release/', 'ubuntu-24\.04\.\d+-live-server-arm64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Ubuntu Server 24.04 LTS (arm64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Debian Netinst (amd64)', 'debian', 'netinst', 'amd64', 'https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/', 'debian-\d+\.\d+\.\d+-amd64-netinst\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Debian Netinst (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Debian Netinst (arm64)', 'debian', 'netinst', 'arm64', 'https://cdimage.debian.org/debian-cd/current/arm64/iso-cd/', 'debian-\d+\.\d+\.\d+-arm64-netinst\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Debian Netinst (arm64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Rocky Linux 9 Minimal (amd64)', 'rocky', 'minimal', 'amd64', 'https://download.rockylinux.org/pub/rocky/9/isos/x86_64/', 'Rocky-9\.[\d]+-x86_64-minimal\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Rocky Linux 9 Minimal (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'AlmaLinux 9 Minimal (amd64)', 'almalinux', 'minimal', 'amd64', 'https://repo.almalinux.org/almalinux/9/isos/x86_64/', 'AlmaLinux-9\.[\d]+-x86_64-minimal\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'AlmaLinux 9 Minimal (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'CentOS Stream 9 (amd64)', 'centos', 'boot', 'amd64', 'https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/iso/', 'CentOS-Stream-9-[\d.]+-x86_64-boot\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'CentOS Stream 9 (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Alpine Standard x86_64', 'alpine', 'standard', 'amd64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/', 'alpine-standard-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Alpine Standard x86_64');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Alpine Virt x86_64', 'alpine', 'virt', 'amd64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/', 'alpine-virt-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Alpine Virt x86_64');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Alpine Standard aarch64', 'alpine', 'standard', 'arm64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/aarch64/', 'alpine-standard-\d+\.\d+\.\d+-aarch64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Alpine Standard aarch64');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Kali Linux (amd64)', 'kali', 'installer', 'amd64', 'https://cdimage.kali.org/current/', 'kali-linux-\d+\.\d+-installer-amd64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Kali Linux (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Arch Linux (x86_64)', 'arch', 'latest', 'amd64', 'https://geo.mirror.pkgbuild.com/iso/latest/', 'archlinux-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Arch Linux (x86_64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'Fedora Server (amd64)', 'fedora', 'server', 'amd64', 'https://mirrors.kernel.org/fedora/releases/42/Server/x86_64/iso/', 'Fedora-Server-netinst-x86_64-[\d.]+-[\d.]+\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'Fedora Server (amd64)');

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status)
SELECT 'openSUSE Leap 15 (amd64)', 'opensuse', 'net', 'amd64', 'https://download.opensuse.org/distribution/leap/15.6/iso/', 'openSUSE-Leap-[\d.]+-NET-x86_64-Current\.iso$', 0, 24, 'available'
WHERE NOT EXISTS (SELECT 1 FROM iso_catalog WHERE name = 'openSUSE Leap 15 (amd64)');

-- ============================================================
-- Seed data: OS install configs (8 entries, from 010 + 013)
-- ============================================================

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

-- ============================================================
-- Seed data: App templates (3 entries, from 012)
-- ============================================================

INSERT INTO app_templates (name, description, image, ports, env, category)
SELECT 'nginx', 'Nginx Web Server', 'nginx:alpine', '[{"host_port":80,"container_port":80,"protocol":"tcp"}]', '{}', 'web'
WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'nginx');

INSERT INTO app_templates (name, description, image, ports, env, category)
SELECT 'redis', 'Redis Key-Value Store', 'redis:alpine', '[{"host_port":6379,"container_port":6379,"protocol":"tcp"}]', '{}', 'database'
WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'redis');

INSERT INTO app_templates (name, description, image, ports, env, category)
SELECT 'postgres', 'PostgreSQL Database', 'postgres:alpine', '[{"host_port":5432,"container_port":5432,"protocol":"tcp"}]', '{"POSTGRES_PASSWORD":"changeme"}', 'database'
WHERE NOT EXISTS (SELECT 1 FROM app_templates WHERE name = 'postgres');
