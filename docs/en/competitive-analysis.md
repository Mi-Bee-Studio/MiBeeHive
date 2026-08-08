# Competitive Analysis

[中文](../zh/competitive-analysis.md)

This document surveys the competitive landscape across four market segments relevant to MiBeeHive's positioning as an **operations-tooling supply platform for resource-constrained ARM64 devices**. Each segment is analyzed with 2–3 deep-dive projects, a comparison table, and brief mentions of adjacent tools. The final section distills adoptable ideas, borrowable patterns, and pitfalls to avoid.

---

## 1. Artifact Repositories

Artifact repositories store, manage, and distribute software packages across development workflows. The market is dominated by heavyweight enterprise solutions that require significant compute resources.

### 1.1 JFrog Artifactory — Deep Dive

**Architecture.** JFrog Artifactory is a Java/JVM application deployed via Docker, Kubernetes Helm charts, or Linux packages. It uses PostgreSQL for metadata and supports S3/GCS/Azure Blob for binary storage. The minimum production requirement is 6 GB RAM and 4 CPU cores, scaling to 12–18 GB for larger deployments ([JFrog docs](https://docs.jfrog.com/)).

**Core positioning.** Universal enterprise artifact repository supporting 30+ package formats (Docker, Maven, npm, PyPI, Go, Helm, OCI, Debian, RPM, Conan). It is the market leader with reported Fortune 100 adoption.

**Borrowable highlights.**
- *Virtual repositories* aggregate local, remote, and other virtual repositories behind a single logical URL. The resolution order (Local → Remote Cache → Remote) and include/exclude patterns cleanly decouple physical storage from client-facing paths. This is the most mature virtual-view implementation in the artifact space ([JFrog virtual repos](https://docs.jfrog.com/artifactory/jfrog-platform/3.x-x/artifactory-user-manage-repositories)).
- *AQL (Artifactory Query Language)* provides a declarative search syntax across all metadata — a powerful pattern for cross-catalog queries.

**Limitations.** 6 GB RAM minimum; enterprise features (SAML, signed URLs, Xray scanning) locked behind expensive tiers ($27K+/year self-hosted). No native WebDAV support. Complex multi-service deployment model.

### 1.2 Sonatype Nexus Repository — Deep Dive

**Architecture.** Java/JVM application with an embedded H2 database (limited to 200K requests/day) or PostgreSQL for production. Supports S3/Azure/GCS blob stores. Minimum 8 GB RAM and 2 CPUs ([Sonatype docs](https://help.sonatype.com/)).

**Core positioning.** Developer-focused repository manager with strong Maven ecosystem roots. Community Edition is free but has component-count limits (40,000 components as of 2025). Pro starts at $135/month ([Sonatype pricing](https://www.sonatype.com/products/pricing)).

**Borrowable highlights.**
- *Free Community Edition* lowers the barrier to entry for small teams — a model MiBeeHive already follows.
- *Repository Groups* aggregate multiple repositories behind a single URL, similar to Artifactory's virtual repos but less sophisticated.

**Limitations.** 8 GB RAM minimum. Community Edition limits controversial in 2025. No native WebDAV. Limited share-link capabilities. Partially open-source (EPL 1.0 core + proprietary additional formats).

### 1.3 GitHub Packages — Deep Dive

**Architecture.** Fully managed SaaS service part of GitHub.com. No self-hosted option. Proprietary infrastructure ([GitHub docs](https://docs.github.com/en/packages)).

**Core positioning.** Package hosting tightly integrated with GitHub repositories and Actions workflows. Supports npm, Docker (ghcr.io), RubyGems, Maven, NuGet. Free for public packages; private storage shared with Actions artifacts.

**Limitations.** SaaS only — no edge deployment. No virtual repository abstraction. Limited format support (no PyPI, Go, Helm, WebDAV). No share links.

### Comparison Table — Artifact Repositories

| Dimension | JFrog Artifactory | Sonatype Nexus | GitHub Packages |
|---|---|---|---|
| **Positioning** | Universal enterprise artifact repo | Developer-focused repository manager | GitHub-native package hosting |
| **File browsing UX** | 2–3 clicks, AQL search, rich filtering | 2–3 clicks, component search | 2–3 clicks, basic search |
| **Virtual view / index** | Yes — Virtual Repositories | Yes — Repository Groups | No |
| **Multi-protocol** | 30+ formats, no WebDAV | 20+ formats, no WebDAV | 6 formats, no WebDAV |
| **Access control** | Granular, LDAP/SAML/AD | Granular, LDAP (SAML in Pro) | Repository-based, GitHub auth |
| **Path hiding** | Include/Exclude patterns, Content Selectors | Content Selectors, Routing Rules | Visibility only |
| **Share links** | Signed URLs (Enterprise tier) | Pre-signed S3 URLs (Pro) | None |
| **Metadata storage** | PostgreSQL + S3/filestore | H2/PostgreSQL + blob stores | GitHub-managed |
| **Resource usage** | 6 GB RAM min, ARM64 via container | 8 GB RAM min, ARM64 via Docker | SaaS (N/A) |
| **Extensibility** | Groovy plugins, JFrog Workers | Community plugins, SDK | GitHub Actions, Webhooks |
| **Mobile** | Responsive web UI | Responsive web UI | GitHub Mobile app |
| **License & activity** | Proprietary, monthly releases | EPL 1.0 + Proprietary, active | Proprietary, part of GitHub |

**Brief mentions:** *Cloudsmith* — cloud-native universal artifact management (29+ formats, no self-hosted). *ProGet* — on-premise, .NET-focused, 27+ formats, Windows-oriented.

---

## 2. WebDAV / File Servers

The WebDAV/file-server segment spans heavyweight productivity platforms to lightweight file managers. MiBeeHive occupies a unique niche: it is not primarily a file server but uses WebDAV as one of its supply protocols.

### 2.1 Nextcloud — Deep Dive

**Architecture.** PHP backend on Apache/Nginx with MySQL/MariaDB/PostgreSQL, Redis for caching, and optional Elasticsearch for search. Built-in SabreDAV server provides WebDAV without external modules. Deployed via LAMP stack or Docker Compose ([Nextcloud docs](https://docs.nextcloud.com/)).

**Core positioning.** Self-hosted productivity platform (Dropbox/Google Drive replacement) with 300+ apps (calendar, contacts, document editing, video chat). WebDAV is one access method among many.

**Borrowable highlights.**
- *Virtual filesystem abstraction* — The `View.php` / `Node` API layer maps mount points to diverse backends (local, NFS, S3, external clouds). Each user's home directory dynamically maps to different storage paths via LDAP attributes. The `oc_filecache` table decouples physical file scanning from display ([Nextcloud dev manual](https://docs.nextcloud.com/server/latest/developer_manual/)).
- *SabreDAV* is a mature, battle-tested WebDAV implementation handling chunked uploads (`X-OC-Mtime`), LOCK/UNLOCK, and custom headers.

**Limitations.** Full stack requires 730 MB–1.3 GB RAM — barely functional on 1 GB, impossible on 469 MB. No APT/PyPI/Go/Helm/OCI serving. Complex operational overhead (300+ apps, multiple services).

### 2.2 FileBrowser Quantum — Deep Dive

**Architecture.** Go backend (gorilla/mux) + Vue 3/TypeScript frontend. Built-in WebDAV via `golang.org/x/net/webdav` at `/dav/`. SQLite (v2.x) for metadata, Afero virtual filesystem for files. Docker image as slim as 15 MB ([GitHub](https://github.com/gtsteffaniak/filebrowser)).

**Core positioning.** Lightweight self-hosted web file manager. Primary value: browse, manage, share files via browser. WebDAV is an alternative access method.

**Borrowable highlights.**
- *Multiple-source abstraction* — Each source is a separate filesystem root with per-source include/exclude rules and permissions (View/Download/Create/Modify/Delete). Users see source names, not filesystem paths. This is directly applicable to MiBeeHive's module-based storage (oss, os-install, webdav).
- *Ultra-lightweight deployment* — 15 MB Docker image, 128 MB minimum RAM proves a feature-rich file manager need not be heavy.

**Limitations.** No APT/PyPI/Go/Helm/OCI serving. No virtual filesystem indirection beyond source configuration. Single-maintainer risk. WebDAV JWT tokens are long; some clients struggle.

### 2.3 Seafile — Deep Dive

**Architecture.** C core (sync engine) + Python (Seahub web UI, WebDAV via WsgiDAV/Gunicorn) + MariaDB + Memcached. Block-level storage with content-defined chunking for deduplication. WebDAV is optional and disabled by default ([Seafile docs](https://manual.seafile.com/)).

**Core positioning.** High-performance team file sync with Git-like deduplication. Strong at large-team collaboration with fast sync.

**Borrowable highlights.**
- *Block-level deduplication* via content-defined chunking — useful for MiBeeHive's versioned binary artifacts.
- *Virtual repos* provide scoped views into libraries without copying data.

**Limitations.** Multiple services (C daemon, Python web UI, MariaDB, Memcached) need 2–4 GB RAM. ARM64 support is community-maintained only. WebDAV is an afterthought (disabled by default, slow for frequent use).

### Comparison Table — WebDAV / File Servers

| Dimension | Nextcloud | FileBrowser Quantum | Seafile |
|---|---|---|---|
| **Positioning** | Self-hosted productivity platform | Lightweight web file manager | Team file sync with dedup |
| **File browsing UX** | Full web UI + desktop sync + WebDAV | Modern web UI + media player + WebDAV | Seahub web UI + desktop sync + WebDAV (opt.) |
| **Virtual view / index** | Full virtual filesystem (View/Node API, mount points) | Multiple sources with per-source config, SQLite index | Virtual repos (folder-level views) |
| **Multi-protocol** | WebDAV + CalDAV + CardDAV + Federation | WebDAV + HTTP only | WebDAV (opt.) + HTTP + seafhttp |
| **Access control** | Granular user/group/share/ACL + LDAP/OIDC + 2FA + encryption | Per-user roles + per-source permissions + access rules | Library/folder-level + group sharing + encryption |
| **Path hiding** | File Access Control rules, encryption at rest | Access control rules, source isolation | Library encryption, virtual repos |
| **Share links** | Public links (password, expiry), federated sharing | Public links (password, expiry, download limits) | Public links (password, expiry), upload-only |
| **Metadata storage** | MySQL/PostgreSQL (file cache, shares, tags) | SQLite with runtime maps + TTL cache | MariaDB + FS objects + block storage |
| **Resource usage** | 730 MB–1.3 GB (full stack) | 128–512 MB, 15 MB Docker image, ARM64 native | 2–4 GB, ARM64 community-only |
| **Extensibility** | 300+ apps, ExApps, WebDAV plugins | Limited (config-based), API, CLI | SeaDoc, limited plugins, REST API |
| **Mobile** | Official Android (5.4k ★) + iOS (2.4k ★) | Responsive web; WebDAV via 3rd-party clients | Official Android + iOS apps |
| **License & activity** | AGPL-3.0, ~35k ★, very active | Apache-2.0, ~7.5k ★, active | AGPLv3, ~14.7k ★, active |

**Brief mentions:** *rclone serve webdav* — single Go binary, serves 70+ backends over WebDAV, no UI or search. *MinIO S3* — enterprise S3-compatible object storage (~61k ★), no WebDAV, programmatic access only.

---

## 3. Ops Tool Supply / Package Proxy

This segment consists of single-protocol tools that each serve one ecosystem's packages. MiBeeHive's differentiation is multi-protocol supply serving from a single binary.

### 3.1 Athens (Go Module Proxy) — Deep Dive

**Architecture.** Go server implementing the [Go Modules Download Protocol](https://docs.gomods.io/intro/protocol/). Pluggable storage backends: MongoDB, S3, Azure, GCS, filesystem, in-memory. Fetcher invokes `go mod download` on cache miss. Network modes: `strict` (merge VCS + storage), `offline` (storage only), `fallback` (storage first, VCS on failure) ([GitHub](https://github.com/gomods/athens), [docs](https://docs.gomods.io/)).

**Core positioning.** Enterprise Go module proxy for immutable storage, private modules, compliance, and air-gapped builds.

**Borrowable highlights.**
- *Network modes* (`fallback`: storage-first, VCS-on-failure) are excellent for edge deployments with intermittent connectivity.
- *SingleFlight wrapper* prevents duplicate concurrent fetches for the same module — valuable for MiBeeHive's crawler deduplication.

**Limitations.** Go-only protocol. Requires MongoDB or object storage for production. ~50–100 MB RAM with MongoDB. No web-based artifact browsing beyond module listing.

### 3.devpi (Python Package Index) — Deep Dive

**Architecture.** Python application using Pyramid web framework with KeyFS transactional key-value storage. Three components: devpi-client (CLI), devpi-server (WSGI), devpi-web (search/UI plugin). SQLite default backend; `--requests-only` mode for read replicas ([GitHub](https://github.com/devpi/devpi), [docs](https://devpi.net/docs/)).

**Core positioning.** Python packaging workflow combining fast PyPI mirror, private indexes, tox testing integration, and release staging.

**Borrowable highlights.**
- *Index inheritance* — hierarchical index system with reference-based inheritance (no copying). Packages propagate up the chain unless overridden. Private uploads hide parent packages. This is the most elegant virtual-view pattern in the package-proxy space.
- *Plugin system* via setuptools entry points allows custom index types.

**Limitations.** PyPI-only protocol. SQLite single-writer limitation. No WebDAV. ~50–100 MB RAM.

### 3.3 reprepro (Debian Repository Creator) — Deep Dive

**Architecture.** Single C binary (~400 KB) managing Debian repositories. Berkeley DB for metadata, `pool/` hierarchy for deduplicated package storage. CLI tool — no daemon process. Static files served by any web server ([Debian Wiki](https://wiki.debian.org/DebianRepository/SetupWithReprepro)).

**Core positioning.** Lightweight Debian package repository management for PPAs, internal corporate distribution, and air-gapped APT repositories.

**Borrowable highlights.**
- *Extreme simplicity* — single binary, no runtime dependencies, no daemon. Trivially runs on any device.
- *Pool deduplication* — packages automatically deduplicated across distributions.

**Limitations.** No web UI (static directory listing only). APT-only. No API. No plugin system. Maintenance-mode activity.

### 3.4 ChartMuseum (Helm Chart Repository) — Deep Dive

**Architecture.** Go HTTP server with 11+ pluggable storage backends (local, S3, GCS, Azure, etcd). Dynamic `index.yaml` generation from storage contents. Multitenancy via `--depth` flag for nested repository paths ([GitHub](https://github.com/helm/chartmuseum), [docs](https://chartmuseum.com/docs/)).

**Core positioning.** Helm chart repository server for private chart hosting, CI/CD pipelines, and air-gapped Kubernetes deployments.

**Borrowable highlights.**
- *Dynamic index generation* — auto-scans storage and builds protocol-specific indexes. No manual index management.
- *Multitenancy via path depth* — elegant nested repository structure without separate instances.

**Limitations.** Helm-only protocol. In-memory index scales poorly with thousands of charts. No WebDAV/APT/PyPI.

### Comparison Table — Ops Tool Supply / Package Proxy

| Dimension | Athens | devpi | reprepro | ChartMuseum |
|---|---|---|---|---|
| **Positioning** | Go module proxy | Python packaging workflow + PyPI mirror | Debian repo creator | Helm chart repository |
| **File browsing UX** | Built-in web UI, 2–3 clicks | devpi-web plugin, search, 2–3 clicks | No UI (static files), CLI only | Separate UI project, 2–3 clicks |
| **Virtual view / index** | Protocol-level (GOPROXY) | Index inheritance with hierarchy | Distribution/component/architecture | Dynamic index.yaml generation |
| **Multi-protocol** | Go only | PyPI only | APT only | Helm only |
| **Access control** | Coarse (filter files, basic auth) | Fine-grained ACLs, per-index | None (web server dependent) | Basic auth or JWT |
| **Path hiding** | Download mode file | Index inheritance hides parents | None | None |
| **Share links** | No | No (index URLs) | No | No |
| **Metadata storage** | MongoDB/S3/filesystem | KeyFS (transactional KV) | Berkeley DB | In-memory or Redis |
| **Resource usage** | ~50–100 MB | ~50–100 MB | Trivial (~400 KB binary) | Moderate (scales with charts) |
| **Extensibility** | Storage drivers | Plugin system (entry points) | None | Storage/auth libraries |
| **Mobile** | No | No | No | No |
| **License & activity** | MIT, 4.7k ★, active | MIT, 1.2k ★, very active | GPL-2.0, ~64 ★, maintenance | Apache-2.0, 3.8k ★, active |

**Brief mentions:** *Linuxbrew (Homebrew on Linux)* — ARM64 Tier 1 as of 2025; client-side package manager, not a repository server. *asdf* — version manager with plugin architecture (25.5k ★, MIT); demonstrates extensibility patterns for crawler/protocol adapters.

---

## 4. NAS File Management

NAS operating systems manage local storage and expose it over standard protocols. They compete with MiBeeHive at the file-sharing layer but diverge sharply in scope — they are not supply-chain tools.

### 4.1 TrueNAS SCALE — Deep Dive

**Architecture.** Full Debian-based Linux distribution built around OpenZFS. Bare-metal ISO install or VM. Requires minimum 8 GB RAM (16–32 GB recommended), 16 GB+ boot SSD. ZFS metadata, checksumming, snapshots, RAIDZ, compression, dedup. Python middleware, Angular/TypeScript web UI ([TrueNAS docs](https://www.truenas.com/docs/)).

**Core positioning.** Enterprise-grade ZFS storage platform with ~400 Docker-based apps, KVM virtualization, and multi-protocol file serving (SMB, NFS, iSCSI, NVMe-oF).

**Borrowable highlights.**
- *WebShare (TrueNAS 26)* — new browser-based file sharing with Google Drive-like UI, shareable links, and TrueSearch content indexing. Relevant as a UX reference for MiBeeHive's artifact browser.
- *TrueSearch* — server-side content indexing for fast search across large file sets.

**Limitations.** 8 GB RAM minimum; completely unsuitable for 469 MB–1 GB devices. ARM64 is unofficial community port only. No APT/PyPI/Go/Helm/OCI serving. WebShare requires cloud account (TrueNAS Connect) for remote access.

### 4.2 OpenMediaVault — Deep Dive

**Architecture.** Debian Linux plugin adding NAS management to standard Debian. ExtJS/PHP web frontend. Plugin system for extensibility (~30 official, OMV-Extras community). Standard Linux storage (ext4, XFS, Btrfs, mdadm). OMV8 supports AMD64 and ARM64 ([OMV docs](https://docs.openmediavault.org/)).

**Core positioning.** Lightweight, extensible home NAS on Debian. Target: home users, SOHO, SBC enthusiasts (Raspberry Pi).

**Borrowable highlights.**
- *Plugin architecture* — install only what you need. Formalized plugin interfaces could inspire MiBeeHive's optional module system (APT serving, Go proxy, etc.).
- *ARM64 first-class support* — demonstrates a NAS OS running well on sub-2 GB ARM64 hardware.

**Limitations.** No built-in web file browser (requires FileBrowser plugin). No share links without external tools. No APT/PyPI/Go/Helm/OCI serving. No content indexing. Single admin model.

### 4.3 Unraid — Brief Mention

Proprietary Linux NAS OS. Custom parity array (not RAID). 4 GB RAM minimum, x86_64 only — no ARM64. 500K+ users, 3,700+ community apps. *User Shares* (virtual shares spanning multiple disks) are the closest NAS equivalent to a virtual-view abstraction. License: $49–$249 ([Unraid pricing](https://unraid.net/pricing)).

### Comparison Table — NAS File Management

| Dimension | TrueNAS SCALE | OpenMediaVault | Unraid |
|---|---|---|---|
| **Positioning** | Enterprise ZFS storage + apps + VMs | Lightweight home NAS on Debian | Flexible NAS + Docker + VM |
| **File browsing UX** | WebShare (v26, beta) + SMB/NFS | No built-in browser; FileBrowser plugin | No built-in browser; SMB/NFS |
| **Virtual view / index** | None — 1:1 dataset-to-share | None — 1:1 Shared Folder-to-mount | User Shares (virtual across disks) |
| **Multi-protocol** | SMB, NFS, iSCSI, NVMe-oF, WebDAV (app) | SMB, NFS, FTP, RSync, SSH, TFTP, WebDAV (plugin) | SMB, NFS (WebDAV via Docker) |
| **Access control** | POSIX/NFSv4 ACLs, AD/LDAP, RBAC (Enterprise), 2FA | Unix user/group, share privileges, 2FA (plugin) | User/group, share-level |
| **Path hiding** | Dataset isolation, SMB share ACLs | Share-level isolation | Share-level isolation |
| **Share links** | WebShare links (v26, beta) | None | None |
| **Metadata storage** | ZFS metadata + TrueSearch (v26) | SQLite (OMV6+), no indexing | Proprietary, no indexing |
| **Resource usage** | 8 GB RAM min, ARM64 unofficial | 512 MB RAM min, ARM64 first-class | 4 GB RAM min, x86_64 only |
| **Extensibility** | ~400 Docker apps, REST API, LXC (v26) | 30+ plugins, OMV-Extras, K3s | 3,700+ community apps |
| **Mobile** | Responsive web UI, TrueControl (3rd-party) | Responsive web UI | Responsive web UI |
| **License & activity** | GPL-3.0, ~2.6k ★, very active | GPL-3.0, ~6.9k ★, active | Proprietary, 500K+ users, active |

---

## 5. Implications for MiBeeHive

### Adopt — Specific ideas to integrate

| Idea | Source | Rationale |
|---|---|---|
| **Virtual repository / index abstraction** | JFrog Virtual Repositories, devpi Index Inheritance | Decouple physical storage from logical artifact views. MiBeeHive's "projects" concept and its live supply layer (APT + PyPI Simple indexes generated on demand over collected artifacts in `oss/`) implement exactly this. |
| **Multiple-source file browsing** | FileBrowser Quantum sources | Expose each module's storage (oss, os-install, webdav) as a configurable source with per-source permissions. Applicable to the WebDAV file browser redesign. |
| **Network fallback mode** | Athens `fallback` network mode | Storage-first, VCS-on-failure pattern ideal for edge deployments with intermittent connectivity. Apply to crawler retry logic. |
| **SingleFlight deduplication** | Athens SingleFlight wrapper | Prevent duplicate concurrent crawls for the same artifact. Already partially implemented in Go's `golang.org/x/sync/singleflight`. |
| **Dynamic index generation** | ChartMuseum auto-scan | Auto-scan storage and build protocol-specific indexes (APT `Packages.gz`, PyPI `index.html`, Go `@v/list`). No manual index management. |
| **Pool deduplication** | reprepro pool/ hierarchy | Content-addressed package storage with automatic dedup across distributions. Relevant for versioned binary artifacts. |
| **WebShare-style file browser** | TrueNAS WebShare | Browser-based file sharing with shareable links and content search. Inspiration for a polished artifact browser beyond raw WebDAV. |
| **Plugin architecture** | OMV plugin system, devpi entry points | Formalize plugin interfaces for crawlers, protocol adapters, and optional modules. |

### Borrow — Patterns to adapt

| Pattern | Adaptation |
|---|---|
| **Index inheritance (devpi)** | Hierarchical package views with reference-based inheritance. Allow "dev → staging → production" artifact promotion without copying files. |
| **Per-source permission granularity (FileBrowser)** | View/Download/Create/Modify/Delete per source. Replace MiBeeHive's simpler anonymous-read/admin-write if finer control is needed. |
| **Content-defined chunking (Seafile)** | Block-level dedup for versioned binary artifacts. Could significantly reduce storage for repeated GitHub releases with minor changes. |
| **SQLite real-time indexing (FileBrowser)** | Index files on access for instant search across collected artifacts without full filesystem scans. |
| **SeaSearch lightweight search (Seafile)** | ZincSearch-based alternative to Elasticsearch that runs on modest hardware. Applicable if MiBeeHive needs full-text search at scale. |

### Avoid — Pitfalls to dodge

| Pitfall | Source | Why Avoid |
|---|---|---|
| **Java/JVM dependency** | Artifactory, Nexus | 6–8 GB RAM minimum. Incompatible with 469 MB–1 GB ARM64 target. |
| **Multi-service deployment** | Nextcloud (PHP+MySQL+Redis), Seafile (C+Python+MariaDB+Memcached) | Operational complexity incompatible with single-binary deployment model. |
| **Enterprise feature lock-in** | Artifactory ($27K+/year), Nexus Pro ($135/month+) | Core features behind expensive tiers. MiBeeHive's open-source model is a differentiator. |
| **SaaS-only deployment** | GitHub Packages | No edge deployment possible. Incompatible with air-gapped/edge use cases. |
| **WebDAV as afterthought** | Seafile (disabled by default, slow) | WebDAV is a core MiBeeHive protocol — it must be first-class, not optional. |
| **Single-protocol focus** | Athens (Go only), devpi (PyPI only), reprepro (APT only), ChartMuseum (Helm only) | MiBeeHive's multi-protocol approach is its key differentiator. Do not narrow scope. |
| **No web file browser** | TrueNAS (pre-v26), OMV, reprepro | Users expect browse-and-download. Raw protocol endpoints are insufficient for admin UX. |

---

## Source Index

All URLs accessed August 2026.

| Source | URL |
|---|---|
| JFrog Documentation | https://docs.jfrog.com/ |
| JFrog Virtual Repos | https://docs.jfrog.com/artifactory/jfrog-platform/3.x-x/artifactory-user-manage-repositories |
| Sonatype Nexus Documentation | https://help.sonatype.com/ |
| Sonatype Pricing | https://www.sonatype.com/products/pricing |
| GitHub Packages Documentation | https://docs.github.com/en/packages |
| Nextcloud Documentation | https://docs.nextcloud.com/ |
| Nextcloud Developer Manual | https://docs.nextcloud.com/server/latest/developer_manual/ |
| FileBrowser Quantum | https://github.com/gtsteffaniak/filebrowser |
| Seafile Manual | https://manual.seafile.com/ |
| Athens GitHub | https://github.com/gomods/athens |
| Athens Documentation | https://docs.gomods.io/ |
| Grab Athens Engineering Blog | https://engineering.grab.com/go-module-proxy |
| devpi GitHub | https://github.com/devpi/devpi |
| devpi Documentation | https://devpi.net/docs/ |
| devpi Architecture (DeepWiki) | https://deepwiki.com/devpi/devpi/1.1-system-architecture |
| reprepro Debian Wiki | https://wiki.debian.org/DebianRepository/SetupWithReprepro |
| ChartMuseum GitHub | https://github.com/helm/chartmuseum |
| ChartMuseum Documentation | https://chartmuseum.com/docs/ |
| TrueNAS Documentation | https://www.truenas.com/docs/ |
| OpenMediaVault Documentation | https://docs.openmediavault.org/ |
| Unraid Pricing | https://unraid.net/pricing |
| rclone serve webdav | https://rclone.org/commands/rclone_serve_webdav/ |
| MinIO GitHub | https://github.com/minio/minio |
| Linuxbrew on Linux | https://docs.brew.sh/Homebrew-on-Linux |
| asdf GitHub | https://github.com/asdf-vm/asdf |
