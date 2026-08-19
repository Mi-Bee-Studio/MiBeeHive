# MiBeeHive Configuration Reference

MiBeeHive is driven by a single YAML file (default `./configs/config.yaml`, override with the `-config` flag). The repo's [`configs/config.yaml`](https://github.com/Mi-Bee-Studio/MiBeeHive/blob/main/configs/config.yaml) is a fully commented sample. Missing files and fields fall back to built-in defaults; a `jwt_secret` is generated automatically on first start when empty.

Some settings can be changed at runtime through the admin UI or API (see "Runtime-changeable settings" at the end); everything else requires a process restart.

## server — HTTP/TLS

```yaml
server:
  port: 9090              # main HTTP port (web UI, API, supply endpoints, PXE)
  bind_addr: "0.0.0.0"    # listen address
  https_port: 9443        # HTTPS port; 0 = disabled (WebDAV is served over HTTPS only)
  # cert_path: ""         # custom TLS cert; leave empty to auto-generate a self-signed one
  # key_path: ""
  tls_ip_addresses: []    # IPs written into the self-signed cert; empty = auto-detect NIC IPs (127.0.0.1 always included)
  tls_dns_names:          # DNS names written into the cert; empty = ["localhost"]
    - localhost
```

- All public endpoints (`/repo/`, `/apt/`, `/simple/`, `/pxe/`, `/health`, `/metrics`) live on the main port, for unattended access by external servers and PXE clients.
- Open `port` (and `https_port`, if enabled) in your firewall.

## database — SQLite

```yaml
database:
  path: "./mibeehive.db"   # SQLite database file path
```

Migrations run automatically at startup (a standalone `cmd/migrate` tool exists for manual intervention). The write path uses a single connection (`SetMaxOpenConns(1)`) to suit low-memory devices.

## storage — layout

```yaml
storage:
  base_path: "./data"      # parent directory for all module storage
  # modules:               # optional: per-module overrides (empty falls back to {base_path}/{module})
  #   oss: "/mnt/bigdisk/oss"
  #   os_install: ""
  #   iso: ""
```

Default layout:

| Subdirectory | Module | Contents |
|--------------|--------|----------|
| `{base_path}/oss/` | Foraging | collected binary releases (.deb, wheels, tarballs, …) |
| `{base_path}/os-install/` | Provisioning | OS install files and ISOs |
| `{base_path}/webdav/` | Sharing | WebDAV-shared files |

> The supply layer has no separate storage: APT / PyPI / generic-repo indexes are generated on demand over the artifacts collected into `oss/` — the same files serve both generic downloads and native protocols.

Storage paths can be changed at runtime in Settings; the change is applied as a background **migration task** that moves files (see the storage-migrations endpoints in the API reference).

## auth — authentication

```yaml
auth:
  password_hash: ""        # bcrypt hash; empty = default password "admin" (change it right after login)
  jwt_secret: ""           # JWT signing secret; auto-generated and persisted on first start when empty
  # password_changed_at:   # maintained by the app — do not edit by hand
```

- Admin endpoints use JWTs (`Authorization: Bearer <token>`, 1-hour validity, renewable via `/api/v1/auth/refresh`).
- WebDAV uses Basic Auth: anonymous read, admin credentials identical to the admin panel.
- Change the password via the UI or `PUT /api/v1/admin/password` (updates hash and timestamp together).

## crawler — foraging

```yaml
crawler:
  max_concurrent: 2              # number of sources fetched concurrently
  default_interval: "6h"         # default crawl interval when a project doesn't specify one
  fetch_timeout: "60s"           # per-source deadline for the whole fetch+retry; "0" disables it (only the HTTP client's 30s remains)
  max_retries: 3                 # max retries for transient errors (timeout/reset/5xx); 4xx and rate-limited never retry
  retry_initial_backoff: "2s"    # base delay before the first retry; later retries back off ×2 with jitter
```

Errors are classified as `network_error` / `rate_limited` / `error`, making it easy to tell transient failures from real upstream problems in the queue and logs.

## monitor — system monitoring

```yaml
monitor:
  sample_interval: 30        # seconds between system stat samples
  retention_days: 7          # days of history to keep (max 30)
  # node_exporter_url: ""    # optional: scrape metrics from a node_exporter URL
  disk_warning_percent: 90   # disk usage warning threshold (%)
  disk_critical_percent: 95  # disk usage critical threshold (%) — triggers degraded mode
  disk_check_enabled: true   # enable/disable disk monitoring
```

Disk thresholds can also be changed at runtime in Settings or via `GET/PUT /api/v1/admin/config/monitor`.

## logging — rotation

Powered by lumberjack:

```yaml
logging:
  filename: "./mibeehive.log"  # log file path
  max_size: 10                 # MB per file before rotation
  max_backups: 3               # number of rotated logs to keep
  max_age: 30                  # days to retain old logs
  compress: true               # compress rotated logs
  local_time: true             # use local time in filenames
```

## backup — scheduled backups

```yaml
backup:
  enabled: false               # enable scheduled backups
  schedule: "03:00"            # daily backup time (HH:MM)
  retention: 5                 # number of backups to keep
  local_path: "./backups"      # backup output directory
  # remote_url: ""             # optional WebDAV URL for remote backup
  # remote_username: ""
  # remote_password: ""
```

Backups cover the database plus config; restore via the admin UI or `POST /api/v1/admin/backups/restore`.

## container — container management

```yaml
container:
  local:
    enabled: true
    docker_host: "unix:///var/run/docker.sock"   # local Docker socket
  remote:
    enabled: true
    sync_concurrency: 2                 # remote registry sync concurrency
    retention_check_interval: "1h"      # retention-policy check interval
```

## projects — foraging project seeds

The `projects` list seeds foraging projects on first start; afterwards the database is the source of truth (add/remove via the admin UI). Each entry:

```yaml
projects:
  - name: "prometheus"             # unique slug
    display_name: "Prometheus"
    source_type: "github"          # github | go | hashicorp | grafana | npm | pypi | crates
    source_url: "https://github.com/prometheus/prometheus"
    crawl_interval: "6h"           # overrides crawler.default_interval
    github_owner: "prometheus"     # github sources need owner/repo
    github_repo: "prometheus"
```

Default seeds include the Prometheus family (prometheus, node/blackbox/mysqld exporters), VictoriaMetrics, official Go, HashiCorp (Consul/Packer/Vagrant/Nomad), and Grafana. The admin UI also offers a **tool catalog** for enabling curated common tools with one click.

## Runtime-changeable settings

The following don't require YAML edits or restarts:

| Setting | Via |
|---------|-----|
| Admin password | Settings page / `PUT /api/v1/admin/password` |
| Disk warning/critical thresholds | Settings page / `GET/PUT /api/v1/admin/config/monitor` |
| Storage paths (with migration tasks) | Settings page / `GET/PUT /api/v1/admin/config/storage`, `GET /api/v1/admin/storage/migrations` |
| Foraging projects, API tokens, crawl control | Foraging module / `/api/v1/admin/projects`, `/api/v1/admin/credentials`, `/api/v1/admin/crawl/*` |

> Production usually only differs in three places: `storage.base_path` (point it at a big volume), `database.path`, and `auth` (change the password). See the [deployment guide](deployment.md).
