package crawler

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// TestISOSource_ParseUbuntu tests parsing Ubuntu releases page.
func TestISOSource_ParseUbuntu(t *testing.T) {
	// Mock Ubuntu releases page HTML
	html := `
<!DOCTYPE html>
<html>
<body>
	<a href="ubuntu-24.04.1-desktop-amd64.iso">ubuntu-24.04.1-desktop-amd64.iso</a><br>
	<a href="ubuntu-24.04.1-live-server-amd64.iso">ubuntu-24.04.1-live-server-amd64.iso</a><br>
	<a href="ubuntu-22.04.4-desktop-amd64.iso">ubuntu-22.04.4-desktop-amd64.iso</a><br>
</body>
</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	assets, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}

	if len(assets) != 3 {
		t.Errorf("Expected 3 ISOs, got %d", len(assets))
	}

	// Check first asset
	if assets[0].Filename != "ubuntu-24.04.1-desktop-amd64.iso" {
		t.Errorf("Expected 'ubuntu-24.04.1-desktop-amd64.iso', got '%s'", assets[0].Filename)
	}
	if !strings.Contains(assets[0].DownloadURL, "ubuntu-24.04.1-desktop-amd64.iso") {
		t.Errorf("DownloadURL should contain filename, got '%s'", assets[0].DownloadURL)
	}
}

// TestISOSource_ParseDebian tests parsing Debian releases page.
func TestISOSource_ParseDebian(t *testing.T) {
	// Mock Debian releases page HTML
	html := `
<!DOCTYPE html>
<html>
<body>
	<a href="debian-12.7.0-amd64-netinst.iso">debian-12.7.0-amd64-netinst.iso</a><br>
	<a href="debian-12.7.0-amd64-DVD-1.iso">debian-12.7.0-amd64-DVD-1.iso</a><br>
</body>
</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	assets, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}

	if len(assets) != 2 {
		t.Errorf("Expected 2 ISOs, got %d", len(assets))
	}

	// Check first asset
	if assets[0].Filename != "debian-12.7.0-amd64-netinst.iso" {
		t.Errorf("Expected 'debian-12.7.0-amd64-netinst.iso', got '%s'", assets[0].Filename)
	}
}

// TestISOSource_ParseCentOS tests parsing CentOS download page.
func TestISOSource_ParseCentOS(t *testing.T) {
	// Mock CentOS download page HTML
	html := `
<!DOCTYPE html>
<html>
<body>
	<a href="9/isos/x86_64/CentOS-9-Stream-20240916.0-x86_64-dvd1.iso">CentOS-9-Stream-20240916.0-x86_64-dvd1.iso</a><br>
	<a href="9/isos/x86_64/CentOS-9-Stream-20240916.0-x86_64-boot.iso">CentOS-9-Stream-20240916.0-x86_64-boot.iso</a><br>
</body>
</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	assets, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}

	if len(assets) != 2 {
		t.Errorf("Expected 2 ISOs, got %d", len(assets))
	}

	// Check first asset
	if !strings.Contains(assets[0].Filename, "CentOS-9-Stream") {
		t.Errorf("Expected CentOS ISO filename, got '%s'", assets[0].Filename)
	}
}

// TestISOSource_EmptyPage tests that an empty page returns empty assets.
func TestISOSource_EmptyPage(t *testing.T) {
	html := `<!DOCTYPE html><html><body></body></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	assets, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}

	if len(assets) != 0 {
		t.Errorf("Expected 0 ISOs from empty page, got %d", len(assets))
	}
}

// TestISOSource_HTTPError tests handling of HTTP errors.
func TestISOSource_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	_, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err == nil {
		t.Error("Expected error for 404 response, got nil")
	}
}

// TestISOSource_Name tests the Name method.
func TestISOSource_Name(t *testing.T) {
	crawler := NewISOSource(nil)
	if crawler.Name() != "iso" {
		t.Errorf("Expected Name() to return 'iso', got '%s'", crawler.Name())
	}
}

// TestISOSource_SourceType tests the SourceType method.
func TestISOSource_SourceType(t *testing.T) {
	crawler := NewISOSource(nil)
	if crawler.SourceType() != model.SourceType("iso") {
		t.Errorf("Expected SourceType() to return 'iso', got '%s'", crawler.SourceType())
	}
}

// TestISOSource_FilterNonISO tests that non-ISO files are filtered out.
func TestISOSource_FilterNonISO(t *testing.T) {
	html := `
<!DOCTYPE html>
<html>
<body>
	<a href="ubuntu-24.04.1-desktop-amd64.iso">ubuntu-24.04.1-desktop-amd64.iso</a><br>
	<a href="ubuntu-24.04.1-desktop-amd64.iso.sha256">ubuntu-24.04.1-desktop-amd64.iso.sha256</a><br>
	<a href="ubuntu-24.04.1-desktop-amd64.iso.zsync">ubuntu-24.04.1-desktop-amd64.iso.zsync</a><br>
	<a href="ubuntu-24.04.1-desktop-amd64.tar.gz">ubuntu-24.04.1-desktop-amd64.tar.gz</a><br>
</body>
</html>
	`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		io.WriteString(w, html)
	}))
	defer server.Close()

	crawler := NewISOSource(nil)

	assets, err := crawler.FetchReleases(context.Background(), "", server.URL)
	if err != nil {
		t.Fatalf("FetchReleases failed: %v", err)
	}

	// Should only return the .iso file, not .sha256, .zsync, or .tar.gz
	if len(assets) != 1 {
		t.Errorf("Expected 1 ISO (filtering out checksums/zsync/tarballs), got %d", len(assets))
	}

	if !strings.HasSuffix(assets[0].Filename, ".iso") {
		t.Errorf("Expected .iso extension, got '%s'", assets[0].Filename)
	}
}