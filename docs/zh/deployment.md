# MiBeeHive 部署指南

[English](../en/deployment.md)

## 目标设备：ARM64 NAS 设备

### 规格说明
- **SSH**: `ssh user@device-ip`
- **操作系统**: Linux (Debian/Ubuntu/Armbian), kernel 6.0+, aarch64
- **硬件**: ARM64 设备，≥1GB 内存，≥32GB 存储
- **设备上无 Go 工具链** — 在本地交叉编译，通过 SCP 传输二进制文件

## 构建命令

### 本地开发构建
```bash
go build -o mibeehive ./cmd/mibeehive              # 为当前架构构建
```

### ARM64 交叉编译
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive  # 为 ARM64 交叉编译
```

### 构建迁移工具
```bash
go build -o migrate ./cmd/migrate                   # 构建迁移工具
```

### 测试
```bash
go test ./...                                       # 运行所有测试
go test -v ./internal/crawler                       # 运行特定包测试
go vet ./...                                        # 静态分析
```

## 设备上的部署布局

```
/opt/mibeehive/
├── bin/mibeehive
├── config.yaml
├── mibeehive.db
├── mibeehive.log
├── backup-*.tar.gz
└── backups/

/var/lib/mibeehive/
├── oss/            # 采蜜：下载的二进制文件（.deb、wheel、tarball 等）
├── os-install/     # 哺育：操作系统安装文件
└── webdav/         # 分享：WebDAV 共享文件
```

> **供应层没有独立的存储目录。** APT（`/apt/`）和 PyPI Simple（`/simple/`）按需在采蜜收集到 `oss/` 的制品之上生成各协议索引，因此同一批采集到的文件既作为通用仓库供应，也通过原生协议对外供应。

## 部署与重启

### 1. 在本地交叉编译（在开发机器上）
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

### 2. 上传到设备
```bash
scp mibeehive-arm64 user@device-ip:/opt/mibeehive/bin/mibeehive
```

### 3. 通过 systemd 重启
```bash
ssh user@device-ip "pkill mibeehive; sleep 1 && sudo systemctl restart mibeehive"
```

## Systemd 服务

### 服务文件
服务文件位于 `configs/mibeehive.service`。在设备上安装并启用：

```bash
sudo systemctl start mibeehive    # 启动
sudo systemctl stop mibeehive     # 停止
sudo systemctl restart mibeehive  # 重启
sudo systemctl status mibeehive   # 状态
journalctl -u mibeehive -f       # 跟踪日志
```

### Systemd 配置
服务针对 ARM64 设备配置了内存限制：
- `GOMEMLIMIT=256MiB` - Go 运行时内存限制
- 失败时自动重启
- 日志记录到 journal

## 验证（在设备上）

### 服务状态
```bash
sudo systemctl status mibeehive
```

### 健康检查
```bash
curl -s http://localhost:9090/ | head -5              # 健康检查（Web 界面）
curl -s -X PROPFIND http://localhost:9090/webdav/     # WebDAV 检查
curl -sk https://localhost:9443/ | head -5            # HTTPS 检查
# 供应层（公开、无需认证）——外部服务器从这些端点消费：
curl -s http://localhost:9090/repo/index | head -c 120        # 通用文件清单（JSON）
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9090/apt/dists/stable/Release   # APT 仓库（200 = 正常）
curl -s http://localhost:9090/simple/ | head -c 120           # PyPI Simple 索引（PEP 503）
```

### 日志监控
```bash
journalctl -u mibeehive -f                           # 跟踪日志
tail -f /var/log/mibeehive/mibeehive.log           # 应用程序日志
```

## 配置

### 生产配置文件
生产配置 `/etc/mibeehive/config.yaml` 与 `configs/config.yaml` 不同：

```yaml
storage:
  base_path: /var/lib/mibeehive/     # 注意：包含 oss 子目录
database:
  path: /opt/mibeehive/mibeehive.db  # SQLite 数据库路径
server:
  port: 9090
  https_port: 9443
auth:
  jwt_secret: your-jwt-secret-here
  password_hash: your-password-hash-here
```

### 配置管理
- 启动时如果文件缺失会自动生成默认配置
- 仅支持 YAML 格式
- 环境特定配置存储在 YAML 中
- 数据库将项目配置与基础设施配置分开存储

## 内存管理

### 目标设备限制
- **总内存**: ≥1GB（推荐 ≥2GB）
- **Go 内存限制**: 256MB（通过 `GOMEMLIMIT`）
- **应用程序可用**: ~213MB
- 针对低内存优化的使用方式：
  - 单个 SQLite 连接（`db.SetMaxOpenConns(1)`）
  - 流式下载（不缓冲整个文件）
  - 高效日志记录（`log/slog` 结构化输出）

## 网络配置

### 端口
- **HTTP**: 9090（主要 Web 界面）
- **HTTPS**: 9443（WebDAV 和管理面板）
- **PXE**: 9090（公共端点，无需认证）

### 防火墙考虑
- 确保端口 9090 和 9443 可访问
- PXE 端点必须公开可访问（无需认证）
- 管理端点需要 JWT 认证

## 备份与恢复

### 备份策略
- 数据库：SQLite 文件（`mibeehive.db`）
- 配置：`/etc/mibeehive/config.yaml`
- 下载的文件：自动备份到 `backup-*.tar.gz`
- Systemd 服务状态：由 systemctl 处理

### 恢复步骤
1. 停止服务：`sudo systemctl stop mibeehive`
2. 备份现有文件
3. 从备份恢复
4. 启动服务：`sudo systemctl start mibeehive`

## 监控与维护

### 日志轮转
通过设备上的 crontab 配置：
- 每周日凌晨 01:00：清理 30 天以上的日志文件在 `/var/log/mibeehive/`
- 每月 1 日 09:00：通过 `/opt/mibeehive/bin/generate-report.sh` 生成下载报告

### 性能监控
- 监控内存使用：`ps aux | grep mibeehive`
- 检查磁盘空间：`df -h`
- 检查应用程序日志中的错误和警告

### 常见问题
1. **内存问题**：监控 `GOMEMLIMIT` 使用情况，检查内存泄漏
2. **磁盘空间**：监控存储路径，特别是下载目录
3. **网络连接**：确保设备有互联网连接用于爬取
4. **数据库损坏**：使用 SQLite 完整性检查