package crawler

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// TestHashiCorpEmptyProduct verifies the fail-fast guard: an empty product
// name (the historical seed bug, issue #60) must produce a clear config error
// before any HTTP request, not a misleading 403 from /v1/releases/.
func TestHashiCorpEmptyProduct(t *testing.T) {
	c := NewHashiCorpCrawler("", nil)

	_, err := c.FetchReleases(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for empty product name")
	}
	if !strings.Contains(err.Error(), "product name is empty") {
		t.Errorf("error should explain the empty product name, got: %v", err)
	}
}

// TestHashiCorp403WithNilLogger verifies the 403 branch does not panic when
// constructed without a logger (the production wiring always passes one, but
// tests and embeddings may not).
func TestHashiCorp403WithNilLogger(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte(`{"message":"authorization failure","code":31002}`))
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "consul", "")
	if err == nil {
		t.Fatal("expected error for 403 response")
	}
	if !strings.Contains(err.Error(), "token required") {
		t.Errorf("403 without token should mention token requirement, got: %v", err)
	}
}

// TestHashiCorpProductFromURL verifies the product-name fallback used by
// getParams for hashicorp projects seeded without github_owner.
func TestHashiCorpProductFromURL(t *testing.T) {
	cases := []struct{ url, want string }{
		{"https://releases.hashicorp.com/consul/", "consul"},
		{"https://releases.hashicorp.com/packer", "packer"},
		{"https://releases.hashicorp.com/nomad/", "nomad"},
		{"", ""},
		{"https://releases.hashicorp.com/", "releases.hashicorp.com"},
	}
	for _, c := range cases {
		if got := hashicorpProductFromURL(c.url); got != c.want {
			t.Errorf("hashicorpProductFromURL(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

// TestMatchAnyGlob verifies the per-project asset filter semantics
// (case-insensitive filepath.Match globs).
func TestMatchAnyGlob(t *testing.T) {
	pats := []string{"vmutils-*"}
	if !matchAnyGlob("vmutils-v1.150.0-linux-arm64.tar.gz", pats) {
		t.Error("vmutils-* should match vmutils-v1.150.0-linux-arm64.tar.gz")
	}
	if !matchAnyGlob("VMUTILS-v1.150.0.tar.gz", pats) {
		t.Error("matching should be case-insensitive")
	}
	if matchAnyGlob("victoria-metrics-linux-arm64.tar.gz", pats) {
		t.Error("vmutils-* should not match victoria-metrics assets")
	}
	if matchAnyGlob("vmutils-x", []string{}) {
		t.Error("empty pattern list should match nothing")
	}
}

// TestGetParams_HashiCorpFallback verifies that a hashicorp project whose
// settings lack github_owner (the legacy seed shape, issue #60) still gets a
// product name derived from its source URL.
func TestGetParams_HashiCorpFallback(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Project with the legacy broken shape: hashicorp, no owner in config.
	if _, err := db.Exec(
		`INSERT INTO projects (name, display_name, source_type, source_url, config)
		 VALUES ('consul', 'Consul', 'hashicorp', 'https://releases.hashicorp.com/consul/', '{"crawl_interval":360}')`,
	); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	mgr := NewCrawlManager(db, nil, makeTestConfig(),
		slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})), nil)

	proj, err := mgr.projectRepo.GetByName(context.Background(), "consul")
	if err != nil || proj == nil {
		t.Fatalf("lookup project: %v", err)
	}
	params := mgr.getParams(proj)
	if params["owner"] != "consul" {
		t.Errorf("params[owner] = %q, want %q (derived from source URL)", params["owner"], "consul")
	}

	// A github project with an explicit owner must not be overridden.
	if _, err := db.Exec(
		`INSERT INTO projects (name, display_name, source_type, source_url, config)
		 VALUES ('vm', 'VM', 'github', 'https://github.com/VictoriaMetrics/VictoriaMetrics',
		         '{"github_owner":"VictoriaMetrics","github_repo":"VictoriaMetrics"}')`,
	); err != nil {
		t.Fatalf("insert github project: %v", err)
	}
	proj2, _ := mgr.projectRepo.GetByName(context.Background(), "vm")
	params2 := mgr.getParams(proj2)
	if params2["owner"] != "VictoriaMetrics" {
		t.Errorf("params2[owner] = %q, want %q", params2["owner"], "VictoriaMetrics")
	}
}

// TestProcessAssets_FilterAndSourceType verifies that (a) filter_patterns
// narrows which assets become file rows and (b) new rows carry the project's
// source_type so the file center 来源 column renders (issue #63).
func TestProcessAssets_FilterAndSourceType(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tmpDir := t.TempDir()
	// vmagent-style project: main repo artifacts, filtered to vmutils-*.
	if _, err := db.Exec(
		`INSERT INTO projects (name, display_name, source_type, source_url, config)
		 VALUES ('vmagent', 'VMAgent', 'github', 'https://github.com/VictoriaMetrics/VictoriaMetrics',
		         '{"github_owner":"VictoriaMetrics","github_repo":"VictoriaMetrics","filter_patterns":["vmutils-*"]}')`,
	); err != nil {
		t.Fatalf("insert project: %v", err)
	}

	cfg := makeTestConfig()
	cfg.Storage.BasePath = tmpDir
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	fileService := service.NewFileService(db, service.NewStorageResolver(cfg), 2, nil)
	mgr := NewCrawlManager(db, fileService, cfg, logger, nil)

	proj, _ := mgr.projectRepo.GetByName(context.Background(), "vmagent")
	releases := []model.ReleaseAsset{
		{Version: "v1.150.0", Filename: "victoria-metrics-linux-arm64-v1.150.0.tar.gz", DownloadURL: "http://example.com/vm"},
		{Version: "v1.150.0", Filename: "vmutils-v1.150.0-linux-arm64.tar.gz", DownloadURL: "http://example.com/vmutils"},
	}

	mgr.processAssets(context.Background(), proj, releases, "vmagent")

	// The filtered-out asset must not even get a file row; the vmutils one
	// must (its download fails against example.com, but the row is created).
	var count int
	if err := db.QueryRow(
		`SELECT COUNT(*) FROM files WHERE filename = ?`, "victoria-metrics-linux-arm64-v1.150.0.tar.gz",
	).Scan(&count); err != nil {
		t.Fatalf("query filtered file: %v", err)
	}
	if count != 0 {
		t.Errorf("filtered asset should have no file row, found %d", count)
	}

	// The created file row must carry the project's source_type.
	var sourceType string
	if err := db.QueryRow(
		`SELECT source_type FROM files WHERE filename = ?`, "vmutils-v1.150.0-linux-arm64.tar.gz",
	).Scan(&sourceType); err != nil {
		t.Fatalf("query created file: %v", err)
	}
	if sourceType != "github" {
		t.Errorf("files.source_type = %q, want %q", sourceType, "github")
	}
}
