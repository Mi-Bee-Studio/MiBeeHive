package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestFetchReleases(t *testing.T) {
	releases := []githubRelease{
		{
			TagName:    "v3.11.3",
			Prerelease: false,
			Draft:      false,
			Assets: []githubAsset{
				{Name: "prometheus-3.11.3.linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/p.tar.gz", Size: 12345678},
				{Name: "prometheus-3.11.3.darwin-amd64.tar.gz", BrowserDownloadURL: "https://example.com/p-darwin.tar.gz", Size: 12345679},
			},
		},
		{
			TagName:    "v3.12.0-rc.0",
			Prerelease: true,
			Draft:      false,
			Assets: []githubAsset{
				{Name: "prometheus-3.12.0-rc.0.linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/p-rc.tar.gz", Size: 12345680},
			},
		},
		{
			TagName:    "v3.13.0",
			Prerelease: false,
			Draft:      true,
			Assets: []githubAsset{
				{Name: "prometheus-3.13.0.linux-arm64.tar.gz", BrowserDownloadURL: "https://example.com/p-draft.tar.gz", Size: 12345681},
			},
		},
	}
	body, _ := json.Marshal(releases)

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if r.URL.Path != "/repos/owner/repo/releases" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("X-RateLimit-Remaining", "60")
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGitHubCrawler("mytoken", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "owner", "repo")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	if gotAuth != "token mytoken" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "token mytoken")
	}

	if assets[0].Filename != "prometheus-3.11.3.linux-arm64.tar.gz" {
		t.Errorf("asset[0].Filename = %q", assets[0].Filename)
	}
	if assets[0].Version != "3.11.3" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "3.11.3")
	}
	if assets[0].OS != "linux" || assets[0].Arch != "arm64" || assets[0].Ext != "tar.gz" {
		t.Errorf("asset[0] OS/Arch/Ext = %q/%q/%q", assets[0].OS, assets[0].Arch, assets[0].Ext)
	}
	if assets[0].SizeBytes != 12345678 {
		t.Errorf("asset[0].SizeBytes = %d, want %d", assets[0].SizeBytes, 12345678)
	}

	if assets[1].OS != "darwin" || assets[1].Arch != "amd64" {
		t.Errorf("asset[1] OS/Arch = %q/%q", assets[1].OS, assets[1].Arch)
	}
}

func TestFilenameParsing(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		wantOS   string
		wantArch string
		wantExt  string
	}{
		{"prometheus style", "prometheus-3.11.3.linux-arm64.tar.gz", "linux", "arm64", "tar.gz"},
		{"victoria-metrics style", "victoria-metrics-darwin-amd64-v1.142.0.tar.gz", "darwin", "amd64", "tar.gz"},
		{"consul underscore style", "consul_1.22.5_linux_arm64.zip", "linux", "arm64", "zip"},
		{"node_exporter style", "node_exporter-1.9.0.linux-amd64.tar.gz", "linux", "amd64", "tar.gz"},
		{"blackbox_exporter style", "blackbox_exporter-0.25.0.linux-arm64.tar.gz", "linux", "arm64", "tar.gz"},
		{"mysqld_exporter style", "mysqld_exporter-0.16.0.linux-arm64.tar.gz", "linux", "arm64", "tar.gz"},
		{"windows amd64 zip", "prometheus-3.11.3.windows-amd64.zip", "windows", "amd64", "zip"},
		{"freebsd arm64", "node_exporter-1.9.0.freebsd-arm64.tar.gz", "freebsd", "arm64", "tar.gz"},
		{"no os arch in filename", "README.txt", "", "", "txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseFilename(tt.filename)
			if got.OS != tt.wantOS {
				t.Errorf("OS = %q, want %q", got.OS, tt.wantOS)
			}
			if got.Arch != tt.wantArch {
				t.Errorf("Arch = %q, want %q", got.Arch, tt.wantArch)
			}
			if got.Ext != tt.wantExt {
				t.Errorf("Ext = %q, want %q", got.Ext, tt.wantExt)
			}
		})
	}
}

func TestRateLimitHandling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "3")
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewGitHubCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected rate limit error, got nil")
	}
	if !isRateLimitError(err) {
		t.Errorf("isRateLimitError should return true for: %v", err)
	}
	if !errors.Is(err, ErrRateLimited) {
		t.Error("errors.Is should detect ErrRateLimited sentinel")
	}
}

func TestRateLimitNotTriggered(t *testing.T) {
	releases := []githubRelease{
		{TagName: "v1.0.0", Prerelease: false, Draft: false, Assets: []githubAsset{{Name: "app-1.0.0.linux-amd64.tar.gz", BrowserDownloadURL: "https://example.com/a.tar.gz", Size: 100}}},
	}
	body, _ := json.Marshal(releases)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "5")
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewGitHubCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "o", "r")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
}

func TestNonOKStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "60")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer srv.Close()

	c := NewGitHubCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFileExt(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"tar.gz", "file.tar.gz", "tar.gz"},
		{"tar.bz2", "file.tar.bz2", "tar.bz2"},
		{"tar.xz", "file.tar.xz", "tar.xz"},
		{"zip", "file.zip", "zip"},
		{"no ext", "file", ""},
		{"tgz", "file.tgz", "tgz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fileExt(tt.in); got != tt.want {
				t.Errorf("fileExt(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNameAndSourceType(t *testing.T) {
	c := NewGitHubCrawler("", nil)
	if c.Name() != "github" {
		t.Errorf("Name() = %q, want %q", c.Name(), "github")
	}
	if c.SourceType() != model.SourceTypeGitHub {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeGitHub)
	}
}
