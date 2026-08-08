# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- **Supply layer — native-protocol endpoints** (public, no auth): APT repository (`/apt/`) over collected `.deb` files with on-demand `Packages`/`Release` generation (mtime-invalidated cache, per-file memoization); PyPI Simple (`/simple/`, PEP 503) over collected wheels/sdists; generic `/repo/index` + `/repo/files/{id}` for everything else. zstd-compressed `.deb` control archives now parse correctly.
- **Two-track crawl layer** (`internal/source/`): `Source`/`Fetcher`/`Registry` abstraction with embedded YAML fingerprints for single-page sources (GitHub, Grafana); classic stateful crawlers wrapped as Go adapters. Dead scheduler-registration code retired.
- **Crawler resilience**: transient network errors (timeout, connection reset, 5xx) now retried with bounded exponential backoff; per-crawl context timeout; classified crawl statuses `network_error` / `rate_limited` / `error` (migration 022) so transient failures are distinguishable from genuine upstream problems. Configurable via `crawler.fetch_timeout` / `max_retries` / `retry_initial_backoff`.
- Foraging module (binary release crawling from GitHub, Go, HashiCorp, Grafana, NPM, PyPI, Crates)
- Provisioning module (OS install config, PXE boot, ISO download management)
- Sharing module (WebDAV file sharing with Basic Authentication)
- Container management (Docker container CRUD, start/stop/restart, logs, stats)
- Dashboard with system monitoring (real-time CPU, memory, disk charts)
- Registry management with cross-registry sync functionality
- Backup and restore functionality
- Admin panel with tabbed navigation for all modules
- Global search across files and configurations
- Background task management and monitoring
- Multi-language support (Chinese/English)

### Security
- JWT authentication with bcrypt password hashing
- Rate limiting middleware
- Basic Authentication for WebDAV (anonymous read + admin write)
- Security headers middleware
- Input validation and sanitization
- TLS encryption support with self-signed certificates

### Changed
- Initial release - all features are new