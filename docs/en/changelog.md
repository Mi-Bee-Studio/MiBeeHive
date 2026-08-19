# MiBeeHive Changelog

Notable changes are recorded here in the spirit of [Keep a Changelog](https://keepachangelog.com/en/1.1.0/). Releases are cut from `v*` Git tags.

## [Unreleased]

### Added

- **Supply layer**: generic artifact repository — `GET /repo/index` (JSON manifest) and `GET /repo/files/{id}` (direct download), public and unauthenticated (#3).
- **APT repository**: `Packages[.gz]` and `Release` generated on demand over collected `.deb` files; the supply page ships a ready-to-paste client apt source snippet (#9, #10).
- **PyPI Simple repository**: `/simple/` endpoint implementing PEP 503, with name normalization and `#sha256=` fragments on file links (#24).
- **Two-track foraging engine**: `Source`/`Fetcher` abstraction plus embedded YAML fingerprints (declarative onboarding for single-page sources) while stateful protocols stay as Go adapters; the Registry is wired into CrawlManager and registers supply endpoints; the `source_type` constraint widened (#2, #5, #6). Fingerprints can subsequently be loaded from the database.
- API tokens are injected into fingerprint requests automatically; the Grafana source migrated to the fingerprint model with single-object JSON support (#7, #8).
- **UX / WebDAV redesign**: virtual index (channels/views/nodes), cross-project file center, supply-chain overview home, and grouped navigation (#53).
- Virtual-index change audit logging (#58).
- **Configurable storage paths**: per-module `storage.modules` overrides with background migration tasks and a settings page showing task progress.
- ISO catalog v2: two-level scraping (distro → version → files), version-aware sorting, distro profiles; frontend template selector.
- Share links: create/list/revoke plus public `/s/{token}` token downloads.
- Cache metrics endpoint `GET /api/v1/admin/metrics/cache`.
- Crawl resilience: per-source fetch timeout, exponential-backoff retry on transient errors, and classified `network_error`/`rate_limited`/`error` statuses.
- Provisioning module enhancements: download-to-local button for ISOs, search, badges, and a mobile-responsive layout.

### Changed

- Docs and README repositioned the product as a lightweight **multi-arch Linux** supply hub (amd64/arm64) — no longer framed as ARM64-NAS-only.
- Security hardening: physical paths no longer leak through API responses or the frontend (#59).
- Frontend architecture cleanup: Preact providers enabled, scoped timers, route cancellation; shared moduleTabs component extracted (batches A–D).

### Fixed

- APT: xz-compressed control.tar supported (the modern .deb default) (#12); zstd-compressed debs, Release signatures cached by mtime, per-file metadata memoization.
- WebDAV: connection URLs generated from the request Host (no more hardcoded localhost); idempotent seeding of the manual-uploads project and directory; VirtualFS base storage path corrected.
- Config: fall back to the default password hash when `password_hash` is empty (#15); `password_changed_at` no longer defaults to startup time (#25).
- Overview page no longer fetches the 3MB `/repo/index`; login redirects immediately; field-mapping fixes.
- i18n: unified status labels, double-escaping fixed, toast feedback restored; stale positioning copy replaced.
- ISO: scraper retry/fallback, Alpine 404, transient-error reset; auto-download uses `resp.FoundURL`.
- Database: migration 024 registration fixed; restored 018_iso_catalog_v2 and renumbered storage_paths to 019.
- Handler test deadlock fixed (graceful event-bus close before db.Close()).

### Performance

- Composite indexes plus a repository-layer index cache (#57).
- Unfiltered `COUNT(*)` cached with a 30s TTL (#57).
- Standalone `created_at` index for the unfiltered sort (#57).

## [v0.1.0] — 2026-05-26

Initial public release.

- The four-module skeleton: Foraging (GitHub / Go / HashiCorp / Grafana sources, download queue, project and token management), Provisioning (OS install configs, preseed/kickstart/autoinstall templates, PXE endpoints, ISO catalog and download queue), Sharing (WebDAV + Basic Auth + self-signed HTTPS), Containers (local Docker lifecycle, images, app templates).
- Aggregated dashboard (single summary API), system status (CPU/memory/disk history), log and task centers, full-text search, backup and restore.
- Auth: JWT login/refresh, default-password detection and change.
- A single static Go binary (pure-Go SQLite driver, `CGO_ENABLED=0`) with an embedded Preact + HTM admin UI; embedded SQL migrations.
