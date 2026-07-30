# MiBeeHive

[English](README.md)

**面向外部服务器的运维工具供应链平台**。MiBeeHive 运行在资源受限的 ARM64 设备（NAS 或存储服务器）上，充当供应中枢：它**自动采集并持续更新**外部服务器集群所需的运维工具，并**按标准协议对外供应**（软件仓库、ISO 仓库、镜像仓库、包代理源）。

> **与一般运维面板的区别：** 那些面板管理**本机**（应用商店、快速建站、消费级应用）。MiBeeHive 面向**外部**那些服务器——它是一条供应链，负责抓取、更新并把工具递交给外部机器。*它们管这台盒子；MiBeeHive 喂饱整个集群。*

蜂巢隐喻与此完全对应：蜂巢不产花蜜，它**采集、酿造、分发**花蜜。MiBeeHive 不发明协议——它实现已有的标准协议，让现成的运维工具能从它这里取原料。核心模块是两项任何其他运维面板都不做的、自给自足的 provisioning 能力：

![许可证](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)
![Go 版本](https://img.shields.io/badge/go-1.26+-00ADD8.svg?style=flat-square)
![构建状态](https://img.shields.io/badge/build-passing-brightgreen.svg)

## 功能特性

### Foraging (采蜜) — 供应引擎
- 从 GitHub、Go、HashiCorp、Grafana、NPM 和 PyPI 爬取并下载二进制发布包——也就是外部服务器将要消费的运维工具
- Web 界面管理爬取源、API 令牌和调度
- 自动重试和完整性验证
- Web 管理面板用于源配置
- *（路线图）将采集到的制品按标准协议对外供应：软件仓库、Go module proxy、PyPI `/simple`、Helm 仓库、APT/YUM 布局*

### Provisioning (哺育) — 纳入新的外部服务器
- 通过 PXE 提供无人值守的操作系统安装配置，让裸机外部服务器能从零装机并被纳入供应
- 操作系统模板生成（preseed/kickstart/autoinstall）
- ISO 下载管理，支持流式传输和自动发现
- 公共 PXE 端点用于网络启动安装

### Sharing (分享) — 对外提供文件
- WebDAV 文件共享，支持基本认证——把采集到的工具/文件暴露给外部服务器
- 匿名只读 + 管理员读写访问
- HTTPS 支持，使用自签名证书
- 可通过 Web 界面配置

### Dashboard（仪表板）
- 所有模块的聚合概览（系统、文件、部署、共享）
- 实时 CPU、内存和磁盘监控图表
- 最近爬取和下载事件的时间线
- 常用操作的快速操作栏
- 可配置的磁盘警告/临界阈值

### Containers（容器）
- Docker 容器管理（CRUD、启动/停止/重启、日志、统计）
- 多注册表支持的注册表管理
- 跨注册表镜像同步
- 带自动清理的标签保留策略

## 技术栈

- **后端**: Go 1.26+（最新版），仅使用标准库 HTTP
- **数据库**: SQLite，使用 modernc.org/sqlite 驱动
- **前端**: **Preact + HTM**，TailwindCSS CDN，Chart.js CDN
- **无外部框架**: 不使用 chi/gin/echo，无 npm，无 cron 库
- **嵌入式**: 前端通过 go:embed 嵌入

## 截图

<!-- 在此处添加截图 -->

## 快速开始

### 前置要求
- 已安装 Go 1.26+（最新版）
- ARM64 目标设备，≥1GB 内存，≥32GB 存储（可选，用于部署）

### 构建
```bash
go build -o mibeehive ./cmd/mibeehive
```

### 配置
复制示例配置文件：
```bash
cp configs/config.yaml config.yaml
```

编辑 `config.yaml` 设置：
- 存储基础路径
- 数据库路径
- 服务器端口
- JWT 密码和密码哈希

### 运行
```bash
./mibeehive
```

访问 Web 界面：`http://localhost:9090`

## 默认登录

- **用户名**: admin
- **密码**: admin（**警告**: 首次登录后立即更改）

## 项目结构

```
mibeehive/
├── cmd/
│   ├── mibeehive/       # 主服务器入口点
│   │   ├── main.go      # 100 行入口点
│   │   └── init.go      # 798 行初始化逻辑（loadConfig、initDatabase、initServices、initHandlers、buildRouter、runServers）
│   └── migrate/         # 迁移工具（独立二进制文件）
├── internal/
│   ├── backup/          # 备份创建/恢复逻辑
│   ├── config/          # 配置管理
│   ├── crawler/         # 爬取系统（6 个源 + 调度器）
│   ├── db/              # SQLite 存储库和迁移
│   ├── docker/          # Docker 客户端包装器
│   ├── handler/         # HTTP 处理器
│   ├── health/          # 健康检查端点
│   ├── metrics/         # Prometheus 指标端点
│   ├── middleware/       # 认证、TLS、速率限制、安全头
│   ├── model/           # 领域类型和路由常量
│   ├── monitor/         # 系统资源监控
│   ├── registry/        # V2 Docker/OCI 注册表客户端
│   ├── service/         # 业务逻辑
│   └── webdav/          # WebDAV 处理器
├── web/                  # 嵌入式前端（38 个 JS 模块 + 1 个 CSS）
│   ├── css/              # style.css 与 CSS 变量
│   └── js/               # Preact SPA
│       ├── core/         # 框架：api、auth、cache、components、drawer、helpers、preact、router、search、state、tooltips
│       ├── layout/       # 壳层：sidebar、shell、bottom-tab
│       └── modules/      # 页面：dashboard、files、deploy、share、containers、settings、login、等
├── configs/              # 配置文件 + systemd 服务
└── docs/                 # 文档（架构、部署、API）
```

## 文档

- [架构文档](docs/zh/architecture.md) - 详细的架构文档
- [部署指南](docs/zh/deployment.md) - ARM64 设备部署指南
- [API 参考](docs/zh/api-reference.md) - 完整的 API 文档
- [供应层路线图](docs/roadmap/supply-layer_zh.md) - 运维工具供应层规划

## 管理界面

Web 管理面板默认进入**供应链总览**首页（采集 → 存储 → 供应），并按该主线分组导航：

1. **总览**（首页）- 供应链落地页：已采集内容、存储用量、供应端点（APT 源行、WebDAV）、最近活动、系统资源
2. **采集**
   - **软件源** - 浏览已下载文件，管理爬取项目，监控下载队列
3. **供应**
   - **供应层** - 对外供应的制品（基于已采集 .deb 的 APT 仓库），可复制客户端配置
   - **文件共享** - WebDAV 文件共享和文件浏览器
4. **部署**
   - **系统部署** - OS 安装配置管理，ISO 目录和下载
5. **运维**
   - **系统状态** - 仪表盘（系统统计、模块卡片、活动时间线）、日志、任务
   - **容器** - Docker 容器管理和注册表操作
6. **设置** - 密码更改、主题、语言、磁盘阈值配置

## 目标设备

MiBeeHive 设计用于 ARM64 设备，要求：
- **最低要求**: 1GB 内存，32GB 存储
- **推荐配置**: ≥2GB 内存，≥64GB 存储
- **操作系统**: 支持 systemd 的 Linux
- **部署**: systemd 服务，支持自动重启

## 开发

### 运行测试
```bash
go test ./...
go test -v ./internal/crawler  # 特定包
go vet ./...                    # 静态分析
```

### 交叉编译
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

### 代码结构
- **处理器**: HTTP 请求处理（Go 1.26+ 方法+路径路由）
- **服务层**: 业务逻辑层
- **存储库**: 数据访问层，使用 SQLite
- **前端**: Preact + HTM，使用哈希路由

## 定位边界（MiBeeHive 是什么 / 不是什么）

**是：** 一条运维工具**供应链**——采集、更新并把运维工具按标准协议供应给外部服务器。它实现**已有**协议（让现成工具能从它取料），不发明新协议。

**不是：**
- 不是本机应用商店 / 快速建站（那是一般运维面板的职责）。
- 不是 TSDB / 指标聚合器——`/metrics` 仅用于 MiBeeHive 自身健康；它把 `node_exporter`/`prometheus` **供应给**外部服务器，而不是与它们竞争。

## 贡献

详细的贡献指南请参阅 [CONTRIBUTING.md](./CONTRIBUTING.md)。

## 许可证

本项目采用 AGPL-3.0 许可证 - 详情请参见 [LICENSE](./LICENSE) 文件。

## 支持

对于问题和反馈：
- 在 [GitHub](https://github.com/Mi-Bee-Studio/mibeehive) 上创建 issue
- 查看 [文档](https://github.com/Mi-Bee-Studio/mibeehive/tree/main/docs)

---

由 [Mi-Bee Studio](https://github.com/Mi-Bee-Studio) 用 ❤️ 构建