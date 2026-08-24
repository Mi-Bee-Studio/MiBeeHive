package handler

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"time"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// SystemHandler handles system-related API endpoints.
type SystemHandler struct {
	fileService     *service.FileService
	fileRepo        *dbrepo.FileRepo
	projectRepo     *dbrepo.ProjectRepo
	statsRepo       *dbrepo.SystemStatsRepo
	basePath        string
	version         string
	nodeExporterURL string
}

// NewSystemHandler creates a new SystemHandler.
func NewSystemHandler(db *sql.DB, fileService *service.FileService, basePath string, version string, nodeExporterURL string) *SystemHandler {
	return &SystemHandler{
		fileService:     fileService,
		fileRepo:        dbrepo.NewFileRepo(db),
		projectRepo:     dbrepo.NewProjectRepo(db),
		statsRepo:       dbrepo.NewSystemStatsRepo(db),
		basePath:        basePath,
		version:         version,
		nodeExporterURL: nodeExporterURL,
	}
}

// Info handles GET /api/v1/system/info.
func (h *SystemHandler) Info(w http.ResponseWriter, r *http.Request) {
	total, used, avail, err := h.fileService.GetDiskUsage(h.basePath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get disk usage: %v", err),
		})
		return
	}

	fileCount, err := h.fileRepo.CountAll(r.Context())
	if err != nil {
		fileCount = 0
	}

	projects, err := h.projectRepo.List(r.Context())
	if err != nil {
		projects = nil
	}

	// LastCrawlAt is the most recent crawl across all projects, so the
	// settings page no longer reports "从未" while crawls are running.
	lastCrawlAt := ""
	if ts, err := h.projectRepo.MaxLastCrawledAt(r.Context()); err == nil && ts != nil {
		lastCrawlAt = ts.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[model.SystemInfoResponse]{
		Success: true,
		Data: model.SystemInfoResponse{
			DiskTotal:    total,
			DiskUsed:     used,
			DiskAvail:    avail,
			FileCount:    fileCount,
			ProjectCount: len(projects),
			LastCrawlAt:  lastCrawlAt,
			Version:      h.version,
		},
	})
}

// Stats handles GET /api/v1/system/stats.
func (h *SystemHandler) Stats(w http.ResponseWriter, r *http.Request) {
	resp, err := service.FetchSystemStats(r.Context(), h.nodeExporterURL)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{Success: false, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[model.SystemStatsResponse]{Success: true, Data: *resp})
}

// StatsHistory handles GET /api/v1/system/stats/history?range=X
func (h *SystemHandler) StatsHistory(w http.ResponseWriter, r *http.Request) {
	rangeParam := r.URL.Query().Get("range")
	if rangeParam == "" {
		rangeParam = "1h"
	}
	since, err := parseTimeRange(rangeParam)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("invalid range: %v", err),
		})
		return
	}

	stats, err := h.statsRepo.QueryHistory(r.Context(), since)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to query stats history: %v", err),
		})
		return
	}

	// Downsample if too many points
	if len(stats) > 500 {
		stats = downsampleStats(stats, 500)
	}

	points := make([]model.SystemStatsHistoryPoint, 0, len(stats))
	for _, s := range stats {
		points = append(points, model.SystemStatsHistoryPoint{
			Timestamp:          s.SampledAt.Format("2006-01-02T15:04:05Z07:00"),
			CpuUsagePercent:    s.CpuUsagePercent,
			MemoryUsagePercent: s.MemoryUsagePercent,
			NetworkRxBytes:     s.NetworkRxBytes,
			NetworkTxBytes:     s.NetworkTxBytes,
		})
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.SystemStatsHistoryPoint]{
		Success: true,
		Data:    points,
	})
}

// parseTimeRange converts a range string like "1h", "24h", "7d" into a time.Time.
func parseTimeRange(rangeStr string) (time.Time, error) {
	d, err := time.ParseDuration(rangeStr)
	if err == nil {
		return time.Now().Add(-d), nil
	}
	// Handle day-based ranges like "3d", "7d", "30d"
	var days int
	n, err := fmt.Sscanf(rangeStr, "%dd", &days)
	if err == nil && n == 1 {
		return time.Now().AddDate(0, 0, -days), nil
	}
	return time.Time{}, fmt.Errorf("invalid range format: %q", rangeStr)
}

// downsampleStats averages every N consecutive samples into one.
func downsampleStats(stats []*dbrepo.SystemStat, maxPoints int) []*dbrepo.SystemStat {
	bucketSize := math.Ceil(float64(len(stats)) / float64(maxPoints))
	result := make([]*dbrepo.SystemStat, 0, maxPoints)
	for i := 0; i < len(stats); {
		end := i + int(bucketSize)
		if end > len(stats) {
			end = len(stats)
		}
		var (
			cpuSum, memPctSum float64
			rxSum, txSum      uint64
		)
		for _, s := range stats[i:end] {
			cpuSum += s.CpuUsagePercent
			memPctSum += s.MemoryUsagePercent
			rxSum += s.NetworkRxBytes
			txSum += s.NetworkTxBytes
		}
		n := float64(end - i)
		result = append(result, &dbrepo.SystemStat{
			SampledAt:          stats[i].SampledAt,
			CpuUsagePercent:    cpuSum / n,
			MemoryUsagePercent: memPctSum / n,
			NetworkRxBytes:     rxSum / uint64(n),
			NetworkTxBytes:     txSum / uint64(n),
		})
		i = end
	}
	return result
}

// SampleAndStore scrapes node_exporter and stores the result in the database.
// It also purges old data based on the retention policy.
func SampleAndStore(ctx context.Context, statsRepo *dbrepo.SystemStatsRepo, retentionDays int, nodeExporterURL string) {
	stats, err := service.FetchSystemStats(ctx, nodeExporterURL)
	if err != nil {
		slog.Debug("failed to sample system stats", "error", err)
		return
	}

	record := &dbrepo.SystemStat{
		SampledAt:          time.Now(),
		CpuUsagePercent:    stats.CpuUsagePercent,
		MemoryTotalBytes:   stats.MemoryTotalBytes,
		MemoryUsedBytes:    stats.MemoryUsedBytes,
		MemoryUsagePercent: stats.MemoryUsagePercent,
		NetworkRxBytes:     stats.NetworkRxBytes,
		NetworkTxBytes:     stats.NetworkTxBytes,
	}

	if err := statsRepo.Insert(ctx, record); err != nil {
		slog.Debug("failed to insert system stat", "error", err)
	}

	// Purge old data
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	deleted, err := statsRepo.PurgeOlderThan(ctx, cutoff)
	if err != nil {
		slog.Debug("failed to purge old stats", "error", err)
	} else if deleted > 0 {
		slog.Debug("purged old system stats", "deleted", deleted)
	}
}
