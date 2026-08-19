# MiBeeHive 配置参考

MiBeeHive 的全部行为由一个 YAML 文件驱动（默认 `./configs/config.yaml`，启动参数 `-config` 可覆盖）。仓库中的 [`configs/config.yaml`](https://github.com/Mi-Bee-Studio/MiBeeHive/blob/main/configs/config.yaml) 是带注释的完整样例。缺失的文件与字段会自动落回内置默认值；`jwt_secret` 为空时首次启动自动生成。

部分设置可在运行时通过管理界面或 API 修改（见文末「运行时可变配置」）；其余改动需要重启进程。

## server — HTTP/TLS 服务

```yaml
server:
  port: 9090              # 主 HTTP 端口（Web 界面、API、供应端点、PXE）
  bind_addr: "0.0.0.0"    # 监听地址
  https_port: 9443        # HTTPS 端口；0 = 禁用（WebDAV 仅在 HTTPS 上提供）
  # cert_path: ""         # 自定义 TLS 证书；留空则首次启动自动生成自签名证书
  # key_path: ""
  tls_ip_addresses: []    # 写入自签名证书的 IP；留空时自动探测网卡 IP（始终包含 127.0.0.1）
  tls_dns_names:          # 写入自签名证书的 DNS 名；留空时使用 ["localhost"]
    - localhost
```

- 公开端点（`/repo/`、`/apt/`、`/simple/`、`/pxe/`、`/health`、`/metrics`）都挂在主端口上，供外部服务器与 PXE 客户端无人值守访问。
- 防火墙上需要放行 `port` 与（若启用）`https_port`。

## database — SQLite

```yaml
database:
  path: "./mibeehive.db"   # SQLite 数据库文件路径
```

迁移在启动时自动执行，无需单独操作（另附 `cmd/migrate` 迁移工具用于手工干预）。写入端使用单连接（`SetMaxOpenConns(1)`）以适配低内存设备。

## storage — 存储布局

```yaml
storage:
  base_path: "./data"      # 所有模块存储的父目录
  # modules:               # 可选：按模块覆盖子路径（空值回退 {base_path}/{module} 约定）
  #   oss: "/mnt/bigdisk/oss"
  #   os_install: ""
  #   iso: ""
```

默认布局：

| 子目录 | 模块 | 内容 |
|--------|------|------|
| `{base_path}/oss/` | 采蜜 | 采集到的二进制发行版（.deb、wheel、tarball…） |
| `{base_path}/os-install/` | 哺育 | OS 安装文件与 ISO |
| `{base_path}/webdav/` | 分享 | WebDAV 共享文件 |

> 供应层没有独立存储：APT / PyPI / 通用仓库索引按需在 `oss/` 中采集到的制品之上生成，同一批文件既走通用下载也走原生协议。

存储路径可在运行时经「设置」修改，改动以**迁移任务**形式在后台搬运文件（见 API 参考中的 storage migrations 端点）。

## auth — 认证

```yaml
auth:
  password_hash: ""        # bcrypt 哈希；空 = 默认密码 admin（登录后立即修改）
  jwt_secret: ""           # JWT 签名密钥；空则首次启动自动生成并回写
  # password_changed_at:   # 由应用维护，勿手工编辑
```

- 管理端点使用 JWT（`Authorization: Bearer <token>`，1 小时有效期，`/api/v1/auth/refresh` 续期）。
- WebDAV 使用 Basic Auth：匿名只读，管理员凭据与管理面板相同。
- 修改密码请走界面或 `PUT /api/v1/admin/password`（会同步更新哈希与时间戳）。

## crawler — 采集

```yaml
crawler:
  max_concurrent: 2              # 并发抓取的源数量
  default_interval: "6h"         # 项目未指定时的默认采集间隔
  fetch_timeout: "60s"           # 单源整个「抓取+重试」的期限；"0" 禁用（仅剩 HTTP 客户端 30s 兜底）
  max_retries: 3                 # 瞬时错误（超时/连接重置/5xx）的最大重试次数；4xx 与限流永不重试
  retry_initial_backoff: "2s"    # 首次重试基础延迟，其后 ×2 指数退避并带抖动
```

错误被分类为 `network_error` / `rate_limited` / `error` 三种状态，便于在队列与日志中区分瞬时故障与真正的上游问题。

## monitor — 系统监控

```yaml
monitor:
  sample_interval: 30        # 系统指标采样间隔（秒）
  retention_days: 7          # 历史保留天数（上限 30）
  # node_exporter_url: ""    # 可选：从 node_exporter 拉取指标的 URL
  disk_warning_percent: 90   # 磁盘使用率警告阈值（%）
  disk_critical_percent: 95  # 磁盘使用率严重阈值（%），达到后进入降级模式
  disk_check_enabled: true   # 是否启用磁盘监控
```

磁盘阈值也可在运行时经「设置」或 `GET/PUT /api/v1/admin/config/monitor` 修改。

## logging — 日志轮转

基于 lumberjack：

```yaml
logging:
  filename: "./mibeehive.log"  # 日志文件路径
  max_size: 10                 # 单文件 MB 上限，超过即轮转
  max_backups: 3               # 保留的旧日志数量
  max_age: 30                  # 旧日志保留天数
  compress: true               # 压缩轮转后的日志
  local_time: true             # 文件名使用本地时间
```

## backup — 自动备份

```yaml
backup:
  enabled: false               # 启用定时备份
  schedule: "03:00"            # 每日备份时刻（HH:MM）
  retention: 5                 # 保留份数
  local_path: "./backups"      # 备份输出目录
  # remote_url: ""             # 可选：WebDAV 远端备份 URL
  # remote_username: ""
  # remote_password: ""
```

备份内容为数据库 + 配置；恢复走管理界面或 `POST /api/v1/admin/backups/restore`。

## container — 容器管理

```yaml
container:
  local:
    enabled: true
    docker_host: "unix:///var/run/docker.sock"   # 本地 Docker socket
  remote:
    enabled: true
    sync_concurrency: 2                 # 远程 registry 同步并发
    retention_check_interval: "1h"      # 保留策略检查间隔
```

## projects — 采集项目种子

`projects` 列表在首次启动时播种采集项目；之后以数据库为准（管理界面增删改）。每项：

```yaml
projects:
  - name: "prometheus"             # 唯一标识
    display_name: "Prometheus"
    source_type: "github"          # github | go | hashicorp | grafana | npm | pypi | crates
    source_url: "https://github.com/prometheus/prometheus"
    crawl_interval: "6h"           # 覆盖 crawler.default_interval
    github_owner: "prometheus"     # github 源需要 owner/repo
    github_repo: "prometheus"
```

默认种子包含 Prometheus 全家桶（prometheus、node/blackbox/mysqld exporter）、VictoriaMetrics、Go 官方、HashiCorp（Consul/Packer/Vagrant/Nomad）、Grafana。管理界面还有**工具目录**（tool catalog）支持对内置常用工具一键启用。

## 运行时可变配置

以下设置不需要改 YAML、不需要重启：

| 设置 | 途径 |
|------|------|
| 管理员密码 | 设置页 / `PUT /api/v1/admin/password` |
| 磁盘警告/严重阈值 | 设置页 / `GET/PUT /api/v1/admin/config/monitor` |
| 存储路径（含迁移任务） | 设置页 / `GET/PUT /api/v1/admin/config/storage`、`GET /api/v1/admin/storage/migrations` |
| 采集项目、API 令牌、爬取控制 | 采蜜模块 / `/api/v1/admin/projects`、`/api/v1/admin/credentials`、`/api/v1/admin/crawl/*` |

> 生产环境的差异点通常只有三处：`storage.base_path`（指向大容量卷）、`database.path`、`auth`（改密码）。参见[部署指南](deployment.md)。
