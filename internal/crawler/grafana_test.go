package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestGrafanaFetchReleases(t *testing.T) {
	release := githubRelease{
		TagName:    "v12.3.2",
		Prerelease: false,
		Draft:      false,
		Assets: []githubAsset{
			{Name: "grafana-12.3.2.linux-amd64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.3.2.linux-amd64.tar.gz", Size: 123456789},
			{Name: "grafana-12.3.2.linux-arm64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.3.2.linux-arm64.tar.gz", Size: 122000000},
			{Name: "grafana-12.3.2.darwin-amd64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.3.2.darwin-amd64.tar.gz", Size: 125000000},
			{Name: "grafana-12.3.2.darwin-arm64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.3.2.darwin-arm64.tar.gz", Size: 124000000},
			{Name: "grafana-12.3.2.windows-amd64.zip", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.3.2.windows-amd64.zip", Size: 130000000},
		},
	}
	body, _ := json.Marshal(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/grafana/grafana/releases/latest" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 5 {
		t.Fatalf("expected 5 assets, got %d", len(assets))
	}

	// Check linux-amd64 asset.
	if assets[0].Version != "12.3.2" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "12.3.2")
	}
	if assets[0].Filename != "grafana-12.3.2.linux-amd64.tar.gz" {
		t.Errorf("asset[0].Filename = %q", assets[0].Filename)
	}
	if assets[0].OS != "linux" || assets[0].Arch != "amd64" {
		t.Errorf("asset[0] OS/Arch = %q/%q", assets[0].OS, assets[0].Arch)
	}
	if assets[0].Ext != "tar.gz" {
		t.Errorf("asset[0].Ext = %q, want %q", assets[0].Ext, "tar.gz")
	}
	if assets[0].DownloadURL != "https://dl.grafana.com/oss/release/grafana-12.3.2.linux-amd64.tar.gz" {
		t.Errorf("asset[0].DownloadURL = %q", assets[0].DownloadURL)
	}
	if assets[0].SizeBytes != 123456789 {
		t.Errorf("asset[0].SizeBytes = %d, want %d", assets[0].SizeBytes, 123456789)
	}

	// Check linux-arm64 asset.
	if assets[1].OS != "linux" || assets[1].Arch != "arm64" {
		t.Errorf("asset[1] OS/Arch = %q/%q", assets[1].OS, assets[1].Arch)
	}

	// Check darwin assets.
	if assets[2].OS != "darwin" || assets[2].Arch != "amd64" {
		t.Errorf("asset[2] OS/Arch = %q/%q", assets[2].OS, assets[2].Arch)
	}
	if assets[3].OS != "darwin" || assets[3].Arch != "arm64" {
		t.Errorf("asset[3] OS/Arch = %q/%q", assets[3].OS, assets[3].Arch)
	}

	// Check windows asset.
	if assets[4].OS != "windows" || assets[4].Arch != "amd64" {
		t.Errorf("asset[4] OS/Arch = %q/%q", assets[4].OS, assets[4].Arch)
	}
	if assets[4].Ext != "zip" {
		t.Errorf("asset[4].Ext = %q, want %q", assets[4].Ext, "zip")
	}
}

func TestGrafanaSkipsPrerelease(t *testing.T) {
	release := githubRelease{
		TagName:    "v12.4.0-beta1",
		Prerelease: true,
		Draft:      false,
		Assets: []githubAsset{
			{Name: "grafana-12.4.0-beta1.linux-amd64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-12.4.0-beta1.linux-amd64.tar.gz", Size: 123456789},
		},
	}
	body, _ := json.Marshal(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets for prerelease, got %d", len(assets))
	}
}

func TestGrafanaSkipsDraft(t *testing.T) {
	release := githubRelease{
		TagName:    "v12.4.0",
		Prerelease: false,
		Draft:      true,
		Assets: []githubAsset{
			{Name: "grafana-12.4.0.linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/grafana-12.4.0.linux-amd64.tar.gz", Size: 123456789},
		},
	}
	body, _ := json.Marshal(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets for draft, got %d", len(assets))
	}
}

func TestGrafanaNonOKStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestGrafanaEmptyAssets(t *testing.T) {
	release := githubRelease{
		TagName:    "v12.3.2",
		Prerelease: false,
		Draft:      false,
		Assets:     []githubAsset{},
	}
	body, _ := json.Marshal(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}

func TestGrafanaNameAndSourceType(t *testing.T) {
	c := NewGrafanaCrawler(nil)
	if c.Name() != "grafana" {
		t.Errorf("Name() = %q, want %q", c.Name(), "grafana")
	}
	if c.SourceType() != model.SourceTypeGrafana {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeGrafana)
	}
}

func TestGrafanaVersionPrefixStripped(t *testing.T) {
	release := githubRelease{
		TagName:    "v11.0.0",
		Prerelease: false,
		Draft:      false,
		Assets: []githubAsset{
			{Name: "grafana-11.0.0.linux-arm64.tar.gz", BrowserDownloadURL: "https://dl.grafana.com/oss/release/grafana-11.0.0.linux-arm64.tar.gz", Size: 100000000},
		},
	}
	body, _ := json.Marshal(release)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGrafanaCrawler(nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "grafana", "grafana")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Version != "11.0.0" {
		t.Errorf("Version = %q, want %q (v prefix should be stripped)", assets[0].Version, "11.0.0")
	}
}
