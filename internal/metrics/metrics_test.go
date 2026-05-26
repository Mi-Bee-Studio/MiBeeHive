package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics() returned nil")
	}
	if m.registry == nil {
		t.Fatal("registry is nil")
	}
	if m.CrawlTotal == nil {
		t.Fatal("CrawlTotal is nil")
	}
	if m.DownloadTotal == nil {
		t.Fatal("DownloadTotal is nil")
	}
	if m.DownloadBytes == nil {
		t.Fatal("DownloadBytes is nil")
	}
	if m.DownloadDuration == nil {
		t.Fatal("DownloadDuration is nil")
	}
	if m.ActiveDownloads == nil {
		t.Fatal("ActiveDownloads is nil")
	}
	if m.QueueDepth == nil {
		t.Fatal("QueueDepth is nil")
	}
	if m.ISODownloadsTotal == nil {
		t.Fatal("ISODownloadsTotal is nil")
	}
	if m.ISOQueueDepth == nil {
		t.Fatal("ISOQueueDepth is nil")
	}
	if m.WebDAVRequestsTotal == nil {
		t.Fatal("WebDAVRequestsTotal is nil")
	}
	if m.DiskUsageBytes == nil {
		t.Fatal("DiskUsageBytes is nil")
	}
	if m.BackupTotal == nil {
		t.Fatal("BackupTotal is nil")
	}
}

func TestHandlerReturnsNonNil(t *testing.T) {
	m := NewMetrics()
	h := m.Handler()
	if h == nil {
		t.Fatal("Handler() returned nil")
	}
}

func TestMetricsIncrement(t *testing.T) {
	m := NewMetrics()

	// CounterVec — should not panic
	m.CrawlTotal.WithLabelValues("github", "success").Inc()
	m.DownloadTotal.WithLabelValues("complete").Inc()
	m.ISODownloadsTotal.WithLabelValues("ubuntu", "success").Inc()
	m.WebDAVRequestsTotal.WithLabelValues("GET", "200").Inc()
	m.BackupTotal.WithLabelValues("success").Inc()

	// Counter — should not panic
	m.DownloadBytes.Add(1024)

	// Histogram — should not panic
	m.DownloadDuration.Observe(0.5)

	// Gauge — should not panic
	m.ActiveDownloads.Inc()
	m.ActiveDownloads.Dec()

	// GaugeVec — should not panic
	m.QueueDepth.WithLabelValues("pending").Set(5)
	m.ISOQueueDepth.WithLabelValues("downloading").Set(2)
	m.DiskUsageBytes.WithLabelValues("used").Set(1e9)
}

func TestHandlerServesGoCollector(t *testing.T) {
	m := NewMetrics()
	h := m.Handler()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "go_goroutines") {
		t.Error("response does not contain go_goroutines — GoCollector not registered")
	}
}

func TestHandlerServesBusinessMetrics(t *testing.T) {
	m := NewMetrics()

	// Emit data for all vec metrics so they appear in output
	m.CrawlTotal.WithLabelValues("github", "success").Inc()
	m.DownloadTotal.WithLabelValues("complete").Inc()
	m.DownloadBytes.Add(512)
	m.QueueDepth.WithLabelValues("pending").Set(1)
	m.ISODownloadsTotal.WithLabelValues("ubuntu", "success").Inc()
	m.ISOQueueDepth.WithLabelValues("downloading").Set(1)
	m.WebDAVRequestsTotal.WithLabelValues("GET", "200").Inc()
	m.DiskUsageBytes.WithLabelValues("used").Set(1)
	m.BackupTotal.WithLabelValues("success").Inc()

	h := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	for _, prefix := range []string{
		"mibeehive_crawls_total",
		"mibeehive_downloads_total",
		"mibeehive_download_bytes_total",
		"mibeehive_download_duration_seconds",
		"mibeehive_active_downloads",
		"mibeehive_queue_depth",
		"mibeehive_iso_downloads_total",
		"mibeehive_iso_queue_depth",
		"mibeehive_webdav_requests_total",
		"mibeehive_disk_usage_bytes",
		"mibeehive_backups_total",
	} {
		if !strings.Contains(body, prefix) {
			t.Errorf("response does not contain metric %q", prefix)
		}
	}
}

func TestCustomRegistryIsolation(t *testing.T) {
	m := NewMetrics()

	// Increment a counter
	m.DownloadBytes.Add(100)

	h := m.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	body := rec.Body.String()

	// Verify our counter has the value we set
	if !strings.Contains(body, "mibeehive_download_bytes_total 100") {
		t.Errorf("expected download_bytes_total=100 in output, body snippet:\n%s",
			trimToLine(body, "mibeehive_download_bytes_total"))
	}
}

// trimToLine returns the line containing the given substring, or "<not found>".
func trimToLine(body, sub string) string {
	for _, line := range strings.Split(body, "\n") {
		if strings.Contains(line, sub) && !strings.HasPrefix(line, "#") {
			return line
		}
	}
	return "<not found>"
}
