package service

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// DashboardService aggregates data from multiple repos and services for the dashboard.
type DashboardService struct {
	fileService    *FileService
	projectRepo    *dbrepo.ProjectRepo
	fileRepo       *dbrepo.FileRepo
	crawlLogRepo   *dbrepo.CrawlLogRepo
	osConfigRepo   *dbrepo.OsInstallConfigRepo
	isoCatalogRepo *dbrepo.ISOCatalogRepo
	cfg            *config.Config
	version        string
	startTime      time.Time
}

// NewDashboardService creates a new DashboardService.
func NewDashboardService(
	fileService *FileService,
	projectRepo *dbrepo.ProjectRepo,
	fileRepo *dbrepo.FileRepo,
	crawlLogRepo *dbrepo.CrawlLogRepo,
	osConfigRepo *dbrepo.OsInstallConfigRepo,
	isoCatalogRepo *dbrepo.ISOCatalogRepo,
	cfg *config.Config,
	version string,
) *DashboardService {
	return &DashboardService{
		fileService:    fileService,
		projectRepo:    projectRepo,
		fileRepo:       fileRepo,
		crawlLogRepo:   crawlLogRepo,
		osConfigRepo:   osConfigRepo,
		isoCatalogRepo: isoCatalogRepo,
		cfg:            cfg,
		version:        version,
		startTime:      time.Now(),
	}
}

// GetSummary aggregates all module statistics for the dashboard.
func (s *DashboardService) GetSummary(ctx context.Context) model.DashboardSummaryResponse {
	return model.DashboardSummaryResponse{
		System:   s.buildSystemStats(ctx),
		Files:    s.buildFilesStats(ctx),
		Deploy:   s.buildDeployStats(ctx),
		Share:    s.buildShareStats(ctx),
		Activity: s.buildActivity(ctx),
	}
}

func (s *DashboardService) buildSystemStats(ctx context.Context) model.SystemModuleStats {
	stats := model.SystemModuleStats{
		Version:           s.version,
		Uptime:            formatUptime(time.Since(s.startTime)),
		ContainersEnabled: s.cfg.Container.Local.Enabled,
	}

	// Try node_exporter for live CPU/mem stats.
	sysStats, err := FetchSystemStats(ctx, s.cfg.Monitor.NodeExporterURL)
	if err != nil {
		slog.Debug("dashboard: node_exporter unavailable, using fallback", "error", err)
		stats.CpuUsage = 0
		stats.MemUsage = 0
	} else {
		stats.CpuUsage = sysStats.CpuUsagePercent
		stats.MemUsage = sysStats.MemoryUsagePercent
		stats.MemTotal = sysStats.MemoryTotalBytes
		stats.MemUsed = sysStats.MemoryUsedBytes
	}

	// Disk usage from storage path.
	total, used, _, err := s.fileService.GetDiskUsage(s.cfg.Storage.BasePath)
	if err == nil {
		stats.DiskTotal = uint64(total)
		stats.DiskUsed = uint64(used)
		if total > 0 {
			stats.DiskUsagePercent = float64(used) / float64(total) * 100
		}
	}

	return stats
}

func (s *DashboardService) buildFilesStats(ctx context.Context) model.FilesModuleStats {
	stats := model.FilesModuleStats{}

	projects, err := s.projectRepo.List(ctx)
	if err == nil {
		stats.ProjectCount = len(projects)
	}

	fileCount, err := s.fileRepo.CountAll(ctx)
	if err == nil {
		stats.TotalFiles = fileCount
	}

	queueStats, err := s.fileRepo.GetQueueStats(ctx)
	if err == nil && queueStats != nil {
		stats.QueuePending = queueStats.Pending
		stats.QueueDownloading = queueStats.Downloading
		stats.QueueComplete = queueStats.Complete
		stats.QueueError = queueStats.Error
	}

	return stats
}

func (s *DashboardService) buildDeployStats(ctx context.Context) model.DeployModuleStats {
	stats := model.DeployModuleStats{}

	configs, err := s.osConfigRepo.List(ctx)
	if err == nil {
		stats.ConfigCount = len(configs)
	}

	entries, err := s.isoCatalogRepo.List(ctx)
	if err == nil {
		stats.IsoCount = len(entries)
		for _, e := range entries {
			if e.DownloadStatus == "pending" || e.DownloadStatus == "downloading" {
				stats.IsoPending++
			}
			if e.DownloadStatus == "downloaded" {
				stats.IsoDownloaded++
			}
		}
	}

	return stats
}

func (s *DashboardService) buildShareStats(ctx context.Context) model.SharedModuleStats {
	stats := model.SharedModuleStats{}

	webdavPath := filepath.Join(s.cfg.Storage.BasePath, "webdav")
	var totalBytes uint64
	var fileCount int

	filepath.WalkDir(webdavPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Warn("walkdir error", "path", path, "error", err)
			return nil
		}
		if !d.IsDir() {
			if info, infoErr := d.Info(); infoErr == nil {
				totalBytes += uint64(info.Size())
				fileCount++
			}
		}
		return nil
	})

	stats.FileCount = fileCount
	stats.TotalBytes = totalBytes
	stats.TotalSize = formatBytes(totalBytes)

	return stats
}

func (s *DashboardService) buildActivity(ctx context.Context) []model.ActivityEvent {
	logs, err := s.crawlLogRepo.ListRecent(ctx, 10)
	if err != nil {
		return []model.ActivityEvent{}
	}

	events := make([]model.ActivityEvent, 0, len(logs))
	for _, l := range logs {
		evt := model.ActivityEvent{
			ID:        fmt.Sprintf("crawl-%d", l.ID),
			Timestamp: l.StartedAt.Format(time.RFC3339),
		}

		if proj, projErr := s.projectRepo.GetByID(ctx, l.ProjectID); projErr == nil && proj != nil {
			evt.Title = proj.DisplayName
		} else {
			evt.Title = fmt.Sprintf("Project #%d", l.ProjectID)
		}

		switch l.Status {
		case "success":
			evt.Type = "crawl_success"
			evt.Subtitle = fmt.Sprintf("Found %d versions, downloaded %d files", l.VersionsFound, l.FilesDownloaded)
		case "error":
			evt.Type = "crawl_error"
			if l.ErrorMessage != "" {
				evt.Subtitle = truncate(l.ErrorMessage, 80)
			} else {
				evt.Subtitle = "Crawl failed"
			}
		default:
			evt.Type = "crawl_" + l.Status
			evt.Subtitle = fmt.Sprintf("Status: %s", l.Status)
		}

		events = append(events, evt)
	}

	if len(events) == 0 {
		return []model.ActivityEvent{}
	}
	return events
}

// formatUptime returns a human-readable uptime string.
func formatUptime(d time.Duration) string {
	days := int(d.Hours()) / 24
	hours := int(d.Hours()) % 24
	minutes := int(d.Minutes()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}

// formatBytes returns a human-readable byte size string.
func formatBytes(b uint64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.1f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// truncate shortens a string to maxLen, adding "..." if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen > 3 {
		return s[:maxLen-3] + "..."
	}
	return s[:maxLen]
}
