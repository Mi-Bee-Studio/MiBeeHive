# MiBeeHive

[English](README.md) · [文档](docs/zh/architecture.md) · [API](docs/zh/api-reference.md)

> **一个轻量、自托管的运维工具供应中枢,喂饱你的整个服务器集群。**
> MiBeeHive 自动采集你的服务器所需的二进制、安装包和 ISO,再按它们本来就会说的协议对外供应:`apt`、`pip`、WebDAV。单个静态 Go 二进制 + 内嵌界面,Linux 哪里都能跑(amd64 / arm64 —— 迷你主机、NAS、虚拟机、旧笔记本都行)。盒子它们自己跑;集群由 MiBeeHive 喂饱。

<p>
  <a href="https://github.com/Mi-Bee-Studio/MiBeeHive/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Mi-Bee-Studio/MiBeeHive/actions/workflows/ci.yml/badge.svg"></a>
  <a href="https://goreportcard.com/report/github.com/Mi-Bee-Studio/MiBeeHive"><img alt="Go Report Card" src="https://goreportcard.com/badge/github.com/Mi-Bee-Studio/MiBeeHive"></a>
  <a href="https://github.com/Mi-Bee-Studio/MiBeeHive/releases"><img alt="Release" src="https://img.shields.io/github/v/release/Mi-Bee-Studio/MiBeeHive?color=blue&include_prereleases"></a>
  <a href="LICENSE"><img alt="License" src="https://img.shields.io/github/license/Mi-Bee-Studio/MiBeeHive?color=AGPL--3.0"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white">
  <img alt="Preact" src="https://img.shields.io/badge/UI-Preact+HTM-673AB7?logo=preact&logoColor=white">
  <img alt="SQLite" src="https://img.shields.io/badge/DB-SQLite-003B57?logo=sqlite&logoColor=white">
  <a href="https://github.com/Mi-Bee-Studio/MiBeeHive/stargazers"><img alt="Stars" src="https://img.shields.io/github/stars/Mi-Bee-Studio/MiBeeHive?style=social"></a>
</p>

---

## 为什么用它

- **轻量、多架构。** 单个静态 Go 二进制,纯标准库 HTTP,内嵌 SPA —— amd64 或 arm64 Linux 都能跑,从 469MB 内存的 NAS 到高性能服务器都行。纯 Go 的 SQLite 驱动,无 CGO,无外部依赖。
- **对集群原生供应。** 你的服务器用自己现成的工具拉取(`apt`、`pip`、WebDAV),无需安装任何 agent 或客户端。
- **采集 → 存储 → 供应。** 抓取源自动保持最新;对外供应的产物永远是当前版本。

## 工作原理

```
   ┌──────────────┐   爬取 + 下载          ┌──────────────┐   按原生协议对外供应
   │ GitHub / Go / │ ───────────────────▶ │  MiBeeHive   │ ─────────────────────────────▶  你的服务器
   │ PyPI / NPM /  │   自动、按计划         │  (任意Linux) │   apt  ·  pip  ·  WebDAV  ·  PXE
   │ HashiCorp …   │                      │              │
   └──────────────┘                       └──────────────┘
```

## 功能特性

**Foraging 采蜜 —— 供应引擎**
- 从 GitHub、Go、HashiCorp、Grafana、NPM、PyPI、Crates 爬取二进制发布包
- 可插拔的双轨源模型:单页源用 YAML 指纹,有状态协议用 Go 适配器
- 带退避的重试、单次爬取超时、独立的 `network_error` / `rate_limited` 状态
- Web 界面管理源、API 令牌与调度

**Supply 供应 —— 原生协议端点** *(无需安装客户端)*
- **APT 仓库**:基于采集到的 `.deb` → `apt update && apt install <pkg>`
- **PyPI Simple**(PEP 503):基于采集到的 wheel → `pip install --index-url …/simple/ <pkg>`
- 通用 `/repo/index` + `/repo/files/{id}`,兜底其他产物

**Provisioning 哺育 —— 纳入新服务器**
- 通过 PXE 无人值守装机(preseed / kickstart / autoinstall)
- 操作系统模板生成、ISO 目录与下载队列

**Sharing 分享**
- WebDAV + Basic Auth(匿名只读、管理员读写),自签名 HTTPS

**Ops 运维**
- 仪表板:CPU / 内存 / 磁盘、活动时间线、日志、任务
- Docker 容器管理 + 多仓库同步与标签保留策略
- 备份 / 恢复、全局搜索、中英双语

## 快速开始

```bash
# 构建(单二进制,无外部依赖)
go build -o mibeehive ./cmd/mibeehive

# 配置并运行
cp configs/config.yaml config.yaml   # 编辑存储路径、端口、密钥
./mibeehive                           # 界面:http://localhost:9090  ·  admin / admin
```

或交叉编译到其他架构(例如 arm64)——MiBeeHive 可编译到任意 Linux/Go 支持的架构:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

> 可跑在任意 Linux + systemd 主机上(amd64 或 arm64)。轻量:从约 1GB 内存 / 32GB 存储起即可舒适运行。详见[部署指南](docs/zh/deployment.md)。release 标签会发布预构建的多架构 Docker 镜像(`linux/amd64`、`linux/arm64`)。

## 供应端点(直接复制给你的集群用)

| 协议 | 客户端命令 |
|---|---|
| APT | `echo "deb http://<host>:9090/apt stable main" \| tee /etc/apt/sources.list.d/mibeehive.list` |
| PyPI | `pip install --index-url http://<host>:9090/simple/ <pkg>` |
| WebDAV | `http://<host>:9090/webdav/` |
| 通用 | `GET /repo/index`(JSON 清单)· `GET /repo/files/{id}`(下载) |

## 文档

- [产品介绍](docs/zh/introduction.md) —— MiBeeHive 是什么、给谁用
- [快速开始](docs/zh/quick-start.md) —— 构建、启动、第一次采集与消费
- [架构](docs/zh/architecture.md) —— 模块、分层、供应协议
- [部署](docs/zh/deployment.md) —— Linux 安装(amd64/arm64)、systemd、健康检查
- [配置参考](docs/zh/configuration.md) —— `config.yaml` 全部选项
- [API 参考](docs/zh/api-reference.md) —— 全部 HTTP 端点
- [开发与贡献](docs/zh/development.md) —— 构建、测试与提交约定
- [更新日志](docs/zh/changelog.md) —— 版本发布历史
- [供应层](docs/roadmap/supply-layer_zh.md) —— 协议路线图(APT ✅、PyPI ✅,更多规划中)

## 范围

MiBeeHive 是一条**供应链**:采集运维工具,按*已有*的标准协议对外供应。它**不是**本机应用商店、建站工具或时序库 —— `/metrics` 仅用于自身健康;它把 `node_exporter`/`prometheus` *供应给*你的集群,而不是与之竞争。

## 贡献与许可

- [贡献指南](CONTRIBUTING_ZH.md) · Conventional Commits · 双语文档(中 / en)
- AGPL-3.0 —— 见 [LICENSE](LICENSE)

---

由 [Mi-Bee Studio](https://github.com/Mi-Bee-Studio) 用 ❤️ 打造。
