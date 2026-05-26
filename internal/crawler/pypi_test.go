package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// --- PyPI JSON API mock response helpers ---

func makePyPIResponse(releases map[string][]pypiFileEntry) pypiAPIResponse {
	return pypiAPIResponse{
		Info:     pypiInfo{Version: "1.0.0", Name: "test-pkg"},
		Releases: releases,
	}
}

func servePyPI(t *testing.T, resp pypiAPIResponse) *httptest.Server {
	t.Helper()
	body, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal pypi response: %v", err)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Accept any package name in the path
		if !strings.Contains(r.URL.Path, "/pypi/") || !strings.HasSuffix(r.URL.Path, "/json") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

// --- Tests ---

func TestPyPINameAndSourceType(t *testing.T) {
	c := NewPyPICrawler("", nil)
	if c.Name() != "pypi" {
		t.Errorf("Name() = %q, want %q", c.Name(), "pypi")
	}
	if c.SourceType() != model.SourceTypePyPI {
		t.Errorf("SourceType() = %q, want %q", c.SourceType(), model.SourceTypePyPI)
	}
}

func TestPyPIFetchReleasesBasic(t *testing.T) {
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"2.31.0": {
			{Filename: "requests-2.31.0-py3-none-any.whl", URL: "https://files.pythonhosted.org/packages/requests-2.31.0-py3-none-any.whl", Size: 62000, PackageType: "bdist_wheel", PythonVersion: "py3"},
			{Filename: "requests-2.31.0.tar.gz", URL: "https://files.pythonhosted.org/packages/requests-2.31.0.tar.gz", Size: 131000, PackageType: "sdist", PythonVersion: "source"},
		},
		"2.30.0": {
			{Filename: "requests-2.30.0-py3-none-any.whl", URL: "https://files.pythonhosted.org/packages/requests-2.30.0-py3-none-any.whl", Size: 61000, PackageType: "bdist_wheel", PythonVersion: "py3"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "ignored", "requests")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Should return assets for up to 5 latest versions.
	// 2 versions, 3 total files.
	if len(assets) != 3 {
		t.Fatalf("expected 3 assets, got %d", len(assets))
	}

	// Check wheel asset from 2.31.0
	var wheelFound bool
	for _, a := range assets {
		if a.Version == "2.31.0" && a.Ext == "whl" {
			wheelFound = true
			if a.Filename != "requests-2.31.0-py3-none-any.whl" {
				t.Errorf("Filename = %q", a.Filename)
			}
			if a.DownloadURL != "https://files.pythonhosted.org/packages/requests-2.31.0-py3-none-any.whl" {
				t.Errorf("DownloadURL = %q", a.DownloadURL)
			}
			if a.SizeBytes != 62000 {
				t.Errorf("SizeBytes = %d, want 62000", a.SizeBytes)
			}
			if a.OS != "any" {
				t.Errorf("OS = %q, want %q", a.OS, "any")
			}
			if a.Arch != "any" {
				t.Errorf("Arch = %q, want %q", a.Arch, "any")
			}
		}
	}
	if !wheelFound {
		t.Error("expected to find wheel asset for 2.31.0")
	}

	// Check sdist asset from 2.31.0
	var sdistFound bool
	for _, a := range assets {
		if a.Version == "2.31.0" && a.Ext == "tar.gz" {
			sdistFound = true
		}
	}
	if !sdistFound {
		t.Error("expected to find sdist asset for 2.31.0")
	}
}

func TestPyPIPreReleaseFiltering(t *testing.T) {
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"3.0.0": {
			{Filename: "pkg-3.0.0.tar.gz", URL: "https://example.com/pkg-3.0.0.tar.gz", Size: 1000, PackageType: "sdist", PythonVersion: "source"},
		},
		"3.0.0rc1": {
			{Filename: "pkg-3.0.0rc1.tar.gz", URL: "https://example.com/pkg-3.0.0rc1.tar.gz", Size: 1000, PackageType: "sdist", PythonVersion: "source"},
		},
		"3.0.0b2": {
			{Filename: "pkg-3.0.0b2.tar.gz", URL: "https://example.com/pkg-3.0.0b2.tar.gz", Size: 1000, PackageType: "sdist", PythonVersion: "source"},
		},
		"3.0.0a1": {
			{Filename: "pkg-3.0.0a1.tar.gz", URL: "https://example.com/pkg-3.0.0a1.tar.gz", Size: 1000, PackageType: "sdist", PythonVersion: "source"},
		},
		"2.9.0": {
			{Filename: "pkg-2.9.0.tar.gz", URL: "https://example.com/pkg-2.9.0.tar.gz", Size: 900, PackageType: "sdist", PythonVersion: "source"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "pkg")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Only stable releases: 3.0.0 and 2.9.0
	if len(assets) != 2 {
		t.Fatalf("expected 2 assets (pre-releases filtered), got %d", len(assets))
	}

	versions := make(map[string]bool)
	for _, a := range assets {
		versions[a.Version] = true
	}
	if !versions["3.0.0"] {
		t.Error("expected version 3.0.0")
	}
	if !versions["2.9.0"] {
		t.Error("expected version 2.9.0")
	}
	if versions["3.0.0rc1"] || versions["3.0.0b2"] || versions["3.0.0a1"] {
		t.Error("pre-release versions should be filtered")
	}
}

func TestPyPIWheelPlatformPreference(t *testing.T) {
	// When both platform-independent wheel and sdist exist,
	// both should be included but wheel should come first for each version.
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"1.0.0": {
			{Filename: "example-1.0.0.tar.gz", URL: "https://example.com/example-1.0.0.tar.gz", Size: 50000, PackageType: "sdist", PythonVersion: "source"},
			{Filename: "example-1.0.0-py3-none-any.whl", URL: "https://example.com/example-1.0.0-py3-none-any.whl", Size: 40000, PackageType: "bdist_wheel", PythonVersion: "py3"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "example")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 2 {
		t.Fatalf("expected 2 assets, got %d", len(assets))
	}

	// Wheel should be first (sorted preference: bdist_wheel > sdist)
	if assets[0].Ext != "whl" {
		t.Errorf("first asset should be wheel, got ext %q", assets[0].Ext)
	}
	if assets[1].Ext != "tar.gz" {
		t.Errorf("second asset should be sdist, got ext %q", assets[1].Ext)
	}
}

func TestPyPIPlatformSpecificWheelFiltering(t *testing.T) {
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"1.0.0": {
			// Platform-any wheel → should be included
			{Filename: "numpy-1.0.0-cp312-cp312-manylinux_2_17_aarch64.manylinux2014_aarch64.whl", URL: "https://example.com/aarch64.whl", Size: 100000, PackageType: "bdist_wheel", PythonVersion: "cp312"},
			// Platform-any wheel → should be included
			{Filename: "numpy-1.0.0-py3-none-any.whl", URL: "https://example.com/any.whl", Size: 50000, PackageType: "bdist_wheel", PythonVersion: "py3"},
			// x86_64 wheel → should be skipped (not linux/arm64 compatible)
			{Filename: "numpy-1.0.0-cp312-cp312-manylinux_2_17_x86_64.manylinux2014_x86_64.whl", URL: "https://example.com/x86_64.whl", Size: 100000, PackageType: "bdist_wheel", PythonVersion: "cp312"},
			// macOS wheel → should be skipped
			{Filename: "numpy-1.0.0-cp312-cp312-macosx_11_0_arm64.whl", URL: "https://example.com/macos.whl", Size: 95000, PackageType: "bdist_wheel", PythonVersion: "cp312"},
			// sdist → should be included
			{Filename: "numpy-1.0.0.tar.gz", URL: "https://example.com/numpy-1.0.0.tar.gz", Size: 200000, PackageType: "sdist", PythonVersion: "source"},
			// Windows wheel → should be skipped
			{Filename: "numpy-1.0.0-cp312-cp312-win_amd64.whl", URL: "https://example.com/win.whl", Size: 98000, PackageType: "bdist_wheel", PythonVersion: "cp312"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "numpy")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	// Should include: aarch64 wheel, any wheel, sdist (3 total)
	// Should skip: x86_64 wheel, macOS wheel, Windows wheel
	if len(assets) != 3 {
		t.Fatalf("expected 3 assets (filtered platform-specific), got %d", len(assets))
		for i, a := range assets {
			t.Logf("  asset[%d]: %s", i, a.Filename)
		}
	}

	// Verify each included asset
	filenames := make(map[string]bool)
	for _, a := range assets {
		filenames[a.Filename] = true
	}
	if !filenames["numpy-1.0.0-py3-none-any.whl"] {
		t.Error("expected platform-any wheel")
	}
	if !filenames["numpy-1.0.0-cp312-cp312-manylinux_2_17_aarch64.manylinux2014_aarch64.whl"] {
		t.Error("expected linux aarch64 wheel")
	}
	if !filenames["numpy-1.0.0.tar.gz"] {
		t.Error("expected sdist")
	}
}

func TestPyPIFilenameParsing(t *testing.T) {
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"2.0.0": {
			{Filename: "mypkg-2.0.0-cp312-cp312-manylinux_2_17_aarch64.manylinux2014_aarch64.whl", URL: "https://example.com/mypkg.whl", Size: 50000, PackageType: "bdist_wheel", PythonVersion: "cp312"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "mypkg")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}

	a := assets[0]
	if a.Ext != "whl" {
		t.Errorf("Ext = %q, want %q", a.Ext, "whl")
	}
	if a.OS != "linux" {
		t.Errorf("OS = %q, want %q", a.OS, "linux")
	}
	if a.Arch != "arm64" {
		t.Errorf("Arch = %q, want %q", a.Arch, "arm64")
	}
	if a.Version != "2.0.0" {
		t.Errorf("Version = %q, want %q", a.Version, "2.0.0")
	}
}

func TestPyPILimitVersions(t *testing.T) {
	// Create 7 versions — only latest 5 should be returned.
	releases := map[string][]pypiFileEntry{}
	for i := 0; i < 7; i++ {
		ver := fmt.Sprintf("1.%d.0", 6-i) // 1.6.0, 1.5.0, ..., 1.0.0
		releases[ver] = []pypiFileEntry{
			{Filename: fmt.Sprintf("pkg-%s.tar.gz", ver), URL: fmt.Sprintf("https://example.com/pkg-%s.tar.gz", ver), Size: 1000, PackageType: "sdist", PythonVersion: "source"},
		}
	}

	resp := makePyPIResponse(releases)
	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "pkg")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 5 {
		t.Fatalf("expected 5 assets (limited to 5 versions), got %d", len(assets))
	}

	// Latest version should be 1.6.0
	if assets[0].Version != "1.6.0" {
		t.Errorf("first asset version = %q, want %q", assets[0].Version, "1.6.0")
	}
	// Oldest included should be 1.2.0
	if assets[4].Version != "1.2.0" {
		t.Errorf("last asset version = %q, want %q", assets[4].Version, "1.2.0")
	}
}

func TestPyPIEmptyReleases(t *testing.T) {
	resp := makePyPIResponse(map[string][]pypiFileEntry{})
	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(assets) != 0 {
		t.Fatalf("expected 0 assets, got %d", len(assets))
	}
}

func TestPyPINonOKStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"message": "Not Found"}`))
	}))
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	_, err := c.FetchReleases(context.Background(), "", "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestPyPITokenAuthorization(t *testing.T) {
	// Without token — no Authorization header.
	var noTokenAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		noTokenAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		resp := makePyPIResponse(map[string][]pypiFileEntry{})
		body, _ := json.Marshal(resp)
		w.Write(body)
	}))
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL
	_, err := c.FetchReleases(context.Background(), "", "pkg")
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
		resp := makePyPIResponse(map[string][]pypiFileEntry{})
		body, _ := json.Marshal(resp)
		w.Write(body)
	}))
	defer srv2.Close()

	c2 := NewPyPICrawler("my-pypi-token", nil)
	c2.baseURL = srv2.URL
	_, err = c2.FetchReleases(context.Background(), "", "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if receivedAuth != "Bearer my-pypi-token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer my-pypi-token")
	}
}

func TestPyPIUserAgent(t *testing.T) {
	var userAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "application/json")
		resp := makePyPIResponse(map[string][]pypiFileEntry{})
		body, _ := json.Marshal(resp)
		w.Write(body)
	}))
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL
	_, err := c.FetchReleases(context.Background(), "", "pkg")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userAgent != UserAgent {
		t.Errorf("User-Agent = %q, want %q", userAgent, UserAgent)
	}
}

func TestPyPIContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := NewPyPICrawler("", nil)
	_, err := c.FetchReleases(ctx, "", "pkg")
	if err == nil {
		t.Fatal("expected error with cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error should wrap context.Canceled, got: %v", err)
	}
}

func TestPyPIOnlySDistNoWheel(t *testing.T) {
	// Package with only sdist, no wheels.
	resp := makePyPIResponse(map[string][]pypiFileEntry{
		"1.0.0": {
			{Filename: "pure-1.0.0.tar.gz", URL: "https://example.com/pure-1.0.0.tar.gz", Size: 25000, PackageType: "sdist", PythonVersion: "source"},
		},
	})

	srv := servePyPI(t, resp)
	defer srv.Close()

	c := NewPyPICrawler("", nil)
	c.baseURL = srv.URL

	assets, err := c.FetchReleases(context.Background(), "", "pure")
	if err != nil {
		t.Fatalf("FetchReleases returned error: %v", err)
	}

	if len(assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(assets))
	}
	if assets[0].Ext != "tar.gz" {
		t.Errorf("Ext = %q, want %q", assets[0].Ext, "tar.gz")
	}
}
