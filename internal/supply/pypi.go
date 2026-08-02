package supply

import (
	"context"
	"net/http"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// PyPIHandler serves collected Python packages (wheels + sdists) as a PEP 503
// "Simple Repository API" index under /simple/. External Python hosts can then
// consume the collected packages with their native tooling:
//
//	pip install --index-url http://<host>/simple/ <pkg>
//	uv pip install --index-url http://<host>/simple/ <pkg>
//
// PEP 503 is a deliberately minimal protocol:
//
//	GET /simple/                → HTML index of all served project names
//	GET /simple/{project}/      → HTML index of that project's distribution files
//
// Each distribution link points at the generic file downloader (/repo/files/{id})
// so the bytes served are byte-for-byte identical to the admin path, and carries
// a #sha256=... fragment so clients verify the download.
//
// Project names come from the projects table (a PyPI-crawled project's Name is
// the package name). Names are normalized per PEP 503 (lowercase, runs of
// -_. collapsed to -) both in the URL and when matching.
type PyPIHandler struct {
	fileRepo  *db.FileRepo
	projectRepo projectLister
	svc       fileStreamer

	// basePublicURL is the host-relative base used to build per-file download
	// links (defaults to "/repo/files"). Kept configurable so tests can point it
	// at a known value without inspecting raw HTML.
	basePublicURL string

	// indexTTL bounds how often the project-name index is rebuilt. The project
	// list changes rarely (only on crawl/seed), so a short cache keeps /simple/
	// cheap without serving stale data for long.
	mu       sync.RWMutex
	index    *pypiIndexCache
	indexTTL time.Duration
	now      func() time.Time // injectable clock for tests
}

// projectLister is the subset of *db.ProjectRepo used by PyPIHandler, duck-typed
// so the concrete repo can be swapped in tests.
type projectLister interface {
	List(ctx context.Context) ([]*db.Project, error)
}

// NewPyPIHandler builds the PyPI Simple repository handler.
func NewPyPIHandler(fileRepo *db.FileRepo, projectRepo projectLister, svc fileStreamer) *PyPIHandler {
	return &PyPIHandler{
		fileRepo:      fileRepo,
		projectRepo:   projectRepo,
		svc:           svc,
		basePublicURL: "/repo/files",
		indexTTL:      30 * time.Second,
		now:           time.Now,
	}
}

// pypiIndexCache holds the built PEP 503 root index (list of project names) and
// the project_id → normalized-name map used to route per-project lookups.
type pypiIndexCache struct {
	html      []byte
	byProject map[int64]string // project_id → normalized pypi project name
	packages  map[string]bool  // normalized names present (for fast /simple/<p>/ existence)
	built     time.Time
}

// pypiFile is a served distribution file joined with its project's normalized name.
type pypiFile struct {
	fileID   int64
	filename string
	sha      string
	project  string // normalized name
}

// simplePrefix is the URL prefix this handler is mounted under. Serve derives
// the requested project from r.URL.Path relative to this prefix, so the handler
// works both behind the ServeMux ({rest...} capture) and when called directly in
// tests (which don't populate PathValue).
const simplePrefix = "/simple/"

// Serve is the mux entry point for /simple/{rest...}. It routes the root index
// vs. a per-project index based on the path after /simple/.
func (h *PyPIHandler) Serve(w http.ResponseWriter, r *http.Request) {
	// Prefer the mux capture when available; fall back to trimming the prefix
	// from the URL path so the handler is unit-testable without a ServeMux.
	rest := r.PathValue("rest")
	if rest == "" && strings.HasPrefix(r.URL.Path, simplePrefix) {
		rest = strings.TrimPrefix(r.URL.Path, simplePrefix)
	}
	rest = strings.Trim(rest, "/")

	if rest == "" {
		h.serveRootIndex(w, r)
		return
	}
	// Per-project index: /simple/{project}/ — rest may or may not have a trailing slash.
	h.serveProjectIndex(w, r, rest)
}

// serveRootIndex handles GET /simple/ — the PEP 503 root listing every project.
func (h *PyPIHandler) serveRootIndex(w http.ResponseWriter, r *http.Request) {
	idx, err := h.buildOrGetIndex(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "build pypi index", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(idx.html)
}

// serveProjectIndex handles GET /simple/{project}/ — the PEP 503 per-project
// page listing every served distribution file with a sha256 fragment.
func (h *PyPIHandler) serveProjectIndex(w http.ResponseWriter, r *http.Request, requested string) {
	idx, err := h.buildOrGetIndex(r.Context())
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "build pypi index", err)
		return
	}
	norm := normalizePyPIProject(requested)
	if !idx.packages[norm] {
		// PEP 503: unknown project → 404.
		http.NotFound(w, r)
		return
	}

	files, err := h.fileRepo.ListComplete(r.Context(), 0)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "list pypi files", err)
		return
	}
	var dists []pypiFile
	for _, f := range files {
		if !isPyPIDist(f.Filename) {
			continue
		}
		proj, ok := idx.byProject[f.ProjectID]
		if !ok || proj != norm {
			continue
		}
		dists = append(dists, pypiFile{
			fileID: f.ID, filename: f.Filename, sha: f.Checksum, project: norm,
		})
	}
	// Stable ordering for deterministic output.
	sort.Slice(dists, func(i, j int) bool { return dists[i].filename < dists[j].filename })

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(renderProjectIndex(norm, dists, h.basePublicURL))
}

// buildOrGetIndex returns the cached root index, rebuilding it if absent or
// older than indexTTL. Reads take an RLock; a rebuild takes the write lock with
// double-checked locking (mirrors the APT cache in apt.go).
func (h *PyPIHandler) buildOrGetIndex(ctx context.Context) (*pypiIndexCache, error) {
	h.mu.RLock()
	if h.index != nil && h.now().Sub(h.index.built) < h.indexTTL {
		idx := h.index
		h.mu.RUnlock()
		return idx, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.index != nil && h.now().Sub(h.index.built) < h.indexTTL {
		return h.index, nil
	}

	projects, err := h.projectRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	files, err := h.fileRepo.ListComplete(ctx, 0)
	if err != nil {
		return nil, err
	}

	byProject := make(map[int64]string)
	packages := make(map[string]bool)
	for _, p := range projects {
		if p.SourceType != string(model.SourceTypePyPI) {
			continue
		}
		byProject[p.ID] = normalizePyPIProject(p.Name)
	}
	// Only advertise packages that actually have at least one served file, so
	// /simple/ doesn't list empty projects that pip would then 404 on.
	servedProject := make(map[int64]bool)
	for _, f := range files {
		if isPyPIDist(f.Filename) {
			servedProject[f.ProjectID] = true
		}
	}
	for pid, name := range byProject {
		if servedProject[pid] {
			packages[name] = true
		}
	}

	idx := &pypiIndexCache{
		byProject: byProject,
		packages:  packages,
		built:     h.now(),
		html:      renderRootIndex(packages),
	}
	h.index = idx
	return idx, nil
}

// renderRootIndex builds the PEP 503 root HTML: one anchor per served project.
func renderRootIndex(packages map[string]bool) []byte {
	names := make([]string, 0, len(packages))
	for n := range packages {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n  <head>\n    <title>Simple Index</title>\n  </head>\n  <body>\n")
	for _, n := range names {
		// rel path is the normalized name + trailing slash, per PEP 503.
		b.WriteString("    <a href=\"")
		b.WriteString(escapeHTMLAttr(n))
		b.WriteString("/\">")
		b.WriteString(escapeHTMLText(n))
		b.WriteString("</a>\n")
	}
	b.WriteString("  </body>\n</html>\n")
	return []byte(b.String())
}

// renderProjectIndex builds the PEP 503 per-project HTML: one anchor per
// distribution file, with a #sha256=<hex> fragment so clients verify the fetch.
func renderProjectIndex(project string, dists []pypiFile, basePublicURL string) []byte {
	var b strings.Builder
	b.WriteString("<!DOCTYPE html>\n<html>\n  <head>\n    <title>Links for ")
	b.WriteString(escapeHTMLText(project))
	b.WriteString("</title>\n  </head>\n  <body>\n    <h1>Links for ")
	b.WriteString(escapeHTMLText(project))
	b.WriteString("</h1>\n")
	for _, d := range dists {
		url := basePublicURL + "/" + itoa(d.fileID) + "/" + escapeHTMLAttr(d.filename)
		b.WriteString("    <a href=\"")
		b.WriteString(url)
		if d.sha != "" {
			b.WriteString("#sha256=")
			b.WriteString(d.sha)
		}
		b.WriteString("\">")
		b.WriteString(escapeHTMLText(d.filename))
		b.WriteString("</a><br/>\n")
	}
	b.WriteString("  </body>\n</html>\n")
	return []byte(b.String())
}

// normalizePyPIProject applies the PEP 503 normalization: lowercase, then every
// run of [-_.] collapses to a single '-'. This is what /simple/<name>/ URLs and
// package-name comparison must use so "My_Pkg", "my-pkg", and "my.pkg" all match.
func normalizePyPIProject(name string) string {
	s := strings.ToLower(name)
	// Collapse runs of - _ . into a single -, preserving other characters.
	var b strings.Builder
	prevSep := false
	for _, r := range s {
		if r == '-' || r == '_' || r == '.' {
			if prevSep {
				continue
			}
			b.WriteRune('-')
			prevSep = true
			continue
		}
		b.WriteRune(r)
		prevSep = false
	}
	return strings.Trim(b.String(), "-")
}

// isPyPIDist reports whether a filename is a servable Python distribution
// (wheel or sdist). PyPI's packagine uses these exact suffixes.
func isPyPIDist(filename string) bool {
	low := strings.ToLower(filename)
	if strings.HasSuffix(low, ".whl") {
		return true
	}
	if strings.HasSuffix(low, ".tar.gz") || strings.HasSuffix(low, ".zip") {
		return true
	}
	if strings.HasSuffix(low, ".tar.bz2") || strings.HasSuffix(low, ".tar.xz") {
		return true
	}
	return false
}

// itoa is a small strconv-free int→string used by render helpers to avoid
// pulling strconv into this file (keeps the import list tight).
func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// escapeHTMLText escapes &, <, >, ' in text content.
func escapeHTMLText(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&#39;")
	return r.Replace(s)
}

// escapeHTMLAttr escapes characters that are special in a double-quoted href.
func escapeHTMLAttr(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&#34;", "'", "&#39;")
	// A project name in a URL path must also be path-safe; the PEP 503
	// normalized form already is, but be defensive against odd input.
	return path.Clean(r.Replace(s))
}
