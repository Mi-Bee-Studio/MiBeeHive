# MiBeeHive Quick Start

Get MiBeeHive running on a Linux host in ten minutes: build it, start it, run your first crawl, then pull from it with `apt` / `pip` from another machine.

## Prerequisites

- A Linux host (amd64 or arm64; a NAS, mini PC, VM, or old laptop all work)
- **On the build machine**: Go 1.26+ and Git (the build machine can be the host itself or your dev machine — cross-compiling is a single command)
- ~500MB of free memory and a few GB of disk (depending on how much you forage)

## 1. Build the binary

```bash
git clone https://github.com/Mi-Bee-Studio/MiBeeHive.git
cd MiBeeHive

# Build directly on the Linux host
go build -o mibeehive ./cmd/mibeehive
```

Cross-compile from another machine (e.g. your dev machine runs Windows/macOS and the target is an ARM64 NAS):

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive ./cmd/mibeehive
# use GOARCH=amd64 for an amd64 target
```

> MiBeeHive is pure Go (`CGO_ENABLED=0`); cross-compiling needs no extra toolchain. Note that it targets Linux: it uses `syscall.Statfs` and therefore cannot build for Windows — when developing on Windows, cross-compile to Linux or run inside WSL.

## 2. Start it

```bash
./mibeehive
```

The default config path is `./configs/config.yaml` (override with `-config`). On first start MiBeeHive will:

- run all SQLite migrations automatically (`modernc.org/sqlite`, the pure-Go driver — nothing to install);
- generate a `jwt_secret` if none is configured;
- seed the foraging projects from the `projects` list in the config.

Then open **http://localhost:9090** and log in with the defaults:

```text
username: admin
password: admin
```

> **Change the password immediately** — either in Settings after logging in, or via `PUT /api/v1/admin/password`. The UI keeps reminding you while the default password is in use.

## 3. First crawl

1. In the web UI, open the **Foraging** module's project management and confirm the seed projects (Prometheus, node exporter, Consul, Grafana, …) are enabled; configure GitHub tokens under **API tokens** if needed.
2. Click **Trigger crawl** (single project or all). The crawler fetches each source's release manifest and queues new versions for download.
3. Watch the download queue page; completed files land under `{storage.base_path}/oss/` (default `./data/oss/`).

Foraging is continuous: each project refreshes automatically on its `crawl_interval` (default 6h), and the download queue handles retries and integrity checks.

## 4. Pull from another machine

This is MiBeeHive's core scenario — external servers take delivery with the tools they **already have**, no client to install. Assume MiBeeHive runs at `192.168.1.10:9090`:

**Debian / Ubuntu host (APT):**

```bash
echo "deb http://192.168.1.10:9090/apt stable main" | sudo tee /etc/apt/sources.list.d/mibeehive.list
sudo apt update
sudo apt install <some-collected-package>
```

**Python host (PyPI Simple, PEP 503):**

```bash
pip install --index-url http://192.168.1.10:9090/simple/ <some-collected-package>
# or: uv pip install --index-url http://192.168.1.10:9090/simple/ <pkg>
```

**Any host (generic manifest / direct link):**

```bash
curl -s http://192.168.1.10:9090/repo/index          # JSON manifest of all suppliable files
curl -O http://192.168.1.10:9090/repo/files/42       # direct download by id
```

Supply endpoints are public and unauthenticated — that's exactly what lets external hosts pull unattended. To peek at the inventory, just open `http://192.168.1.10:9090/repo/index` in a browser.

## 5. A quick sanity check

```bash
curl -s http://localhost:9090/health    # -> OK
curl -s http://localhost:9090/metrics   # Prometheus metrics (MiBeeHive's own health)
```

## Next steps

- [Configuration](configuration.md) — every option: ports, storage paths, crawl cadence, backups
- [Deployment](deployment.md) — systemd service, production layout, HTTPS, backup/restore
- [Architecture](architecture.md) — layers and data flow
- [API Reference](api-reference.md) — all HTTP endpoints
