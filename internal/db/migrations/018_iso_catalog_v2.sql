-- 018_iso_catalog_v2.sql
-- Add two-level scraping configuration columns to iso_catalog.
-- Replace version-specific seed entries with distro-level entries.

-- Add new columns for two-level directory scraping
ALTER TABLE iso_catalog ADD COLUMN base_url TEXT NOT NULL DEFAULT '';
ALTER TABLE iso_catalog ADD COLUMN version_dir_pattern TEXT NOT NULL DEFAULT '';
ALTER TABLE iso_catalog ADD COLUMN iso_path_template TEXT NOT NULL DEFAULT '';

-- Delete old version-specific seed entries that haven't been user-modified
DELETE FROM iso_catalog WHERE current_url = '' AND auto_update = 0
  AND name IN (
    'Ubuntu Server 22.04 LTS (amd64)',
    'Ubuntu Server 24.04 LTS (amd64)',
    'Ubuntu Server 22.04 LTS (arm64)',
    'Ubuntu Server 24.04 LTS (arm64)',
    'Debian Netinst (amd64)',
    'Debian Netinst (arm64)',
    'Rocky Linux 9 Minimal (amd64)',
    'AlmaLinux 9 Minimal (amd64)',
    'CentOS Stream 9 (amd64)',
    'Alpine Standard x86_64',
    'Alpine Virt x86_64',
    'Alpine Standard aarch64',
    'Kali Linux (amd64)',
    'Arch Linux (x86_64)',
    'Fedora Server (amd64)',
    'openSUSE Leap 15 (amd64)'
  );

-- Insert new distro-level seed entries with two-level scraping configuration
INSERT INTO iso_catalog (name, distro, variant, arch, base_url, version_dir_pattern, iso_path_template, filename_pattern, check_url, auto_update, check_interval_hours, status) VALUES
('Ubuntu Server (amd64)', 'ubuntu', 'server', 'amd64', 'https://releases.ubuntu.com/', '\d{2}\.\d{2}', '{version}/', 'ubuntu-[\d.]+-live-server-amd64\.iso$', '', 0, 24, 'available'),
('Ubuntu Server (arm64)', 'ubuntu', 'server', 'arm64', 'https://cdimage.ubuntu.com/releases/', '\d{2}\.\d{2}', '{version}/release/', 'ubuntu-[\d.]+-live-server-arm64\.iso$', '', 0, 24, 'available'),
('Debian Netinst (amd64)', 'debian', 'netinst', 'amd64', 'https://cdimage.debian.org/debian-cd/', '', 'current/amd64/iso-cd/', 'debian-[\d.]+-amd64-netinst\.iso$', '', 0, 24, 'available'),
('Debian Netinst (arm64)', 'debian', 'netinst', 'arm64', 'https://cdimage.debian.org/debian-cd/', '', 'current/arm64/iso-cd/', 'debian-[\d.]+-arm64-netinst\.iso$', '', 0, 24, 'available'),
('Rocky Minimal (amd64)', 'rocky', 'minimal', 'amd64', 'https://download.rockylinux.org/pub/rocky/', '\d+', '{version}/isos/x86_64/', 'Rocky-[\d.]+-x86_64-minimal\.iso$', '', 0, 24, 'available'),
('Alpine Standard (amd64)', 'alpine', 'standard', 'amd64', 'https://dl-cdn.alpinelinux.org/alpine/', 'v\d+\.\d+', '{version}/releases/x86_64/', 'alpine-standard-[\d.]+-x86_64\.iso$', '', 0, 24, 'available');
