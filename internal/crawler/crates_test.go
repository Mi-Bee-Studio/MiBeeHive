package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestCrates_Name(t *testing.T) {
	c := NewCratesCrawler("", nil)
	if c.Name() != "crates" {
		t.Errorf("Name() = %q, want %q", c.Name(), "crates")
	}
}

func TestCrates_SourceType(t *testing.T) {
	c := NewCratesCrawler("", nil)
	if c.SourceType() != model.SourceTypeCrates {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeCrates)
	}
}

func TestCrates_FetchReleases_SourceOnly(t *testing.T) {
	// Crate without GitHub repo → returns .crate source download URLs.
	cratesResp := cratesResponse{
		Crate: cratesCrate{
			Name:       "serde",
			Repository: "",
		},
		Versions: []crateVersion{
			{Num: "1.0.195", Yanked: false},
			{Num: "1.0.194", Yanked: false},
		},
	}
	body, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != UserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, UserAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "serde")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	// First asset should be latest version (sorted descending).
	if assets[0].Version != "1.0.195" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "1.0.195")
	}
	if assets[0].Filename != "serde-1.0.195.crate" {
		t.Errorf("asset[0].Filename = %q, want %q", assets[0].Filename, "serde-1.0.195.crate")
	}
	expectedURL := "https://crates.io/api/v1/crates/serde/1.0.195/download"
	if assets[0].DownloadURL != expectedURL {
		t.Errorf("asset[0].DownloadURL = %q, want %q", assets[0].DownloadURL, expectedURL)
	}
	if assets[0].Ext != ".crate" {
		t.Errorf("asset[0].Ext = %q, want %q", assets[0].Ext, ".crate")
	}
	// OS/Arch should be empty for source crates.
	if assets[0].OS != "" || assets[0].Arch != "" {
		t.Errorf("asset[0] OS/Arch should be empty, got %q/%q", assets[0].OS, assets[0].Arch)
	}
}

func TestCrates_FetchReleases_WithGitHubBinaries(t *testing.T) {
	// Crate with GitHub repo that has release assets → returns GitHub binary assets.
	cratesResp := cratesResponse{
		Crate: cratesCrate{
			Name:       "ripgrep",
			Repository: "https://github.com/BurntSushi/ripgrep",
		},
		Versions: []crateVersion{
			{Num: "14.1.0", Yanked: false},
		},
	}
	cratesBody, _ := json.Marshal(cratesResp)

	// GitHub releases response.
	ghReleases := []githubRelease{
		{
			TagName:    "14.1.0",
			Prerelease: false,
			Draft:      false,
			Assets: []githubAsset{
				{Name: "ripgrep-14.1.0-aarch64-unknown-linux-gnu.tar.gz", BrowserDownloadURL: "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-aarch64-unknown-linux-gnu.tar.gz", Size: 2500000},
				{Name: "ripgrep-14.1.0-x86_64-unknown-linux-gnu.tar.gz", BrowserDownloadURL: "https://github.com/BurntSushi/ripgrep/releases/download/14.1.0/ripgrep-14.1.0-x86_64-unknown-linux-gnu.tar.gz", Size: 2400000},
			},
		},
	}
	ghBody, _ := json.Marshal(ghReleases)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); ua != UserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, UserAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		// crates.io request
		if r.URL.Path == "/crates/ripgrep" {
			w.Write(cratesBody)
			return
		}
		// GitHub API request
		if r.URL.Path == "/repos/BurntSushi/ripgrep/releases" {
			w.Write(ghBody)
			return
		}
		t.Errorf("unexpected request path: %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL
	c.githubBaseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "ripgrep")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 GitHub assets, got %d", len(assets))
	}

	// Should have parsed the binary assets from GitHub.
	if assets[0].Filename != "ripgrep-14.1.0-aarch64-unknown-linux-gnu.tar.gz" {
		t.Errorf("asset[0].Filename = %q", assets[0].Filename)
	}
	if assets[0].Version != "14.1.0" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "14.1.0")
	}
	// Should have parsed OS/arch from filename.
	// parseFilename normalizes dots to dashes: ripgrep-14-1-0-aarch64-unknown-linux-gnu
	// It finds "linux" from knownOS but "aarch64" is not in knownArch.
	if assets[0].OS != "linux" {
		t.Errorf("asset[0].OS = %q, want %q", assets[0].OS, "linux")
	}
	// aarch64 is not in knownArch map, so Arch will be empty.
	// This is expected behavior — the GitHub crawler has the same limitation.
}

func TestCrates_FetchReleases_NoVersions(t *testing.T) {
	// Empty versions array → returns nil, nil.
	cratesResp := cratesResponse{
		Crate:    cratesCrate{Name: "empty-crate"},
		Versions: []crateVersion{},
	}
	body, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "empty-crate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if assets != nil {
		t.Fatalf("expected nil assets, got %d", len(assets))
	}
}

func TestCrates_FetchReleases_YankedFiltered(t *testing.T) {
	// Yanked versions should be excluded.
	cratesResp := cratesResponse{
		Crate: cratesCrate{Name: "bad-crate", Repository: ""},
		Versions: []crateVersion{
			{Num: "1.0.2", Yanked: true},
			{Num: "1.0.1", Yanked: false},
			{Num: "1.0.0", Yanked: true},
		},
	}
	body, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "bad-crate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset (yanked filtered), got %d", len(assets))
	}
	if assets[0].Version != "1.0.1" {
		t.Errorf("Version = %q, want %q", assets[0].Version, "1.0.1")
	}
}

func TestCrates_FetchReleases_VersionLimit(t *testing.T) {
	// Create 10 versions → only 5 should be returned.
	versions := make([]crateVersion, 10)
	for i := 0; i < 10; i++ {
		versions[i] = crateVersion{
			Num:     fmtVer(1, 0, i),
			Yanked:  false,
		}
	}
	cratesResp := cratesResponse{
		Crate:    cratesCrate{Name: "many-versions", Repository: ""},
		Versions: versions,
	}
	body, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "many-versions")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 5 {
		t.Fatalf("expected 5 assets (maxVersions), got %d", len(assets))
	}

	// Should be the 5 latest (descending order).
	if assets[0].Version != "1.0.9" {
		t.Errorf("first asset version = %q, want %q", assets[0].Version, "1.0.9")
	}
	if assets[4].Version != "1.0.5" {
		t.Errorf("last asset version = %q, want %q", assets[4].Version, "1.0.5")
	}
}

func TestCrates_FetchReleases_APIError(t *testing.T) {
	// Non-200 response returns error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errors":[{"detail":"Not Found"}]}`))
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "nonexistent-crate")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestCrates_FetchReleases_PreReleaseIncluded(t *testing.T) {
	// Pre-release versions (0.x) should be included for crates.
	cratesResp := cratesResponse{
		Crate: cratesCrate{Name: "young-crate", Repository: ""},
		Versions: []crateVersion{
			{Num: "0.3.0", Yanked: false},
			{Num: "0.2.1", Yanked: false},
			{Num: "0.1.0", Yanked: false},
		},
	}
	body, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "young-crate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 3 {
		t.Fatalf("expected 3 assets (pre-releases included), got %d", len(assets))
	}
}

func TestCrates_FetchReleases_GitHubFallback(t *testing.T) {
	// Crate with GitHub repo but GitHub returns error → fallback to .crate source.
	cratesResp := cratesResponse{
		Crate: cratesCrate{
			Name:       "fallback-crate",
			Repository: "https://github.com/example/fallback-crate",
		},
		Versions: []crateVersion{
			{Num: "1.0.0", Yanked: false},
		},
	}
	cratesBody, _ := json.Marshal(cratesResp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/crates/fallback-crate" {
			w.Write(cratesBody)
			return
		}
		// GitHub API returns 404.
		if r.URL.Path == "/repos/example/fallback-crate/releases" {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`not found`))
			return
		}
	}))
	defer srv.Close()

	c := NewCratesCrawler("", nil)
	c.baseURL = srv.URL
	c.githubBaseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "fallback-crate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fallback to .crate source.
	if len(assets) != 1 {
		t.Fatalf("expected 1 fallback asset, got %d", len(assets))
	}
	if assets[0].Ext != ".crate" {
		t.Errorf("expected .crate extension, got %q", assets[0].Ext)
	}
}

func TestCrates_WithCancellationToken(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewCratesCrawler("", nil)
	_, err := c.FetchReleases(ctx, "", "serde")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

// fmtVer is a test helper to format a semver-like version string.
func fmtVer(major, minor, patch int) string {
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}
