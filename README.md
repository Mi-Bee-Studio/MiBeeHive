# MiBeeHive

[中文](README.zh.md)

An **operations tooling supply platform for external servers**. MiBeeHive runs on resource-constrained ARM64 devices (a NAS or storage server) and acts as a supply hub: it **automatically collects and keeps up-to-date** the ops tools your fleet needs, and **serves them to external servers over standard protocols** (software repositories, ISO repositories, image registries, package-proxy sources).

> **Where it differs from typical ops panels:** those manage the *local* machine (app store, quick site building, consumer apps). MiBeeHive targets the *other* servers — it is a supply chain that fetches, updates, and hands tools to external machines. *They run the box; MiBeeHive keeps the fleet stocked.*

The bee-hive metaphor maps directly onto this: the hive does not make honey, it **collects, ages, and distributes** it. MiBeeHive does not invent protocols — it implements existing standard protocols so off-the-shelf tools can pull their materials from it. The core modules are the two self-sufficient provisioning capabilities that no other ops panel does:

![License](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)
![Go Version](https://img.shields.io/badge/go-1.26+-00ADD8.svg?style=flat-square)
![Build](https://img.shields.io/badge/build-passing-brightgreen.svg)

## Features

### Foraging (采蜜) — the supply engine
- Crawl and download binary releases from GitHub, Go, HashiCorp, Grafana, NPM, and PyPI — the ops tools your external servers will consume
- Web management for crawl sources, API tokens, and scheduling
- Automatic retry with integrity verification
- Web admin panel for source configuration
- *(Roadmap) Expose collected artifacts to external servers over standard protocols: software repositories, Go module proxy, PyPI `/simple`, Helm repo, APT/YUM layout*

### Provisioning (哺育) — bring new external servers online
- Provide unattended OS installation configuration via PXE, so bare-metal external servers can be enrolled and stocked from scratch
- OS template generation (preseed/kickstart/autoinstall)
- ISO download management with streaming and auto-discovery
- Public PXE endpoints for network boot installations

### Sharing (分享) — serve files out
- WebDAV file sharing with Basic Authentication — expose the collected tooling/files to external servers
- Anonymous read-only + admin read-write access
- HTTPS support with self-signed certificates
- Configurable via web UI

### Dashboard
- Aggregated overview of all modules (system, files, deploy, share)
- Real-time CPU, memory, and disk monitoring with charts
- Activity timeline for recent crawl and download events
- Quick actions bar for common operations
- Configurable disk warning/critical thresholds

### Containers
- Docker container management (CRUD, start/stop/restart, logs, stats)
- Registry management with multi-registry support
- Image synchronization across registries
- Tag retention policies with automated cleanup

## Tech Stack

- **Backend**: Go 1.26+ (latest) with stdlib HTTP only
- **Database**: SQLite with modernc.org/sqlite driver
- **Frontend**: **Preact + HTM**, TailwindCSS CDN, Chart.js CDN
- **No external frameworks**: No chi/gin/echo, no npm, no cron libraries
- **Embedded**: Frontend embedded via go:embed

## Screenshots

<!-- Add screenshots here -->

## Quick Start

### Prerequisites
- Go 1.26+ (latest) installed
- ARM64 target device with ≥1GB RAM, ≥32GB storage (optional, for deployment)

### Build
```bash
go build -o mibeehive ./cmd/mibeehive
```

### Configure
Copy the example configuration:
```bash
cp configs/config.yaml config.yaml
```

Edit `config.yaml` to set:
- Storage base path
- Database path
- Server ports
- JWT secret and password hash

### Run
```bash
./mibeehive
```

Access the web interface at `http://localhost:9090`

## Default Login

- **Username**: admin
- **Password**: admin (**WARNING**: Change immediately after first login)

## Project Structure

```
mibeehive/
├── cmd/
│   ├── mibeehive/       # Main server entry point
│   │   ├── main.go      # 100-line entry point
│   │   └── init.go      # 798-line init logic (loadConfig, initDatabase, initServices, initHandlers, buildRouter, runServers)
│   └── migrate/         # Migration tool (separate binary)
├── internal/
│   ├── backup/          # Backup create/restore logic
│   ├── config/          # Configuration management
│   ├── crawler/         # Crawl system (6 sources + scheduler)
│   ├── db/              # SQLite repositories and migrations
│   ├── docker/          # Docker client wrapper
│   ├── handler/         # HTTP handlers
│   ├── health/          # Health check endpoint
│   ├── metrics/         # Prometheus metrics endpoint
│   ├── middleware/       # Authentication, TLS, rate limiting, security headers
│   ├── model/           # Domain types and route constants
│   ├── monitor/         # System resource monitoring
│   ├── registry/        # V2 Docker/OCI registry client
│   ├── service/         # Business logic
│   └── webdav/          # WebDAV handler
├── web/                  # Embedded frontend (38 JS modules + 1 CSS)
│   ├── css/              # style.css with CSS variables
│   └── js/               # Preact SPA
│       ├── core/         # Framework: api, auth, cache, components, drawer, helpers, preact, router, search, state, tooltips
│       ├── layout/       # Shell: sidebar, shell, bottom-tab
│       └── modules/      # Pages: dashboard, files, deploy, share, containers, settings, login, etc.
├── configs/              # Configuration files + systemd service
└── docs/                 # Documentation (architecture, deployment, API)
```

## Documentation

- [Architecture](docs/en/architecture.md) - Comprehensive architecture documentation
- [Deployment](docs/en/deployment.md) - Deployment guide for ARM64 devices
- [API Reference](docs/en/api-reference.md) - Complete API documentation
- [Supply Layer Roadmap](docs/roadmap/supply-layer.md) - Plans for the ops-tool supply layer

## Admin Interface

The web admin panel provides a dashboard overview and tabbed navigation for managing all modules:

1. **Dashboard** - Aggregated overview with system stats, module cards, activity timeline, and quick actions
2. **Files** - Browse downloaded files, manage crawl projects, monitor download queue
3. **Deploy** - OS install config management, ISO catalog and downloading
4. **Share** - WebDAV file sharing and file browser
5. **Containers** - Docker container management and registry operations
6. **Settings** - Password change, theme, language, disk threshold configuration

## Target Device

MiBeeHive is designed for ARM64 devices with:
- **Minimum**: 1GB RAM, 32GB storage
- **Recommended**: ≥2GB RAM, ≥64GB storage
- **OS**: Linux with systemd support
- **Deployment**: Systemd service with automatic restart

## Development

### Running Tests
```bash
go test ./...
go test -v ./internal/crawler  # Specific package
go vet ./...                    # Static analysis
```

### Cross-Compilation
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

### Code Structure
- **Handlers**: HTTP request handling (Go 1.26+ method+path routing)
- **Services**: Business logic layer
- **Repositories**: Data access layer with SQLite
- **Frontend**: Preact + HTM with hash-based routing

## Scope Boundary (What MiBeeHive Is / Is Not)

**Is:** An operations-tooling **supply chain** that collects, updates, and serves ops tools to external servers over standard protocols. It implements *existing* protocols (so off-the-shelf tools can pull from it) — it does not invent new ones.

**Is Not:**
- Not a local-machine app store / quick site builder (that is what typical ops panels do).
- Not a TSDB / metrics aggregator — `/metrics` is only for MiBeeHive's own health; it supplies `node_exporter`/`prometheus` *to* external servers rather than competing with them.

## Contributing

Read [CONTRIBUTING.md](./CONTRIBUTING.md) for detailed contribution guidelines.

## License

This project is licensed under the AGPL-3.0 License - see the [LICENSE](./LICENSE) file for details.

## Support

For issues and questions:
- Create an issue on [GitHub](https://github.com/Mi-Bee-Studio/mibeehive)
- Check the [documentation](https://github.com/Mi-Bee-Studio/mibeehive/tree/main/docs)

---

Built with ❤️ by [Mi-Bee Studio](https://github.com/Mi-Bee-Studio)