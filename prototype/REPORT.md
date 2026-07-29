# Issue #1 — Validation Report: Rule-Fingerprint Crawl Engine

**Status:** ✅ Validated (prototype) — recommendation below.
**Date:** 2026-07-29

This report answers, with runnable code rather than speculation, whether a
declarative YAML fingerprint can describe real ops-tool sources — and where its
boundary lies. It is the evidence base for the Issue #2 architecture decision.

## TL;DR / Recommendation

**Proceed with the two-track design (Issue #2), incrementally — do NOT rewrite.**

A YAML fingerprint covers **single-page** sources (JSON APIs, HTML listings,
static dirs) cleanly and with far less code than per-source Go. It does **not**
cover **multi-step** sources (e.g. HashiCorp's version→file two-level pages).
That is not a flaw to engineer around — it is the natural seam between the two
tracks:

- Rule fingerprint engine → single-page sources (the majority: GitHub, Crates,
  NPM, Grafana, static download dirs).
- Protocol adapter (Go package) → multi-step / stateful sources (HashiCorp,
  Go proxy, PyPI, APT, OCI).

## What was built (all offline, fixture-based, no live network)

| File | Role |
|------|------|
| `internal/rulesrc/spec.go` | Fingerprint YAML schema (`Spec`) + `LoadSpec`/`ParseSpec` |
| `internal/rulesrc/jsonpath.go` | ~100-line stdlib JSON dot-path navigator (`[]`, `.assets[]`, `.tag_name`) |
| `internal/rulesrc/htmlpick.go` | `golang.org/x/net/html`-based `<a>` picker (already a dep, **no new dep**) |
| `internal/rulesrc/classify.go` | os/arch/ext detection (local copy of crawler logic) |
| `internal/rulesrc/fetcher.go` | Engine: `Spec → []model.ReleaseAsset`, reusing the existing output type |
| `internal/rulesrc/json.go` | json decode helper |
| `internal/rulesrc/fetcher_test.go` | 4-source unit tests (all green) |
| `internal/rulesrc/testdata/*.{json,html,yaml}` | Recorded fixtures |
| `internal/db/repo_file.go` | `FileRepo.ListComplete` (new, additive — status='complete') |
| `internal/supply/handler.go` | `GET /repo/index` + `GET /repo/files/{id}` (reuses `FileService.StreamFile`) |
| `prototype/main.go` | Closed-loop demo: fingerprint → fetch → FileRepo+FileService → serve |

**Zero new dependencies** — `golang.org/x/net` and `gopkg.in/yaml.v3` were
already present. This is itself a validation win (the project's "no new deps"
constraint survives the rule engine).

## Results by source type

| Source | Format | Covered by fingerprint? | Finding |
|--------|--------|-------------------------|---------|
| GitHub Releases | JSON | ✅ Clean | `[]` + `.assets[]` + skip `.prerelease`/`.draft` + `version_strip: v` |
| Crates.io | JSON | ✅ Clean | Flat `.versions[]`, each unit is its own asset |
| Apache autoindex | HTML | ✅ Clean | `selector: a` + `resolve: url` + `include: *.tar.gz` + version regex |
| **HashiCorp** | HTML (2-level) | ⚠️ **Partial — the boundary** | A single spec fetches the **version list only**; descending into each version's files needs a per-version follow-up request |

## Key findings

1. **Single-page sources are elegantly declarative.** GitHub, Crates, and
   Apache dir each needed one small YAML, not 150-330 lines of Go (the size of
   today's per-source crawler files). This is strong support for the rule track.

2. **The multi-step source is the real boundary.** HashiCorp requires N+1
   requests (1 version index + 1 per version). A declarative single-`request`
   spec fundamentally cannot express "for each result, fetch another URL".
   **Forcing it in would mutate the DSL into a scripting language** — the exact
   DSL-creep failure mode to avoid. Conclusion: multi-step sources belong in
   the **protocol-adapter** (code) track, not the rule track. This *confirms*
   the two-track split is correct and grounded in evidence.

3. **The existing `Crawler` interface cannot cheaply absorb rule sources.**
   `FetchReleases(ctx, owner, repo)` is GitHub-shaped (owner/repo args). Rule
   sources have no owner/repo — they have a URL + selectors. So Issue #2 should
   introduce a new, orthogonal abstraction: **`Source → Fetcher → Index`**
   (source definition; rule-fetcher OR protocol-adapter; output protocol).

4. **The pipeline tail is fully reusable, unchanged.** The prototype feeds the
   rule engine's `[]ReleaseAsset` straight into `FileRepo.Create` +
   `FileService.DownloadFile` — the exact path `CrawlManager.processAssets`
   uses. **No change to download/store/verify logic.** This means the crawl
   refactor is a *head* replacement, not a *pipeline* replacement: low risk,
   high payoff.

5. **Hand-written JSON/HTML helpers were enough.** ~100 lines of stdlib JSON
   navigation + `x/net/html` covered all single-page sources. No gojq/cascadia
   dependency needed at this stage. (A richer query lib can be revisited if real
   sources prove more complex; the prototype deliberately starts minimal.)

## Answers to the prototype's key questions

1. *Which sources clean vs force code?* — Single-page: clean. Multi-step
   (HashiCorp): forces code. → two-track.
2. *Does HashiCorp expose the DSL boundary?* — Yes, decisively.
3. *Can `Crawler` absorb rule sources?* — No; new `Source`/`Fetcher` abstraction
   needed (Issue #2).
4. *Need a query dependency?* — No, stdlib + `x/net/html` sufficed.

## Acceptance criteria (from roadmap)

- [x] Rule engine covers single-page sources (JSON + HTML) with fingerprints;
      clear finding on multi-step sources.
- [x] Fingerprint-collected artifacts flow through the existing pipeline unchanged.
- [x] A public supply endpoint serves collected artifacts to an external client
      (`GET /repo/index` + `GET /repo/files/{id}`).
- [x] Decision recorded: **incremental refactor (head replacement)**, with evidence.

## How to run

```bash
# Linux only (service layer uses syscall.Statfs).
go run ./prototype -spec prototype/fingerprints/github_releases.yaml -base ./tmp-proto
# then: curl http://127.0.0.1:9099/repo/index | jq
```

Tests are platform-independent and run everywhere:
```bash
go test ./internal/rulesrc/
```

## Non-goals achieved (kept out of scope)

- No change to main-line `Crawler` interface, DB schema, or frontend.
- No real protocols (Go proxy/PyPI/APT/OCI) — generic repo index only.
- No Web UI / hot-reload — fixtures + CLI.

## Next step

Issue #2: design the `Source → Fetcher → Index` abstraction, keep
`CrawlManager`/`Scheduler`, migrate single-page crawlers to fingerprints, and
add a protocol-adapter track for multi-step sources. **Incremental, not rewrite.**
