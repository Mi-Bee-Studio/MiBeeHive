# MiBeeHive API 参考文档

MiBeeHive 的全部 HTTP 端点。**认证模型**：除下述**公共端点**外，所有 `/api/v1/*` 端点都需要 JWT（`Authorization: Bearer <token>`）：

- `POST /api/v1/auth/login`（登录本身）
- `GET /api/v1/files/{id|token}/download`、`GET /s/{token}`（令牌下载）
- `GET /api/v1/isos`（公共 ISO 列表）
- 供应层：`/repo/*`、`/apt/*`、`/simple/*`
- `GET /pxe/{format}/{name}`（PXE 客户端无法登录）
- `GET /health`、`GET /metrics`
- `/webdav/*`（独立的基本认证：匿名只读、管理员读写）

## 认证端点

### POST /api/v1/auth/login
**描述**：用户认证和 JWT 令牌生成
**认证**：无
**请求体**：
```json
{
  "username": "admin",
  "password": "password"
}
```
**响应**：
```json
{
  "token": "jwt-token-here",
  "expires_in": 3600
}
```

### POST /api/v1/auth/refresh
**描述**：在令牌过期前续期，返回新 JWT
**认证**：需要 JWT

### GET /api/v1/auth/password-status
**描述**：检查是否需要更改密码（例如仍在使用默认密码时）
**认证**：需要 JWT
**响应**：
```json
{
  "success": true,
  "data": {
    "must_change": false
  }
}
```

## 文件端点

### GET /api/v1/files/{id}/download
**描述**：下载特定文件。`{id}` 既可以是数字 ID，也可以是**分享令牌**（base58）——后者是分享链接的下载通道，无需登录
**认证**：无
**参数**：
- `id`（路径）：文件 ID 或分享令牌
**响应**：文件下载流

### GET /api/v1/files/search
**描述**：搜索文件
**认证**：需要 JWT
**查询参数**：
- `query`（字符串）：搜索查询
- `type`（字符串）：文件类型过滤（可选）
- `limit`（整数）：结果限制（可选，默认 50）
**响应**：
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
**描述**：获取下载队列状态
**认证**：需要 JWT
**响应**：
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
**描述**：获取下载队列统计信息
**认证**：需要 JWT
**响应**：
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
**描述**：获取下载队列的实时进度（供轮询）
**认证**：需要 JWT

## 项目端点（只读）

`/api/v1/projects` 为只读视图；项目的增删改走下方管理面板的 `/api/v1/admin/projects`。

### GET /api/v1/projects
**描述**：列出所有项目
**认证**：需要 JWT
**响应**：
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
**描述**：获取项目详情
**认证**：需要 JWT
**参数**：
- `id`（路径）：项目 ID

### GET /api/v1/projects/{id}/files
**描述**：列出项目文件
**认证**：需要 JWT
**参数**：
- `id`（路径）：项目 ID

## 爬取端点

### GET /api/v1/crawl/status
**描述**：获取爬取状态
**认证**：需要 JWT
**响应**：
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
**描述**：手动触发爬取
**认证**：需要 JWT
**请求体**：
```json
{
  "project": "github",
  "force": false
}
```

### GET /api/v1/crawl/logs
**描述**：获取爬取日志
**认证**：需要 JWT
**查询参数**：
- `project`（字符串）：项目过滤（可选）
- `limit`（整数）：日志限制（可选）
**响应**：
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

## 系统信息端点

### GET /api/v1/system/info
**描述**：获取系统信息
**认证**：需要 JWT
**响应**：
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
**描述**：获取当前系统统计信息（CPU、内存、网络）
**认证**：需要 JWT
**响应**：
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
**描述**：获取系统统计历史
**认证**：需要 JWT
**查询参数**：
- `hours`（整数）：历史小时数（可选，默认 24）
**响应**：
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

## 操作系统安装端点

### GET /api/v1/os-install/configs
**描述**：列出操作系统安装配置
**认证**：需要 JWT
**响应**：
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

## 仪表板端点（需要 JWT）

### GET /api/v1/admin/dashboard/summary
**描述**：获取聚合仪表板摘要，包含所有模块的统计信息
**认证**：需要 JWT
**响应**：
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
**描述**：内部缓存指标（供应层索引/计数缓存命中情况）
**认证**：需要 JWT

## 管理面板端点（需要 JWT）

### 项目管理
- **GET** `/api/v1/admin/projects` - 列出项目
- **POST** `/api/v1/admin/projects` - 创建项目
- **GET** `/api/v1/admin/projects/{id}` - 获取项目
- **PUT** `/api/v1/admin/projects/{id}` - 更新项目
- **DELETE** `/api/v1/admin/projects/{id}` - 删除项目
- **PATCH** `/api/v1/admin/projects/{id}/toggle` - 启用/禁用项目

### 爬取管理
- **GET** `/api/v1/admin/crawl/status` - 获取管理爬取状态
- **POST** `/api/v1/admin/crawl/trigger/{name}` - 触发特定项目
- **POST** `/api/v1/admin/crawl/trigger-all` - 触发所有项目
- **POST** `/api/v1/admin/crawl/pause/{name}` - 暂停项目
- **POST** `/api/v1/admin/crawl/resume/{name}` - 恢复项目

### 令牌管理
- **GET** `/api/v1/admin/credentials` - 列出 API 令牌
- **PUT** `/api/v1/admin/credentials` - 创建/更新令牌

### 安全管理
- **PUT** `/api/v1/admin/password` - 更改管理员密码

### 监控配置
- **GET** `/api/v1/admin/config/monitor` - 获取磁盘警告/关键阈值
- **PUT** `/api/v1/admin/config/monitor` - 更新磁盘阈值
**PUT 请求体**：
```json
{
  "disk_warning_percent": 80,
  "disk_critical_percent": 95
}
```

### 存储配置与迁移
- **GET** `/api/v1/admin/config/storage` - 获取存储路径配置
- **PUT** `/api/v1/admin/config/storage` - 更新存储路径（触发后台迁移任务）
- **GET** `/api/v1/admin/storage/migrations` - 列出存储迁移任务
- **GET** `/api/v1/admin/storage/migrations/{id}` - 获取迁移任务详情
- **POST** `/api/v1/admin/storage/migrations/{id}/cancel` - 取消迁移任务

### 文件中心与文件管理
- **GET** `/api/v1/admin/files` - 跨项目文件列表（过滤、分页）
- **POST** `/api/v1/admin/files/{id}/retry` - 重新排队失败的下载
- **GET** `/api/v1/admin/files/{id}/internal` - 文件内部元数据（路径、校验等诊断信息）

### 分享链接管理
- **GET** `/api/v1/admin/share-links` - 列出分享链接
- **POST** `/api/v1/admin/share-links` - 创建分享链接（生成下载令牌）
- **DELETE** `/api/v1/admin/share-links/{token}` - 吊销分享链接

### 操作系统安装管理
- **GET** `/api/v1/admin/os-install/configs` - 列出配置
- **POST** `/api/v1/admin/os-install/configs` - 创建配置
- **PUT** `/api/v1/admin/os-install/configs/{id}` - 更新配置
- **DELETE** `/api/v1/admin/os-install/configs/{id}` - 删除配置
- **GET** `/api/v1/admin/os-install/configs/{id}` - 获取配置
- **POST** `/api/v1/admin/os-install/configs/preview` - 预览配置

### ISO 管理
- **GET** `/api/v1/admin/os-install/isos` - 列出 ISO
- **POST** `/api/v1/admin/os-install/iso/download` - 下载 ISO
- **DELETE** `/api/v1/admin/os-install/isos/{name}` - 删除 ISO
- **GET** `/api/v1/admin/os-install/catalog` - 列出 ISO 目录条目
- **POST** `/api/v1/admin/os-install/catalog` - 创建目录条目
- **PUT** `/api/v1/admin/os-install/catalog/{id}` - 更新目录条目
- **DELETE** `/api/v1/admin/os-install/catalog/{id}` - 删除目录条目
- **POST** `/api/v1/admin/os-install/catalog/{id}/check` - 检查最新版本
- **POST** `/api/v1/admin/os-install/catalog/{id}/download` - 触发目录下载
- **POST** `/api/v1/admin/os-install/catalog/{id}/retry` - 重试失败的下载
- **POST** `/api/v1/admin/os-install/catalog/{id}/cancel` - 取消下载
- **POST** `/api/v1/admin/os-install/catalog/check-all` - 检查所有版本
- **GET** `/api/v1/admin/os-install/catalog/profiles` - 发行版档案（两级抓取模板）
- **GET** `/api/v1/admin/os-install/catalog/queue` - 获取 ISO 下载队列统计
- **POST** `/api/v1/admin/os-install/catalog/download-all` - 队列所有可用的 ISO
- **GET** `/api/v1/admin/os-install/catalog/progress` - 获取 ISO 下载进度

### 容器管理
- **GET** `/api/v1/admin/containers` - 列出容器
- **POST** `/api/v1/admin/containers` - 创建容器
- **GET** `/api/v1/admin/containers/{id}` - 获取容器详情
- **PUT** `/api/v1/admin/containers/{id}` - 更新容器
- **DELETE** `/api/v1/admin/containers/{id}` - 删除容器
- **POST** `/api/v1/admin/containers/{id}/start` - 启动容器
- **POST** `/api/v1/admin/containers/{id}/stop` - 停止容器
- **POST** `/api/v1/admin/containers/{id}/restart` - 重启容器
- **GET** `/api/v1/admin/containers/{id}/logs` - 获取容器日志
- **GET** `/api/v1/admin/containers/{id}/stats` - 获取容器统计

### 镜像管理
- **GET** `/api/v1/admin/images` - 列出 Docker 镜像
- **POST** `/api/v1/admin/images/pull` - 拉取 Docker 镜像
- **DELETE** `/api/v1/admin/images/{id}` - 删除 Docker 镜像

### 应用模板
- **GET** `/api/v1/admin/templates` - 列出应用模板
- **POST** `/api/v1/admin/templates` - 创建应用模板
- **GET** `/api/v1/admin/templates/{id}` - 获取应用模板
- **DELETE** `/api/v1/admin/templates/{id}` - 删除应用模板

### Registry 管理（远程镜像仓库）
- **GET** `/api/v1/admin/registries` - 列出 registry
- **POST** `/api/v1/admin/registries` - 创建 registry
- **GET** `/api/v1/admin/registries/{id}` - 获取 registry
- **PUT** `/api/v1/admin/registries/{id}` - 更新 registry
- **DELETE** `/api/v1/admin/registries/{id}` - 删除 registry
- **POST** `/api/v1/admin/registries/test-connection` - 测试连接
- **GET** `/api/v1/admin/registries/{id}/catalog` - 浏览仓库目录
- **GET** `/api/v1/admin/registries/{id}/tags` - 列出某仓库的标签
- **GET** `/api/v1/admin/registries/{id}/tags/{tag}` - 标签详情
- **DELETE** `/api/v1/admin/registries/{id}/tags/{tag}` - 删除标签

### 同步任务
- **POST** `/api/v1/admin/sync` - 创建 registry 同步任务
- **GET** `/api/v1/admin/sync` - 列出同步任务
- **GET** `/api/v1/admin/sync/{id}` - 获取同步任务
- **POST** `/api/v1/admin/sync/{id}/cancel` - 取消同步任务

### 保留策略
- **GET** `/api/v1/admin/retention` - 列出保留策略
- **POST** `/api/v1/admin/retention` - 创建保留策略
- **PUT** `/api/v1/admin/retention/{id}` - 更新保留策略
- **DELETE** `/api/v1/admin/retention/{id}` - 删除保留策略
- **POST** `/api/v1/admin/retention/{id}/execute` - 立即执行一次清理

### 虚拟索引（WebDAV 目录树）
- **GET/POST** `/api/v1/admin/channels` - 列出/创建频道
- **GET/PUT/DELETE** `/api/v1/admin/channels/{id}` - 获取/更新/删除频道
- **GET/POST** `/api/v1/admin/channels/{channel_id}/views` - 列出/创建视图
- **GET/PUT/DELETE** `/api/v1/admin/views/{id}` - 获取/更新/删除视图
- **GET** `/api/v1/admin/views/{view_id}/tree` - 获取视图完整节点树
- **GET/POST** `/api/v1/admin/views/{view_id}/nodes` - 列出/创建节点
- **PUT/DELETE** `/api/v1/admin/nodes/{id}` - 更新/删除节点
- **GET** `/api/v1/admin/virtual-audit` - 虚拟索引变更审计日志

### 工具目录
- **GET** `/api/v1/admin/tool-catalog` - 内置常用工具目录
- **POST** `/api/v1/admin/tool-catalog/{slug}/enable` - 一键启用某工具（自动建项目）

### 搜索
- **GET** `/api/v1/admin/search` - 在文件和配置中全文搜索
**查询参数**：
- `q`（字符串）：搜索查询
- `type`（字符串）：过滤类型（files、configs、all）

### 日志
- **GET** `/api/v1/admin/logs` - 获取系统日志
**查询参数**：
- `level`（字符串）：日志级别过滤（可选）
- `limit`（整数）：结果限制（可选）
- `source`（字符串）：源过滤（可选）

### 任务
- **GET** `/api/v1/admin/tasks` - 列出后台任务

### 备份
- **GET** `/api/v1/admin/backups` - 列出可用备份
- **POST** `/api/v1/admin/backups/restore` - 从备份档案恢复

## ISO 公开端点

### GET /api/v1/isos
**描述**：列出可用的 ISO 文件（公共，无需认证——供 PXE/装机脚本发现镜像）
**认证**：无
**响应**：
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
**描述**：下载 ISO 文件（需要 JWT）
**认证**：通过 Authorization 头或 ?token= 查询参数的 JWT
**参数**：
- `name`（路径）：ISO 名称
**响应**：文件下载流
**示例请求头**：
```
Authorization: Bearer <jwt-token>
```
**示例请求 URL**：
```
/api/v1/isos/ubuntu-22.04/download?token=<jwt-token>
```

## 供应端点（公开，无需认证）

MiBeeHive 把采集（采蜜）到的产物通过外部服务器的**原生协议**对外供应，使整个服务器集群能用各自的工具直接消费采集到的 ops 工具，无需专用客户端。这些端点是公开的（无需 JWT），便于外部主机无人值守地拉取。

### GET /repo/index
**描述**：所有可供应（status=complete）文件的通用 JSON 清单。
**认证**：无
**响应**：`{ "count": N, "items": [ { "id", "filename", "version", "size_bytes", "checksum", "download_url": "/repo/files/{id}", ... } ] }`

### GET /repo/files/{id}
**描述**：按 id 流式下载单个采集产物（通用兜底下载）。
**认证**：无
**参数**：`id`（路径）：文件 id
**响应**：文件下载流

### GET /apt/{rest...}
**描述**：基于采集到的 `.deb` 构建的 APT 仓库。按需生成
`dists/{suite}/main/binary-{arch}/Packages[.gz]` 与 `Release`（带缓存、按
mtime 失效），并提供 pool 下载。外部 Debian/Ubuntu 主机将其加入 apt 源：
```
echo "deb http://<host>:9090/apt stable main" | tee /etc/apt/sources.list.d/mibeehive.list
apt update && apt install <pkg>
```
**认证**：无

### GET /simple/{rest...}
**描述**：基于采集到的 Python wheel/sdist 构建的 PyPI「Simple Repository
API」（PEP 503）。`GET /simple/` 列出已供应的项目；`GET /simple/<project>/`
列出该项目的分发包并附带 `#sha256=...` 校验片段。项目名按 PEP 503 归一化
（`My_Pkg`、`my-pkg`、`my.pkg` 均可匹配）。外部 Python 主机用原生工具安装：
```
pip install --index-url http://<host>:9090/simple/ <pkg>
# 或
uv pip install --index-url http://<host>:9090/simple/ <pkg>
```
**认证**：无

## 分享链接端点（公开，令牌即凭证）

### GET /s/{token}
**描述**：通过分享令牌下载文件。令牌由管理端 `/api/v1/admin/share-links` 创建，可随时吊销；同一个令牌也可用于 `GET /api/v1/files/{token}/download`。
**认证**：无（令牌本身即凭证）
**参数**：`token`（路径）：分享令牌

## WebDAV 端点

### /webdav/（及子路径）
**描述**：WebDAV 文件服务（`PROPFIND`、`GET`、`PUT`、`MKCOL`、`DELETE`、`MOVE`、`COPY`）。**仅在 HTTPS 端口提供**（HTTP 端口会重定向）。
**认证**：基本认证——匿名只读；管理员凭据与管理面板相同，可读写。目录树由虚拟索引（频道/视图/节点）组织，手工上传落在 Manual Uploads 项目下。

## 公共 PXE 端点（无需认证）

### GET /pxe/{format}/{name}
**描述**：提供 PXE 配置文件
**认证**：无
**参数**：
- `format`（路径）：配置格式（preseed、kickstart、autoinstall）
- `name`（路径）：配置名称
**响应**：配置文件内容

## 健康与指标端点

### GET /health
**描述**：健康检查端点
**认证**：无
**响应**：`OK`

### GET /metrics
**描述**：Prometheus 指标端点
**认证**：无
**响应**：Prometheus 格式的指标

## 响应格式

所有 API 端点使用一致的响应格式：

### 成功响应
```json
{
  "success": true,
  "data": {...}
}
```

### 错误响应
```json
{
  "success": false,
  "message": "错误描述"
}
```

### 常见错误代码
- `400 Bad Request`： malformed 请求
- `401 Unauthorized`：无效或缺失 JWT 令牌
- `404 Not Found`：资源未找到
- `500 Internal Server Error`：服务器错误

## 认证

### JWT 令牌
- 管理端点需要在 `Authorization` 头中提供有效的 JWT 令牌
- 令牌格式：`Authorization: Bearer <token>`
- 令牌过期：1 小时（3600 秒）
- 令牌由 `/api/v1/auth/login` 端点提供，`/api/v1/auth/refresh` 续期

### WebDAV 认证
- 需要基本认证
- 匿名用户：只读访问
- 管理员用户：读写访问
- 凭据与 Web 管理面板相同

### 分享令牌
- `GET /s/{token}` 与 `GET /api/v1/files/{token}/download` 以令牌代替登录
- 令牌可由管理员随时吊销
