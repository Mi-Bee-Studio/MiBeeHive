# MiBeeHive API Reference

[中文](../zh/api-reference.md)


## Authentication Endpoints

### POST /api/v1/auth/login
**Description**: User authentication and JWT token generation
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

### GET /api/v1/auth/password-status
**Description**: Check if password needs to be changed
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

## File Management Endpoints

### GET /api/v1/files/{id}/download
**Description**: Download a specific file
**Authentication**: None
**Parameters**: 
- `id` (path): File ID
**Response**: File download stream

### GET /api/v1/files/search
**Description**: Search for files
**Authentication**: None
**Query Parameters**:
- `query` (string): Search query
- `type` (string): File type filter (optional)
- `limit` (int): Result limit (optional, default 50)
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
**Authentication**: None
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
**Authentication**: None
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

## Project Management Endpoints

### GET /api/v1/projects
**Description**: List all projects
**Authentication**: None
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

### POST /api/v1/projects
**Description**: Create a new project
**Authentication**: None
**Request Body**:
```json
{
  "name": "New Project",
  "enabled": true,
  "config": {...}
}
```

### GET /api/v1/projects/{id}
**Description**: Get project details
**Authentication**: None
**Parameters**: 
- `id` (path): Project ID

### PUT /api/v1/projects/{id}
**Description**: Update project
**Authentication**: None
**Parameters**: 
- `id` (path): Project ID
**Request Body**: Same as create

### DELETE /api/v1/projects/{id}
**Description**: Delete project (soft delete)
**Authentication**: None
**Parameters**: 
- `id` (path): Project ID

### GET /api/v1/projects/{id}/files
**Description**: List project files
**Authentication**: None
**Parameters**: 
- `id` (path): Project ID

## Crawl Management Endpoints

### GET /api/v1/crawl/status
**Description**: Get crawl status
**Authentication**: None
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
**Description**: Trigger crawl manually
**Authentication**: None
**Request Body**:
```json
{
  "project": "github",
  "force": false
}
```

### GET /api/v1/crawl/logs
**Description**: Get crawl logs
**Authentication**: None
**Query Parameters**:
- `project` (string): Project filter (optional)
- `limit` (int): Log limit (optional)
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

## System Information Endpoints

### GET /api/v1/system/info
**Description**: Get system information
**Authentication**: None
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
**Description**: Get current system statistics (CPU, memory, network)
**Authentication**: None
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
**Description**: Get system statistics history
**Authentication**: None
**Query Parameters**:
- `hours` (int): Hours of history (optional, default 24)
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

## OS Installation Endpoints

### GET /api/v1/os-install/configs
**Description**: List OS installation configurations
**Authentication**: None
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

## Dashboard Endpoints (JWT Required)

### GET /api/v1/admin/dashboard/summary
**Description**: Get aggregated dashboard summary with stats from all modules
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

## Admin Panel Endpoints (JWT Required)

### Project Management
- **GET** `/api/v1/admin/projects` - List projects
- **POST** `/api/v1/admin/projects` - Create project
- **PUT** `/api/v1/admin/projects/{id}` - Update project
- **DELETE** `/api/v1/admin/projects/{id}` - Delete project
- **PUT** `/api/v1/admin/projects/{id}/toggle` - Enable/disable project

### Crawl Management
- **GET** `/api/v1/admin/crawl/status` - Get admin crawl status
- **POST** `/api/v1/admin/crawl/trigger/{name}` - Trigger specific project
- **POST** `/api/v1/admin/crawl/trigger-all` - Trigger all projects
- **PUT** `/api/v1/admin/crawl/pause/{name}` - Pause project
- **PUT** `/api/v1/admin/crawl/resume/{name}` - Resume project

### Token Management
- **GET** `/api/v1/admin/credentials` - List API tokens
- **POST** `/api/v1/admin/credentials` - Create/update token

### Security Management
- **PUT** `/api/v1/admin/password` - Change admin password

### Monitor Configuration
- **GET** `/api/v1/admin/config/monitor` - Get disk warning/critical thresholds
- **PUT** `/api/v1/admin/config/monitor` - Update disk thresholds
**PUT Request Body**:
```json
{
  "disk_warning_percent": 80,
  "disk_critical_percent": 95
}
```

### OS Installation Management
- **GET** `/api/v1/admin/os-install/configs` - List configs
- **POST** `/api/v1/admin/os-install/configs` - Create config
- **PUT** `/api/v1/admin/os-install/configs/{id}` - Update config
- **DELETE** `/api/v1/admin/os-install/configs/{id}` - Delete config
- **GET** `/api/v1/admin/os-install/configs/{id}` - Get config
- **POST** `/api/v1/admin/os-install/configs/preview` - Preview config

### ISO Management
- **GET** `/api/v1/admin/os-install/isos` - List ISOs
- **POST** `/api/v1/admin/os-install/iso/download` - Download ISO
- **DELETE** `/api/v1/admin/os-install/isos/{name}` - Delete ISO
- **GET** `/api/v1/admin/os-install/catalog` - List ISO catalog entries
- **POST** `/api/v1/admin/os-install/catalog` - Create catalog entry
- **PUT** `/api/v1/admin/os-install/catalog/{id}` - Update catalog entry
- **DELETE** `/api/v1/admin/os-install/catalog/{id}` - Delete catalog entry
- **POST** `/api/v1/admin/os-install/catalog/{id}/check` - Check latest version
- **POST** `/api/v1/admin/os-install/catalog/{id}/download` - Trigger catalog download
- **POST** `/api/v1/admin/os-install/catalog/check-all` - Check all versions
- **GET** `/api/v1/admin/os-install/catalog/queue` - Get ISO download queue stats
- **POST** `/api/v1/admin/os-install/catalog/download-all` - Queue all available ISOs
- **GET** `/api/v1/admin/os-install/catalog/progress` - Get ISO download progress

### Container Management
- **GET** `/api/v1/admin/containers` - List containers
- **POST** `/api/v1/admin/containers` - Create container
- **GET** `/api/v1/admin/containers/{id}` - Get container details
- **PUT** `/api/v1/admin/containers/{id}` - Update container
- **DELETE** `/api/v1/admin/containers/{id}` - Delete container
- **POST** `/api/v1/admin/containers/{id}/start` - Start container
- **POST** `/api/v1/admin/containers/{id}/stop` - Stop container
- **POST** `/api/v1/admin/containers/{id}/restart` - Restart container
- **GET** `/api/v1/admin/containers/{id}/logs` - Get container logs
- **GET** `/api/v1/admin/containers/{id}/stats` - Get container stats

### Image Management
- **GET** `/api/v1/admin/images` - List Docker images
- **POST** `/api/v1/admin/images/pull` - Pull Docker image
- **DELETE** `/api/v1/admin/images/{id}` - Delete Docker image

### Application Templates
- **GET** `/api/v1/admin/templates` - List app templates
- **POST** `/api/v1/admin/templates` - Create app template
- **GET** `/api/v1/admin/templates/{id}` - Get app template
- **DELETE** `/api/v1/admin/templates/{id}` - Delete app template

### WebDAV Management
- **GET** `/api/v1/admin/webdav/status` - Get WebDAV status
- **GET** `/api/v1/admin/webdav/files` - List WebDAV files

### Search
- **GET** `/api/v1/admin/search` - Full-text search across files and configs
**Query Parameters**:
- `q` (string): Search query
- `type` (string): Filter type (files, configs, all)

### Logs
- **GET** `/api/v1/admin/logs` - Get system logs
**Query Parameters**:
- `level` (string): Log level filter (optional)
- `limit` (int): Result limit (optional)
- `source` (string): Source filter (optional)

### Tasks
- **GET** `/api/v1/admin/tasks` - List background tasks

### Backup
- **GET** `/api/v1/admin/backups` - List available backups
- **POST** `/api/v1/admin/backups/restore` - Restore from backup archive

## ISO Public Endpoints

### GET /api/v1/isos
**Description**: List available ISO files (public, no authentication)
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
**Description**: Download ISO file (JWT required)
**Authentication**: JWT via Authorization header or ?token= query parameter
**Parameters**: 
- `name` (path): ISO name
**Response**: File download stream
**Example Request Headers**:
```
Authorization: Bearer <jwt-token>
```
**Example Request URL**:
```
/api/v1/isos/ubuntu-22.04/download?token=<jwt-token>
```

## Public PXE Endpoints (No Authentication)

### GET /pxe/{format}/{name}
**Description**: Serve PXE configuration files
**Authentication**: None
**Parameters**:
- `format` (path): Configuration format (preseed, kickstart, autoinstall)
- `name` (path): Configuration name
**Response**: Configuration file content

## Health & Metrics Endpoints

### GET /health
**Description**: Health check endpoint
**Authentication**: None
**Response**: `OK`

### GET /metrics
**Description**: Prometheus metrics endpoint
**Authentication**: None
**Response**: Prometheus-formatted metrics

## Response Format

All API endpoints use a consistent response format:

### Success Response
```json
{
  "success": true,
  "data": {...}
}
```

### Error Response
```json
{
  "success": false,
  "message": "Error description"
}
```

### Common Error Codes
- `400 Bad Request`: Malformed request
- `401 Unauthorized`: Invalid or missing JWT token
- `404 Not Found`: Resource not found
- `500 Internal Server Error`: Server error

## Authentication

### JWT Tokens
- Admin endpoints require valid JWT token in `Authorization` header
- Token format: `Authorization: Bearer <token>`
- Token expiration: 1 hour (3600 seconds)
- Token provided by `/api/v1/auth/login` endpoint

### WebDAV Authentication
- Basic Authentication required
- Anonymous users: Read-only access
- Admin users: Read-write access
- Credentials same as web admin panel
