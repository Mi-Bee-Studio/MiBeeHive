# MiBeeHive 更新日志

本日志遵循 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/) 的精神记录值得注意的变更。版本发布以 Git 标签 `v*` 为准。

## [未发布]

### 新增

- **供应层**：通用制品仓库——`GET /repo/index`（JSON 清单）与 `GET /repo/files/{id}`（直链下载），公开无需认证（#3）。
- **APT 仓库**：基于采集到的 `.deb` 按需生成 `Packages[.gz]` 与 `Release`；供应页提供客户端 apt 源配置片段（#9、#10）。
- **PyPI Simple 仓库**：`/simple/` 端点实现 PEP 503，项目名归一化、文件链接带 `#sha256=` 校验片段（#24）。
- **双轨采集引擎**：`Source`/`Fetcher` 抽象 + 嵌入式 YAML 指纹（单页源声明式接入），有状态协议保留 Go 适配器；Registry 接入 CrawlManager 并注册供应端点；`source_type` 约束扩展（#2、#5、#6）。指纹随后支持从数据库加载。
- API 令牌自动注入指纹请求；Grafana 源迁移到指纹模型并支持单对象 JSON（#7、#8）。
- **UX / WebDAV 重设计**：虚拟索引（频道/视图/节点）、跨项目文件中心、供应链总览首页与分组导航（#53）。
- 虚拟索引变更审计日志（#58）。
- **可配置存储路径**：`storage.modules` 按模块覆盖 + 后台迁移任务 + 设置界面展示任务进度。
- ISO 目录 v2：两级抓取（发行版 → 版本 → 文件）、版本感知排序、发行版档案；前端模板选择器。
- 分享链接：创建/列出/吊销 + 公开 `/s/{token}` 令牌下载。
- 缓存指标端点 `GET /api/v1/admin/metrics/cache`。
- 爬取健壮性：单源抓取超时、瞬时错误指数退避重试、`network_error`/`rate_limited`/`error` 分类状态。
- 哺育模块增强：ISO 下载到本地按钮、搜索、徽章、移动端响应式布局。

### 变更

- 文档与 README 重新定位：轻量**多架构 Linux** 供应中枢（amd64/arm64），不再限定 ARM64 NAS。
- 安全加固：API 响应与前端不再暴露物理路径（#59）。
- 前端架构整理：启用 Preact Provider、作用域定时器、路由取消；抽取共享 moduleTabs 组件（批次 A–D）。

### 修复

- APT：支持 xz 压缩的 control.tar（现代 .deb 默认）（#12）；zstd 压缩 deb、Release 签名按 mtime 缓存、按文件缓存元数据。
- WebDAV：连接 URL 按请求 Host 生成（不再硬编码 localhost）；手工上传项目与目录的幂等播种；VirtualFS 基础存储路径修正。
- 配置：`password_hash` 为空时回退默认哈希（#15）；`password_changed_at` 不再默认取启动时间（#25）。
- 总览页不再拉取 3MB 的 `/repo/index`；登录后立即跳转总览；字段映射修正。
- i18n：状态标签统一、双重转义修复、toast 反馈恢复；过时的定位文案替换。
- ISO：抓取器重试与回退、Alpine 404、瞬时错误重置；自动下载使用 `resp.FoundURL`。
- 数据库：迁移 024 注册修正；恢复 018_iso_catalog_v2 并将 storage_paths 顺延为 019。
- 处理器测试死锁修复（db.Close() 前优雅关闭事件总线）。

### 性能

- 组合索引 + 仓库层索引缓存（#57）。
- 未过滤 `COUNT(*)` 以 30 秒 TTL 缓存（#57）。
- 为未过滤排序新增独立 `created_at` 索引（#57）。

## [v0.1.0] — 2026-05-26

首个公开发布。

- 四大模块骨架：采蜜（GitHub / Go / HashiCorp / Grafana 源、下载队列、项目与令牌管理）、哺育（OS 安装配置、preseed/kickstart/autoinstall 模板、PXE 端点、ISO 目录与下载队列）、分享（WebDAV + Basic Auth + 自签名 HTTPS）、容器（本地 Docker 生命周期、镜像、应用模板）。
- 聚合仪表板（单一 summary API）、系统状态（CPU/内存/磁盘历史）、日志与任务中心、全文搜索、备份与恢复。
- 认证：JWT 登录/刷新、默认密码检测与修改。
- 单个静态 Go 二进制（纯 Go SQLite 驱动，`CGO_ENABLED=0`）+ 内嵌 Preact + HTM 管理界面；嵌入式 SQL 迁移。
