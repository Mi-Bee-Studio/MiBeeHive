-- 007_iso_catalog_seed.sql
-- Seed data for popular Linux distributions in the iso_catalog table.
-- All entries have auto_update OFF by default; users must opt in.
-- check_url points to a directory listing page where the scraper can find .iso links.
-- filename_pattern is a Go regex that matches the latest ISO filename in that listing.

INSERT INTO iso_catalog (name, distro, variant, arch, check_url, filename_pattern, auto_update, check_interval_hours, status) VALUES
-- Ubuntu Server (amd64)
('Ubuntu Server 22.04 LTS (amd64)', 'ubuntu', 'server', 'amd64', 'https://releases.ubuntu.com/22.04/', 'ubuntu-22\.04\.\d+-live-server-amd64\.iso$', 0, 24, 'available'),
('Ubuntu Server 24.04 LTS (amd64)', 'ubuntu', 'server', 'amd64', 'https://releases.ubuntu.com/24.04/', 'ubuntu-24\.04\.\d+-live-server-amd64\.iso$', 0, 24, 'available'),

-- Ubuntu Server (arm64)
('Ubuntu Server 22.04 LTS (arm64)', 'ubuntu', 'server', 'arm64', 'https://cdimage.ubuntu.com/releases/22.04/release/', 'ubuntu-22\.04\.\d+-live-server-arm64\.iso$', 0, 24, 'available'),
('Ubuntu Server 24.04 LTS (arm64)', 'ubuntu', 'server', 'arm64', 'https://cdimage.ubuntu.com/releases/24.04/release/', 'ubuntu-24\.04\.\d+-live-server-arm64\.iso$', 0, 24, 'available'),

-- Debian Netinst (amd64/arm64) -- current/ symlink tracks latest stable release
('Debian Netinst (amd64)', 'debian', 'netinst', 'amd64', 'https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/', 'debian-\d+\.\d+\.\d+-amd64-netinst\.iso$', 0, 24, 'available'),
('Debian Netinst (arm64)', 'debian', 'netinst', 'arm64', 'https://cdimage.debian.org/debian-cd/current/arm64/iso-cd/', 'debian-\d+\.\d+\.\d+-arm64-netinst\.iso$', 0, 24, 'available'),

-- RHEL family
('Rocky Linux 9 Minimal (amd64)', 'rocky', 'minimal', 'amd64', 'https://download.rockylinux.org/pub/rocky/9/isos/x86_64/', 'Rocky-9\.[\d]+-x86_64-minimal\.iso$', 0, 24, 'available'),
('AlmaLinux 9 Minimal (amd64)', 'almalinux', 'minimal', 'amd64', 'https://repo.almalinux.org/almalinux/9/isos/x86_64/', 'AlmaLinux-9\.[\d]+-x86_64-minimal\.iso$', 0, 24, 'available'),
('CentOS Stream 9 (amd64)', 'centos', 'boot', 'amd64', 'https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/iso/', 'CentOS-Stream-9-[\d.]+-x86_64-boot\.iso$', 0, 24, 'available'),

-- Alpine Linux
('Alpine Standard x86_64', 'alpine', 'standard', 'amd64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/', 'alpine-standard-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'),
('Alpine Virt x86_64', 'alpine', 'virt', 'amd64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/x86_64/', 'alpine-virt-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'),
('Alpine Standard aarch64', 'alpine', 'standard', 'arm64', 'https://dl-cdn.alpinelinux.org/alpine/v3.21/releases/aarch64/', 'alpine-standard-\d+\.\d+\.\d+-aarch64\.iso$', 0, 24, 'available'),

-- Other popular distros
('Kali Linux (amd64)', 'kali', 'installer', 'amd64', 'https://cdimage.kali.org/current/', 'kali-linux-\d+\.\d+-installer-amd64\.iso$', 0, 24, 'available'),
('Arch Linux (x86_64)', 'arch', 'latest', 'amd64', 'https://geo.mirror.pkgbuild.com/iso/latest/', 'archlinux-\d+\.\d+\.\d+-x86_64\.iso$', 0, 24, 'available'),
('Fedora Server (amd64)', 'fedora', 'server', 'amd64', 'https://mirrors.kernel.org/fedora/releases/42/Server/x86_64/iso/', 'Fedora-Server-netinst-x86_64-[\d.]+-[\d.]+\.iso$', 0, 24, 'available'),
('openSUSE Leap 15 (amd64)', 'opensuse', 'net', 'amd64', 'https://download.opensuse.org/distribution/leap/15.6/iso/', 'openSUSE-Leap-[\d.]+-NET-x86_64-Current\.iso$', 0, 24, 'available');
