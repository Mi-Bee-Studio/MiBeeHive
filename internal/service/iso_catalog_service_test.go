package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	_ "modernc.org/sqlite"
)

// setupCatalogTestDB creates an in-memory SQLite database with the iso_catalog table.
func setupCatalogTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	d.SetMaxOpenConns(1)

	_, err = d.Exec(`
		CREATE TABLE iso_catalog (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			distro TEXT NOT NULL,
			variant TEXT NOT NULL DEFAULT '',
			arch TEXT NOT NULL DEFAULT 'amd64',
			check_url TEXT NOT NULL,
			filename_pattern TEXT NOT NULL,
			current_url TEXT DEFAULT '',
			base_url TEXT NOT NULL DEFAULT '',
			version_dir_pattern TEXT NOT NULL DEFAULT '',
			iso_path_template TEXT NOT NULL DEFAULT '',
			auto_update INTEGER DEFAULT 0,
			check_interval_hours INTEGER DEFAULT 24,
			last_checked DATETIME,
			last_error TEXT DEFAULT '',
			status TEXT DEFAULT 'available',
			download_status TEXT NOT NULL DEFAULT '',
			sha256 TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("create iso_catalog table: %v", err)
	}
	return d
}

// newTestCatalogService creates an ISOCatalogService backed by the test DB.
// isoService may be nil for CRUD-only tests.
func newTestCatalogService(t *testing.T, database *sql.DB, isoService *ISOService) *ISOCatalogService {
	t.Helper()
	repo := db.NewISOCatalogRepo(database)
	return NewISOCatalogService(repo, isoService, slog.Default(), nil)
}

// newTestISOService creates an ISOService rooted at a temp directory with an httptest server.
// Returns the service, the temp dir (for assertions), and a server URL builder.
func newCatalogTestISOService(t *testing.T) (*ISOService, string) {
	t.Helper()
	tmpDir := t.TempDir()
	isoDir := filepath.Join(tmpDir, "os-install")
	if err := os.MkdirAll(isoDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	svc := NewISOService(tmpDir, 2, nil)
	t.Cleanup(func() { svc.Shutdown() })
	return svc, tmpDir
}

// ptr helpers for update request fields.
func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }
func boolPtr(b bool) *bool    { return &b }

// ---------- Test 1: Create ----------

func TestCatalogService_Create(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		id, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Ubuntu 24.04",
			Distro:          "ubuntu",
			Arch:            "amd64",
			CheckURL:        "https://releases.ubuntu.com/24.04/",
			FilenamePattern: `ubuntu-24\.04\.\d+-live-server-amd64\.iso`,
		})
		if err != nil {
			t.Fatalf("CreateCatalogEntry: %v", err)
		}
		if id <= 0 {
			t.Fatalf("expected positive ID, got %d", id)
		}
	})

	t.Run("defaults_arch_to_amd64", func(t *testing.T) {
		id, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Debian 12",
			Distro:          "debian",
			CheckURL:        "https://cdimage.debian.org/release/12/",
			FilenamePattern: `debian-12\..*-amd64-netinst\.iso`,
		})
		if err != nil {
			t.Fatalf("CreateCatalogEntry: %v", err)
		}
		entry, err := svc.GetCatalogEntry(ctx, id)
		if err != nil {
			t.Fatalf("GetCatalogEntry: %v", err)
		}
		if entry.Arch != "amd64" {
			t.Errorf("expected arch=default amd64, got %q", entry.Arch)
		}
		if entry.CheckIntervalHours != 24 {
			t.Errorf("expected default interval 24, got %d", entry.CheckIntervalHours)
		}
	})

	t.Run("missing_required_fields", func(t *testing.T) {
		_, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name: "No URL",
		})
		if err == nil {
			t.Fatal("expected error for missing required fields")
		}
		if !strings.Contains(err.Error(), "required") {
			t.Errorf("error should mention required fields, got: %v", err)
		}
	})

	t.Run("sets_status_to_available", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Fedora 41",
			Distro:          "fedora",
			CheckURL:        "https://example.com/fedora/",
			FilenamePattern: `Fedora-Server-dvd-x86_64-\d+\.iso`,
			AutoUpdate:      true,
			CheckIntervalHours: 12,
			SHA256:          "abc123",
		})
		entry, err := svc.GetCatalogEntry(ctx, id)
		if err != nil {
			t.Fatalf("GetCatalogEntry: %v", err)
		}
		if entry.Status != "available" {
			t.Errorf("expected status=available, got %q", entry.Status)
		}
		if entry.AutoUpdate != true {
			t.Error("expected auto_update=true")
		}
		if entry.SHA256 != "abc123" {
			t.Errorf("expected sha256=abc123, got %q", entry.SHA256)
		}
		if entry.CheckIntervalHours != 12 {
			t.Errorf("expected interval=12, got %d", entry.CheckIntervalHours)
		}
	})
}

// ---------- Test 2: GetByID ----------

func TestCatalogService_GetByID(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	id, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Alpine 3.20",
		Distro:          "alpine",
		Variant:         "virt",
		Arch:            "aarch64",
		CheckURL:        "https://alpinelinux.org/releases/",
		FilenamePattern: `alpine-virt-\d+\.\d+\.\d+-aarch64\.iso`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("found", func(t *testing.T) {
		entry, err := svc.GetCatalogEntry(ctx, id)
		if err != nil {
			t.Fatalf("GetCatalogEntry: %v", err)
		}
		if entry == nil {
			t.Fatal("expected entry, got nil")
		}
		if entry.Name != "Alpine 3.20" {
			t.Errorf("expected name='Alpine 3.20', got %q", entry.Name)
		}
		if entry.Distro != "alpine" {
			t.Errorf("expected distro=alpine, got %q", entry.Distro)
		}
		if entry.Variant != "virt" {
			t.Errorf("expected variant=virt, got %q", entry.Variant)
		}
		if entry.Arch != "aarch64" {
			t.Errorf("expected arch=aarch64, got %q", entry.Arch)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		entry, err := svc.GetCatalogEntry(ctx, 99999)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry != nil {
			t.Errorf("expected nil for not found, got %+v", entry)
		}
	})
}

// ---------- Test 3: List ----------

func TestCatalogService_List(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	names := []string{"Ubuntu", "Debian", "Alpine"}
	for _, n := range names {
		_, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            n,
			Distro:          strings.ToLower(n),
			CheckURL:        "https://example.com/" + strings.ToLower(n) + "/",
			FilenamePattern: n + `-\d+\.iso`,
		})
		if err != nil {
			t.Fatalf("create %s: %v", n, err)
		}
	}

	entries, err := svc.ListCatalog(ctx)
	if err != nil {
		t.Fatalf("ListCatalog: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	gotNames := make(map[string]bool)
	for _, e := range entries {
		gotNames[e.Name] = true
	}
	for _, n := range names {
		if !gotNames[n] {
			t.Errorf("missing entry %q in list", n)
		}
	}
}

// ---------- Test 4: Update ----------

func TestCatalogService_Update(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	id, err := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Original",
		Distro:          "test",
		CheckURL:        "https://example.com/original/",
		FilenamePattern: `original-\d+\.iso`,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	t.Run("partial_update", func(t *testing.T) {
		err := svc.UpdateCatalogEntry(ctx, id, model.ISOCatalogUpdateRequest{
			Name: strPtr("Updated"),
		})
		if err != nil {
			t.Fatalf("UpdateCatalogEntry: %v", err)
		}
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.Name != "Updated" {
			t.Errorf("expected name=Updated, got %q", entry.Name)
		}
		// Unchanged fields preserved.
		if entry.Distro != "test" {
			t.Errorf("distro should be unchanged, got %q", entry.Distro)
		}
	})

	t.Run("update_multiple_fields", func(t *testing.T) {
		err := svc.UpdateCatalogEntry(ctx, id, model.ISOCatalogUpdateRequest{
			Distro:             strPtr("updated-distro"),
			Arch:               strPtr("arm64"),
			AutoUpdate:         boolPtr(true),
			CheckIntervalHours: intPtr(48),
			SHA256:             strPtr("sha256val"),
		})
		if err != nil {
			t.Fatalf("UpdateCatalogEntry: %v", err)
		}
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.Distro != "updated-distro" {
			t.Errorf("expected distro=updated-distro, got %q", entry.Distro)
		}
		if entry.Arch != "arm64" {
			t.Errorf("expected arch=arm64, got %q", entry.Arch)
		}
		if !entry.AutoUpdate {
			t.Error("expected auto_update=true")
		}
		if entry.CheckIntervalHours != 48 {
			t.Errorf("expected interval=48, got %d", entry.CheckIntervalHours)
		}
		if entry.SHA256 != "sha256val" {
			t.Errorf("expected sha256=sha256val, got %q", entry.SHA256)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		err := svc.UpdateCatalogEntry(ctx, 99999, model.ISOCatalogUpdateRequest{
			Name: strPtr("nope"),
		})
		if err == nil {
			t.Fatal("expected error for not-found entry")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("error should mention not found, got: %v", err)
		}
	})
}

// ---------- Test 5: Delete ----------

func TestCatalogService_Delete(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "ToDelete",
		Distro:          "test",
		CheckURL:        "https://example.com/del/",
		FilenamePattern: `del-\d+\.iso`,
	})

	t.Run("success", func(t *testing.T) {
		err := svc.DeleteCatalogEntry(ctx, id)
		if err != nil {
			t.Fatalf("DeleteCatalogEntry: %v", err)
		}
		entry, err := svc.GetCatalogEntry(ctx, id)
		if err != nil {
			t.Fatalf("GetCatalogEntry after delete: %v", err)
		}
		if entry != nil {
			t.Error("expected nil after delete")
		}
	})

	t.Run("delete_nonexistent", func(t *testing.T) {
		// Deleting a non-existent ID should not error (SQLite DELETE is idempotent).
		err := svc.DeleteCatalogEntry(ctx, 99999)
		if err != nil {
			t.Fatalf("delete nonexistent should not error: %v", err)
		}
	})
}

// ---------- Test 6: VersionCheck ----------

func TestCatalogService_VersionCheck(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	// Set up a fake HTTP server serving a directory listing page.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
<a href="ubuntu-24.04.1-live-server-amd64.iso">ubuntu-24.04.1-live-server-amd64.iso</a>
<a href="ubuntu-24.04.2-live-server-amd64.iso">ubuntu-24.04.2-live-server-amd64.iso</a>
<a href="SHA256SUMS">SHA256SUMS</a>
</body></html>`)
	}))
	defer srv.Close()

	t.Run("new_version_detected", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Ubuntu 24.04",
			Distro:          "ubuntu",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `ubuntu-24\.04\.\d+-live-server-amd64\.iso`,
		})
		resp, err := svc.CheckVersion(ctx, id)
		if err != nil {
			t.Fatalf("CheckVersion: %v", err)
		}
		if resp.Status != "new_version" {
			t.Errorf("expected status=new_version, got %q", resp.Status)
		}
		if !strings.Contains(resp.FoundURL, "ubuntu-24.04.2") {
			t.Errorf("expected latest ISO URL with 24.04.2, got %q", resp.FoundURL)
		}
		// Verify DB was updated with the found URL.
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if !strings.Contains(entry.CurrentURL, "ubuntu-24.04.2") {
			t.Errorf("current_url should be updated, got %q", entry.CurrentURL)
		}
	})

	t.Run("up_to_date", func(t *testing.T) {
		// Set current_url to the latest version first.
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Ubuntu Duplicate",
			Distro:          "ubuntu",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `ubuntu-24\.04\.\d+-live-server-amd64\.iso`,
		})
		// First check sets current_url.
		svc.CheckVersion(ctx, id)
		// Second check should find same version → up_to_date.
		resp, err := svc.CheckVersion(ctx, id)
		if err != nil {
			t.Fatalf("CheckVersion: %v", err)
		}
		if resp.Status != "up_to_date" {
			t.Errorf("expected status=up_to_date, got %q", resp.Status)
		}
	})

	t.Run("no_match", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Nonexistent ISO",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `nonexistent-\d+\.iso`,
		})
		resp, err := svc.CheckVersion(ctx, id)
		if err != nil {
			t.Fatalf("CheckVersion: %v", err)
		}
		if resp.Status != "no_match" {
			t.Errorf("expected status=no_match, got %q", resp.Status)
		}
	})

	t.Run("not_found_entry", func(t *testing.T) {
		_, err := svc.CheckVersion(ctx, 99999)
		if err == nil {
			t.Fatal("expected error for not-found entry")
		}
	})

	t.Run("bad_http_response", func(t *testing.T) {
		badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer badSrv.Close()

		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Bad URL",
			Distro:          "test",
			CheckURL:        badSrv.URL + "/",
			FilenamePattern: `test-\d+\.iso`,
		})
		_, err := svc.CheckVersion(ctx, id)
		if err == nil {
			t.Fatal("expected error for HTTP 500")
		}
		// Verify error status persisted.
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.Status != "error" {
			t.Errorf("expected status=error, got %q", entry.Status)
		}
		if entry.LastError == "" {
			t.Error("expected last_error to be set")
		}
	})
}

// ---------- Test 7: QueueProcessing ----------

func TestCatalogService_QueueProcessing(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	isoSvc, _ := newCatalogTestISOService(t)
	svc := newTestCatalogService(t, dbase, isoSvc)
	ctx := context.Background()

	// Serve a small ISO file from test server.
	isoContent := strings.Repeat("ISO-DATA-", 100) // 900 bytes
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(isoContent)))
		fmt.Fprint(w, isoContent)
	}))
	defer srv.Close()

	t.Run("QueueDownloadAll_skips_empty_url", func(t *testing.T) {
		// Entry with no current_url should be skipped.
		_, _ = svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "No URL",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		err := svc.QueueDownloadAll(ctx)
		if err != nil {
			t.Fatalf("QueueDownloadAll: %v", err)
		}
		// Verify no entry has download_status set.
		entries, _ := svc.ListCatalog(ctx)
		for _, e := range entries {
			if e.DownloadStatus != "" {
				t.Errorf("entry %q should not be queued (no URL), but got status %q", e.Name, e.DownloadStatus)
			}
		}
	})

	t.Run("QueueDownloadAll_queues_entries_with_url", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Has URL",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `test-\d+\.iso`,
		})
		// Set current_url via DB directly (simulating a version check result).
		dbRepo := db.NewISOCatalogRepo(dbase)
		_ = dbRepo.UpdateAfterCheck(ctx, id, srv.URL+"/test-1.iso", "new_version", "")

		err := svc.QueueDownloadAll(ctx)
		if err != nil {
			t.Fatalf("QueueDownloadAll: %v", err)
		}
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.DownloadStatus != "pending" {
			t.Errorf("expected download_status=pending, got %q", entry.DownloadStatus)
		}
	})

	t.Run("ProcessQueue_downloads_pending", func(t *testing.T) {
		// Create entry with URL and pending status.
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Queue Entry",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `queue-\d+\.iso`,
		})
		dbRepo := db.NewISOCatalogRepo(dbase)
		_ = dbRepo.UpdateAfterCheck(ctx, id, srv.URL+"/queue-test.iso", "new_version", "")
		_ = dbRepo.UpdateDownloadStatus(ctx, id, "pending")

		svc.ProcessQueue(ctx)

		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.DownloadStatus != "downloaded" {
			t.Errorf("expected download_status=downloaded, got %q", entry.DownloadStatus)
		}
		if entry.Status != "downloaded" {
			t.Errorf("expected status=downloaded, got %q", entry.Status)
		}
	})

	t.Run("skips_already_downloaded", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Already Done",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `done-\d+\.iso`,
		})
		dbRepo := db.NewISOCatalogRepo(dbase)
		_ = dbRepo.UpdateAfterCheck(ctx, id, srv.URL+"/done-test.iso", "downloaded", "")
		_ = dbRepo.UpdateDownloadStatus(ctx, id, "downloaded")

		// Should be a no-op — downloaded entries are not in the queue list.
		svc.ProcessQueue(ctx)
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.DownloadStatus != "downloaded" {
			t.Errorf("status should remain downloaded, got %q", entry.DownloadStatus)
		}
	})
}

// ---------- Test 8: AutoDownload ----------

func TestCatalogService_AutoDownload(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	isoSvc, tmpDir := newCatalogTestISOService(t)
	svc := newTestCatalogService(t, dbase, isoSvc)
	ctx := context.Background()

	// Set up test server that serves a directory listing AND the ISO file.
	isoContent := "FAKE-ISO-CONTENT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "listing/") || r.URL.Path == "/listing/" {
			w.Header().Set("Content-Type", "text/html")
			// Link to the ISO file on the same server.
			fmt.Fprintf(w, `<html><body><a href="/iso/auto-test.iso">auto-test.iso</a></body></html>`)
			return
		}
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(isoContent)))
		fmt.Fprint(w, isoContent)
	}))
	defer srv.Close()

	t.Run("CheckAllAutoUpdate_checks_and_downloads", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "AutoISO",
			Distro:          "auto-test",
			CheckURL:        srv.URL + "/listing/",
			FilenamePattern: `auto-test\.iso`,
			AutoUpdate:      true,
		})
		// Set a current_url so the entry is considered for auto-download after version check.
		dbRepo := db.NewISOCatalogRepo(dbase)
		_ = dbRepo.UpdateAfterCheck(ctx, id, "https://old.example.com/old-auto-test.iso", "available", "")

		err := svc.CheckAllAutoUpdate(ctx)
		if err != nil {
			t.Fatalf("CheckAllAutoUpdate: %v", err)
		}

		entry, _ := svc.GetCatalogEntry(ctx, id)
		// Should have detected new version (different URL from old).
		if entry.Status != "downloaded" && entry.Status != "new_version" {
			t.Errorf("expected status downloaded or new_version, got %q", entry.Status)
		}
		// Verify ISO file was actually written to disk.
		isoPath := filepath.Join(tmpDir, "os-install", "auto-test.iso")
		if _, err := os.Stat(isoPath); err != nil {
			t.Errorf("ISO file not found at %s: %v", isoPath, err)
		}
	})

	t.Run("CheckAllAutoUpdate_no_auto_entries", func(t *testing.T) {
		// Create a non-auto-update entry — should be skipped.
		_, _ = svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Manual Only",
			Distro:          "manual",
			CheckURL:        srv.URL + "/listing/",
			FilenamePattern: `manual\.iso`,
			AutoUpdate:      false,
		})
		err := svc.CheckAllAutoUpdate(ctx)
		if err != nil {
			t.Fatalf("CheckAllAutoUpdate with no auto entries: %v", err)
		}
	})
}

// ---------- Test 9: GetQueueStats ----------

func TestCatalogService_GetQueueStats(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	repo := db.NewISOCatalogRepo(dbase)

	// Create entries and set various download statuses.
	ids := make([]int64, 5)
	for i := 0; i < 5; i++ {
		ids[i], _ = svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            fmt.Sprintf("Entry-%d", i),
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
	}
	_ = repo.UpdateDownloadStatus(ctx, ids[0], "pending")
	_ = repo.UpdateDownloadStatus(ctx, ids[1], "pending")
	_ = repo.UpdateDownloadStatus(ctx, ids[2], "downloading")
	_ = repo.UpdateDownloadStatus(ctx, ids[3], "downloaded")
	_ = repo.UpdateDownloadStatus(ctx, ids[4], "error")

	stats, err := svc.GetQueueStats(ctx)
	if err != nil {
		t.Fatalf("GetQueueStats: %v", err)
	}
	if stats.Pending != 2 {
		t.Errorf("expected pending=2, got %d", stats.Pending)
	}
	if stats.Downloading != 1 {
		t.Errorf("expected downloading=1, got %d", stats.Downloading)
	}
	if stats.Downloaded != 1 {
		t.Errorf("expected downloaded=1, got %d", stats.Downloaded)
	}
	if stats.Error != 1 {
		t.Errorf("expected error=1, got %d", stats.Error)
	}
	if stats.Total != 5 {
		t.Errorf("expected total=5, got %d", stats.Total)
	}
}

// ---------- Test 10: GetQueueList ----------

func TestCatalogService_GetQueueList(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	repo := db.NewISOCatalogRepo(dbase)

	// Create two entries; one with empty download_status (should be excluded), one with status.
	emptyID, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Empty Status",
		Distro:          "test",
		CheckURL:        "https://example.com/",
		FilenamePattern: `test-\d+\.iso`,
	})
	_ = emptyID // no download_status set → should not appear in queue

	activeID, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Active",
		Distro:          "test",
		CheckURL:        "https://example.com/",
		FilenamePattern: `test-\d+\.iso`,
	})
	_ = repo.UpdateDownloadStatus(ctx, activeID, "pending")

	items, err := svc.GetQueueList(ctx)
	if err != nil {
		t.Fatalf("GetQueueList: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 queue item, got %d", len(items))
	}
	if items[0].Name != "Active" {
		t.Errorf("expected name=Active, got %q", items[0].Name)
	}
	if items[0].DownloadStatus != "pending" {
		t.Errorf("expected download_status=pending, got %q", items[0].DownloadStatus)
	}
}

// ---------- Test 11: RetryCatalogDownload ----------

func TestCatalogService_RetryCatalogDownload(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	repo := db.NewISOCatalogRepo(dbase)

	t.Run("success_resets_error_to_pending", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Retry Me",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		_ = repo.UpdateDownloadStatus(ctx, id, "error")

		err := svc.RetryCatalogDownload(ctx, id)
		if err != nil {
			t.Fatalf("RetryCatalogDownload: %v", err)
		}
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.DownloadStatus != "pending" {
			t.Errorf("expected download_status=pending after retry, got %q", entry.DownloadStatus)
		}
	})

	t.Run("rejects_non_error_status", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Not Error",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		_ = repo.UpdateDownloadStatus(ctx, id, "downloaded")

		err := svc.RetryCatalogDownload(ctx, id)
		if err == nil {
			t.Fatal("expected error for non-error status")
		}
		if !strings.Contains(err.Error(), "expected 'error'") {
			t.Errorf("error should mention expected 'error', got: %v", err)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		err := svc.RetryCatalogDownload(ctx, 99999)
		if err == nil {
			t.Fatal("expected error for not-found entry")
		}
	})
}

// ---------- Test 12: DownloadFromCatalog ----------

func TestCatalogService_DownloadFromCatalog(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	isoSvc, tmpDir := newCatalogTestISOService(t)
	svc := newTestCatalogService(t, dbase, isoSvc)
	ctx := context.Background()

	isoContent := "ISO-DOWNLOAD-CONTENT"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(isoContent)))
		fmt.Fprint(w, isoContent)
	}))
	defer srv.Close()

	t.Run("success", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Download Test",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `test-\d+\.iso`,
		})
		repo := db.NewISOCatalogRepo(dbase)
		isoURL := srv.URL + "/download-test.iso"
		_ = repo.UpdateAfterCheck(ctx, id, isoURL, "new_version", "")

		err := svc.DownloadFromCatalog(ctx, id)
		if err != nil {
			t.Fatalf("DownloadFromCatalog: %v", err)
		}
		// Verify file on disk.
		isoPath := filepath.Join(tmpDir, "os-install", "download-test.iso")
		data, err := os.ReadFile(isoPath)
		if err != nil {
			t.Fatalf("reading downloaded ISO: %v", err)
		}
		if string(data) != isoContent {
			t.Errorf("ISO content mismatch: got %d bytes", len(data))
		}
		// Verify status updated.
		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.Status != "downloaded" {
			t.Errorf("expected status=downloaded, got %q", entry.Status)
		}
	})

	t.Run("no_url_available", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "No URL",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		err := svc.DownloadFromCatalog(ctx, id)
		if err == nil {
			t.Fatal("expected error when no URL available")
		}
		if !strings.Contains(err.Error(), "run version check first") {
			t.Errorf("error should mention version check, got: %v", err)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		err := svc.DownloadFromCatalog(ctx, 99999)
		if err == nil {
			t.Fatal("expected error for not-found entry")
		}
	})
}

// ---------- Test 13: CancelDownload ----------

func TestCatalogService_CancelDownload(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	t.Run("not_downloading_status", func(t *testing.T) {
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Not Downloading",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		err := svc.CancelDownload(ctx, id)
		if err == nil {
			t.Fatal("expected error for non-downloading status")
		}
		if !strings.Contains(err.Error(), "not currently downloading") {
			t.Errorf("error should mention not downloading, got: %v", err)
		}
	})

	t.Run("not_found", func(t *testing.T) {
		err := svc.CancelDownload(ctx, 99999)
		if err == nil {
			t.Fatal("expected error for not-found entry")
		}
	})

	t.Run("downloading_but_no_cancel_func", func(t *testing.T) {
		repo := db.NewISOCatalogRepo(dbase)
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Orphan Download",
			Distro:          "test",
			CheckURL:        "https://example.com/",
			FilenamePattern: `test-\d+\.iso`,
		})
		_ = repo.UpdateDownloadStatus(ctx, id, "downloading")

		err := svc.CancelDownload(ctx, id)
		if err == nil {
			t.Fatal("expected error when no cancel func registered")
		}
		if !strings.Contains(err.Error(), "no active download") {
			t.Errorf("error should mention no active download, got: %v", err)
		}
	})
}

// ---------- Test 14: Stale download detection in ProcessQueue ----------

func TestCatalogService_ProcessQueue_StaleReset(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	isoSvc, tmpDir := newCatalogTestISOService(t)
	svc := newTestCatalogService(t, dbase, isoSvc)
	ctx := context.Background()

	isoContent := "STALE-RECOVERY-ISO"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprintf("%d", len(isoContent)))
		fmt.Fprint(w, isoContent)
	}))
	defer srv.Close()

	t.Run("resets_stale_downloading_with_no_temp_file", func(t *testing.T) {
		repo := db.NewISOCatalogRepo(dbase)
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Stale Entry",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `stale-\d+\.iso`,
		})
		isoURL := srv.URL + "/stale-test.iso"
		_ = repo.UpdateAfterCheck(ctx, id, isoURL, "new_version", "")
		_ = repo.UpdateDownloadStatus(ctx, id, "downloading")
		// No temp file on disk → stale, should reset to pending, then process.

		svc.ProcessQueue(ctx)

		entry, _ := svc.GetCatalogEntry(ctx, id)
		if entry.DownloadStatus != "downloaded" {
			t.Errorf("expected download_status=downloaded after stale reset+process, got %q", entry.DownloadStatus)
		}
	})

	t.Run("marks_downloaded_when_final_file_exists", func(t *testing.T) {
		repo := db.NewISOCatalogRepo(dbase)
		id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
			Name:            "Completed Stale",
			Distro:          "test",
			CheckURL:        srv.URL + "/",
			FilenamePattern: `completed-\d+\.iso`,
		})
		filename := "completed-test.iso"
		isoURL := srv.URL + "/" + filename
		_ = repo.UpdateAfterCheck(ctx, id, isoURL, "new_version", "")
		_ = repo.UpdateDownloadStatus(ctx, id, "downloading")

		// Write the final file to disk — simulates download completed but status not updated.
		isoDir := filepath.Join(tmpDir, "os-install")
		finalPath := filepath.Join(isoDir, filename)
		if err := os.WriteFile(finalPath, []byte("already-downloaded"), 0644); err != nil {
			t.Fatalf("writing final file: %v", err)
		}

		svc.ProcessQueue(ctx)

		entry, _ := svc.GetCatalogEntry(ctx, id)
		// Should have been detected as completed, not re-downloaded.
		if entry.DownloadStatus != "downloaded" {
			t.Errorf("expected download_status=downloaded, got %q", entry.DownloadStatus)
		}
	})
}

// ---------- Test 15: ProcessQueue error handling ----------

func TestCatalogService_ProcessQueue_DownloadError(t *testing.T) {
	dbase := setupCatalogTestDB(t)
	isoSvc, _ := newCatalogTestISOService(t)
	svc := newTestCatalogService(t, dbase, isoSvc)
	ctx := context.Background()

	// Server that returns 403 — will cause download failure.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	repo := db.NewISOCatalogRepo(dbase)
	id, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Fail Download",
		Distro:          "test",
		CheckURL:        srv.URL + "/",
		FilenamePattern: `fail-\d+\.iso`,
	})
	_ = repo.UpdateAfterCheck(ctx, id, srv.URL+"/fail-test.iso", "new_version", "")
	_ = repo.UpdateDownloadStatus(ctx, id, "pending")

	svc.ProcessQueue(ctx)

	entry, _ := svc.GetCatalogEntry(ctx, id)
	if entry.DownloadStatus != "error" {
		t.Errorf("expected download_status=error after failed download, got %q", entry.DownloadStatus)
	}
	if entry.Status != "error" {
		t.Errorf("expected status=error after failed download, got %q", entry.Status)
	}
	if entry.LastError == "" {
		t.Error("expected last_error to be set on failure")
	}
}

// ---------- Test 16: Concurrency (QueueDownloadAll skips downloading) ----------

func TestCatalogService_QueueDownloadAll_SkipsActive(t *testing.T) {
	t.Parallel()
	dbase := setupCatalogTestDB(t)
	svc := newTestCatalogService(t, dbase, nil)
	ctx := context.Background()

	repo := db.NewISOCatalogRepo(dbase)

	// Entry with downloading status — should be skipped.
	id1, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Active Download",
		Distro:          "test",
		CheckURL:        "https://example.com/",
		FilenamePattern: `test-\d+\.iso`,
	})
	_ = repo.UpdateAfterCheck(ctx, id1, "https://example.com/test.iso", "new_version", "")
	_ = repo.UpdateDownloadStatus(ctx, id1, "downloading")

	// Entry with downloaded status — should be skipped.
	id2, _ := svc.CreateCatalogEntry(ctx, model.ISOCatalogCreateRequest{
		Name:            "Already Downloaded",
		Distro:          "test",
		CheckURL:        "https://example.com/",
		FilenamePattern: `test-\d+\.iso`,
	})
	_ = repo.UpdateAfterCheck(ctx, id2, "https://example.com/test2.iso", "downloaded", "")
	_ = repo.UpdateDownloadStatus(ctx, id2, "downloaded")

	err := svc.QueueDownloadAll(ctx)
	if err != nil {
		t.Fatalf("QueueDownloadAll: %v", err)
	}

	// Verify statuses unchanged.
	e1, _ := svc.GetCatalogEntry(ctx, id1)
	if e1.DownloadStatus != "downloading" {
		t.Errorf("downloading entry should be untouched, got %q", e1.DownloadStatus)
	}
	e2, _ := svc.GetCatalogEntry(ctx, id2)
	if e2.DownloadStatus != "downloaded" {
		t.Errorf("downloaded entry should be untouched, got %q", e2.DownloadStatus)
	}
}

// ---------- Test 17: ScrapeLatestISO ----------

func TestScrapeLatestISO(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body>
<a href="debian-12.5.0-amd64-netinst.iso">debian-12.5.0-amd64-netinst.iso</a>
<a href="debian-12.6.0-amd64-netinst.iso">debian-12.6.0-amd64-netinst.iso</a>
<a href="debian-12.7.0-amd64-netinst.iso">debian-12.7.0-amd64-netinst.iso</a>
<a href="SHA256SUMS">SHA256SUMS</a>
</body></html>`)
	}))
	defer srv.Close()

	t.Run("finds_latest_match", func(t *testing.T) {
		url, err := ScrapeLatestISO(context.Background(), srv.URL+"/", `debian-12\.\d+\.\d+-amd64-netinst\.iso`)
		if err != nil {
			t.Fatalf("ScrapeLatestISO: %v", err)
		}
		if !strings.Contains(url, "debian-12.7.0") {
			t.Errorf("expected latest (12.7.0), got %q", url)
		}
	})

	t.Run("no_match_returns_empty", func(t *testing.T) {
		url, err := ScrapeLatestISO(context.Background(), srv.URL+"/", `nonexistent-\d+\.iso`)
		if err != nil {
			t.Fatalf("ScrapeLatestISO: %v", err)
		}
		if url != "" {
			t.Errorf("expected empty string for no match, got %q", url)
		}
	})

	t.Run("invalid_pattern", func(t *testing.T) {
		_, err := ScrapeLatestISO(context.Background(), srv.URL+"/", `[invalid`)
		if err == nil {
			t.Fatal("expected error for invalid regex")
		}
	})

	t.Run("http_error", func(t *testing.T) {
		badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer badSrv.Close()

		_, err := ScrapeLatestISO(context.Background(), badSrv.URL+"/", `test-\d+\.iso`)
		if err == nil {
			t.Fatal("expected error for HTTP 404")
		}
	})
}

// ---------- Test 18: dbEntryToModel helper ----------

func TestDbEntryToModel(t *testing.T) {
	now := time.Now()
	e := &db.ISOCatalogDBEntry{
		ID:                 42,
		Name:               "Test",
		Distro:             "debian",
		Variant:            "netinst",
		Arch:               "amd64",
		CheckURL:           "https://example.com/",
		FilenamePattern:    `debian-\d+\.iso`,
		CurrentURL:         "https://example.com/debian-12.iso",
		AutoUpdate:         true,
		CheckIntervalHours: 12,
		LastChecked:        sql.NullString{String: "2025-01-01", Valid: true},
		LastError:          "",
		Status:             "available",
		DownloadStatus:     "downloaded",
		SHA256:             "abc",
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	m := dbEntryToModel(e)
	if m.ID != 42 {
		t.Errorf("expected ID=42, got %d", m.ID)
	}
	if m.Name != "Test" {
		t.Errorf("expected Name=Test, got %q", m.Name)
	}
	if m.Distro != "debian" {
		t.Errorf("expected Distro=debian, got %q", m.Distro)
	}
	if !m.AutoUpdate {
		t.Error("expected AutoUpdate=true")
	}
	if m.LastChecked != "2025-01-01" {
		t.Errorf("expected LastChecked='2025-01-01', got %q", m.LastChecked)
	}
	if m.DownloadStatus != "downloaded" {
		t.Errorf("expected DownloadStatus=downloaded, got %q", m.DownloadStatus)
	}
}
