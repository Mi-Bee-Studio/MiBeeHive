# MiBeeHive

[中文](README.zh.md) · [Docs](https://www.mlsbs.top/docs/mibeehive) · [API](https://www.mlsbs.top/docs/mibeehive/api-reference)

> **A lightweight, self-hosted ops-tooling supply hub for your whole fleet.**
> MiBeeHive auto-collects the binaries, packages, and ISOs your servers need — and serves them back out over the protocols those servers already speak: `apt`, `pip`, WebDAV. One static Go binary, an embedded UI, runs anywhere Linux does (amd64 / arm64 — a mini PC, a NAS, a VM, an old laptop). They run the box; MiBeeHive keeps the fleet stocked.

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

## Why

- **Lightweight & multi-arch.** A single static Go binary, pure stdlib HTTP, embedded SPA — runs on amd64 or arm64 Linux, from a 469 MB RAM NAS to a beefy server. Pure-Go SQLite driver, no CGO, no external dependencies.
- **Fleet-native supply.** Your servers pull with their own tooling (`apt`, `pip`, WebDAV). No agent, no client to install.
- **Collect → store → serve.** Crawl sources stay up to date automatically; served artifacts are always current.

## How it works

```
   ┌──────────────┐   crawl + download   ┌──────────────┐   serve over native protocols
   │ GitHub / Go / │ ───────────────────▶ │  MiBeeHive   │ ─────────────────────────────▶  your servers
   │ PyPI / NPM /  │   auto, on schedule  │  (any Linux) │   apt  ·  pip  ·  WebDAV  ·  PXE
   │ HashiCorp …   │                      │              │
   └──────────────┘                       └──────────────┘
```

## Features

**Foraging — the supply engine**
- Crawl binary releases from GitHub, Go, HashiCorp, Grafana, NPM, PyPI, Crates
- Pluggable two-track source model: YAML fingerprints for single-page sources + Go adapters for stateful protocols
- Retry with backoff, per-crawl timeout, and distinct `network_error` / `rate_limited` statuses
- Web UI for sources, API tokens, scheduling

**Supply — native-protocol endpoints** *(no client to install)*
- **APT repository** over collected `.deb` files → `apt update && apt install <pkg>`
- **PyPI Simple** (PEP 503) over collected wheels → `pip install --index-url …/simple/ <pkg>`
- Generic `/repo/index` + `/repo/files/{id}` for everything else

**Provisioning — bring new servers online**
- Unattended OS install via PXE (preseed / kickstart / autoinstall)
- OS template generation, ISO catalog + download queue

**Sharing**
- WebDAV with Basic Auth (anonymous read, admin write), self-signed HTTPS

**Ops**
- Dashboard: CPU / memory / disk, activity timeline, logs, tasks
- Docker container management + multi-registry sync & tag retention
- Backup / restore, global search, bilingual (中文 / English)

## Quick start

```bash
# Build (single binary, no external deps)
go build -o mibeehive ./cmd/mibeehive

# Configure & run
cp configs/config.yaml config.yaml   # edit storage path, ports, secrets
./mibeehive                           # UI at http://localhost:9090  ·  admin / admin
```

Or cross-compile for another architecture (e.g. arm64) — MiBeeHive builds for any Linux/Go-supported arch:

```bash
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

> Runs on any Linux + systemd host (amd64 or arm64). Lightweight: comfortable from ~1 GB RAM / 32 GB storage up. See the [deployment guide](https://www.mlsbs.top/docs/mibeehive/deployment). A prebuilt multi-arch Docker image (`linux/amd64`, `linux/arm64`) is published on release tags.

## Supply endpoints (copy-paste for your fleet)

| Protocol | Client command |
|---|---|
| APT | `echo "deb http://<host>:9090/apt stable main" \| tee /etc/apt/sources.list.d/mibeehive.list` |
| PyPI | `pip install --index-url http://<host>:9090/simple/ <pkg>` |
| WebDAV | `https://<host>:9443/webdav/` (HTTPS only) |
| Generic | `GET /repo/index` (JSON manifest) · `GET /repo/files/{id}` (download) |

## Documentation

Manuals (introduction, quick start, architecture, deployment, configuration, API reference, development) are published from the [mibee-docs](https://github.com/Mi-Bee-Studio/mibee-docs) repository to the docs center at **[www.mlsbs.top/docs/mibeehive](https://www.mlsbs.top/docs/mibeehive)**. Manual changes go there via PR — submission rules are in that repo's [README](https://github.com/Mi-Bee-Studio/mibee-docs#提交规范各项目仓库请遵守); this repo's `docs/` only holds internal planning notes.

- [Supply Layer](docs/roadmap/supply-layer.md) — protocol roadmap (APT ✅, PyPI ✅, more coming)

## Scope

MiBeeHive is a **supply chain**: it collects ops tools and serves them over *existing* standard protocols. It is **not** a local-machine app store, site builder, or TSDB — `/metrics` is for its own health; it *supplies* `node_exporter`/`prometheus` to your fleet rather than competing with them.

## Contributing & License

- [Contributing guide](CONTRIBUTING.md) · Conventional Commits · manuals in [mibee-docs](https://github.com/Mi-Bee-Studio/mibee-docs) (zh-CN / en-US)
- AGPL-3.0 — see [LICENSE](LICENSE)

---

Built with ❤️ by [Mi-Bee Studio](https://github.com/Mi-Bee-Studio).
