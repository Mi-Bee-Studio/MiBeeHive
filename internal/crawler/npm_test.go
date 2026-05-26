package crawler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestNPMFetchReleases(t *testing.T) {
	resp := npmRegistryResponse{
		Name: "express",
		Versions: map[string]npmVersionEntry{
			"4.18.2": {Dist: npmDist{Tarball: "https://registry.npmjs.org/express/-/express-4.18.2.tgz", Size: 210000, Checksum: "abc123"}},
			"4.18.1": {Dist: npmDist{Tarball: "https://registry.npmjs.org/express/-/express-4.18.1.tgz", Size: 208000, Checksum: "def456"}},
			"4.17.3": {Dist: npmDist{Tarball: "https://registry.npmjs.org/express/-/express-4.17.3.tgz", Size: 200000, Checksum: "ghi789"}},
		},
	}
	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/express" {
			t.Errorf("unexpected path: %s, want /express", r.URL.Path)
		}
		if ua := r.Header.Get("User-Agent"); ua != UserAgent {
			t.Errorf("User-Agent = %q, want %q", ua, UserAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "express")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(assets))
	}

	// Check first asset (sorted by version descending, latest first)
	if assets[0].Version != "4.18.2" {
		t.Errorf("asset[0].Version = %q, want %q", assets[0].Version, "4.18.2")
	}
	if assets[0].Filename != "express-4.18.2.tgz" {
		t.Errorf("asset[0].Filename = %q, want %q", assets[0].Filename, "express-4.18.2.tgz")
	}
	if assets[0].DownloadURL != "https://registry.npmjs.org/express/-/express-4.18.2.tgz" {
		t.Errorf("asset[0].DownloadURL = %q", assets[0].DownloadURL)
	}
	if assets[0].Ext != ".tgz" {
		t.Errorf("asset[0].Ext = %q, want %q", assets[0].Ext, ".tgz")
	}
	if assets[0].SizeBytes != 210000 {
		t.Errorf("asset[0].SizeBytes = %d, want %d", assets[0].SizeBytes, 210000)
	}
	if assets[0].Checksum != "abc123" {
		t.Errorf("asset[0].Checksum = %q, want %q", assets[0].Checksum, "abc123")
	}
	// OS and Arch should be empty for NPM packages
	if assets[0].OS != "" || assets[0].Arch != "" {
		t.Errorf("asset[0] OS/Arch should be empty, got %q/%q", assets[0].OS, assets[0].Arch)
	}
}

func TestNPMPreReleaseFiltering(t *testing.T) {
	resp := npmRegistryResponse{
		Name: "lodash",
		Versions: map[string]npmVersionEntry{
			"4.17.21":       {Dist: npmDist{Tarball: "https://registry.npmjs.org/lodash/-/lodash-4.17.21.tgz", Size: 300000}},
			"4.18.0-beta.1": {Dist: npmDist{Tarball: "https://registry.npmjs.org/lodash/-/lodash-4.18.0-beta.1.tgz", Size: 310000}},
			"4.17.20-alpha": {Dist: npmDist{Tarball: "https://registry.npmjs.org/lodash/-/lodash-4.17.20-alpha.tgz", Size: 290000}},
			"4.17.19-rc.0":  {Dist: npmDist{Tarball: "https://registry.npmjs.org/lodash/-/lodash-4.17.19-rc.0.tgz", Size: 280000}},
		},
	}
	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "lodash")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 stable asset, got %d", len(assets))
	}
	if assets[0].Version != "4.17.21" {
		t.Errorf("Version = %q, want %q", assets[0].Version, "4.17.21")
	}
}

func TestNPMVersionLimit(t *testing.T) {
	// Create 10 versions to test that only the latest N are returned
	versions := make(map[string]npmVersionEntry)
	for i := 10; i >= 1; i-- {
		ver := "1.0." + string(rune('0'+i))
		versions[ver] = npmVersionEntry{
			Dist: npmDist{Tarball: "https://registry.npmjs.org/pkg/-/pkg-" + ver + ".tgz", Size: 1000},
		}
	}
	resp := npmRegistryResponse{
		Name:     "pkg",
		Versions: versions,
	}
	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL
	c.maxVersions = 5

	assets, err := c.FetchReleases(context.Background(), "", "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 5 {
		t.Fatalf("expected 5 assets (maxVersions), got %d", len(assets))
	}
}

func TestNPMScopePackages(t *testing.T) {
	resp := npmRegistryResponse{
		Name: "@types/node",
		Versions: map[string]npmVersionEntry{
			"20.11.0": {Dist: npmDist{Tarball: "https://registry.npmjs.org/@types/node/-/node-20.11.0.tgz", Size: 45000}},
			"20.10.0": {Dist: npmDist{Tarball: "https://registry.npmjs.org/@types/node/-/node-20.10.0.tgz", Size: 44000}},
		},
	}
	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Scoped package URL path: /@types/node
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "@types", "node")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}
	if assets[0].Filename != "node-20.11.0.tgz" {
		t.Errorf("asset[0].Filename = %q, want %q", assets[0].Filename, "node-20.11.0.tgz")
	}
}

func TestNPMErrorHandling404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"Not found"}`))
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "nonexistent-package")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestNPMErrorHandling500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`internal server error`))
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "some-package")
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestNPMEmptyVersions(t *testing.T) {
	resp := npmRegistryResponse{
		Name:     "empty-pkg",
		Versions: map[string]npmVersionEntry{},
	}
	body, _ := json.Marshal(resp)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "empty-pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}

func TestNPMInvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`not valid json`))
	}))
	defer srv.Close()

	c := NewNPMCrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "bad-pkg")
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestNPMNameAndSourceType(t *testing.T) {
	c := NewNPMCrawler("", nil)
	if c.Name() != "npm" {
		t.Errorf("Name() = %q, want %q", c.Name(), "npm")
	}
	if c.SourceType() != model.SourceTypeNPM {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypeNPM)
	}
}

func TestNPMWithContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewNPMCrawler("", nil)
	_, err := c.FetchReleases(ctx, "", "express")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}
