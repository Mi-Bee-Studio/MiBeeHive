# MiBeeHive Deployment Guide

## Target: Any Linux Host (amd64 / arm64)

### Reference Specifications
- **SSH**: `ssh user@device-ip`
- **OS**: Linux (Debian/Ubuntu/Armbian all work), kernel 6.0+
- **Hardware**: amd64 or arm64; runs in as little as 469MB RAM (NAS / mini PC), ≥1GB recommended; storage depends on how much you forage (≥32GB to start)
- **No Go toolchain needed on the device** — cross-compile locally and SCP the binary over (a direct `go build` on the device works too, if Go is present)

## Build Commands

### Local Development Build
```bash
go build -o mibeehive ./cmd/mibeehive              # Build for current architecture
```

### ARM64 Cross-Compile
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive  # Cross-compile for ARM64
```

### Build Migration Tool
```bash
go build -o migrate ./cmd/migrate                   # Build migration tool
```

### Testing
```bash
go test ./...                                       # Run all tests
go test -v ./internal/crawler                       # Run specific package tests
go vet ./...                                        # Static analysis
```

## Deployment Layout (on device)

```
/opt/mibeehive/
├── bin/mibeehive
├── config.yaml
├── mibeehive.db
├── mibeehive.log
├── backup-*.tar.gz
└── backups/

/var/lib/mibeehive/
├── oss/            # Foraging: downloaded binary releases (.deb, wheels, tarballs, …)
├── os-install/     # Provisioning: OS installation files
└── webdav/         # Sharing: WebDAV shared files
```

> **Supply layer has no storage directory of its own.** APT (`/apt/`) and PyPI Simple (`/simple/`) generate their protocol indexes on demand over the artifacts Foraging collected in `oss/`, so the same collected files are served both as a generic repo and via native protocols.

## Deploy & Restart

### 1. Cross-compile locally (on dev machine)
```bash
GOARCH=arm64 CGO_ENABLED=0 go build -o mibeehive-arm64 ./cmd/mibeehive
```

### 2. Upload to device
```bash
scp mibeehive-arm64 user@device-ip:/opt/mibeehive/bin/mibeehive
```

### 3. Restart via systemd
```bash
ssh user@device-ip "pkill mibeehive; sleep 1 && sudo systemctl restart mibeehive"
```

## Systemd Service

### Service File
Service file is located at `configs/mibeehive.service`. Install and enable on device:

```bash
sudo systemctl start mibeehive    # Start
sudo systemctl stop mibeehive     # Stop
sudo systemctl restart mibeehive  # Restart
sudo systemctl status mibeehive   # Status
journalctl -u mibeehive -f       # Follow logs
```

### Systemd Configuration
The service is configured with memory limits appropriate for ARM64 devices:
- `GOMEMLIMIT=256MiB` - Memory limit for Go runtime
- Auto-restart on failure
- Logging to journal

## Verification (on device)

### Service Status
```bash
sudo systemctl status mibeehive
```

### Health Checks
```bash
curl -s http://localhost:9090/ | head -5              # Health check (web UI)
curl -s -X PROPFIND http://localhost:9090/webdav/     # WebDAV check
curl -sk https://localhost:9443/ | head -5            # HTTPS check
# Supply layer (public, no auth) — external servers consume from these:
curl -s http://localhost:9090/repo/index | head -c 120        # Generic file manifest (JSON)
curl -s -o /dev/null -w "%{http_code}\n" http://localhost:9090/apt/dists/stable/Release   # APT repo (200 = OK)
curl -s http://localhost:9090/simple/ | head -c 120           # PyPI Simple index (PEP 503)
```

### Log Monitoring
```bash
journalctl -u mibeehive -f                           # Follow logs
tail -f /var/log/mibeehive/mibeehive.log           # Application logs
```

## Configuration

### Production Config File
Production config at `/etc/mibeehive/config.yaml` differs from `configs/config.yaml`:

```yaml
storage:
  base_path: /var/lib/mibeehive/     # Note: includes oss subdirectory
database:
  path: /opt/mibeehive/mibeehive.db  # Path to SQLite database
server:
  port: 9090
  https_port: 9443
auth:
  jwt_secret: your-jwt-secret-here
  password_hash: your-password-hash-here
```

### Configuration Management
- Auto-generates default config if file missing on startup
- Supports YAML format only
- Environment-specific configurations stored in YAML
- Database stores project configuration separately from infrastructure config

For every option (crawl cadence, monitoring, log rotation, backups, containers, foraging project seeds, …) see the [configuration reference](configuration.md).

## Memory Management

### Target Device Constraints
- **Total RAM**: ≥1GB (recommended ≥2GB)
- **Go memory limit**: 256MB (via `GOMEMLIMIT`)
- **Available for application**: ~213MB
- Optimized for low-memory usage:
  - Single SQLite connection (`db.SetMaxOpenConns(1)`)
  - Streaming downloads (no buffering entire files)
  - Efficient logging (`log/slog` with structured output)

## Network Configuration

### Ports
- **HTTP**: 9090 (main web interface)
- **HTTPS**: 9443 (WebDAV and admin panel)
- **PXE**: 9090 (public endpoints, no auth)

### Firewall Considerations
- Ensure ports 9090 and 9443 are accessible
- PXE endpoints must be publicly accessible (no auth)
- Admin endpoints require JWT authentication

## Backup and Recovery

### Backup Strategy
- Database: SQLite file (`mibeehive.db`)
- Configuration: `/etc/mibeehive/config.yaml`
- Downloaded files: Automated backup to `backup-*.tar.gz`
- Systemd service state: Handled by systemctl

### Recovery Steps
1. Stop the service: `sudo systemctl stop mibeehive`
2. Backup existing files
3. Restore from backup
4. Start the service: `sudo systemctl start mibeehive`

## Monitoring and Maintenance

### Log Rotation
Configured via crontab on device:
- Weekly Sunday 01:00: Clean logs older than 30 days in `/var/log/mibeehive/`
- Monthly 1st 09:00: Generate download report via `/opt/mibeehive/bin/generate-report.sh`

### Performance Monitoring
- Monitor memory usage: `ps aux | grep mibeehive`
- Check disk space: `df -h`
- Review application logs for errors and warnings

### Common Issues
1. **Memory issues**: Monitor `GOMEMLIMIT` usage, check for memory leaks
2. **Disk space**: Monitor storage paths, especially download directories
3. **Network connectivity**: Ensure device has internet for crawling
4. **Database corruption**: Use SQLite integrity checks