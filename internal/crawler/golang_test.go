package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestGoFetchReleases(t *testing.T) {
	versions := []goVersion{
		{
			Version: "go1.26.2",
			Files: []goFile{
				{OS: "linux", Arch: "amd64", Kind: "archive", Filename: "go1.26.2.linux-amd64.tar.gz", Size: 145000000, SHA256: "abc123"},
				{OS: "linux", Arch: "arm64", Kind: "archive", Filename: "go1.26.2.linux-arm64.tar.gz", Size: 138000000, SHA256: "def456"},
				{OS: "darwin", Arch: "arm64", Kind: "archive", Filename: "go1.26.2.darwin-arm64.tar.gz", Size: 142000000, SHA256: "ghi789"},
				{OS: "windows", Arch: "amd64", Kind: "archive", Filename: "go1.26.2.windows-amd64.zip", Size: 150000000, SHA256: "jkl012"},
				{OS: "", Arch: "", Kind: "source", Filename: "go1.26.2.src.tar.gz", Size: 30000000, SHA256: "src111"},
			},
		},
		{
			Version: "go1.26.1",
			Files: []goFile{
				{OS: "linux", Arch: "amd64", Kind: "archive", Filename: "go1.26.1.linux-amd64.tar.gz", Size: 144000000, SHA256: "old222"},
			},
		},
	}
	body, _ := json.Marshal(versions)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/dl/" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if q := r.URL.Query().Get("mode"); q != "json" {
			t.Errorf("mode query = %q, want %q", q, "json")
		}
		if q := r.URL.Query().Get("include"); q != "all" {
			t.Errorf("include query = %q, want %q", q, "all")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGoCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "ignored", "ignored")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// 4 archive files from latest (1.26.2), minus 1 source = 4 assets.
	// Previous version (1.26.1) should NOT be included.
	if len(assets) != 4 {
		t.Fatalf("expected 4 assets, got %d", len(assets))
	}

	// Check first asset.
	if assets[0].Version != "1.26.2" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "1.26.2")
	}
	if assets[0].Filename != "go1.26.2.linux-amd64.tar.gz" {
		t.Errorf("asset[0].Filename = %q", assets[0].Filename)
	}
	if assets[0].OS != "linux" || assets[0].Arch != "amd64" {
		t.Errorf("asset[0] OS/Arch = %q/%q", assets[0].OS, assets[0].Arch)
	}
	if assets[0].Ext != "tar.gz" {
		t.Errorf("asset[0].Ext = %q, want %q", assets[0].Ext, "tar.gz")
	}
	if assets[0].DownloadURL != "https://go.dev/dl/go1.26.2.linux-amd64.tar.gz" {
		t.Errorf("asset[0].DownloadURL = %q", assets[0].DownloadURL)
	}
	if assets[0].SizeBytes != 145000000 {
		t.Errorf("asset[0].SizeBytes = %d, want %d", assets[0].SizeBytes, 145000000)
	}
	if assets[0].Checksum != "abc123" {
		t.Errorf("asset[0].Checksum = %q, want %q", assets[0].Checksum, "abc123")
	}

	// Check darwin asset.
	if assets[2].OS != "darwin" || assets[2].Arch != "arm64" {
		t.Errorf("asset[2] OS/Arch = %q/%q", assets[2].OS, assets[2].Arch)
	}
	// Check windows asset uses zip.
	if assets[3].Ext != "zip" {
		t.Errorf("asset[3].Ext = %q, want %q", assets[3].Ext, "zip")
	}
}

func TestGoSkipsSourceFiles(t *testing.T) {
	versions := []goVersion{
		{
			Version: "go1.26.2",
			Files: []goFile{
				{OS: "", Arch: "", Kind: "source", Filename: "go1.26.2.src.tar.gz", Size: 30000000, SHA256: "src1"},
				{OS: "", Arch: "", Kind: "archive", Filename: "go1.26.2.src.tar.gz", Size: 30000001, SHA256: "src2"},
				{OS: "linux", Arch: "arm64", Kind: "archive", Filename: "go1.26.2.linux-arm64.tar.gz", Size: 138000000, SHA256: "bin1"},
			},
		},
	}
	body, _ := json.Marshal(versions)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGoCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset (only binary), got %d", len(assets))
	}
	if assets[0].Filename != "go1.26.2.linux-arm64.tar.gz" {
		t.Errorf("unexpected asset: %q", assets[0].Filename)
	}
}

func TestGoOnlyLatestVersion(t *testing.T) {
	versions := []goVersion{
		{
			Version: "go1.26.2",
			Files: []goFile{
				{OS: "linux", Arch: "arm64", Kind: "archive", Filename: "go1.26.2.linux-arm64.tar.gz", Size: 138000000, SHA256: "latest"},
			},
		},
		{
			Version: "go1.25.1",
			Files: []goFile{
				{OS: "linux", Arch: "arm64", Kind: "archive", Filename: "go1.25.1.linux-arm64.tar.gz", Size: 130000000, SHA256: "old"},
			},
		},
	}
	body, _ := json.Marshal(versions)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGoCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset from latest only, got %d", len(assets))
	}
	if assets[0].Version != "1.26.2" {
		t.Errorf("Version = %q, want %q", assets[0].Version, "1.26.2")
	}
	if assets[0].Checksum != "latest" {
		t.Errorf("got old version asset, expected latest")
	}
}

func TestGoEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewGoCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}

func TestGoNonOKStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`error`))
	}))
	defer srv.Close()

	c := NewGoCrawler(nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGoNameAndSourceType(t *testing.T) {
	c := NewGoCrawler(nil)
	if c.Name() != "go" {
		t.Errorf("Name() = %q, want %q", c.Name(), "go")
	}
	if c.SourceType() != model.SourceTypeGo {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeGo)
	}
}
