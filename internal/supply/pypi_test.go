package supply

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// seedPyPIProject creates a pypi-sourced project and returns its id.
func seedPyPIProject(t *testing.T, pRepo *db.ProjectRepo, name string) int64 {
	t.Helper()
	ctx := context.Background()
	p, err := pRepo.Create(ctx, name, name, "pypi", "https://pypi.org/project/"+name)
	if err != nil {
		t.Fatalf("create pypi project: %v", err)
	}
	return p.ID
}

// seedPyPIFile inserts a complete pypi dist file row.
func seedPyPIFile(t *testing.T, fRepo *db.FileRepo, projectID int64, version, filename, sha string) {
	t.Helper()
	ctx := context.Background()
	file := &db.File{
		ProjectID: projectID, Version: version, Filename: filename,
		Ext: "whl", SizeBytes: 1, DownloadURL: "u", LocalPath: "/x/" + filename,
		Status: "complete", Checksum: sha,
	}
	if _, err := fRepo.Create(ctx, file); err != nil {
		t.Fatalf("create file: %v", err)
	}
}

func TestPyPI_RootIndex_ListsServedProjects(t *testing.T) {
	fRepo, pRepo := supplyTestDB(t)
	pid := seedPyPIProject(t, pRepo, "requests")
	seedPyPIFile(t, fRepo, pid, "2.31.0", "requests-2.31.0-py3-none-any.whl", "abc123")

	// A pypi project with NO served files must NOT appear (PEP 503 clients would
	// then 404 on the per-project page). Seed one to verify it's excluded.
	emptyPID := seedPyPIProject(t, pRepo, "orphan-pkg")
	_ = emptyPID

	h := NewPyPIHandler(fRepo, pRepo, nil)
	req := httptest.NewRequest(http.MethodGet, "/simple/", nil)
	rr := httptest.NewRecorder()
	h.Serve(rr, req)

	body := rr.Body.String()
	if !strings.Contains(body, `href="requests/">`) {
		t.Errorf("root index missing served project 'requests'; body:\n%s", body)
	}
	if strings.Contains(body, "orphan-pkg") {
		t.Errorf("root index lists project with no served files; body:\n%s", body)
	}
	if rr.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want text/html; charset=utf-8", rr.Header().Get("Content-Type"))
	}
}

func TestPyPI_ProjectIndex_IncludesSha256Fragment(t *testing.T) {
	fRepo, pRepo := supplyTestDB(t)
	pid := seedPyPIProject(t, pRepo, "requests")
	seedPyPIFile(t, fRepo, pid, "2.31.0", "requests-2.31.0-py3-none-any.whl", "abc123")
	seedPyPIFile(t, fRepo, pid, "2.30.0", "requests-2.30.0.tar.gz", "def456")

	h := NewPyPIHandler(fRepo, pRepo, nil)
	h.basePublicURL = "/repo/files" // explicit for the assertion
	req := httptest.NewRequest(http.MethodGet, "/simple/requests/", nil)
	rr := httptest.NewRecorder()
	h.Serve(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	body := rr.Body.String()
	// Each dist must link to the generic file downloader with a #sha256 fragment.
	if !strings.Contains(body, `/repo/files/1/requests-2.31.0-py3-none-any.whl#sha256=abc123`) {
		t.Errorf("project index missing wheel link+sha; body:\n%s", body)
	}
	if !strings.Contains(body, `/repo/files/2/requests-2.30.0.tar.gz#sha256=def456`) {
		t.Errorf("project index missing sdist link+sha; body:\n%s", body)
	}
}

func TestPyPI_ProjectIndex_PEP503Normalization(t *testing.T) {
	// PEP 503: "My_Pkg", "my-pkg", "my.pkg" all normalize to "my-pkg" and must be
	// reachable under any of those forms. The project is registered as "My_Pkg".
	fRepo, pRepo := supplyTestDB(t)
	pid := seedPyPIProject(t, pRepo, "My_Pkg")
	seedPyPIFile(t, fRepo, pid, "1.0", "My_Pkg-1.0-py3-none-any.whl", "abc")

	h := NewPyPIHandler(fRepo, pRepo, nil)
	for _, form := range []string{"My_Pkg", "my-pkg", "my.pkg", "MY_PKG"} {
		req := httptest.NewRequest(http.MethodGet, "/simple/"+form+"/", nil)
		rr := httptest.NewRecorder()
		h.Serve(rr, req)
		if rr.Code != http.StatusOK {
			t.Errorf("form %q: status=%d want 200 (PEP 503 normalization)", form, rr.Code)
		}
	}
	// And the root index must list the normalized name.
	req := httptest.NewRequest(http.MethodGet, "/simple/", nil)
	rr := httptest.NewRecorder()
	h.Serve(rr, req)
	if !strings.Contains(rr.Body.String(), `href="my-pkg/">`) {
		t.Errorf("root index should use normalized name 'my-pkg'; body:\n%s", rr.Body.String())
	}
}

func TestPyPI_UnknownProject_404(t *testing.T) {
	fRepo, pRepo := supplyTestDB(t)
	// One served project so the index is non-empty.
	pid := seedPyPIProject(t, pRepo, "requests")
	seedPyPIFile(t, fRepo, pid, "2.31.0", "requests-2.31.0-py3-none-any.whl", "abc")

	h := NewPyPIHandler(fRepo, pRepo, nil)
	req := httptest.NewRequest(http.MethodGet, "/simple/does-not-exist/", nil)
	rr := httptest.NewRecorder()
	h.Serve(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Errorf("unknown project: status=%d want 404", rr.Code)
	}
}

func TestPyPI_NonDistFilesExcluded(t *testing.T) {
	// A non-Python file (e.g. a .deb) on a pypi project must not appear in the
	// index, and must not make the project appear if it's the only file.
	fRepo, pRepo := supplyTestDB(t)
	pid := seedPyPIProject(t, pRepo, "weird")
	// Seed only a .deb (not a wheel/sdist) under the pypi project.
	ctx := context.Background()
	if _, err := fRepo.Create(ctx, &db.File{
		ProjectID: pid, Version: "1.0", Filename: "weird_1.0_amd64.deb",
		Ext: "deb", SizeBytes: 1, DownloadURL: "u", LocalPath: "/x/weird.deb",
		Status: "complete",
	}); err != nil {
		t.Fatalf("create file: %v", err)
	}

	h := NewPyPIHandler(fRepo, pRepo, nil)
	req := httptest.NewRequest(http.MethodGet, "/simple/", nil)
	rr := httptest.NewRecorder()
	h.Serve(rr, req)
	if strings.Contains(rr.Body.String(), "weird") {
		t.Errorf("project with only a .deb should not appear in pypi index; body:\n%s", rr.Body.String())
	}

	// And the project page itself must 404 (no servable dists).
	req2 := httptest.NewRequest(http.MethodGet, "/simple/weird/", nil)
	rr2 := httptest.NewRecorder()
	h.Serve(rr2, req2)
	if rr2.Code != http.StatusNotFound {
		t.Errorf("project with only .deb: status=%d want 404", rr2.Code)
	}
}

// TestNormalizePyPIProject is a focused unit test on the PEP 503 rule.
func TestNormalizePyPIProject(t *testing.T) {
	cases := map[string]string{
		"requests":       "requests",
		"My_Pkg":         "my-pkg",
		"my.pkg":         "my-pkg",
		"MY---PKG":       "my-pkg",
		"Flask-RESTful":  "flask-restful",
		"":               "",
		"a..b__c.d":      "a-b-c-d",
	}
	for in, want := range cases {
		if got := normalizePyPIProject(in); got != want {
			t.Errorf("normalizePyPIProject(%q) = %q, want %q", in, got, want)
		}
	}
}
