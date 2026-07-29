package supply

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	db "github.com/Mi-Bee-Studio/mibeehive/internal/db"
)

// supplyHandlerTestDB mirrors db.testDB but lives in-package; the db test helper
// is package-private. We open an in-memory sqlite via db.Open (a temp path is
// fine and cleaned by t.TempDir).
func supplyTestDB(t *testing.T) (*db.FileRepo, *db.ProjectRepo) {
	t.Helper()
	database, err := db.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db.NewFileRepo(database), db.NewProjectRepo(database)
}

func TestServeIndex_ListsCompleteFiles(t *testing.T) {
	fRepo, pRepo := supplyTestDB(t)
	ctx := context.Background()
	// NOTE: source_type is constrained to github/go/hashicorp/grafana by a DB
	// CHECK constraint. The test only needs a placeholder project; the value is
	// irrelevant here. (This constraint is itself evidence for REPORT.md: the
	// schema is source-type-specific and cannot yet hold a "rulesrc" source.)
	p, err := pRepo.Create(ctx, "t", "T", "github", "")
	if err != nil {
		t.Fatal(err)
	}
	// Two complete + one pending; only complete must appear.
	mk := func(name, status string) {
		if _, err := fRepo.Create(ctx, &db.File{ProjectID: p.ID, Filename: name, Status: status, LocalPath: "/p/" + name}); err != nil {
			t.Fatal(err)
		}
	}
	mk("a.tar.gz", "complete")
	mk("b.tar.gz", "complete")
	mk("c.tar.gz", "pending")

	// FileService.StreamFile needs a real on-disk file; for the index test we
	// pass nil — ServeIndex never calls it.
	h := NewHandler(fRepo, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repo/index", h.ServeIndex)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/repo/index", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", rec.Code)
	}
	var got indexResponse
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Count != 2 {
		t.Fatalf("count: want 2, got %d (%+v)", got.Count, got.Items)
	}
	for _, it := range got.Items {
		if !strings.HasPrefix(it.DownloadURL, "/repo/files/") {
			t.Errorf("download_url not relative: %q", it.DownloadURL)
		}
	}
}

func TestServeFile_RejectsBadID(t *testing.T) {
	fRepo, _ := supplyTestDB(t)
	h := NewHandler(fRepo, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repo/files/{id}", h.ServeFile)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/repo/files/abc", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for non-numeric id, got %d", rec.Code)
	}
}

func TestServeFile_NotFound(t *testing.T) {
	fRepo, _ := supplyTestDB(t)
	h := NewHandler(fRepo, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repo/files/{id}", h.ServeFile)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/repo/files/99999", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("want 404 for missing id, got %d", rec.Code)
	}
}
