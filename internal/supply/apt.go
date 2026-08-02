package supply

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/apt"
	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// AptHandler serves the collected .deb files as a standard Debian APT
// repository under /apt/. It parses each .deb's control metadata on demand
// (per-file memoized, mtime-invalidated) and generates dists/.../Packages +
// Release.
//
// Cache concurrency: metadata reads take an RLock so warm requests don't
// serialize; only a cache miss (rebuild) takes the write lock. Each .deb's
// parsed DebInfo is memoized keyed by id+mtime+size, so a rebuild triggered by
// one changed file only re-parses that file — unchanged files reuse the cached
// DebInfo.
//
// Routes (registered by init.go):
//
//	GET /apt/dists/{suite}/{path...}   metadata (Packages, Packages.gz, Release)
//	GET /apt/pool/{path...}            raw .deb download (reuses FileService.StreamFile)
type AptHandler struct {
	fileRepo *db.FileRepo
	svc      fileStreamer
	basePath string // storage base path, to read .deb files for metadata parsing

	mu      sync.RWMutex
	cache   *aptCache // lazy-built, mtime-invalidated
	debMemo *debMemo  // per-file DebInfo memo, keyed by id+mtime+size
	suite   string
}

// fileStreamer is the StreamFile subset of *service.FileService (duck-typed to
// avoid importing service here where avoidable; the concrete type is passed in).
type fileStreamer interface {
	StreamFile(w http.ResponseWriter, file *model.File) error
}

// NewAptHandler builds the APT repository handler.
func NewAptHandler(fileRepo *db.FileRepo, svc fileStreamer, basePath, suite string) *AptHandler {
	if suite == "" {
		suite = "stable"
	}
	return &AptHandler{fileRepo: fileRepo, svc: svc, basePath: basePath, suite: suite, debMemo: newDebMemo()}
}

// aptCache holds the generated repo files plus the signature they were built
// from, so we can invalidate when underlying .deb files change.
type aptCache struct {
	files []apt.RepoFile
	built time.Time
	// sig is a concatenation of file id+mtime+size+filename for all served .debs.
	sig string
}

// debMemo caches parsed control metadata per served .deb so a cache rebuild only
// re-parses files whose signature entry actually changed. The key is id|mtime|size,
// which uniquely identifies a file's contents even across in-place replacement
// (same id/filename, new bytes → new mtime). The mutex guards the map; entries
// are immutable so readers do not need to copy.
type debMemo struct {
	mu  sync.Mutex
	now func() time.Time // injectable clock for tests
	get func(path string) (time.Time, int64, error) // injectable stat for tests
	entries map[string]*debMemoEntry
}

type debMemoEntry struct {
	info   *apt.DebInfo
	parseC int // number of times this entry's file was parsed (test hook)
}

func newDebMemo() *debMemo {
	return &debMemo{
		now:     time.Now,
		get:     fileModtimeSize,
		entries: make(map[string]*debMemoEntry),
	}
}

// Serve is the mux entry point for /apt/{rest...}. It dispatches metadata vs
// pool downloads.
func (h *AptHandler) Serve(w http.ResponseWriter, r *http.Request) {
	rest := r.PathValue("rest")
	if rest == "" {
		http.NotFound(w, r)
		return
	}

	// Pool: raw package download -> resolve by filename and stream.
	if strings.HasPrefix(rest, "pool/") {
		h.servePool(w, r, strings.TrimPrefix(rest, "pool/"))
		return
	}

	// Metadata: dists/.../* -> serve generated file.
	files, err := h.repoFiles(r)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "build apt repo", err)
		return
	}
	// The generated paths are relative (e.g. dists/stable/...); rest already
	// includes that prefix since the route is /apt/{rest...}.
	for i := range files {
		if files[i].Path == rest {
			w.Header().Set("Content-Type", files[i].ContentType)
			_, _ = w.Write(files[i].Content)
			return
		}
	}
	http.NotFound(w, r)
}

// servePool streams a .deb by matching its filename against the files table.
func (h *AptHandler) servePool(w http.ResponseWriter, r *http.Request, poolPath string) {
	// poolPath like "main/n/node-exporter/node-exporter_1.8.2-1_amd64.deb".
	name := path.Base(poolPath)
	files, err := h.fileRepo.ListComplete(r.Context(), 0)
	if err != nil {
		middleware.WriteError(w, http.StatusInternalServerError, model.ERR_INTERNAL, "list deb files", err)
		return
	}
	for _, f := range files {
		if f.Ext == "deb" && f.Filename == name {
			mFile := &model.File{
				ID: f.ID, ProjectID: int(f.ProjectID), Version: f.Version,
				Filename: f.Filename, OS: f.OS, Arch: f.Arch, Ext: f.Ext,
				SizeBytes: f.SizeBytes, LocalPath: f.LocalPath, Checksum: f.Checksum,
				Status: model.FileStatus(f.Status),
			}
			_ = h.svc.StreamFile(w, mFile)
			return
		}
	}
	http.NotFound(w, r)
}

// repoFiles returns the generated APT metadata, building/refreshing the cache
// from the current .deb files (mtime-invalidated). Warm reads take only an
// RLock; a cache miss takes the write lock for a single rebuild.
func (h *AptHandler) repoFiles(r *http.Request) ([]apt.RepoFile, error) {
	debs, err := h.fileRepo.ListComplete(r.Context(), 0)
	if err != nil {
		return nil, err
	}

	// Cheap fast path: build the signature under no lock first, then RLock to
	// compare. If it matches the cached signature, serve without ever taking
	// the write lock, so concurrent metadata reads don't serialize.
	sig, err := h.debSignature(debs)
	if err != nil {
		// A stat failure (e.g. a file removed between list and stat) should not
		// 500 the whole repo; fall back to a no-mtime signature so the rebuild
		// still proceeds and the missing file is simply not indexed.
		sig = debSignatureNoMtime(debs)
	}

	h.mu.RLock()
	if h.cache != nil && h.cache.sig == sig {
		files := h.cache.files
		h.mu.RUnlock()
		return files, nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	// Re-check under the write lock: another goroutine may have rebuilt while we
	// were waiting. This is the classic double-checked locking pattern.
	if h.cache != nil && h.cache.sig == sig {
		return h.cache.files, nil
	}

	entries := make([]apt.PackageEntry, 0, len(debs))
	for _, f := range debs {
		if f.Ext != "deb" {
			continue
		}
		info, err := h.debMemo.parse(f.LocalPath, nil)
		if err != nil {
			// Skip unparseable debs rather than failing the whole repo; they are
			// still downloadable via /repo/files but excluded from the index.
			continue
		}
		entries = append(entries, apt.PackageEntry{
			DebInfo:  *info,
			Filename: poolPathFor(info, f.Filename),
			Size:     f.SizeBytes,
			SHA256:   f.Checksum,
		})
	}
	if len(entries) == 0 {
		// Empty repo still returns a valid (empty) Packages so apt update works.
		entries = []apt.PackageEntry{}
	}

	files, err := apt.GenerateRepo(entries, h.suite, time.Now())
	if err != nil {
		return nil, err
	}
	h.cache = &aptCache{files: files, built: time.Now(), sig: sig}
	return files, nil
}

// debSignature builds the cache validity key from id+mtime+size+filename for
// all served .debs. The on-disk mtime is what makes an in-place replacement
// (same id/size/filename, new bytes) invalidate the cache: the DB row is
// unchanged but the file's mtime advances. A stat failure for a single file is
// returned so the caller can fall back to a mtime-less signature rather than
// aborting the whole index.
func (h *AptHandler) debSignature(debs []*db.File) (string, error) {
	var b bytes.Buffer
	for _, f := range debs {
		if f.Ext != "deb" {
			continue
		}
		mtime, _, err := h.debMemo.get(f.LocalPath)
		if err != nil {
			return "", fmt.Errorf("stat %s: %w", f.LocalPath, err)
		}
		b.WriteString(strconv.FormatInt(f.ID, 10))
		b.WriteByte('|')
		b.WriteString(strconv.FormatInt(mtime.UnixNano(), 10))
		b.WriteByte('|')
		b.WriteString(strconv.FormatInt(f.SizeBytes, 10))
		b.WriteByte('|')
		b.WriteString(f.Filename)
		b.WriteByte(';')
	}
	return b.String(), nil
}

// debSignatureNoMtime is the fallback signature when mtime cannot be obtained
// (e.g. a transient stat error). It keeps id+size+filename so the index still
// rebuilds on real set changes, at the cost of not detecting in-place byte
// replacement until the next successful stat.
func debSignatureNoMtime(debs []*db.File) string {
	var b bytes.Buffer
	for _, f := range debs {
		if f.Ext == "deb" {
			b.WriteString(strconv.FormatInt(f.ID, 10))
			b.WriteByte('|')
			b.WriteString(strconv.FormatInt(f.SizeBytes, 10))
			b.WriteByte('|')
			b.WriteString(f.Filename)
			b.WriteByte(';')
		}
	}
	return b.String()
}

// poolPathFor builds the APT pool path for a package: pool/main/<first>/<pkg>/<filename>.
func poolPathFor(info *apt.DebInfo, filename string) string {
	pkg := info.Package
	first := "x"
	if pkg != "" {
		first = string(pkg[0])
		if strings.HasPrefix(pkg, "lib") && len(pkg) > 3 {
			first = pkg[:4]
		}
	}
	return "pool/main/" + first + "/" + pkg + "/" + filename
}

// parse reads and parses a .deb's control metadata, returning a cached result
// when the file has not changed since the last parse. The cache key is
// id|mtime|size derived from the on-disk stat, so in-place content replacement
// (new mtime) forces a fresh parse while unchanged files reuse the memoized
// DebInfo. parseCountP, when non-nil, receives the running parse count for the
// file (test hook for #22's "only re-parse changed files" acceptance).
func (m *debMemo) parse(localPath string, parseCountP *int) (*apt.DebInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mtime, size, err := m.get(localPath)
	if err != nil {
		return nil, err
	}
	key := debMemoKey(localPath, mtime, size)
	if e, ok := m.entries[key]; ok {
		if parseCountP != nil {
			*parseCountP = e.parseC
		}
		return e.info, nil
	}

	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	info, err := apt.ParseDeb(f)
	if err != nil {
		return nil, err
	}
	// Evict stale entries for this path (different mtime) so the memo does not
	// grow unbounded across many in-place replacements.
	for k := range m.entries {
		if pathFromKey(k) == localPath {
			delete(m.entries, k)
		}
	}
	e := &debMemoEntry{info: info, parseC: 1}
	m.entries[key] = e
	if parseCountP != nil {
		*parseCountP = e.parseC
	}
	return info, nil
}

func debMemoKey(path string, mtime time.Time, size int64) string {
	return path + "|" + strconv.FormatInt(mtime.UnixNano(), 10) + "|" + strconv.FormatInt(size, 10)
}

func pathFromKey(key string) string {
	if i := strings.LastIndexByte(key, '|'); i > 0 {
		// size is the last segment; mtime is the second-to-last.
		key = key[:i]
		if j := strings.LastIndexByte(key, '|'); j > 0 {
			return key[:j]
		}
	}
	return key
}

// fileModtimeSize returns the on-disk modification time and size for a file.
func fileModtimeSize(path string) (time.Time, int64, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return time.Time{}, 0, err
	}
	return fi.ModTime(), fi.Size(), nil
}
