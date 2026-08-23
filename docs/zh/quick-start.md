# MiBeeHive 快速开始

十分钟内让 MiBeeHive 在一台 Linux 主机上跑起来：构建、启动、完成第一次采集，然后从另一台机器上用 `apt` / `pip` 取走它。

## 前置条件

- 一台 Linux 主机（amd64 或 arm64；NAS、迷你主机、虚拟机、旧笔记本均可）
- **构建机**上安装 Go 1.26+ 与 Git（构建机可以是这台主机本身，也可以是你的开发机——交叉编译只需一条命令）
- 约 500MB 可用内存与数 GB 磁盘（取决于你要采集多少制品）

## 1. 构建二进制

```bash
git clone https://github.com/Mi-Bee-Studio/MiBeeHive.git
cd MiBeeHive

# 在 Linux 主机上直接构建
go build -o mibeehive ./cmd/mibeehive
```

在别的机器上交叉编译（例如开发机是 Windows/macOS，目标是 ARM64 NAS）：

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive ./cmd/mibeehive
# 目标是 amd64 则 GOARCH=amd64
```

> MiBeeHive 是纯 Go（`CGO_ENABLED=0`），交叉编译无任何工具链依赖。注意它面向 Linux：由于使用了 `syscall.Statfs`，无法构建 Windows 目标；在 Windows 上开发时请交叉编译到 Linux 或在 WSL 中运行。

## 2. 启动

```bash
./mibeehive
```

默认读取 `./configs/config.yaml`（可用 `-config` 指定其他路径）。首次启动会：

- 自动执行 SQLite 数据库迁移（`modernc.org/sqlite` 纯 Go 驱动，无需安装任何东西）；
- `jwt_secret` 为空时自动生成；
- 按配置中的 `projects` 种子清单初始化采集项目。

然后打开 **http://localhost:9090**，使用默认账号登录：

```text
用户名: admin
密码:   admin
```

> **请立即修改密码**：登录后进入「设置」修改，或 `PUT /api/v1/admin/password`。使用默认密码时界面会持续提醒。

## 3. 第一次采集

1. 在 Web 界面进入**采蜜**模块的项目管理，确认种子项目（Prometheus、node_exporter、Consul、Grafana 等）已启用；需要 GitHub 令牌的源在「API 令牌」中配置。
2. 点击**触发爬取**（单个项目或全部）。爬取器会抓取每个源的发行版清单，把新版本排入下载队列。
3. 在下载队列页观察进度；完成后文件落在 `{storage.base_path}/oss/`（默认 `./data/oss/`）。

采集是持续性的：每个项目按 `crawl_interval`（默认 6 小时）自动刷新，下载队列带重试与完整性校验。

## 4. 从另一台机器消费

这是 MiBeeHive 的核心场景——外部服务器用它**原生**的工具取料，无需安装任何客户端。假设 MiBeeHive 运行在 `192.168.1.10:9090`：

**Debian / Ubuntu 主机（APT）：**

```bash
echo "deb http://192.168.1.10:9090/apt stable main" | sudo tee /etc/apt/sources.list.d/mibeehive.list
sudo apt update
sudo apt install <已采集的某个包>
```

**Python 主机（PyPI Simple，PEP 503）：**

```bash
pip install --index-url http://192.168.1.10:9090/simple/ <已采集的某个包>
# 或 uv pip install --index-url http://192.168.1.10:9090/simple/ <pkg>
```

**任意主机（通用清单 / 直链）：**

```bash
curl -s http://192.168.1.10:9090/repo/index          # 全部可供应文件的 JSON 清单
curl -O http://192.168.1.10:9090/repo/files/42       # 按 id 直链下载
```

供应端点公开、无需认证——这正是外部主机可以无人值守拉取的原因。想先看看有没有货，直接在浏览器打开 `http://192.168.1.10:9090/repo/index` 即可。

## 5. 顺手验证

```bash
curl -s http://localhost:9090/health    # -> OK
curl -s http://localhost:9090/metrics   # Prometheus 指标（MiBeeHive 自身健康）
```

## 下一步

- [配置参考](configuration.md)——端口、存储路径、爬取节奏、备份等全部选项
- [部署指南](deployment.md)——systemd 服务、生产目录布局、HTTPS、备份恢复
- [架构文档](architecture.md)——分层结构与数据流
- [API 参考](api-reference.md)——全部 HTTP 端点
