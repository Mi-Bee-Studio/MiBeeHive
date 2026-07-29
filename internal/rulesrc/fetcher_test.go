package rulesrc

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// stubGetter serves fixed bytes for a given fixture filename, offline.
type stubGetter struct {
	fixture string
	baseURL string
}

func (s stubGetter) Get(_ context.Context, _ Request) (io.ReadCloser, string, error) {
	f, err := os.Open(filepath.Join("testdata", s.fixture))
	if err != nil {
		return nil, s.baseURL, err
	}
	return f, s.baseURL, nil
}

func mustLoadSpec(t *testing.T, src string) *Spec {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", src+".yaml"))
	if err != nil {
		t.Fatalf("read spec %s: %v", src, err)
	}
	spec, err := ParseSpec(data)
	if err != nil {
		t.Fatalf("parse spec %s: %v", src, err)
	}
	return spec
}

// Source A: GitHub Releases (JSON API). Baseline: nested []assets, skip prereleases.
func TestFetchGitHubReleases(t *testing.T) {
	spec := mustLoadSpec(t, "github_releases")
	f := newFetcherWith(stubGetter{fixture: "github_releases.json", baseURL: "https://api.github.com/repos/prometheus/prometheus/releases"})

	assets, err := f.Fetch(context.Background(), spec)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// 3 stable assets; the prerelease release must be skipped entirely.
	if len(assets) != 3 {
		t.Fatalf("want 3 assets (prerelease skipped), got %d: %+v", len(assets), assets)
	}
	// First asset: version stripped of "v", os/arch/ext classified.
	a := assets[0]
	if a.Version != "3.11.3" {
		t.Errorf("version: want 3.11.3, got %q", a.Version)
	}
	if a.OS != "linux" || a.Arch != "amd64" || a.Ext != "tar.gz" {
		t.Errorf("classify linux/amd64/tar.gz, got %s/%s/%s", a.OS, a.Arch, a.Ext)
	}
	if a.SizeBytes != 104857600 {
		t.Errorf("size: want 104857600, got %d", a.SizeBytes)
	}
	if a.DownloadURL == "" {
		t.Error("download_url empty")
	}
}

// Source C: structured JSON API (Crates.io style). Flat versions[]; each unit
// is its own asset; relative dl_path resolved is the engine's caller concern here.
func TestFetchCratesJSON(t *testing.T) {
	spec := mustLoadSpec(t, "crates_versions")
	f := newFetcherWith(stubGetter{fixture: "crates_versions.json", baseURL: "https://crates.io/api/v1/crates/ripgrep"})

	assets, err := f.Fetch(context.Background(), spec)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(assets) != 3 {
		t.Fatalf("want 3 versions, got %d", len(assets))
	}
	want := map[string]bool{"14.1.1": false, "14.1.0": false, "13.0.0": false}
	for _, a := range assets {
		if _, ok := want[a.Version]; !ok {
			t.Errorf("unexpected version %q", a.Version)
		}
		want[a.Version] = true
		if a.SizeBytes == 0 {
			t.Errorf("version %q: size not read", a.Version)
		}
	}
}

// Source B: Apache autoindex HTML. Relative hrefs resolved against base URL;
// parent dir and query links dropped; include glob keeps only tar.gz.
func TestFetchApacheDirHTML(t *testing.T) {
	spec := mustLoadSpec(t, "apache_dir")
	f := newFetcherWith(stubGetter{fixture: "apache_dir.html", baseURL: "https://dl.example.org/tools/"})

	assets, err := f.Fetch(context.Background(), spec)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	assets = ApplyFilter(assets, spec)
	// 3 tar.gz entries; README.txt and query/parent links dropped.
	if len(assets) != 3 {
		t.Fatalf("want 3 tar.gz assets after filter, got %d: %+v", len(assets), assets)
	}
	a := assets[0]
	if a.OS != "linux" || a.Arch != "amd64" {
		t.Errorf("classify linux/amd64, got %s/%s", a.OS, a.Arch)
	}
	// download_url must be absolute (resolved against base).
	if a.DownloadURL != "https://dl.example.org/tools/mytool-1.2.3-linux-amd64.tar.gz" {
		t.Errorf("download_url not resolved: %q", a.DownloadURL)
	}
	// version extracted from filename via regex capture group.
	if a.Version != "1.2.3" {
		t.Errorf("version: want 1.2.3, got %q", a.Version)
	}
}

// Source D (stress test): HashiCorp two-level structure (version dir -> file dir).
// This test deliberately documents the LIMITATION: a single-request fingerprint
// can only fetch the version LIST, not the files inside each version. That
// multi-step behavior cannot be expressed by one declarative spec.
func TestFetchHashicorpVersionListOnly(t *testing.T) {
	spec := mustLoadSpec(t, "hashicorp_versions")
	f := newFetcherWith(stubGetter{fixture: "hashicorp_versions.html", baseURL: "https://releases.hashicorp.com/consul/"})

	assets, err := f.Fetch(context.Background(), spec)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	// We can enumerate versions (3), but each "asset" here is a version
	// directory, NOT a downloadable file. This is the boundary.
	if len(assets) != 3 {
		t.Fatalf("want 3 version dirs, got %d", len(assets))
	}
	for _, a := range assets {
		// A directory name like "1.22.5/" has no OS/arch — the key finding is
		// that no real downloadable artifact metadata is available from this
		// single page (Ext/OS are noise from classifying a dir name).
		if a.OS != "" {
			t.Errorf("version dir unexpectedly has an OS: %+v", a)
		}
	}
}

func TestParseSpecValidation(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{"missing name", "apiVersion: rulesrc/v1\nkind: Source\n", true},
		{"missing url", "apiVersion: rulesrc/v1\nkind: Source\nname: x\nrequest:\n  format: json\n", true},
		{"bad format", "name: x\nrequest:\n  url: http://x\n  format: xml\nlist:\n  path: \"[]\"\n", true},
		{"json ok", "name: x\nrequest:\n  url: http://x\n  format: json\nlist:\n  path: \"[]\"\n", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseSpec([]byte(c.yaml))
			if (err != nil) != c.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, c.wantErr)
			}
		})
	}
}

func TestClassifyMatches(t *testing.T) {
	cases := []struct {
		name          string
		os, arch, ext string
	}{
		{"prometheus-3.11.3.linux-arm64.tar.gz", "linux", "arm64", "tar.gz"},
		{"consul_1.22.5_linux_arm64.zip", "linux", "arm64", "zip"},
		{"node_exporter-1.9.0.linux-amd64.tar.gz", "linux", "amd64", "tar.gz"},
	}
	for _, c := range cases {
		got := classify(c.name)
		if got.os != c.os || got.arch != c.arch || got.ext != c.ext {
			t.Errorf("classify(%q) = %+v, want %s/%s/%s", c.name, got, c.os, c.arch, c.ext)
		}
	}
}

// ensure model.ReleaseAsset is the output type (compile-time contract).
var _ = []model.ReleaseAsset{}
