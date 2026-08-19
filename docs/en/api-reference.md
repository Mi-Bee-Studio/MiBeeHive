# MiBeeHive API Reference

All HTTP endpoints. **Auth model**: apart from the **public endpoints** listed below, every `/api/v1/*` endpoint requires a JWT (`Authorization: Bearer <token>`):

- `POST /api/v1/auth/login` (the login itself)
- `GET /api/v1/files/{id|token}/download`, `GET /s/{token}` (token downloads)
- `GET /api/v1/isos` (public ISO listing)
- Supply layer: `/repo/*`, `/apt/*`, `/simple/*`
- `GET /pxe/{format}/{name}` (PXE clients can't authenticate)
- `GET /health`, `GET /metrics`
- `/webdav/*` (separate Basic Auth: anonymous read, admin write)

## Authentication Endpoints

### POST /api/v1/auth/login
**Description**: Authenticate and obtain a JWT
**Authentication**: None
**Request Body**:
```json
{
  "username": "admin",
  "password": "password"
}
```
**Response**:
```json
{
  "token": "jwt-token-here",
  "expires_in": 3600
}
```

### POST /api/v1/auth/refresh
**Description**: Renew a token before it expires; returns a fresh JWT
**Authentication**: JWT required

### GET /api/v1/auth/password-status
**Description**: Check whether the password must be changed (e.g. still default)
**Authentication**: JWT required
**Response**:
```json
{
  "success": true,
  "data": {
    "must_change": false
  }
}
```

## File Endpoints

### GET /api/v1/files/{id}/download
**Description**: Download a file. `{id}` may be either the numeric ID or a **share token** (base58) — the token form is the share-link download channel and needs no login
**Authentication**: None
**Parameters**:
- `id` (path): file ID or share token
**Response**: file download stream

### GET /api/v1/files/search
**Description**: Search files
**Authentication**: JWT required
**Query Parameters**:
- `query` (string): search query
- `type` (string): file type filter (optional)
- `limit` (int): result limit (optional, default 50)
**Response**:
```json
{
  "data": [
    {
      "id": 1,
      "name": "example.zip",
      "size": 1024,
      "type": "binary",
      "created_at": "2023-01-01T00:00:00Z"
    }
  ]
}
```

### GET /api/v1/files/queue
**Description**: Get download queue status
**Authentication**: JWT required
**Response**:
```json
{
  "data": {
    "pending": 5,
    "active": 2,
    "completed": 100,
    "failed": 3
  }
}
```

### GET /api/v1/files/queue/stats
**Description**: Get download queue statistics
**Authentication**: JWT required
**Response**:
```json
{
  "data": {
    "total_downloaded": 1000,
    "total_size": "10GB",
    "average_speed": "2.5MB/s",
    "success_rate": 95.5
  }
}
```

### GET /api/v1/files/queue/progress
**Description**: Live progress of the download queue (for polling)
**Authentication**: JWT required

## Project Endpoints (read-only)

`/api/v1/projects` is a read-only view; project writes go through `/api/v1/admin/projects` below.

### GET /api/v1/projects
**Description**: List all projects
**Authentication**: JWT required
**Response**:
```json
{
  "data": [
    {
      "id": 1,
      "name": "GitHub Releases",
      "enabled": true,
      "created_at": "2023-01-01T00:00:00Z",
      "config": {...}
    }
  ]
}
```

### GET /api/v1/projects/{id}
**Description**: Get project details
**Authentication**: JWT required
**Parameters**:
- `id` (path): project ID

### GET /api/v1/projects/{id}/files
**Description**: List a project's files
**Authentication**: JWT required
**Parameters**:
- `id` (path): project ID

## Crawl Endpoints

### GET /api/v1/crawl/status
**Description**: Get crawl status
**Authentication**: JWT required
**Response**:
```json
{
  "data": {
    "projects": [
      {
        "name": "github",
        "status": "running",
        "last_run": "2023-01-01T00:00:00Z",
        "next_run": "2023-01-02T00:00:00Z"
      }
    ]
  }
}
```

### POST /api/v1/crawl/trigger
**Description**: Trigger a crawl manually
**Authentication**: JWT required
**Request Body**:
```json
{
  "project": "github",
  "force": false
}
```

### GET /api/v1/crawl/logs
**Description**: Get crawl logs
**Authentication**: JWT required
**Query Parameters**:
- `project` (string): project filter (optional)
- `limit` (int): log limit (optional)
**Response**:
```json
{
  "data": [
    {
      "timestamp": "2023-01-01T00:00:00Z",
      "level": "info",
      "message": "Crawl started",
      "project": "github"
    }
  ]
}
```

## System Info Endpoints

### GET /api/v1/system/info
**Description**: Get system info
**Authentication**: JWT required
**Response**:
```json
{
  "data": {
    "version": "1.0.0",
    "uptime": "24h",
    "memory_usage": "128MB",
    "disk_usage": "45%",
    "running_since": "2023-01-01T00:00:00Z"
  }
}
```

### GET /api/v1/system/stats
**Description**: Get current system stats (CPU, memory, network)
**Authentication**: JWT required
**Response**:
```json
{
  "data": {
    "cpu_usage_percent": 23.5,
    "memory_usage_percent": 45.2,
    "memory_total_bytes": 491122688,
    "memory_used_bytes": 222000000,
    "network": {...}
  }
}
```

### GET /api/v1/system/stats/history
**Description**: Get system stats history
**Authentication**: JWT required
**Query Parameters**:
- `hours` (int): hours of history (optional, default 24)
**Response**:
```json
{
  "data": [
    {
      "timestamp": "2023-01-01T00:00:00Z",
      "cpu_usage_percent": 23.5,
      "memory_usage_percent": 45.2
    }
  ]
}
```

## OS Install Endpoints

### GET /api/v1/os-install/configs
**Description**: List OS install configs
**Authentication**: JWT required
**Response**:
```json
{
  "data": [
    {
      "id": 1,
      "name": "Ubuntu 22.04",
      "enabled": true,
      "format": "preseed",
      "created_at": "2023-01-01T00:00:00Z"
    }
  ]
}
```

## Dashboard Endpoints (JWT required)

### GET /api/v1/admin/dashboard/summary
**Description**: Aggregated dashboard summary with stats from every module
**Authentication**: JWT required
**Response**:
```json
{
  "success": true,
  "data": {
    "system": {
      "version": "1.0.0",
      "uptime": "5d 3h 22m",
      "cpu_usage": 23.5,
      "mem_usage": 45.2,
      "mem_total": 491122688,
      "mem_used": 222000000,
      "disk_total": 61236858880,
      "disk_used": 28456726528,
      "disk_usage_percent": 46.5,
      "containers_enabled": true
    },
    "files": {
      "project_count": 6,
      "total_files": 142,
      "queue_pending": 5,
      "queue_downloading": 1,
      "queue_complete": 130,
      "queue_error": 2
    },
    "deploy": {
      "config_count": 8,
      "iso_count": 12,
      "iso_pending": 3,
      "iso_downloaded": 9
    },
    "share": {
      "file_count": 24,
      "total_bytes": 5368709120,
      "total_size": "5.0 GB"
    },
    "activity": [
      {
        "id": "crawl-42",
        "type": "crawl_success",
        "title": "HashiCorp Terraform",
        "subtitle": "Found 3 versions, downloaded 5 files",
        "timestamp": "2026-05-17T10:30:00Z"
      }
    ]
  }
}
```

### GET /api/v1/admin/metrics/cache
**Description**: Internal cache metrics (supply-layer index/count cache hits)
**Authentication**: JWT required

## Admin Panel Endpoints (JWT required)

### Project management
- **GET** `/api/v1/admin/projects` - list projects
- **POST** `/api/v1/admin/projects` - create project
- **GET** `/api/v1/admin/projects/{id}` - get project
- **PUT** `/api/v1/admin/projects/{id}` - update project
- **DELETE** `/api/v1/admin/projects/{id}` - delete project
- **PATCH** `/api/v1/admin/projects/{id}/toggle` - enable/disable project

### Crawl management
- **GET** `/api/v1/admin/crawl/status` - admin crawl status
- **POST** `/api/v1/admin/crawl/trigger/{name}` - trigger a specific project
- **POST** `/api/v1/admin/crawl/trigger-all` - trigger all projects
- **POST** `/api/v1/admin/crawl/pause/{name}` - pause a project
- **POST** `/api/v1/admin/crawl/resume/{name}` - resume a project

### Token management
- **GET** `/api/v1/admin/credentials` - list API tokens
- **PUT** `/api/v1/admin/credentials` - create/update a token

### Security
- **PUT** `/api/v1/admin/password` - change the admin password

### Monitor config
- **GET** `/api/v1/admin/config/monitor` - get disk warning/critical thresholds
- **PUT** `/api/v1/admin/config/monitor` - update disk thresholds
**PUT body**:
```json
{
  "disk_warning_percent": 80,
  "disk_critical_percent": 95
}
```

### Storage config & migrations
- **GET** `/api/v1/admin/config/storage` - get storage path config
- **PUT** `/api/v1/admin/config/storage` - update storage paths (spawns background migration tasks)
- **GET** `/api/v1/admin/storage/migrations` - list storage migration tasks
- **GET** `/api/v1/admin/storage/migrations/{id}` - get migration task detail
- **POST** `/api/v1/admin/storage/migrations/{id}/cancel` - cancel a migration task

### File center & file management
- **GET** `/api/v1/admin/files` - cross-project file listing (filters, paging)
- **POST** `/api/v1/admin/files/{id}/retry` - requeue a failed download
- **GET** `/api/v1/admin/files/{id}/internal` - internal file metadata (paths, checksums — diagnostics)

### Share-link management
- **GET** `/api/v1/admin/share-links` - list share links
- **POST** `/api/v1/admin/share-links` - create a share link (mints a download token)
- **DELETE** `/api/v1/admin/share-links/{token}` - revoke a share link

### OS install management
- **GET** `/api/v1/admin/os-install/configs` - list configs
- **POST** `/api/v1/admin/os-install/configs` - create config
- **PUT** `/api/v1/admin/os-install/configs/{id}` - update config
- **DELETE** `/api/v1/admin/os-install/configs/{id}` - delete config
- **GET** `/api/v1/admin/os-install/configs/{id}` - get config
- **POST** `/api/v1/admin/os-install/configs/preview` - preview the generated config

### ISO management
- **GET** `/api/v1/admin/os-install/isos` - list ISOs
- **POST** `/api/v1/admin/os-install/iso/download` - download an ISO
- **DELETE** `/api/v1/admin/os-install/isos/{name}` - delete an ISO
- **GET** `/api/v1/admin/os-install/catalog` - list ISO catalog entries
- **POST** `/api/v1/admin/os-install/catalog` - create catalog entry
- **PUT** `/api/v1/admin/os-install/catalog/{id}` - update catalog entry
- **DELETE** `/api/v1/admin/os-install/catalog/{id}` - delete catalog entry
- **POST** `/api/v1/admin/os-install/catalog/{id}/check` - check for the latest version
- **POST** `/api/v1/admin/os-install/catalog/{id}/download` - trigger a catalog download
- **POST** `/api/v1/admin/os-install/catalog/{id}/retry` - retry a failed download
- **POST** `/api/v1/admin/os-install/catalog/{id}/cancel` - cancel a download
- **POST** `/api/v1/admin/os-install/catalog/check-all` - check all versions
- **GET** `/api/v1/admin/os-install/catalog/profiles` - distro profiles (two-level scraping templates)
- **GET** `/api/v1/admin/os-install/catalog/queue` - ISO download queue stats
- **POST** `/api/v1/admin/os-install/catalog/download-all` - queue all available ISOs
- **GET** `/api/v1/admin/os-install/catalog/progress` - ISO download progress

### Container management
- **GET** `/api/v1/admin/containers` - list containers
- **POST** `/api/v1/admin/containers` - create container
- **GET** `/api/v1/admin/containers/{id}` - get container detail
- **PUT** `/api/v1/admin/containers/{id}` - update container
- **DELETE** `/api/v1/admin/containers/{id}` - delete container
- **POST** `/api/v1/admin/containers/{id}/start` - start container
- **POST** `/api/v1/admin/containers/{id}/stop` - stop container
- **POST** `/api/v1/admin/containers/{id}/restart` - restart container
- **GET** `/api/v1/admin/containers/{id}/logs` - container logs
- **GET** `/api/v1/admin/containers/{id}/stats` - container stats

### Image management
- **GET** `/api/v1/admin/images` - list Docker images
- **POST** `/api/v1/admin/images/pull` - pull a Docker image
- **DELETE** `/api/v1/admin/images/{id}` - delete a Docker image

### App templates
- **GET** `/api/v1/admin/templates` - list app templates
- **POST** `/api/v1/admin/templates` - create app template
- **GET** `/api/v1/admin/templates/{id}` - get app template
- **DELETE** `/api/v1/admin/templates/{id}` - delete app template

### Registry management (remote image registries)
- **GET** `/api/v1/admin/registries` - list registries
- **POST** `/api/v1/admin/registries` - create registry
- **GET** `/api/v1/admin/registries/{id}` - get registry
- **PUT** `/api/v1/admin/registries/{id}` - update registry
- **DELETE** `/api/v1/admin/registries/{id}` - delete registry
- **POST** `/api/v1/admin/registries/test-connection` - test the connection
- **GET** `/api/v1/admin/registries/{id}/catalog` - browse the repository catalog
- **GET** `/api/v1/admin/registries/{id}/tags` - list a repository's tags
- **GET** `/api/v1/admin/registries/{id}/tags/{tag}` - tag detail
- **DELETE** `/api/v1/admin/registries/{id}/tags/{tag}` - delete a tag

### Sync tasks
- **POST** `/api/v1/admin/sync` - create a registry sync task
- **GET** `/api/v1/admin/sync` - list sync tasks
- **GET** `/api/v1/admin/sync/{id}` - get sync task
- **POST** `/api/v1/admin/sync/{id}/cancel` - cancel a sync task

### Retention policies
- **GET** `/api/v1/admin/retention` - list retention policies
- **POST** `/api/v1/admin/retention` - create retention policy
- **PUT** `/api/v1/admin/retention/{id}` - update retention policy
- **DELETE** `/api/v1/admin/retention/{id}` - delete retention policy
- **POST** `/api/v1/admin/retention/{id}/execute` - run a cleanup immediately

### Virtual index (WebDAV directory tree)
- **GET/POST** `/api/v1/admin/channels` - list/create channels
- **GET/PUT/DELETE** `/api/v1/admin/channels/{id}` - get/update/delete channel
- **GET/POST** `/api/v1/admin/channels/{channel_id}/views` - list/create views
- **GET/PUT/DELETE** `/api/v1/admin/views/{id}` - get/update/delete view
- **GET** `/api/v1/admin/views/{view_id}/tree` - full node tree of a view
- **GET/POST** `/api/v1/admin/views/{view_id}/nodes` - list/create nodes
- **PUT/DELETE** `/api/v1/admin/nodes/{id}` - update/delete node
- **GET** `/api/v1/admin/virtual-audit` - virtual-index change audit log

### Tool catalog
- **GET** `/api/v1/admin/tool-catalog` - curated built-in tool catalog
- **POST** `/api/v1/admin/tool-catalog/{slug}/enable` - enable a tool with one click (creates the project)

### Search
- **GET** `/api/v1/admin/search` - full-text search across files and configs
**Query Parameters**:
- `q` (string): search query
- `type` (string): type filter (files, configs, all)

### Logs
- **GET** `/api/v1/admin/logs` - get system logs
**Query Parameters**:
- `level` (string): log level filter (optional)
- `limit` (int): result limit (optional)
- `source` (string): source filter (optional)

### Tasks
- **GET** `/api/v1/admin/tasks` - list background tasks

### Backup
- **GET** `/api/v1/admin/backups` - list available backups
- **POST** `/api/v1/admin/backups/restore` - restore from a backup archive

## Public ISO Endpoints

### GET /api/v1/isos
**Description**: List available ISO files (public, no auth — lets PXE/install scripts discover images)
**Authentication**: None
**Response**:
```json
{
  "data": [
    {
      "id": 1,
      "name": "ubuntu-22.04.iso",
      "distro": "Ubuntu",
      "arch": "amd64",
      "size": "4.7GB",
      "current_url": "https://releases.ubuntu.com/22.04.3/ubuntu-22.04.3-live-server-amd64.iso",
      "download_status": "pending"
    }
  ]
}
```

### GET /api/v1/isos/{name}/download
**Description**: Download an ISO file (JWT required)
**Authentication**: JWT via the Authorization header or a `?token=` query parameter
**Parameters**:
- `name` (path): ISO name
**Response**: file download stream
**Example header**:
```
Authorization: Bearer <jwt-token>
```
**Example URL**:
```
/api/v1/isos/ubuntu-22.04/download?token=<jwt-token>
```

## Supply Endpoints (public, no auth)

MiBeeHive serves what Foraging collected over the **native protocols** of external servers, so the whole fleet consumes collected ops tooling with its own tools — no dedicated client. These endpoints are public (no JWT) so external hosts can pull unattended.

### GET /repo/index
**Description**: Generic JSON manifest of all suppliable (status=complete) files.
**Authentication**: None
**Response**: `{ "count": N, "items": [ { "id", "filename", "version", "size_bytes", "checksum", "download_url": "/repo/files/{id}", ... } ] }`

### GET /repo/files/{id}
**Description**: Stream a single collected artifact by id (generic fallback download).
**Authentication**: None
**Parameters**: `id` (path): file id
**Response**: file download stream

### GET /apt/{rest...}
**Description**: An APT repository built over collected `.deb` files. It generates
`dists/{suite}/main/binary-{arch}/Packages[.gz]` and `Release` on demand (cached,
invalidated by mtime) and serves pool downloads. External Debian/Ubuntu hosts add
it as an apt source:
```
echo "deb http://<host>:9090/apt stable main" | tee /etc/apt/sources.list.d/mibeehive.list
apt update && apt install <pkg>
```
**Authentication**: None

### GET /simple/{rest...}
**Description**: A PyPI "Simple Repository API" (PEP 503) built over collected
Python wheels/sdists. `GET /simple/` lists supplied projects; `GET /simple/<project>/`
lists the project's distributions with `#sha256=...` fragments. Project names are
PEP 503-normalized (`My_Pkg`, `my-pkg`, and `my.pkg` all match). External Python
hosts install with their native tools:
```
pip install --index-url http://<host>:9090/simple/ <pkg>
# or
uv pip install --index-url http://<host>:9090/simple/ <pkg>
```
**Authentication**: None

## Share-Link Endpoints (public — the token is the credential)

### GET /s/{token}
**Description**: Download a file via a share token. Tokens are minted by the admin endpoint `/api/v1/admin/share-links` and can be revoked at any time; the same token also works as `GET /api/v1/files/{token}/download`.
**Authentication**: None (the token itself is the credential)
**Parameters**: `token` (path): share token

## WebDAV Endpoint

### /webdav/ (and subpaths)
**Description**: WebDAV file service (`PROPFIND`, `GET`, `PUT`, `MKCOL`, `DELETE`, `MOVE`, `COPY`). **Served on the HTTPS port only** (the HTTP port redirects). The directory tree is organized by the virtual index (channels/views/nodes); manual uploads land in the Manual Uploads project.
**Authentication**: Basic Auth — anonymous read; admin credentials (same as the admin panel) grant read/write.

## Public PXE Endpoints (no auth)

### GET /pxe/{format}/{name}
**Description**: Serve a PXE config file
**Authentication**: None
**Parameters**:
- `format` (path): config format (preseed, kickstart, autoinstall)
- `name` (path): config name
**Response**: config file contents

## Health & Metrics Endpoints

### GET /health
**Description**: Health check endpoint
**Authentication**: None
**Response**: `OK`

### GET /metrics
**Description**: Prometheus metrics endpoint
**Authentication**: None
**Response**: Prometheus-format metrics

## Response Format

All API endpoints use a consistent response format:

### Success
```json
{
  "success": true,
  "data": {...}
}
```

### Error
```json
{
  "success": false,
  "message": "error description"
}
```

### Common error codes
- `400 Bad Request`: malformed request
- `401 Unauthorized`: invalid or missing JWT
- `404 Not Found`: resource not found
- `500 Internal Server Error`: server error

## Authentication

### JWT tokens
- Admin endpoints require a valid JWT in the `Authorization` header
- Token format: `Authorization: Bearer <token>`
- Token expiry: 1 hour (3600 seconds)
- Tokens are issued by `/api/v1/auth/login` and renewed via `/api/v1/auth/refresh`

### WebDAV auth
- Basic Auth required
- Anonymous: read-only access
- Admin: read/write access
- Credentials are shared with the web admin panel

### Share tokens
- `GET /s/{token}` and `GET /api/v1/files/{token}/download` replace login with a token
- Tokens can be revoked by an admin at any time
