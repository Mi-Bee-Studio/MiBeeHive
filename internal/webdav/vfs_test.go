package webdav

import (
	"bytes"
	"context"
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/net/webdav"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// setupVFS builds a fully-seeded VirtualFS + VirtualHandler over an in-memory
// database and a temp storage directory. It seeds:
//   - channel "public"
//   - readonly view "ro" with a folder "docs", a file_ref "tool.txt", and a
//     rule_folder "latest" (with one matched rule entry)
//   - writable view "rw"
func setupVFS(t *testing.T) (*VirtualFS, *sql.DB, string, []byte) {
	t.Helper()

	dbConn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { dbConn.Close() })
	if err := db.Migrate(dbConn); err != nil {
		t.Fatalf("db.Migrate: %v", err)
	}
	// Manual uploads register with project_id=0 (matching production ImportWebDAVFiles);
	// disable FK enforcement so the test DB mirrors production behavior.
	if _, err := dbConn.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
		t.Fatalf("disable foreign_keys: %v", err)
	}

	storagePath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(storagePath, "webdav", "manual_uploads"), 0o755); err != nil {
		t.Fatalf("mkdir manual_uploads: %v", err)
	}

	repo := db.NewVirtualRepo(dbConn)
	ctx := context.Background()

	channelID, err := repo.CreateChannel(ctx, &db.Channel{Slug: "public", Name: "Public", AuthMode: "anonymous"})
	if err != nil {
		t.Fatalf("create channel: %v", err)
	}
	roView, err := repo.CreateView(ctx, &db.View{Slug: "ro", Name: "ReadOnly", ChannelID: channelID, Mode: "curated", Writable: false})
	if err != nil {
		t.Fatalf("create ro view: %v", err)
	}
	rwView, err := repo.CreateView(ctx, &db.View{Slug: "rw", Name: "ReadWrite", ChannelID: channelID, Mode: "curated", Writable: true})
	if err != nil {
		t.Fatalf("create rw view: %v", err)
	}
	_ = rwView

	// Physical file for the file_ref node.
	fileContent := []byte("hello virtual webdav")
	filePath := filepath.Join(storagePath, "oss", "proj", "tool.txt")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("mkdir file dir: %v", err)
	}
	if err := os.WriteFile(filePath, fileContent, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO projects (name, display_name, source_type, source_url) VALUES ('proj', 'Proj', 'github', 'https://example.com')`); err != nil {
		t.Fatalf("insert project: %v", err)
	}
	res, err := dbConn.Exec(`INSERT INTO files
		(project_id, version, filename, os, arch, ext, size_bytes, download_url, local_path,
		 checksum, status, source_type, category, storage_subdir, public_token)
		VALUES (1, '1.0', 'tool.txt', '', '', 'txt', ?, '', 'oss/proj/tool.txt',
		        '', 'complete', 'github', 'github', 'oss', 'tok123')`, int64(len(fileContent)))
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}
	fileID, err := res.LastInsertId()
	if err != nil {
		t.Fatalf("last insert id: %v", err)
	}

	// Nodes in the readonly view.
	if _, err := repo.CreateNode(ctx, &db.Node{ViewID: roView, Name: "docs", NodeType: "folder", Status: "visible"}); err != nil {
		t.Fatalf("create folder node: %v", err)
	}
	if _, err := repo.CreateNode(ctx, &db.Node{ViewID: roView, Name: "tool.txt", NodeType: "file_ref", FileID: &fileID, Status: "visible"}); err != nil {
		t.Fatalf("create file_ref node: %v", err)
	}
	ruleConfig := `{"source_type":"manual"}`
	ruleID, err := repo.CreateNode(ctx, &db.Node{ViewID: roView, Name: "latest", NodeType: "rule_folder", RuleConfig: &ruleConfig, Status: "visible"})
	if err != nil {
		t.Fatalf("create rule_folder node: %v", err)
	}
	if _, err := dbConn.Exec(`INSERT INTO virtual_rule_entries (rule_node_id, file_id) VALUES (?, ?)`, ruleID, fileID); err != nil {
		t.Fatalf("insert rule entry: %v", err)
	}

	vpathSvc := service.NewVPathIndexService(dbConn, dbConn, slog.Default())
	fs := NewVirtualFS(dbConn, dbConn, storagePath, vpathSvc, repo, nil, slog.Default())
	return fs, dbConn, storagePath, fileContent
}

// newHandler builds the HTTP handler for a VirtualFS.
func newHandler(t *testing.T) (http.Handler, *sql.DB, string, []byte) {
	t.Helper()
	fs, dbConn, _, content := setupVFS(t)
	return NewVirtualHandler(fs), dbConn, "", content
}

func TestVFSImplementsFileSystem(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	var _ webdav.FileSystem = fs
}

func TestVFSStatViewRoot(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	fi, err := fs.Stat(context.Background(), "/public/ro")
	if err != nil {
		t.Fatalf("Stat view root: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("view root IsDir = false, want true")
	}
	if fi.Name() != "ro" {
		t.Errorf("view root name = %q, want %q", fi.Name(), "ro")
	}
}

func TestVFSStatFolder(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	fi, err := fs.Stat(context.Background(), "/public/ro/docs")
	if err != nil {
		t.Fatalf("Stat folder: %v", err)
	}
	if !fi.IsDir() {
		t.Errorf("folder IsDir = false, want true")
	}
}

func TestVFSStatFileRef(t *testing.T) {
	fs, _, _, content := setupVFS(t)
	fi, err := fs.Stat(context.Background(), "/public/ro/tool.txt")
	if err != nil {
		t.Fatalf("Stat file_ref: %v", err)
	}
	if fi.IsDir() {
		t.Errorf("file_ref IsDir = true, want false")
	}
	if fi.Size() != int64(len(content)) {
		t.Errorf("file_ref size = %d, want %d", fi.Size(), len(content))
	}
}

func TestVFSStatNotFound(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	if _, err := fs.Stat(context.Background(), "/public/ro/missing.txt"); !os.IsNotExist(err) {
		t.Errorf("Stat missing = %v, want os.ErrNotExist", err)
	}
	if _, err := fs.Stat(context.Background(), "/nope/ro"); !os.IsNotExist(err) {
		t.Errorf("Stat missing channel = %v, want os.ErrNotExist", err)
	}
}

func TestVFSReaddirListsVirtualNodes(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	f, err := fs.OpenFile(context.Background(), "/public/ro", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile view root: %v", err)
	}
	defer f.Close()
	infos, err := f.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir: %v", err)
	}
	names := map[string]bool{}
	for _, fi := range infos {
		names[fi.Name()] = true
	}
	for _, want := range []string{"docs", "tool.txt", "latest"} {
		if !names[want] {
			t.Errorf("Readdir missing %q (got %v)", want, names)
		}
	}
}

func TestVFSReaddirRuleFolderIncludesEntries(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	f, err := fs.OpenFile(context.Background(), "/public/ro/latest", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile rule folder: %v", err)
	}
	defer f.Close()
	infos, err := f.Readdir(-1)
	if err != nil {
		t.Fatalf("Readdir rule folder: %v", err)
	}
	found := false
	for _, fi := range infos {
		if fi.Name() == "tool.txt" {
			found = true
		}
	}
	if !found {
		t.Errorf("rule folder Readdir missing matched entry tool.txt (got %v)", infos)
	}
}

func TestVFSGetFileContent(t *testing.T) {
	h, _, _, content := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/public/ro/tool.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Errorf("GET body = %q, want %q", rec.Body.String(), string(content))
	}
}

func TestVFSPropfindDepth1(t *testing.T) {
	h, _, _, _ := newHandler(t)
	req := httptest.NewRequest("PROPFIND", "/public/ro/", nil)
	req.Header.Set("Depth", "1")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 207 {
		t.Fatalf("PROPFIND = %d, want 207", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"docs", "tool.txt", "latest"} {
		if !strings.Contains(body, want) {
			t.Errorf("PROPFIND body missing %q", want)
		}
	}
}

func TestVFSPropfindDepthInfinityTruncated(t *testing.T) {
	h, _, _, _ := newHandler(t)
	req := httptest.NewRequest("PROPFIND", "/public/ro/", nil)
	req.Header.Set("Depth", "infinity")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	// Must not hang or error; truncated to depth 1 → 207.
	if rec.Code != 207 {
		t.Fatalf("PROPFIND depth=infinity = %d, want 207", rec.Code)
	}
}

func TestVFSWriteDeniedOnReadonlyView(t *testing.T) {
	h, _, _, _ := newHandler(t)
	for _, method := range []string{http.MethodPut, http.MethodDelete, "MKCOL"} {
		req := httptest.NewRequest(method, "/public/ro/new.txt", bytes.NewReader([]byte("x")))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("%s on readonly view = %d, want 403", method, rec.Code)
		}
	}
}

func TestVFSWriteAllowedOnWritableView(t *testing.T) {
	fs, dbConn, _, _ := setupVFS(t)
	h := NewVirtualHandler(fs)
	content := []byte("uploaded via webdav")
	req := httptest.NewRequest(http.MethodPut, "/public/rw/upload.txt", bytes.NewReader(content))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("PUT writable = %d, want 201", rec.Code)
	}

	// The physical file must exist under manual_uploads.
	diskPath := filepath.Join(fs.storagePath, "webdav", "manual_uploads", "upload.txt")
	diskContent, err := os.ReadFile(diskPath)
	if err != nil {
		t.Fatalf("uploaded file not on disk: %v", err)
	}
	if !bytes.Equal(diskContent, content) {
		t.Errorf("disk content = %q, want %q", string(diskContent), string(content))
	}

	// The file should be registered in the files table.
	var count int
	if err := dbConn.QueryRow(`SELECT COUNT(*) FROM files WHERE local_path = 'webdav/manual_uploads/upload.txt'`).Scan(&count); err != nil {
		t.Fatalf("query files: %v", err)
	}
	if count != 1 {
		t.Errorf("registered file count = %d, want 1", count)
	}

	// The upload should now be visible through the virtual tree.
	req = httptest.NewRequest(http.MethodGet, "/public/rw/upload.txt", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET uploaded file = %d, want 200", rec.Code)
	}
	if !bytes.Equal(rec.Body.Bytes(), content) {
		t.Errorf("GET uploaded body = %q, want %q", rec.Body.String(), string(content))
	}
}

func TestVFSMkcolOnWritableView(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	h := NewVirtualHandler(fs)
	req := httptest.NewRequest("MKCOL", "/public/rw/newdir", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("MKCOL writable = %d, want 201", rec.Code)
	}
	dirPath := filepath.Join(fs.storagePath, "webdav", "manual_uploads", "newdir")
	if fi, err := os.Stat(dirPath); err != nil || !fi.IsDir() {
		t.Errorf("MKCOL dir not created on disk: %v", err)
	}
}

func TestVFSDeleteOnWritableView(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	h := NewVirtualHandler(fs)
	// Upload a file first.
	req := httptest.NewRequest(http.MethodPut, "/public/rw/del.txt", bytes.NewReader([]byte("to delete")))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("PUT = %d, want 201", rec.Code)
	}
	// Delete it.
	req = httptest.NewRequest(http.MethodDelete, "/public/rw/del.txt", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE = %d, want 204", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(fs.storagePath, "webdav", "manual_uploads", "del.txt")); !os.IsNotExist(err) {
		t.Errorf("deleted file still on disk: %v", err)
	}
}

func TestVFSGetNonExistent(t *testing.T) {
	h, _, _, _ := newHandler(t)
	req := httptest.NewRequest(http.MethodGet, "/public/ro/nope.txt", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("GET missing = %d, want 404", rec.Code)
	}
}

func TestVFSReadFileReaddirReturnsError(t *testing.T) {
	fs, _, _, _ := setupVFS(t)
	f, err := fs.OpenFile(context.Background(), "/public/ro/tool.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile file_ref: %v", err)
	}
	defer f.Close()
	if _, err := f.Readdir(-1); err == nil {
		t.Errorf("Readdir on file should return an error")
	}
}

func TestVFSReadStreamsContent(t *testing.T) {
	fs, _, _, content := setupVFS(t)
	f, err := fs.OpenFile(context.Background(), "/public/ro/tool.txt", os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("OpenFile: %v", err)
	}
	defer f.Close()
	got, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Errorf("read content = %q, want %q", string(got), string(content))
	}
}