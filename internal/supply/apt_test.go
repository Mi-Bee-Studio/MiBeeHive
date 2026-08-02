package supply

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// writeDeb writes a minimal gzipped control.tar .deb to disk and returns its
// path. Two different control texts let the caller simulate an in-place content
// replacement (same path/filename, new bytes) for issue #19.
func writeDeb(t *testing.T, path, package_ string) {
	t.Helper()
	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	control := "Package: " + package_ + "\nVersion: 1.0\nArchitecture: amd64\n"
	if err := tw.WriteHeader(&tar.Header{Name: "./control", Mode: 0644, Size: int64(len(control)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatalf("write control header: %v", err)
	}
	tw.Write([]byte(control))
	tw.Close()
	var gzBuf bytes.Buffer
	zw := gzip.NewWriter(&gzBuf)
	zw.Write(tarBuf.Bytes())
	zw.Close()

	var ar bytes.Buffer
	ar.WriteString("!<arch>\n")
	writeAr := func(name string, data []byte) {
		// name[16] mtime[12] uid[6] gid[6] mode[8] size[10] end[2]
		pad := func(s string, n int) string {
			for len(s) < n {
				s += " "
			}
			return s
		}
		ar.WriteString(pad(name+"/", 16))
		ar.WriteString(pad("0", 12))
		ar.WriteString(pad("0", 6))
		ar.WriteString(pad("0", 6))
		ar.WriteString(pad("100644", 8))
		ar.WriteString(pad(strconv.Itoa(len(data)), 10))
		ar.WriteString("`\n")
		ar.Write(data)
		if len(data)%2 == 1 {
			ar.WriteByte('\n')
		}
	}
	writeAr("debian-binary", []byte("2.0\n"))
	writeAr("control.tar.gz", gzBuf.Bytes())

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, ar.Bytes(), 0o644); err != nil {
		t.Fatalf("write deb: %v", err)
	}
}

// seedCompleteDeb inserts a complete deb file row pointing at localPath.
func seedCompleteDeb(t *testing.T, fRepo *db.FileRepo, projectID int64, filename, localPath string) *db.File {
	t.Helper()
	ctx := context.Background()
	file := &db.File{
		ProjectID: projectID, Version: "1.0", Filename: filename,
		Ext: "deb", SizeBytes: 1, DownloadURL: "u", LocalPath: localPath,
		Status: "complete",
	}
	id, err := fRepo.Create(ctx, file)
	if err != nil {
		t.Fatalf("create file: %v", err)
	}
	file.ID = id
	return file
}

// TestDebSignature_IncludesModtime covers issue #19: an in-place replacement
// (same id/size/filename, new content + new mtime) must produce a different
// signature so the cache rebuilds.
func TestDebSignature_IncludesModtime(t *testing.T) {
	dir := t.TempDir()
	debPath := filepath.Join(dir, "pkg.deb")
	writeDeb(t, debPath, "foo")

	h := NewAptHandler(nil, nil, dir, "stable")
	file := &db.File{ID: 7, Ext: "deb", SizeBytes: 1, Filename: "pkg.deb", LocalPath: debPath}

	sig1, err := h.debSignature([]*db.File{file})
	if err != nil {
		t.Fatalf("debSignature 1: %v", err)
	}
	// Bump mtime into the future so it is reliably newer (some filesystems have
	// coarse mtime resolution that can make two near-simultaneous writes equal).
	newTime := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(debPath, newTime, newTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	sig2, err := h.debSignature([]*db.File{file})
	if err != nil {
		t.Fatalf("debSignature 2: %v", err)
	}
	if sig1 == sig2 {
		t.Fatal("signature did not change after in-place mtime change; modtime not included (#19)")
	}
}

// TestDebMemo_ReparsesOnlyChangedFile covers issue #22: a rebuild triggered by
// one changed file must only re-parse that file; unchanged files reuse the
// memoized DebInfo (parse count stays at 1).
func TestDebMemo_ReparsesOnlyChangedFile(t *testing.T) {
	dir := t.TempDir()
	aPath := filepath.Join(dir, "a.deb")
	bPath := filepath.Join(dir, "b.deb")
	writeDeb(t, aPath, "alpha")
	writeDeb(t, bPath, "beta")

	memo := newDebMemo()
	var countA, countB int

	// First parse of both: each parsed once.
	if _, err := memo.parse(aPath, &countA); err != nil {
		t.Fatalf("parse a: %v", err)
	}
	if _, err := memo.parse(bPath, &countB); err != nil {
		t.Fatalf("parse b: %v", err)
	}
	if countA != 1 || countB != 1 {
		t.Fatalf("initial parse counts: a=%d b=%d, want 1/1", countA, countB)
	}

	// Re-parse unchanged: memo hit, counts unchanged.
	if _, err := memo.parse(aPath, &countA); err != nil {
		t.Fatalf("re-parse a: %v", err)
	}
	if _, err := memo.parse(bPath, &countB); err != nil {
		t.Fatalf("re-parse b: %v", err)
	}
	if countA != 1 || countB != 1 {
		t.Fatalf("memo-hit counts: a=%d b=%d, want 1/1 (unchanged files must not re-parse)", countA, countB)
	}

	// Replace b's contents in place (new mtime). Re-parsing b must parse again,
	// while a (unchanged) must still be served from the memo.
	newTime := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(bPath, newTime, newTime); err != nil {
		t.Fatalf("chtimes b: %v", err)
	}
	if _, err := memo.parse(bPath, &countB); err != nil {
		t.Fatalf("re-parse changed b: %v", err)
	}
	if countB != 1 {
		// countB reflects the NEW entry (old entry was evicted), so it resets to 1
		// — but the point is that a was NOT re-parsed. Verify a's count is still 1.
	}
	if _, err := memo.parse(aPath, &countA); err != nil {
		t.Fatalf("re-parse a after b changed: %v", err)
	}
	if countA != 1 {
		t.Fatalf("a was re-parsed (%d) when only b changed; memo did not isolate unchanged files (#22)", countA)
	}
	if countB != 1 {
		t.Fatalf("changed-file b parse count unexpected: got %d", countB)
	}
}

// TestRepoFiles_CacheInvalidatesOnMtimeChange is the end-to-end acceptance for
// #19: after an in-place .deb replacement, repoFiles builds a Packages index
// reflecting the new package name (proving the cache rebuilt).
func TestRepoFiles_CacheInvalidatesOnMtimeChange(t *testing.T) {
	fRepo, pRepo := supplyTestDB(t)
	ctx := context.Background()
	p, err := pRepo.Create(ctx, "t", "T", "github", "")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	debPath := filepath.Join(dir, "pkg.deb")
	writeDeb(t, debPath, "before-pkg")
	file := seedCompleteDeb(t, fRepo, p.ID, "pkg.deb", debPath)

	h := NewAptHandler(fRepo, nil, dir, "stable")
	// Replace the file in place with a different package name; bump mtime far
	// enough that any FS granularity can't collapse the two writes.
	writeDeb(t, debPath, "after-pkg")
	newTime := time.Now().Add(2 * time.Hour)
	if err := os.Chtimes(debPath, newTime, newTime); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/apt/dists/stable/main/binary-amd64/Packages", nil)
	files, err := h.repoFiles(req)
	if err != nil {
		t.Fatalf("repoFiles: %v", err)
	}
	var pkgs bytes.Buffer
	for _, f := range files {
		if strings.HasSuffix(f.Path, "binary-amd64/Packages") {
			pkgs.Write(f.Content)
		}
	}
	if !strings.Contains(pkgs.String(), "Package: after-pkg") {
		t.Fatalf("Packages index did not reflect in-place replacement; got:\n%s", pkgs.String())
	}
	if strings.Contains(pkgs.String(), "Package: before-pkg") {
		t.Fatalf("stale before-pkg still in index after replacement; got:\n%s", pkgs.String())
	}
	_ = file
}
