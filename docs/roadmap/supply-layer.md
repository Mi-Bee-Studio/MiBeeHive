# Direction 7: Supply Layer — Serve Ops Tools to External Servers

## Status: ❌ Not Started

[中文](./supply-layer_zh.md)

## Goal

Turn MiBeeHive from a "download-and-store" tool into an **ops-tooling supply platform for external servers**: automatically collect and keep ops tools up to date, and expose them to external servers over **existing standard protocols**. The guiding principle: MiBeeHive is the supply chain (collect → store → serve); it implements existing protocols so off-the-shelf tools can pull from it without knowing MiBeeHive exists.

> **Boundary:** This is *not* an app store / site builder (that is 1Panel's domain, local machine). This direction is about **supplying ops tools to the fleet of external servers**, not running consumer apps on the box.

## Two Sub-Problems

This direction has two independent halves that compose:

1. **Crawl layer (collection/update)** — make collecting new sources cheap and extensible.
2. **Supply endpoints (serving)** — expose collected artifacts to external servers over standard protocols.

## Crawl Layer: Two-Track Architecture

Not every source is the same. Forcing one mechanism onto all sources is the root cause of "hard to extend". The plan is a **two-track** design:

| Track | Covers | Extension model |
|-------|--------|-----------------|
| **Rule fingerprint engine** | Single-page sources: GitHub Releases (JSON), structured JSON APIs (Crates.io, NPM), HTML listing pages, static directory listings | Add a source = add a YAML fingerprint (data-driven), like nmap adding a probe. Target: **built-in library + user-defined at runtime**. |
| **Protocol adapters** | Real-protocol sources with stateful endpoints: Go module proxy, PyPI, APT/YUM, OCI registry | Add a source = write a Go package implementing an adapter interface (code-driven). |

The existing `Crawler` interface (`FetchReleases(ctx, owner, repo)`) is GitHub-centric and is the friction point. The two-track design keeps the existing `CrawlManager`/`Scheduler` orchestration (it is sound) and only replaces how *sources are described and loaded*.

> **Validate before committing:** whether a YAML fingerprint can cover enough sources — especially multi-step ones (e.g. HashiCorp's two-level version→file directories) — is the biggest open question. It must be prototyped against real sources before any rewrite. See Issue #1.

## Supply Endpoints: Protocol Priority

Collected artifacts should be served to external servers over standard protocols, in this priority (by leverage ÷ effort):

| Batch | Protocol | Leverage | Effort | Notes |
|-------|----------|----------|--------|-------|
| **1** | Software tool repo + ISO repo + Helm repo (`index.yaml`) | Very high | **Low** | Natural extension of current Foraging (files already stored; only index/protocol layout is new). Ship first. |
| **2** | Go module proxy (`/goproxy/`) | Med-high | Med | `GOPROXY` protocol is clear; `golang` already collected. |
| **2** | PyPI `/simple` / NPM registry | Med | Med | Common in SOHO IT; pick one first. |
| **3** | APT / YUM repo layout | High | Med-high | Needs `Packages.gz`/`repomd.xml` metadata; install-scenario-critical but heavier. |
| **4** | OCI image registry | Very high | **Very heavy** | OCI distribution spec (blob/manifest/content addressing) is complex. Evaluate forwarding to an existing registry vs self-hosting. |

> `/metrics` is **out of scope** as a supply protocol — it stays for MiBeeHive's own health only. MiBeeHive supplies `node_exporter`/`prometheus` *to* external servers; it does not become a TSDB.

## Operations Model

- **Now (Track A — supply-first):** MiBeeHive is a repository + protocol endpoints. External servers pull from it passively. Low risk, high leverage.
- **Later (Track B — active ops):** MiBeeHive actively controls external servers via SSH/agent (deploy/update/inspect). Layered on top of a stable supply layer. Higher complexity; deferred.

## Sequencing (Validate-First)

1. **Prototype & validate** the rule fingerprint engine against 4 maximally-different real sources, end-to-end through to a generic file-repo supply endpoint. Produce an honest report. *(Issue #1)*
2. Based on the report, **decide**: incremental refactor of the crawl layer vs rewrite vs abandon rule engine. *(Issue #2)*
3. **First supply endpoints** (software repo + ISO repo + Helm). *(Issue #3)*
4. Subsequent protocol batches.

## Acceptance Criteria

- [ ] Rule engine prototype successfully covers single-page sources (JSON + HTML) with fingerprints; clear finding on multi-step sources.
- [ ] Fingerprint-collected artifacts flow through the existing download/store pipeline unchanged.
- [ ] At least one public supply endpoint serves collected artifacts to an external client over a standard protocol.
- [ ] Decision recorded: incremental refactor vs rewrite, with evidence.

## Out of Scope

- Modifying the main-line `Crawler` interface / DB schema / frontend before validation.
- Real protocols (Go proxy / PyPI / APT / OCI) — only a generic repo index in the validation phase.
- Web UI / hot-reload UI for fingerprints (validation runs via fixtures + CLI).
- Active remote control of external servers (Track B, deferred).
