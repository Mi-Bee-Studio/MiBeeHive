package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all Prometheus metric instruments for MiBeeHive.
type Metrics struct {
	registry *prometheus.Registry

	// Business metrics
	CrawlTotal        *prometheus.CounterVec
	DownloadTotal     *prometheus.CounterVec
	DownloadBytes     prometheus.Counter
	DownloadDuration  prometheus.Histogram
	ActiveDownloads   prometheus.Gauge
	QueueDepth        *prometheus.GaugeVec
	ISODownloadsTotal *prometheus.CounterVec
	ISOQueueDepth     *prometheus.GaugeVec
	WebDAVRequestsTotal *prometheus.CounterVec
	DiskUsageBytes    *prometheus.GaugeVec
	BackupTotal       *prometheus.CounterVec
}

// NewMetrics creates a new Metrics instance with a custom Prometheus registry.
// It registers Go runtime and process collectors automatically.
func NewMetrics() *Metrics {
	reg := prometheus.NewRegistry()

	// Register Go runtime and process collectors
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	m := &Metrics{
		registry: reg,
		CrawlTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mibeehive_crawls_total",
			Help: "Total number of crawl operations",
		}, []string{"source_type", "status"}),
		DownloadTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mibeehive_downloads_total",
			Help: "Total number of file downloads",
		}, []string{"status"}),
		DownloadBytes: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "mibeehive_download_bytes_total",
			Help: "Total bytes downloaded",
		}),
		DownloadDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "mibeehive_download_duration_seconds",
			Help:    "Download duration in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 0.1s to ~51s
		}),
		ActiveDownloads: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "mibeehive_active_downloads",
			Help: "Number of currently active downloads",
		}),
		QueueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mibeehive_queue_depth",
			Help: "Number of items in download queue",
		}, []string{"status"}),
		ISODownloadsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mibeehive_iso_downloads_total",
			Help: "Total number of ISO downloads",
		}, []string{"distro", "status"}),
		ISOQueueDepth: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mibeehive_iso_queue_depth",
			Help: "Number of items in ISO download queue",
		}, []string{"status"}),
		WebDAVRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mibeehive_webdav_requests_total",
			Help: "Total number of WebDAV requests",
		}, []string{"method", "status_code"}),
		DiskUsageBytes: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "mibeehive_disk_usage_bytes",
			Help: "Disk usage in bytes",
		}, []string{"type"}), // type: total, used, available
		BackupTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "mibeehive_backups_total",
			Help: "Total number of backup operations",
		}, []string{"status"}),
	}

	// Register all business metrics
	reg.MustRegister(
		m.CrawlTotal,
		m.DownloadTotal,
		m.DownloadBytes,
		m.DownloadDuration,
		m.ActiveDownloads,
		m.QueueDepth,
		m.ISODownloadsTotal,
		m.ISOQueueDepth,
		m.WebDAVRequestsTotal,
		m.DiskUsageBytes,
		m.BackupTotal,
	)

	return m
}

// Handler returns an http.Handler that serves Prometheus metrics.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}
