package service

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ---------- Single-level scraping tests (backward compatible) ----------

func TestScrapeLatestISO_SingleLevel_ValidHTML(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<a href="ubuntu-22.04.3-live-server-arm64.iso">ubuntu-22.04.3-live-server-arm64.iso</a>
<a href="ubuntu-22.04.2-live-server-arm64.iso">ubuntu-22.04.2-live-server-arm64.iso</a>
<a href="ubuntu-22.04.1-live-server-arm64.iso">ubuntu-22.04.1-live-server-arm64.iso</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `ubuntu-[\d.]+-live-server-arm64\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.Contains(url, "22.04.3") {
		t.Errorf("expected latest version URL, got %q", url)
	}
}

func TestScrapeLatestISO_SingleLevel_NoMatches(t *testing.T) {
	html := `<!DOCTYPE html>
<html><body>
<a href="some-other-file.txt">some-other-file.txt</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `ubuntu-[\d.]+-live-server-arm64\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for no matches, got %q", url)
	}
}

func TestScrapeLatestISO_SingleLevel_EmptyPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "")
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `.*\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "" {
		t.Errorf("expected empty URL for empty page, got %q", url)
	}
}

func TestScrapeLatestISO_InvalidFilenamePattern(t *testing.T) {
	_, err := ScrapeLatestISO(context.Background(), "http://example.com", "", "", `[invalid(`, "arm64")
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
}

func TestScrapeLatestISO_InvalidVersionDirPattern(t *testing.T) {
	_, err := ScrapeLatestISO(context.Background(), "http://example.com", `[invalid(`, "{version}/", `.*\.iso`, "arm64")
	if err == nil {
		t.Fatal("expected error for invalid version directory pattern")
	}
}

func TestScrapeLatestISO_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	_, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `.*\.iso`, "arm64")
	if err == nil {
		t.Fatal("expected error for HTTP 500")
	}
}

func TestScrapeLatestISO_SingleLevel_URLConstruction(t *testing.T) {
	html := `<html><body>
<a href="debian-12.4.0-arm64-netinst.iso">debian-12.4.0-arm64-netinst.iso</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	url, err := ScrapeLatestISO(context.Background(), server.URL+"/isos/", "", "release/", `debian-[\d.]+-arm64-netinst\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == "" {
		t.Fatal("expected non-empty URL")
	}
	if !strings.Contains(url, "debian-12.4.0-arm64-netinst.iso") {
		t.Errorf("expected URL to contain filename, got %q", url)
	}
}

func TestScrapeLatestISO_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := ScrapeLatestISO(ctx, "http://example.com", "", "", `.*\.iso`, "arm64")
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestScrapeLatestISO_UserAgentHeader(t *testing.T) {
	var userAgent string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		fmt.Fprint(w, `<html><body><a href="test.iso">test</a></body></html>`)
	}))
	defer server.Close()

	ScrapeLatestISO(context.Background(), server.URL, "", "", `test\.iso`, "arm64")

	if userAgent != "MiBeeHive/1.0" {
		t.Errorf("expected User-Agent 'MiBeeHive/1.0', got %q", userAgent)
	}
}

// ---------- Two-level scraping tests ----------

func TestScrapeLatestISO_TwoLevel_PicksLatestVersion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="22.04/">22.04/</a><a href="24.04/">24.04/</a><a href="20.04/">20.04/</a></html>`)
		case "/24.04/":
			fmt.Fprint(w, `<html><a href="ubuntu-24.04.2-live-server-amd64.iso">iso</a></html>`)
		case "/22.04/":
			fmt.Fprint(w, `<html><a href="ubuntu-22.04.3-live-server-amd64.iso">iso</a></html>`)
		case "/20.04/":
			fmt.Fprint(w, `<html><a href="ubuntu-20.04.6-live-server-amd64.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d{2}\.\d{2}`, "{version}/", `ubuntu-[\d.]+-live-server-amd64\.iso$`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "24.04") {
		t.Errorf("expected URL from 24.04 directory, got %q", result)
	}
	if !strings.Contains(result, "ubuntu-24.04.2-live-server-amd64.iso") {
		t.Errorf("expected URL containing ISO filename, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_ArchSubstitution(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="12/">12/</a><a href="13/">13/</a></html>`)
		case "/13/":
			fmt.Fprint(w, `<html><a href="debian-13.0.0-aarch64-netinst.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+`, "{version}/", `debian-[\d.]+-aarch64-netinst\.iso$`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "debian-13.0.0-aarch64-netinst.iso") {
		t.Errorf("expected URL with aarch64 ISO, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_NoVersionDirectoriesMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<html><a href="README.txt">README</a><a href="index.html">index</a></html>`)
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+\.\d+`, "{version}/", `.*\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty URL when no version directories match, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_VersionFoundButNoISOs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="9.0/">9.0/</a></html>`)
		case "/9.0/":
			fmt.Fprint(w, `<html><a href="README.txt">README</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+\.\d+`, "{version}/", `.*\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "" {
		t.Errorf("expected empty URL when version dir found but no ISOs, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_MultipleISOsInLatestDir(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="9/">9/</a><a href="10/">10/</a></html>`)
		case "/10/":
			fmt.Fprint(w, `<html>
<a href="rocky-10.0-amd64-minimal.iso">minimal</a>
<a href="rocky-10.1-amd64-minimal.iso">minimal</a>
<a href="rocky-10.2-amd64-minimal.iso">minimal</a>
<a href="SHA256SUMS">sums</a>
</html>`)
		case "/9/":
			fmt.Fprint(w, `<html><a href="rocky-9.5-amd64-minimal.iso">minimal</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+`, "{version}/", `rocky-[\d.]+-amd64-minimal\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "rocky-10.2-amd64-minimal.iso") {
		t.Errorf("expected latest ISO in latest version dir, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_VersionDirWithoutSlash(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			// Some mirrors list dirs without trailing slash
			fmt.Fprint(w, `<html><a href="3.18">3.18</a><a href="3.19">3.19</a></html>`)
		case "/3.19/":
			fmt.Fprint(w, `<html><a href="alpine-3.19.0-aarch64.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+\.\d+`, "{version}/", `alpine-[\d.]+-aarch64\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "alpine-3.19.0-aarch64.iso") {
		t.Errorf("expected URL with alpine ISO, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_ComplexPath(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/releases/":
			fmt.Fprint(w, `<html><a href="42/">42/</a><a href="41/">41/</a></html>`)
		case "/releases/42/":
			fmt.Fprint(w, `<html><a href="fedora-42-workstation-x86_64.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/releases/", `\d+`, "{version}/", `fedora-\d+-workstation-x86_64\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "fedora-42-workstation-x86_64.iso") {
		t.Errorf("expected fedora ISO URL, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_ArchAndVersionInTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="8/">8/</a><a href="9/">9/</a></html>`)
		case "/9/":
			fmt.Fprint(w, `<html><a href="centos-9-x86_64-minimal.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+`, "{version}/", `centos-\d+-x86_64-minimal\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "centos-9-x86_64-minimal.iso") {
		t.Errorf("expected centos ISO URL, got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_HTTPErrorOnBaseURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+`, "{version}/", `.*\.iso`, "amd64")
	if err == nil {
		t.Fatal("expected error for HTTP error on base URL")
	}
}

func TestScrapeLatestISO_TwoLevel_HTTPErrorOnVersionDirURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="1/">1/</a></html>`)
		case "/1/":
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	_, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+`, "{version}/", `.*\.iso`, "amd64")
	if err == nil {
		t.Fatal("expected error when version dir listing returns HTTP error")
	}
}

// ---------- Single-level with isoPathTemplate and arch ----------

func TestScrapeLatestISO_SingleLevel_WithArchTemplate(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/aarch64/" {
			fmt.Fprint(w, `<html><a href="alpine-3.19.0-aarch64.iso">iso</a></html>`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", "", "{arch}/", `alpine-[\d.]+-aarch64\.iso`, "arm64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "alpine-3.19.0-aarch64.iso") {
		t.Errorf("expected URL with aarch64 ISO, got %q", result)
	}
}

func TestScrapeLatestISO_SingleLevel_WithEmptyTemplate(t *testing.T) {
	html := `<html><body>
<a href="debian-12.7.0-amd64-netinst.iso">iso</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `debian-[\d.]+-amd64-netinst\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "debian-12.7.0-amd64-netinst.iso") {
		t.Errorf("expected URL with debian ISO, got %q", result)
	}
}

// ---------- Version sorting edge cases ----------

func TestScrapeLatestISO_VersionSorting_NumericComparison(t *testing.T) {
	html := `<html><body>
<a href="app-1.9.iso">1.9</a>
<a href="app-1.10.iso">1.10</a>
<a href="app-1.11.iso">1.11</a>
</body></html>`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, html)
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL, "", "", `app-[\d.]+\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result, "app-1.11.iso") {
		t.Errorf("expected app-1.11.iso (numeric sort), got %q", result)
	}
}

func TestScrapeLatestISO_TwoLevel_PatchesVersionComparison(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			fmt.Fprint(w, `<html><a href="12.0/">12.0/</a><a href="12.1/">12.1/</a><a href="12.0.1/">12.0.1/</a></html>`)
		case "/12.1/":
			fmt.Fprint(w, `<html><a href="test-12.1.iso">iso</a></html>`)
		case "/12.0.1/":
			fmt.Fprint(w, `<html><a href="test-12.0.1.iso">iso</a></html>`)
		case "/12.0/":
			fmt.Fprint(w, `<html><a href="test-12.0.iso">iso</a></html>`)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	result, err := ScrapeLatestISO(context.Background(), server.URL+"/", `\d+\.\d+`, "{version}/", `test-[\d.]+\.iso`, "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 12.1 > 12.0.1 > 12.0 in version comparison
	if !strings.Contains(result, "test-12.1.iso") {
		t.Errorf("expected test-12.1.iso from 12.1 dir, got %q", result)
	}
}
