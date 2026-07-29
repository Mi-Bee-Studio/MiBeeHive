# Design: Two-Track Crawl Layer (Source / Fetcher / Index)

**Issue:** #2 — Crawl-layer two-track refactor
**Status:** Design (not yet implemented)
**Depends on:** #1 (validated, merged) — see `prototype/REPORT.md`
**Scope:** incremental refactor of the crawl *head*; the pipeline *tail* (download/store/verify) is unchanged.

## Problem

The current `Crawler` interface is GitHub-shaped and brittle:

```go
type Crawler interface {
    Name() string
    SourceType() model.SourceType
    FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) // ← owner/repo
}
```

1. **`owner/repo` are positional strings overloaded per source.** GitHub uses them literally; Go ignores both; HashiCorp treats `owner` as a product name; NPM/Crates/PyPI treat `repo` as a package name. There is no type safety — a misconfigured project silently mis-crawls.
2. **Each source is ~150–330 lines of Go compiled into the binary.** Adding a source requires code + recompile. Issue #1 proved single-page sources are fully describable by a YAML fingerprint.
3. **`ProjectSettings` is GitHub-baked** (`GitHubOwner`/`GitHubRepo` fields), even for non-GitHub sources.
4. **Latent bug — DB CHECK constraint mismatch.** `001_init.sql` and `017_consolidated.sql` constrain `projects.source_type` to `'github','go','hashicorp','grafana'`, but the code registers 7 crawlers including `npm/pypi/crates`. An npm/pypi/crates project INSERT silently fails the CHECK. (This was the CI failure in #4.)

## Target: three orthogonal concepts

Decouple "what a source is" from "how it's fetched" from "how its output is served." (The nmap analogy: a probe description vs the scanning engine vs the reported result.)

```
 Source  ──uses──►  Fetcher  ──produces──►  []Asset  ──served via──►  Index
(definition)     (rule OR adapter)         (ReleaseAsset)            (protocol)
```

- **Source** — a persisted record: name, source-type, params (URL, owner/repo, or a fingerprint ref). Replaces the implicit "project + ProjectSettings + SourceType" tuple.
- **Fetcher** — two implementations behind one interface:
  - *RuleFetcher* — runs a YAML fingerprint (the `internal/rulesrc` engine from #1). Covers single-page sources. **Adding a source = adding YAML, no recompile.**
  - *ProtocolAdapter* — a Go package for multi-step / stateful sources (HashiCorp two-level, future Go proxy/PyPI/APT/OCI).
- **Asset** — unchanged: `model.ReleaseAsset`. The pipeline tail (`FileRepo.Create` + `FileService.DownloadFile`) is untouched.

## The new interface

```go
// Fetcher fetches assets for a Source. Two implementations exist:
// a rule-fingerprint engine and protocol adapters (code).
type Fetcher interface {
    Fetch(ctx context.Context, src Source) ([]model.ReleaseAsset, error)
    // Sources returns the source-type keys this Fetcher handles.
    Sources() []string
}

// Source is the resolved definition of a crawlable source.
type Source struct {
    Name    string            // project name
    Type    string            // "github" | "rulesrc" | "hashicorp" | ... (free-form key into the registry)
    Params  map[string]string // type-specific: {"owner","repo"} or {"fingerprint":"prometheus"} or {"url":...}
}
```

Key changes from today:
- **`Fetch(ctx, Source)` replaces `FetchReleases(ctx, owner, repo)`.** No positional overloading — `Params` is an explicit map. This is the type-safety win.
- **`Source.Type` is a free-form registry key, not a closed enum + DB CHECK.** This removes the latent bug (no CHECK constraint can be violated) and lets `rulesrc` be a first-class type.

## Migration strategy (incremental, no big-bang)

The refactor is staged so main stays green at every step. Order matters.

### Step 1 — Fix the latent CHECK-constraint bug (small, do first)
The npm/pypi/crates crawlers exist but their projects can't be inserted. Add migration `021_source_type_any.sql`:
```sql
-- Drop the closed CHECK; source_type is now a free-form registry key (e.g.
-- 'github','go','hashicorp','grafana','npm','pypi','crates','rulesrc',...).
-- Validation moves to the Fetcher registry (does a Fetcher handle this type?).
CREATE TABLE projects_new AS SELECT * FROM projects;  -- SQLite table-rebuild pattern
-- (rebuild without the CHECK; copy data; swap names) — see 017_consolidated for the precedent.
```
This unblocks existing crawlers AND the new `rulesrc` type. **Never modify 001_init.sql**; always a new numbered migration.

### Step 2 — Introduce the `Fetcher` + `Source` abstraction (additive)
- New package `internal/source/` with `Fetcher` interface, `Source` type, and a `Registry` (map type→Fetcher), mirroring today's `Scheduler.crawlers` map but keyed on the new `Source.Type`.
- Do NOT delete `Crawler` yet. The 7 existing crawlers get a **thin adapter** that implements `Fetcher` by translating `Source.Params` → their existing constructor args, then delegates to `FetchReleases`. This is ~10 lines per crawler.

### Step 3 — Route `CrawlManager` through `Fetcher`
- `CrawlManager.resolveCrawlSetup` calls `Registry.Get(src.Type)` instead of `Scheduler.GetCrawler`.
- `fetchReleases` calls `fetcher.Fetch(ctx, src)` instead of `FetchReleases(ctx, owner, repo)`.
- `getOwnerRepo` is replaced by `Source.Params` construction from `ProjectSettings` (a transitional mapper).
- **`Scheduler` orchestration (timers, goroutines, per-project locks) is untouched** — it was validated as sound in #1.

### Step 4 — Migrate single-page crawlers to rule fingerprints
- Migrated to fingerprints (simple array/single-object JSON, GitHub-style assets):
  - **github** → `fingerprints/github.yaml` (array `releases?per_page=10`)
  - **grafana** → `fingerprints/grafana.yaml` (single-object `releases/latest`; required an engine extension for `list.path: "."` to treat the root object as one unit)
- **Kept as LegacyAdapters** (NOT migrated — their logic is too complex for a declarative fingerprint, confirming the #1 boundary):
  - **go** — reads per-file OS/Arch from the response (not from filename), filters by `kind`, builds templated URLs, carries SHA256
  - **npm** — scoped packages (`@scope/name`), skips pre-release (`-`), sorts/limits versions, extracts filename from tarball URL
  - **pypi** — complex wheel/tarball + platform filtering
  - **crates** — multi-step (tries GitHub binaries, falls back to `.crate` source)
  - **hashicorp** — multi-step (version dir → file dir)
  - Forcing any of these into a fingerprint would mutate the DSL into a scripting language — the exact failure mode #1 warned against.

### Step 5 — Retire `Crawler` interface
- **Deferred (cannot complete yet):** the LegacyAdapters in Step 4 still wrap `crawler.Crawler`, so the interface is still referenced. It can only be deleted once those adapters are either (a) rewritten as standalone `ProtocolAdapter` Go packages that don't implement `Crawler`, or (b) the sources are dropped. This is a later cleanup, not blocking.

## What does NOT change
- `model.ReleaseAsset` — sole output type, unchanged.
- `CrawlManager` orchestration: `processAssets`, `finalizeCrawl`, per-project mutex, retry — untouched.
- `Scheduler` timer/goroutine lifecycle — untouched.
- Download/store pipeline: `FileRepo`, `FileService` — untouched.
- `model.CrawlResult`, handler layer — untouched (they consume `ReleaseAsset`).
- All existing tests for the *pipeline* remain valid.

## Risks & mitigations
- **Behavioral drift during migration** → Step 2's adapter delegates to existing `FetchReleases`, so output is byte-identical to today. Tests comparing before/after per source catch drift.
- **Test schemas duplicate the CHECK** (`manager_test.go:77`, `migration_service_test.go:34`) → update these in Step 1 so tests reflect the new schema.
- **`ProjectSettings` GitHub fields** → keep them in Step 2-3 (transitional mapper reads them into `Source.Params`); they can be deprecated once fingerprints carry their own params.

## Open questions (resolve during Step 2)
- Where do user-defined fingerprints live at runtime? (Issue #1 deferred this.) Proposal: a `source_fingerprints` table + embedded built-ins; out of scope for the structural refactor, can land after.
- Does `Source.Params` need typed schemas per type, or is `map[string]string` enough for now? (Lean: map now, typed later if it hurts.)

## Acceptance criteria (from roadmap)
- [ ] A new single-page source can be added **without compiling** (YAML fingerprint).
- [ ] npm/pypi/crates projects can be inserted (CHECK bug fixed).
- [ ] Existing crawlers still work during migration; no main-line regression.
- [ ] Multi-step sources (HashiCorp) plug in as a ProtocolAdapter.
- [ ] `Scheduler`/`CrawlManager` orchestration unchanged.
