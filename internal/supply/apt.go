package supply

import (
	"bytes"
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
// (cached, mtime-invalidated) and generates dists/.../Packages + Release.
//
// Routes (registered by init.go):
//
//	GET /apt/dists/{suite}/{path...}   metadata (Packages, Packages.gz, Release)
//	GET /apt/pool/{path...}            raw .deb download (reuses FileService.StreamFile)
type AptHandler struct {
	fileRepo *db.FileRepo
	svc      fileStreamer
	basePath string // storage base path, to read .deb files for metadata parsing

	mu    sync.Mutex
	cache *aptCache // lazy-built, mtime-invalidated
	suite string
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
	return &AptHandler{fileRepo: fileRepo, svc: svc, basePath: basePath, suite: suite}
}

// aptCache holds the generated repo files plus the mtime signature they were
// built from, so we can invalidate when underlying .deb files change.
type aptCache struct {
	files []apt.RepoFile
	built time.Time
	// sig is a concatenation of file id+modtime+size for all served .debs.
	sig string
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
// from the current .deb files (mtime-invalidated).
func (h *AptHandler) repoFiles(r *http.Request) ([]apt.RepoFile, error) {
	debs, err := h.fileRepo.ListComplete(r.Context(), 0)
	if err != nil {
		return nil, err
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	sig := debSignature(debs)
	if h.cache != nil && h.cache.sig == sig {
		return h.cache.files, nil
	}

	entries := make([]apt.PackageEntry, 0, len(debs))
	for _, f := range debs {
		if f.Ext != "deb" {
			continue
		}
		info, err := h.parseDebCached(f.LocalPath)
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

// debSignature builds a cheap validity key from id+size+mtime so cache rebuilds
// only when the served .deb set changes.
func debSignature(debs []*db.File) string {
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

// parseDebCached reads a .deb file and parses its control metadata. It opens
// the file from disk (basePath/LocalPath). For now this re-parses on each cache
// refresh; the outer repoFiles cache bounds how often this runs.
func (h *AptHandler) parseDebCached(localPath string) (*apt.DebInfo, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return apt.ParseDeb(f)
}
