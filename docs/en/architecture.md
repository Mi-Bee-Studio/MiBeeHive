# MiBeeHive Architecture

[中文](../zh/architecture.md)


## BeeHive Philosophy

MiBeeHive abstracts file operations as a BeeHive — a small team file utility platform with three functional modules:

- **Foraging** (采蜜): Crawl and download binary releases from public sources for intranet use
- **Provisioning** (哺育): Provide unattended OS installation configuration via web UI with specific URLs
- **Sharing** (分享): Basic WebDAV capabilities for file sharing, configurable via web UI

Each module has isolated storage paths under a configurable parent: `{base_path}/{oss,os-install,webdav}`

### Phase Roadmap
- Phase 1 (Complete): Foraging — Web management for crawl sources, API tokens, crawl control, password change
- Phase 2 (Complete): Provisioning — OS install config management, PXE endpoints, ISO downloading
- Phase 3 (Complete): Sharing — WebDAV server, Basic Auth, HTTPS support

## System Architecture

MiBeeHive is a monolithic Go binary that crawls, downloads, and serves binary releases (GitHub, Go, HashiCorp, Grafana, NPM, PyPI) for a resource-constrained ARM64 NAS device (469MB RAM). It embeds a **Preact + HTM** SPA frontend via `go:embed` and includes a web admin panel with dashboard overview and tabbed navigation for managing all three modules plus containers, search, logs, tasks, and backup.

### Architecture Overview
```
┌─────────────────────────────────────────────────────────────┐
│                    MiBeeHive Application                      │
├─────────────────────────────────────────────────────────────┤
│  Go Backend (cmd/mibeehive)                                │
│  ├── HTTP Handlers (internal/handler/)                     │
│  ├── Business Logic (internal/service/)                    │
│  ├── Data Layer (internal/db/)                             │
│  ├── Configuration (internal/config/)                       │
│  ├── Middleware (internal/middleware/)                     │
│  ├── Docker Client (internal/docker/)                      │
│  ├── Monitor (internal/monitor/)                           │
│  └── WebDAV (internal/webdav/)                             │
│                                                             │
│  Embedded Frontend (web/)                                   │
│  ├── HTML/CSS (CSS variables, responsive)                  │
│  └── JavaScript Modules (31 files, 3-tier)                 │
│                                                             │
│  SQLite Database                                            │
│  └── 14 Embedded Migrations                                 │
└─────────────────────────────────────────────────────────────┘
```

## Frontend Module Structure

The frontend is a **Preact + HTM** SPA organized into 38 modules across 3 tiers. See `web/js/AGENTS.md` for detailed documentation.

### Core Layer (web/js/core/)
- `api.js` - HTTP client wrapper (fetch + JWT header injection)
- `auth.js` - Login/logout, token management, localStorage
- `components.js` - Reusable UI components (toast, modal, table, tabs, skeletonCard)
- `drawer.js` - Slide-out drawer panel
- `helpers.js` - Utility functions (formatDate, formatSize, debounce, etc.)
- `router.js` - Hash-based SPA routing with route guards
- `search.js` - Global search functionality
- `state.js` - Global App singleton with event bus and timer management
- `preact.js` - **Preact bridge** providing h, html, render, Component, Fragment + all hooks
- `i18n.js` - i18n system (zh/en) with `t('key')` function and `{count}` interpolation

### Layout Layer (web/js/layout/)
- `sidebar.js` - Desktop sidebar navigation (with hexagon brand icon)
- `shell.js` - App shell (renders sidebar + main content area)
- `bottom-tab.js` - Mobile bottom tab navigation

### Module Layer (web/js/modules/)
- `dashboard.js` - Aggregated dashboard with welcome banner, status cards, charts, activity timeline, quick actions (727 lines)
- `files.js` - Files tab container (sub-module router)
- `files-crawl.js` - Crawl control sub-module
- `files-projects.js` - Project management sub-module
- `files-queue.js` - Download queue sub-module
- `deploy.js` - Deploy tab container
- `deploy-configs.js` - OS install config management
- `deploy-iso.js` - ISO catalog + download management
- `share.js` - Share tab container (WebDAV)
- `share-files.js` - WebDAV file browser
- `settings.js` - Settings (password, theme, language, disk thresholds)
- `login.js` - Login page
- `containers.js` - Container list and management
- `containers-detail.js` - Container detail view (logs, stats, env vars)
- `containers-images.js` - Docker image management
- `containers-templates.js` - Application templates
- `logs.js` - System logs viewer
- `search.js` - Search results page
- `tasks.js` - Background tasks viewer

## Backend Architecture

### Layer Structure
```
HTTP Request → Handler → Service → Repository → Database
```

### Handler Layer (internal/handler/)
- `auth.go` - Authentication endpoints (login, JWT validation)
- `admin.go` - Admin panel endpoints (projects, tokens, crawl, security, os-install, webdav, monitor config)
- `backup.go` - Backup list and restore
- `container.go` - Container CRUD, start/stop/restart, logs, stats
- `crawl.go` - Crawl management (status, trigger, logs)
- `dashboard.go` - Aggregated dashboard summary (single API for all module stats)
- `file.go` - File operations (download, search, queue)
- `iso.go` - ISO download management, catalog CRUD, queue
- `logs.go` - System logs endpoint
- `os_install.go` - OS installation configuration, PXE serving, config preview
- `project.go` - Project CRUD operations
- `search.go` - Full-text search endpoint
- `system.go` - System information and statistics
- `tasks.go` - Background tasks endpoint
- `app_template.go` - Application template management
- `stats.go` - System stats fetching (scrapes node_exporter)

### Service Layer (internal/service/)
- `file_service.go` - File download with retry, integrity checks
- `os_template.go` - OS template generation (preseed/kickstart/autoinstall)
- `iso_downloader.go` - ISO download with streaming and disk checks
- `iso_catalog_service.go` - ISO catalog queue processor with background goroutine
- `container_service.go` - Docker container lifecycle management
- `search_service.go` - Full-text search across files and configs
- `log_service.go` - Log aggregation and querying
- `task_service.go` - Background task management
- `app_template_service.go` - Application template processing
- `image_service.go` - Docker image pull/delete operations

### Repository Layer (internal/db/repo_*.go)
- `repo_project.go` - Project data access
- `repo_credential.go` - API token management
- `repo_file.go` - File metadata and queue operations
- `repo_os_install_config.go` - OS installation configuration
- `repo_iso_catalog.go` - ISO catalog queue management
- `repo_container.go` - Container configuration storage
- `repo_crawl_log.go` - Crawl log storage and querying

## Three Modules Overview

### 1. Foraging (Binary Release Management)
**Purpose**: Crawl and download binary releases from public sources
**Storage**: `{base_path}/oss/`
**Features**:
- GitHub releases
- Go binary downloads
- HashiCorp product releases
- Grafana releases
- NPM package downloads
- PyPI package downloads
- Web UI for source management
- API token authentication
- Download scheduling and retry logic

### 2. Provisioning (OS Installation)
**Purpose**: Provide unattended OS installation configuration
**Storage**: `{base_path}/os-install/`
**Features**:
- PXE configuration serving
- OS template generation (preseed/kickstart/autoinstall)
- ISO download management
- ISO catalog auto-discovery with queue management
- Web UI for configuration management
- Public endpoints for PXE clients
- Config preview functionality
- Background queue processor for ISO downloads

### 3. Sharing (WebDAV File Sharing)
**Purpose**: Basic WebDAV capabilities for file sharing
**Storage**: `{base_path}/webdav/`
**Features**:
- WebDAV file serving
- Basic Authentication (anonymous read + admin write)
- HTTPS support with self-signed certificates
- File listing and management
- Configurable via web UI

## Dashboard Architecture

The dashboard provides an aggregated overview of all modules through a single API endpoint.

### Backend
- **Endpoint**: `GET /api/v1/admin/dashboard/summary` (JWT required)
- **Handler**: `DashboardHandler.Summary()` in `handler/dashboard.go`
- **Response**: `DashboardSummaryResponse` containing:
  - `SystemModuleStats` — version, uptime, CPU, memory, disk usage
  - `FilesModuleStats` — project count, file count, queue stats
  - `DeployModuleStats` — config count, ISO count/downloaded/pending
  - `SharedModuleStats` — WebDAV file count and total size
  - `[]ActivityEvent` — recent crawl activity with project names

### Frontend
- Single API call on init, then polls every 30s with incremental DOM updates
- Separate 10s poll for real-time system stats (CPU/mem/network charts)
- Sections: welcome banner, status cards grid, merged CPU/Mem chart, disk gauge with threshold lines, activity timeline, quick actions bar, crawl activity chart, queue sections

### Monitor Config
- **Endpoints**: `GET/PUT /api/v1/admin/config/monitor` (JWT required)
- **Handler**: `AdminHandler.GetMonitorConfig()` / `UpdateMonitorConfig()`
- **Purpose**: Disk warning/critical threshold configuration (persisted in config.yaml)

## Data Flow

### Dashboard Flow
```
Dashboard UI → Single /admin/dashboard/summary → DashboardHandler → Multiple Repos → Aggregated Response
```

### Crawl and Download Flow
```
User Request → Admin UI → Crawl Trigger → Crawler → Download Service → File Storage
```

### File Access Flow
```
Client Request → File Search → File Service → Download Stream → Client
```

### WebDAV Flow
```
WebDAV Client → Basic Auth → File System → File Operations
```

### OS Installation Flow
```
PXE Client → Public Endpoint → Config Generation → Boot Files → Installation
```

## Key Design Principles

- **Monolithic Architecture**: Single Go binary for deployment simplicity
- **Embedded Frontend**: No separate web server required
- **SQLite Database**: Lightweight, file-based storage (pure-Go driver)
- **Preact + HTM**: No frameworks, minimal dependencies (~950KB total)
- **Stdlib Only**: No external web frameworks or cron libraries
- **Resource Efficient**: Optimized for 469MB ARM64 device
- **Modular Design**: Clear separation between the three functional modules
- **Queue Processing**: Background goroutines for download queue management
- **Incremental DOM Updates**: Periodic refresh uses targeted DOM patching, never innerHTML
- **Single Dashboard API**: One aggregated endpoint reduces request count on dashboard

[中文](../zh/architecture.md)