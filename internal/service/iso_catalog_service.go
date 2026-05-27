package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"sync"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/metrics"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ISOCatalogService manages the ISO catalog and version checking.
type ISOCatalogService struct {
	catalogRepo *db.ISOCatalogRepo
	isoService  *ISOService
	logger      *slog.Logger
	metrics     *metrics.Metrics
	mu          sync.Mutex
	cancelFuncs sync.Map // int64 (entry ID) → context.CancelFunc
}

// NewISOCatalogService creates a new ISOCatalogService.
func NewISOCatalogService(catalogRepo *db.ISOCatalogRepo, isoService *ISOService, logger *slog.Logger, m *metrics.Metrics) *ISOCatalogService {
	return &ISOCatalogService{
		catalogRepo: catalogRepo,
		isoService:  isoService,
		logger:      logger,
		metrics:     m,
	}
}

func dbEntryToModel(e *db.ISOCatalogDBEntry) model.ISOCatalogEntry {
	lastChecked := ""
	if e.LastChecked.Valid {
		lastChecked = e.LastChecked.String
	}
	return model.ISOCatalogEntry{
		ID:                 int(e.ID),
		Name:               e.Name,
		Distro:             e.Distro,
		Variant:            e.Variant,
		Arch:               e.Arch,
		CheckURL:           e.CheckURL,
		FilenamePattern:    e.FilenamePattern,
		BaseURL:            e.BaseURL,
		VersionDirPattern:  e.VersionDirPattern,
		ISOPathTemplate:    e.ISOPathTemplate,
		CurrentURL:         e.CurrentURL,
		AutoUpdate:         e.AutoUpdate,
		CheckIntervalHours: e.CheckIntervalHours,
		LastChecked:        lastChecked,
		LastError:          e.LastError,
		Status:             e.Status,
		DownloadStatus:     e.DownloadStatus,
		SHA256:             e.SHA256,
		CreatedAt:          e.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          e.UpdatedAt.Format(time.RFC3339),
	}
}

// ListCatalog returns all catalog entries as API models.
func (s *ISOCatalogService) ListCatalog(ctx context.Context) ([]model.ISOCatalogEntry, error) {
	entries, err := s.catalogRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]model.ISOCatalogEntry, len(entries))
	for i, e := range entries {
		result[i] = dbEntryToModel(&e)
	}
	return result, nil
}

// GetCatalogEntry returns a single catalog entry by ID.
func (s *ISOCatalogService) GetCatalogEntry(ctx context.Context, id int64) (*model.ISOCatalogEntry, error) {
	e, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if e == nil {
		return nil, nil
	}
	m := dbEntryToModel(e)
	return &m, nil
}

// CreateCatalogEntry validates and creates a new catalog entry.
func (s *ISOCatalogService) CreateCatalogEntry(ctx context.Context, req model.ISOCatalogCreateRequest) (int64, error) {
	if req.Name == "" || req.FilenamePattern == "" {
		return 0, fmt.Errorf("name and filename_pattern are required")
	}
	if req.CheckURL == "" && req.BaseURL == "" {
		return 0, fmt.Errorf("either check_url or base_url is required")
	}
	if req.Arch == "" {
		req.Arch = "amd64"
	}
	if req.CheckIntervalHours == 0 {
		req.CheckIntervalHours = 24
	}
	e := &db.ISOCatalogDBEntry{
		Name:               req.Name,
		Distro:             req.Distro,
		Variant:            req.Variant,
		Arch:               req.Arch,
		CheckURL:           req.CheckURL,
		FilenamePattern:    req.FilenamePattern,
		BaseURL:            req.BaseURL,
		VersionDirPattern:  req.VersionDirPattern,
		ISOPathTemplate:    req.ISOPathTemplate,
		AutoUpdate:         req.AutoUpdate,
		CheckIntervalHours: req.CheckIntervalHours,
		SHA256:             req.SHA256,
		Status:             "available",
	}
	return s.catalogRepo.Create(ctx, e)
}

// UpdateCatalogEntry partially updates a catalog entry.
func (s *ISOCatalogService) UpdateCatalogEntry(ctx context.Context, id int64, req model.ISOCatalogUpdateRequest) error {
	existing, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("catalog entry %d not found", id)
	}
	if req.Name != nil {
		existing.Name = *req.Name
	}
	if req.Distro != nil {
		existing.Distro = *req.Distro
	}
	if req.Variant != nil {
		existing.Variant = *req.Variant
	}
	if req.Arch != nil {
		existing.Arch = *req.Arch
	}
	if req.CheckURL != nil {
		existing.CheckURL = *req.CheckURL
	}
	if req.FilenamePattern != nil {
		existing.FilenamePattern = *req.FilenamePattern
	}
	if req.BaseURL != nil {
		existing.BaseURL = *req.BaseURL
	}
	if req.VersionDirPattern != nil {
		existing.VersionDirPattern = *req.VersionDirPattern
	}
	if req.ISOPathTemplate != nil {
		existing.ISOPathTemplate = *req.ISOPathTemplate
	}
	if req.AutoUpdate != nil {
		existing.AutoUpdate = *req.AutoUpdate
	}
	if req.CheckIntervalHours != nil {
		existing.CheckIntervalHours = *req.CheckIntervalHours
	}
	if req.SHA256 != nil {
		existing.SHA256 = *req.SHA256
	}
	return s.catalogRepo.Update(ctx, id, existing)
}

// DeleteCatalogEntry removes a catalog entry by ID.
func (s *ISOCatalogService) DeleteCatalogEntry(ctx context.Context, id int64) error {
	return s.catalogRepo.Delete(ctx, id)
}

// CheckVersion scrapes the source URL for a catalog entry and compares with the stored URL.
func (s *ISOCatalogService) CheckVersion(ctx context.Context, id int64) (*model.ISOCatalogCheckResponse, error) {
	entry, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if entry == nil {
		return nil, fmt.Errorf("catalog entry %d not found", id)
	}
	// Use base_url for two-level scraping; fall back to check_url for backward compatibility.
	baseURL := entry.BaseURL
	if baseURL == "" {
		baseURL = entry.CheckURL
	}
	var foundURL string
	foundURL, err = ScrapeLatestISO(ctx, baseURL, entry.VersionDirPattern, entry.ISOPathTemplate, entry.FilenamePattern, entry.Arch)
	if err != nil {
		_ = s.catalogRepo.UpdateAfterCheck(ctx, id, "", "error", err.Error())
		return nil, err
	}

	if foundURL == "" {
		_ = s.catalogRepo.UpdateAfterCheck(ctx, id, entry.CurrentURL, "no_match", "")
		return &model.ISOCatalogCheckResponse{Status: "no_match"}, nil
	}

	status := "up_to_date"
	if foundURL != entry.CurrentURL {
		status = "new_version"
	}
	_ = s.catalogRepo.UpdateAfterCheck(ctx, id, foundURL, status, "")
	return &model.ISOCatalogCheckResponse{FoundURL: foundURL, Status: status}, nil
}

// DownloadFromCatalog triggers an ISO download for a catalog entry using its current URL.
func (s *ISOCatalogService) DownloadFromCatalog(ctx context.Context, id int64) error {
	entry, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("catalog entry %d not found", id)
	}
	if entry.CurrentURL == "" {
		return fmt.Errorf("no ISO URL available for entry %d, run version check first", id)
	}

	filename := path.Base(entry.CurrentURL)

	// Create a cancelable context for download cancellation support.
	downloadCtx, cancel := context.WithCancel(ctx)
	s.cancelFuncs.Store(id, cancel)

	s.logger.Info("starting catalog ISO download", "entry", entry.Name, "url", entry.CurrentURL)

	err = s.isoService.DownloadISO(downloadCtx, filename, entry.CurrentURL, entry.SHA256)

	cancel()
	s.cancelFuncs.Delete(id)

	if err != nil {
		_ = s.catalogRepo.UpdateAfterCheck(ctx, id, entry.CurrentURL, "error", err.Error())
		if s.metrics != nil {
			s.metrics.ISODownloadsTotal.WithLabelValues(entry.Distro, "error").Inc()
		}
		return err
	}
	if s.metrics != nil {
		s.metrics.ISODownloadsTotal.WithLabelValues(entry.Distro, "success").Inc()
	}
	return s.catalogRepo.UpdateAfterCheck(ctx, id, entry.CurrentURL, "downloaded", "")
}

// RetryCatalogDownload resets a failed catalog download to pending status.
func (s *ISOCatalogService) RetryCatalogDownload(ctx context.Context, id int64) error {
	entry, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if entry == nil {
		return fmt.Errorf("catalog entry %d not found", id)
	}
	if entry.DownloadStatus != "error" {
		return fmt.Errorf("catalog entry %d has status %q, expected 'error'", id, entry.DownloadStatus)
	}
	return s.catalogRepo.UpdateDownloadStatus(ctx, id, "pending")
}

// CancelDownload cancels an active ISO download for a catalog entry.
// It finds the entry, checks it is currently downloading, calls the cancel
// function, and resets the download status to "pending" for retry.
func (s *ISOCatalogService) CancelDownload(ctx context.Context, id int64) error {
	entry, err := s.catalogRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("getting catalog entry: %w", err)
	}
	if entry == nil {
		return fmt.Errorf("catalog entry %d not found", id)
	}
	if entry.DownloadStatus != "downloading" {
		return fmt.Errorf("catalog entry %d is not currently downloading (status: %s)", id, entry.DownloadStatus)
	}

	cancelVal, ok := s.cancelFuncs.Load(id)
	if !ok {
		return fmt.Errorf("no active download found for catalog entry %d", id)
	}
	cancel := cancelVal.(context.CancelFunc)
	cancel()
	s.cancelFuncs.Delete(id)

	if err := s.catalogRepo.UpdateDownloadStatus(ctx, id, "pending"); err != nil {
		return fmt.Errorf("resetting download status: %w", err)
	}

	s.logger.Info("cancelled catalog ISO download", "entry", entry.Name, "id", id)
	return nil
}

// CheckAllAutoUpdate iterates over auto-update entries, checks versions, and downloads new versions.
func (s *ISOCatalogService) CheckAllAutoUpdate(ctx context.Context) error {
	entries, err := s.catalogRepo.ListAutoUpdate(ctx)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		resp, err := s.CheckVersion(ctx, entry.ID)
		if err != nil {
			s.logger.Error("auto-check failed", "entry", entry.Name, "error", err)
			continue
		}
		shouldDownload := resp.FoundURL != "" && (resp.Status == "new_version" || (resp.Status == "up_to_date" && entry.DownloadStatus != "downloaded"))
		if shouldDownload {
			if err := s.DownloadFromCatalog(ctx, entry.ID); err != nil {
				s.logger.Error("auto-download failed", "entry", entry.Name, "error", err)
			}
		}
	}
	return nil
}

// StartVersionChecker runs a periodic version check in a goroutine.
// It follows the scheduler pattern from internal/crawler/scheduler.go.
func (s *ISOCatalogService) StartVersionChecker(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		s.logger.Info("ISO catalog version checker started", "interval", interval)
		// Run initial check immediately, then periodically.
		if err := s.CheckAllAutoUpdate(ctx); err != nil {
			s.logger.Error("initial version check failed", "error", err)
		}
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("ISO catalog version checker stopped")
				return
			case <-ticker.C:
				if err := s.CheckAllAutoUpdate(ctx); err != nil {
					s.logger.Error("version check cycle failed", "error", err)
				}
			}
		}
	}()
}

// QueueDownloadAll sets download_status='pending' for all entries that have a current_url
// but are not already downloading or downloaded.
func (s *ISOCatalogService) QueueDownloadAll(ctx context.Context) error {
	entries, err := s.catalogRepo.List(ctx)
	if err != nil {
		return fmt.Errorf("listing catalog for queue: %w", err)
	}
	queued := 0
	for _, e := range entries {
		if e.CurrentURL == "" {
			continue
		}
		if e.DownloadStatus == "downloading" || e.DownloadStatus == "downloaded" {
			continue
		}
		if err := s.catalogRepo.UpdateDownloadStatus(ctx, e.ID, "pending"); err != nil {
			s.logger.Error("failed to queue download", "entry", e.Name, "error", err)
			continue
		}
		queued++
	}
	s.logger.Info("queued ISO downloads", "count", queued)
	return nil
}

// ProcessQueue picks pending entries from the queue and downloads them one at a time.
// It also detects stale "downloading" entries (interrupted downloads) and resets them.
func (s *ISOCatalogService) ProcessQueue(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := s.catalogRepo.ListDownloadQueue(ctx)
	if err != nil {
		s.logger.Error("failed to list download queue", "error", err)
		return
	}

	// Detect stale downloading entries and reset them to pending.
	var staleChecks []StaleISOCheck
	for _, entry := range entries {
		if entry.DownloadStatus == "downloading" && entry.CurrentURL != "" {
			staleChecks = append(staleChecks, StaleISOCheck{
				ID:       entry.ID,
				Filename: path.Base(entry.CurrentURL),
				Status:   entry.DownloadStatus,
			})
		}
	}
	if len(staleChecks) > 0 {
		reset, err := s.isoService.ResetStaleDownloads(ctx, staleChecks, func(id int64, status string) error {
			return s.catalogRepo.UpdateDownloadStatus(ctx, id, status)
		})
		if err != nil {
			s.logger.Error("failed to reset stale ISO downloads", "error", err)
		}
		if reset > 0 {
			s.logger.Info("reset stale ISO downloads", "count", reset)
			// Re-fetch entries after reset.
			entries, err = s.catalogRepo.ListDownloadQueue(ctx)
			if err != nil {
				s.logger.Error("failed to re-list download queue", "error", err)
				return
			}
		}
	}

	for _, entry := range entries {
		if entry.DownloadStatus != "pending" {
			continue
		}

		if err := s.catalogRepo.UpdateDownloadStatus(ctx, entry.ID, "downloading"); err != nil {
			s.logger.Error("failed to set downloading status", "entry", entry.Name, "error", err)
			continue
		}

		filename := path.Base(entry.CurrentURL)
		s.logger.Info("processing queue entry", "entry", entry.Name, "url", entry.CurrentURL)

		// Create a cancelable context for download cancellation support.
		downloadCtx, cancel := context.WithCancel(ctx)
		s.cancelFuncs.Store(entry.ID, cancel)

		err := s.isoService.DownloadISO(downloadCtx, filename, entry.CurrentURL, entry.SHA256)

		cancel()
		s.cancelFuncs.Delete(entry.ID)

		if err != nil {
			// If cancelled via CancelDownload, the status was already reset to pending.
			if errors.Is(err, context.Canceled) {
				continue
			}
			_ = s.catalogRepo.UpdateDownloadStatus(ctx, entry.ID, "error")
			_ = s.catalogRepo.UpdateAfterCheck(ctx, entry.ID, entry.CurrentURL, "error", err.Error())
			s.logger.Error("queue download failed", "entry", entry.Name, "error", err)
			if s.metrics != nil {
				s.metrics.ISODownloadsTotal.WithLabelValues(entry.Distro, "error").Inc()
			}
			continue
		}

		if err := s.catalogRepo.UpdateDownloadStatus(ctx, entry.ID, "downloaded"); err != nil {
			s.logger.Error("failed to set downloaded status", "entry", entry.Name, "error", err)
		}
		_ = s.catalogRepo.UpdateAfterCheck(ctx, entry.ID, entry.CurrentURL, "downloaded", "")
		if s.metrics != nil {
			s.metrics.ISODownloadsTotal.WithLabelValues(entry.Distro, "success").Inc()
		}
	}
}

// StartQueueProcessor starts a goroutine that polls the queue every 30 seconds.
func (s *ISOCatalogService) StartQueueProcessor(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		s.logger.Info("ISO download queue processor started", "interval", "30s")
		// Process immediately on start.
		s.ProcessQueue(ctx)
		for {
			select {
			case <-ctx.Done():
				s.logger.Info("ISO download queue processor stopped")
				return
			case <-ticker.C:
				s.ProcessQueue(ctx)
			}
		}
	}()
}

// GetQueueStats returns counts per download_status for the ISO catalog.
func (s *ISOCatalogService) GetQueueStats(ctx context.Context) (*model.ISOQueueStats, error) {
	all, err := s.catalogRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	var stats model.ISOQueueStats
	for _, e := range all {
		ds := e.DownloadStatus
		if ds == "" {
			if e.CurrentURL != "" {
				ds = "available"
			} else {
				ds = "pending"
			}
		}
	switch ds {
		case "pending":
			stats.Pending++
		case "downloading":
			stats.Downloading++
		case "downloaded":
			stats.Downloaded++
		case "available":
			stats.Available++
		case "error":
			stats.Error++
		}
	}
	stats.Total = stats.Pending + stats.Downloading + stats.Downloaded + stats.Available + stats.Error
	return &stats, nil
}

// GetQueueList returns all entries with non-empty download_status.
func (s *ISOCatalogService) GetQueueList(ctx context.Context) ([]model.ISOQueueItem, error) {
	entries, err := s.catalogRepo.List(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]model.ISOQueueItem, 0, len(entries))
	for _, e := range entries {
		ds := e.DownloadStatus
		if ds == "" {
			if e.CurrentURL != "" {
				ds = "available"
			} else {
				ds = "pending"
			}
		}
		items = append(items, model.ISOQueueItem{
			ID:             e.ID,
			Name:           e.Name,
			Distro:         e.Distro,
			Arch:           e.Arch,
			CurrentURL:     e.CurrentURL,
			DownloadStatus: ds,
			Status:         e.Status,
		})
	}
	return items, nil
}
