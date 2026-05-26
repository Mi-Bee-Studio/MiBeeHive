# Roadmap - 已完成方向

本文档汇总所有已完成的开发方向。

---

## Direction 1: UI Enhancement（用户界面增强）— ✅ 已完成

**目标**：全面提升 MiBeeHive 管理后台的用户交互体验。

### 已实现功能

**File Drawer（文件抽屉）** — 三标签页设计（Files / ISO / OS Configs），树形目录浏览与搜索过滤，文件预览、快速复制路径、一键下载。

**Log Viewer（日志查看器）** — 多视图日志查看（Crawl Logs / App Logs / Download Logs），日志级别过滤与时间范围选择。前端：`web/js/modules/logs.js`

**Task Center（任务中心）** — 统一任务状态视图（调度 / 下载 / 备份 / ISO），任务操作（暂停 / 恢复 / 取消）。前端：`web/js/modules/tasks.js`

**Global Search（全局搜索）** — 跨模块统一搜索（Projects / Files / Configs / ISOs），键盘快捷键 Ctrl+K。前端：`web/js/core/search.js`

---

## Direction 3: Test Coverage Enhancement（测试覆盖增强）— ✅ 已完成

**目标**：系统性提升测试覆盖率，建立测试规范。

### 已实现成果

- 从 17 个测试文件增长至 **75 个测试文件**，覆盖 14 个包
- **Handler 层**：HTTP 测试覆盖所有 API 端点
- **Service 层**：业务逻辑测试（文件下载、ISO、容器、同步/保留策略）
- **DB 层**：所有 Repo 的 CRUD 测试，使用内存 SQLite
- **Crawler 层**：Mock HTTP 服务器测试各 Crawler 实现
- **Middleware 层**：JWT auth、Basic Auth、TLS、安全头、限流
- **Registry 层**：V2 协议客户端测试（auth、blobs、manifests、sync）
- 所有测试使用 `modernc.org/sqlite` 内存数据库，不依赖外部环境

---

## Direction 4: Crawler & Template Expansion（爬虫与模板扩展）— ✅ 已完成

**目标**：扩展 Foraging 模块新增包管理器支持，扩展 Provisioning 模块支持更多发行版，引入 Container Management。

### 已实现功能

**新增 Crawler** — NPM Crawler（npmjs.com，支持 scoped packages），PyPI Crawler（wheels/tarballs，平台过滤），Rust Crates Crawler（crates.io binary crate）。

**OS 安装模板** — Rocky Linux（Kickstart），AlmaLinux（Kickstart），Fedora Server（Kickstart），openSUSE（AutoYAST）。

**Container Management（容器管理）** — 容器生命周期（创建/启动/停止/重启/删除），镜像管理（拉取/列表/删除），容器日志与资源监控，应用模板一键部署，Registry 管理（多仓库、V2 协议），跨仓库镜像同步，Tag 保留策略与自动清理。

---

## Direction 5: Observability Enhancement（可观测性增强）— ✅ 已完成

**目标**：建立完善的可观测性体系，快速定位问题。

### 已实现功能

**健康检查** — `GET /health` 返回各组件状态（DB、存储、WebDAV）及运行时长。实现：`internal/health/health.go`

**Prometheus 指标** — `GET /metrics` 输出 Prometheus 文本格式，纯手工实现无外部依赖。实现：`internal/metrics/metrics.go`

**结构化日志** — `log/slog` 标准库，关键路径耗时日志，JSON 格式输出支持。

---

## Direction 6: Backup & Recovery System（备份与恢复系统）— ✅ 已完成

**目标**：建立自动化备份与恢复机制，确保快速恢复服务。

### 已实现功能

**核心备份** — 全量备份（SQLite + 配置 + 文件索引），定时自动备份，保留策略与自动清理，一键恢复。

**远程备份** — SCP 远程备份传输，可配置远程目标。

**Web UI** — 备份列表（时间、大小、状态），手动触发，恢复操作（二次确认）。实现：`internal/backup/`、`internal/handler/backup.go`
