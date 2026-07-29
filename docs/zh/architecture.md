# MiBeeHive 架构文档

[English](../en/architecture.md)

## 蜂巢哲学

MiBeeHive 是**面向外部服务器的运维工具供应链平台**。蜂巢是最贴切的隐喻：蜂巢不产花蜜，它**采集、酿造、分发**花蜜。MiBeeHive 不发明协议——它从公共源采集运维工具、持续更新，并按已有的标准协议对外供应给外部服务器。产品的本质是一条供应链，而下面两项自给自足的 provisioning 能力，是任何其他运维面板都没有的核心差异点：

- **采蜜 (Foraging)**: 供应引擎——从公共源抓取和下载运维工具（二进制发行版），再按标准协议对外供应给外部服务器
- **哺育 (Provisioning)**: 纳入新的外部服务器——通过 PXE 提供无人值守操作系统安装，让裸机能从零装机并被纳入供应
- **分享 (Sharing)**: 对外提供文件——基本的 WebDAV 能力，可通过 Web UI 配置

> **vs 一般运维面板：** 那些面板管理**本机**（应用商店、建站）。MiBeeHive 面向**外部**那些服务器——它是喂饱整个集群的供应链。

每个模块都在可配置的父路径下有独立的存储路径：`{base_path}/{oss,os-install,webdav}`

### 阶段路线图
- 第一阶段（已完成）：采蜜 — Web 管理爬取源、API 令牌、爬取控制、密码更改
- 第二阶段（已完成）：哺育 — 操作系统安装配置管理、PXE 端点、ISO 下载
- 第三阶段（已完成）：分享 — WebDAV 服务器、基础认证、HTTPS 支持

## 系统架构

MiBeeHive 是一个单体 Go 二进制文件，运行在资源受限的 ARM64 NAS/存储设备（469MB RAM）上，充当**外部服务器的供应中枢**：它爬取、下载并对外提供运维工具（GitHub、Go、HashiCorp、Grafana、NPM、PyPI），让外部服务器能从它这里取料。它通过 `go:embed` 嵌入一个 **Preact + HTM** SPA 前端，并包含一个 Web 管理面板，具有仪表板概览和标签式导航，用于管理所有三个模块以及容器、搜索、日志、任务和备份。前进方向是**供应层**（见[供应层路线图](../roadmap/supply-layer_zh.md)）：将采集到的制品按标准协议对外暴露给外部服务器。

### 定位边界

- **是：** 一条运维工具**供应链**——采集、更新并按**已有**标准协议把运维工具供应给外部服务器。它实现协议，不发明协议。
- **不是**本机应用商店 / 快速建站（那是一般运维面板的职责）。
- **不是** TSDB / 指标聚合器。`/metrics` 仅用于 MiBeeHive 自身健康——MiBeeHive 把 `node_exporter`/`prometheus` **供应给**外部服务器，而不是与它们竞争。
- **运维模型：** 供应优先（被动地按协议供应制品）。对外部服务器的主动远程控制（SSH/agent）是远期方向，叠加在稳定的供应层之上。

### 架构概览
```
┌─────────────────────────────────────────────────────────────┐
│                    MiBeeHive 应用程序                         │
├─────────────────────────────────────────────────────────────┤
│  Go 后端 (cmd/mibeehive)                                  │
│  ├── HTTP 处理器 (internal/handler/)                      │
│  ├── 业务逻辑 (internal/service/)                         │
│  ├── 数据层 (internal/db/)                                │
│  ├── 配置 (internal/config/)                              │
│  ├── 中间件 (internal/middleware/)                        │
│  ├── Docker 客户端 (internal/docker/)                    │
│  ├── 监控器 (internal/monitor/)                          │
│  └── WebDAV (internal/webdav/)                            │
│                                                             │
│  嵌入式前端 (web/)                                         │
│  ├── HTML/CSS (CSS 变量、响应式)                          │
│  └── JavaScript 模块 (31 个文件、3 层架构)                 │
│                                                             │
│  SQLite 数据库                                             │
│  └── 14 个嵌入式迁移文件                                   │
└─────────────────────────────────────────────────────────────┘
```

## 前端模块结构

前端是一个 **Preact + HTM** SPA，按 3 层架构组织成 38 个模块。详细文档请参见 `web/js/AGENTS.md`。

### 核心层 (web/js/core/)
- `api.js` - HTTP 客户端包装器（fetch + JWT 头注入）
- `auth.js` - 登录/登出、令牌管理、localStorage
- `components.js` - 可重用 UI 组件（toast、modal、table、tabs、skeletonCard）
- `drawer.js` - 滑出式抽屉面板
- `helpers.js` - 工具函数（formatDate、formatSize、debounce 等）
- `router.js` - 基于 SPA 的哈希路由，带有路由守卫
- `search.js` - 全局搜索功能
- `state.js` - 全局 App 单例，具有事件总线和定时器管理
- `preact.js` - **Preact 桥接**，提供 h、html、render、Component、Fragment + 所有 hooks
- `i18n.js` - i18n 系统（zh/en），带有 `t('key')` 函数和 `{count}` 插值

### 布局层 (web/js/layout/)
- `sidebar.js` - 桌面侧边栏导航（带有六边形品牌图标）
- `shell.js` - 应用外壳（渲染侧边栏 + 主内容区域）
- `bottom-tab.js` - 移动端底部标签导航

### 模块层 (web/js/modules/)
- `dashboard.js` - 聚合仪表板，包含欢迎横幅、状态卡片、图表、活动时间线、快速操作（727 行）
- `files.js` - 文件标签容器（子模块路由）
- `files-crawl.js` - 爬取控制子模块
- `files-projects.js` - 项目管理子模块
- `files-queue.js` - 下载队列子模块
- `deploy.js` - 部署标签容器
- `deploy-configs.js` - 操作系统安装配置管理
- `deploy-iso.js` - ISO 目录 + 下载管理
- `share.js` - 分享标签容器（WebDAV）
- `share-files.js` - WebDAV 文件浏览器
- `settings.js` - 设置（密码、主题、语言、磁盘阈值）
- `login.js` - 登录页面
- `containers.js` - 容器列表和管理
- `containers-detail.js` - 容器详情视图（日志、统计、环境变量）
- `containers-images.js` - Docker 镜像管理
- `containers-templates.js` - 应用模板
- `logs.js` - 系统日志查看器
- `search.js` - 搜索结果页面
- `tasks.js` - 后台任务查看器

## 后端架构

### 层结构
```
HTTP 请求 → 处理器 → 服务 → 仓库 → 数据库
```

### 处理器层 (internal/handler/)
- `auth.go` - 认证端点（登录、JWT 验证）
- `admin.go` - 管理面板端点（项目、令牌、爬取、安全、操作系统安装、WebDAV、监控配置）
- `backup.go` - 备份列表和恢复
- `container.go` - 容器 CRUD、启动/停止/重启、日志、统计
- `crawl.go` - 爬取管理（状态、触发、日志）
- `dashboard.go` - 聚合仪表板摘要（所有模块的单个 API）
- `file.go` - 文件操作（下载、搜索、队列）
- `iso.go` - ISO 下载管理、目录 CRUD、队列
- `logs.go` - 系统日志端点
- `os_install.go` - 操作系统安装配置、PXE 服务、配置预览
- `project.go` - 项目 CRUD 操作
- `search.go` - 全文搜索端点
- `system.go` - 系统信息和统计
- `tasks.go` - 后台任务端点
- `app_template.go` - 应用模板管理
- `stats.go` - 系统统计获取（抓取 node_exporter）

### 服务层 (internal/service/)
- `file_service.go` - 文件下载，带有重试和完整性检查
- `os_template.go` - 操作系统模板生成（preseed/kickstart/autoinstall）
- `iso_downloader.go` - ISO 下载，带有流式传输和磁盘检查
- `iso_catalog_service.go` - ISO 目录队列处理器，带有后台协程
- `container_service.go` - Docker 容器生命周期管理
- `search_service.go` - 文件和配置的全文搜索
- `log_service.go` - 日志聚合和查询
- `task_service.go` - 后台任务管理
- `app_template_service.go` - 应用模板处理
- `image_service.go` - Docker 镜像拉取/删除操作

### 仓库层 (internal/db/repo_*.go)
- `repo_project.go` - 项目数据访问
- `repo_credential.go` - API 令牌管理
- `repo_file.go` - 文件元数据和队列操作
- `repo_os_install_config.go` - 操作系统安装配置
- `repo_iso_catalog.go` - ISO 目录队列管理
- `repo_container.go` - 容器配置存储
- `repo_crawl_log.go` - 爬取日志存储和查询

## 三大模块概览

### 1. 采蜜（二进制发行版管理）
**用途**：从公共源爬取和下载二进制发行版
**存储**：`{base_path}/oss/`
**功能**：
- GitHub 发行版
- Go 二进制下载
- HashiCorp 产品发行版
- Grafana 发行版
- NPM 包下载
- PyPI 包下载
- 源管理的 Web UI
- API 令牌认证
- 下载调度和重试逻辑

### 2. 哺育（操作系统安装）
**用途**：提供无人值守的操作系统安装配置
**存储**：`{base_path}/os-install/`
**功能**：
- PXE 配置服务
- 操作系统模板生成（preseed/kickstart/autoinstall）
- ISO 下载管理
- ISO 目录自动发现和队列管理
- 配置管理的 Web UI
- PXE 客户端的公共端点
- 配置预览功能
- ISO 下载的后台队列处理器

### 3. 分享（WebDAV 文件共享）
**用途**：用于文件共享的基本 WebDAV 功能
**存储**：`{base_path}/webdav/`
**功能**：
- WebDAV 文件服务
- 基础认证（匿名只读 + 管理员读写）
- 支持 HTTPS 和自签名证书
- 文件列表和管理
- 可通过 Web UI 配置

## 仪表板架构

仪表板通过单个 API 端点提供所有模块的聚合概览。

### 后端
- **端点**：`GET /api/v1/admin/dashboard/summary`（需要 JWT）
- **处理器**：`handler/dashboard.go` 中的 `DashboardHandler.Summary()`
- **响应**：`DashboardSummaryResponse`，包含：
  - `SystemModuleStats` — 版本、运行时间、CPU、内存、磁盘使用情况
  - `FilesModuleStats` — 项目数量、文件数量、队列统计
  - `DeployModuleStats` — 配置数量、ISO 数量/已下载/待处理
  - `SharedModuleStats` — WebDAV 文件数量和总大小
  - `[]ActivityEvent` — 最近的爬取活动和项目名称

### 前端
- 初始化时单个 API 调用，然后每 30 秒使用增量 DOM 更新进行轮询
- 每 10 秒单独轮询实时系统统计（CPU/内存/网络图表）
- 部分：欢迎横幅、状态卡片网格、合并的 CPU/内存图表、带阈值线的磁盘仪表板、活动时间线、快速操作栏、爬取活动图表、队列部分

### 监控配置
- **端点**：`GET/PUT /api/v1/admin/config/monitor`（需要 JWT）
- **处理器**：`AdminHandler.GetMonitorConfig()` / `UpdateMonitorConfig()`
- **用途**：磁盘警告/关键阈值配置（在 config.yaml 中持久化）

## 数据流

### 仪表板流
```
仪表板 UI → 单个 /admin/dashboard/summary → DashboardHandler → 多个仓库 → 聚合响应
```

### 爬取和下载流
```
用户请求 → 管理 UI → 爬取触发 → 爬取器 → 下载服务 → 文件存储
```

### 文件访问流
```
客户端请求 → 文件搜索 → 文件服务 → 下载流 → 客户端
```

### WebDAV 流
```
WebDAV 客户端 → 基础认证 → 文件系统 → 文件操作
```

### 操作系统安装流
```
PXE 客户端 → 公共端点 → 配置生成 → 引导文件 → 安装
```

## 关键设计原则

- **单体架构**：单个 Go 二进制文件，便于部署
- **嵌入式前端**：无需单独的 Web 服务器
- **SQLite 数据库**：轻量级、基于文件的存储（纯 Go 驱动程序）
- **Preact + HTM**：无框架，最小依赖（约 950KB 总量）
- **仅使用标准库**：无外部 Web 框架或 cron 库
- **资源高效**：针对 469MB ARM64 设备优化
- **模块化设计**：三个功能模块之间的清晰分离
- **队列处理**：用于下载队列管理的后台协程
- **增量 DOM 更新**：定期刷新使用目标 DOM 补丁，从不使用 innerHTML
- **单一仪表板 API**：单个聚合端点减少仪表板上的请求数量