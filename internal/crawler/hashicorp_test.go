package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestHashiCorpFetchReleases(t *testing.T) {
	releases := []hashiCorpRelease{
		{
			Version: "1.22.5",
			Builds: []hashiCorpBuild{
				{OS: "linux", Arch: "arm64", URL: "https://releases.hashicorp.com/consul/1.22.5/consul_1.22.5_linux_arm64.zip", Filename: "consul_1.22.5_linux_arm64.zip", SHA256: "abc123def"},
				{OS: "linux", Arch: "amd64", URL: "https://releases.hashicorp.com/consul/1.22.5/consul_1.22.5_linux_amd64.zip", Filename: "consul_1.22.5_linux_amd64.zip", SHA256: "def456ghi"},
				{OS: "darwin", Arch: "arm64", URL: "https://releases.hashicorp.com/consul/1.22.5/consul_1.22.5_darwin_arm64.zip", Filename: "consul_1.22.5_darwin_arm64.zip", SHA256: "jkl789mno"},
				{OS: "windows", Arch: "amd64", URL: "https://releases.hashicorp.com/consul/1.22.5/consul_1.22.5_windows_amd64.zip", Filename: "consul_1.22.5_windows_amd64.zip", SHA256: "pqr012stu"},
			},
		},
		{
			Version: "1.22.4",
			Builds: []hashiCorpBuild{
				{OS: "linux", Arch: "arm64", URL: "https://releases.hashicorp.com/consul/1.22.4/consul_1.22.4_linux_arm64.zip", Filename: "consul_1.22.4_linux_arm64.zip", SHA256: "old"},
			},
		},
	}
	body, _ := json.Marshal(releases)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/releases/consul" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "consul", "ignored")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Only latest version (1.22.5), 4 builds.
	if len(assets) != 4 {
		t.Fatalf("expected 4 assets, got %d", len(assets))
	}

	// Check first asset.
	if assets[0].Version != "1.22.5" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "1.22.5")
	}
	if assets[0].Filename != "consul_1.22.5_linux_arm64.zip" {
		t.Errorf("asset[0].Filename = %q", assets[0].Filename)
	}
	if assets[0].OS != "linux" || assets[0].Arch != "arm64" {
		t.Errorf("asset[0] OS/Arch = %q/%q", assets[0].OS, assets[0].Arch)
	}
	if assets[0].Ext != "zip" {
		t.Errorf("asset[0].Ext = %q, want %q", assets[0].Ext, "zip")
	}
	if assets[0].DownloadURL != "https://releases.hashicorp.com/consul/1.22.5/consul_1.22.5_linux_arm64.zip" {
		t.Errorf("asset[0].DownloadURL = %q", assets[0].DownloadURL)
	}
	if assets[0].Checksum != "abc123def" {
		t.Errorf("asset[0].Checksum = %q, want %q", assets[0].Checksum, "abc123def")
	}

	// Check darwin asset.
	if assets[2].OS != "darwin" || assets[2].Arch != "arm64" {
		t.Errorf("asset[2] OS/Arch = %q/%q", assets[2].OS, assets[2].Arch)
	}

	// Check windows asset.
	if assets[3].OS != "windows" || assets[3].Arch != "amd64" {
		t.Errorf("asset[3] OS/Arch = %q/%q", assets[3].OS, assets[3].Arch)
	}
}

func TestHashiCorpOnlyLatestVersion(t *testing.T) {
	releases := []hashiCorpRelease{
		{
			Version: "1.8.5",
			Builds: []hashiCorpBuild{
				{OS: "linux", Arch: "arm64", URL: "https://example.com/packer_1.8.5_linux_arm64.zip", Filename: "packer_1.8.5_linux_arm64.zip", SHA256: "latest"},
			},
		},
		{
			Version: "1.8.4",
			Builds: []hashiCorpBuild{
				{OS: "linux", Arch: "arm64", URL: "https://example.com/packer_1.8.4_linux_arm64.zip", Filename: "packer_1.8.4_linux_arm64.zip", SHA256: "old"},
			},
		},
	}
	body, _ := json.Marshal(releases)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "packer", "ignored")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset from latest only, got %d", len(assets))
	}
	if assets[0].Version != "1.8.5" {
		t.Errorf("Version = %q, want %q", assets[0].Version, "1.8.5")
	}
	if assets[0].Checksum != "latest" {
		t.Errorf("got old version asset, expected latest")
	}
}

func TestHashiCorpEmptyResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "consul", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}

func TestHashiCorpNonOKStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "product not found"}`))
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "nonexistent", "")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestHashiCorpNameAndSourceType(t *testing.T) {
	c := NewHashiCorpCrawler("test-token", nil)
	if c.Name() != "hashicorp" {
		t.Errorf("Name() = %q, want %q", c.Name(), "hashicorp")
	}
	if c.SourceType() != model.SourceTypeHashiCorp {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeHashiCorp)
	}
}

func TestHashiCorpTokenAuthorization(t *testing.T) {
	// Without token — no Authorization header.
	var noTokenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noTokenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewHashiCorpCrawler("", nil)
	c.baseURL = srv.URL
	_, err := c.FetchReleases(context.Background(), "consul", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if noTokenAuth != "" {
		t.Errorf("expected no Authorization header, got %q", noTokenAuth)
	}

	// With token — Authorization header should be set.
	var receivedAuth string
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv2.Close()

	c2 := NewHashiCorpCrawler("my-secret-token", nil)
	c2.baseURL = srv2.URL
	_, err = c2.FetchReleases(context.Background(), "consul", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer my-secret-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer my-secret-token")
	}
}
