package crawler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"sync"
	"time"

	dbrepo "github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/metrics"
)

// CrawlStatusInfo summarizes a project's crawl state for status queries.
type CrawlStatusInfo struct {
	ProjectName   string           `json:"project_name"`
	SourceType    model.SourceType `json:"source_type"`
	Running       bool             `json:"running"`
	Interval      time.Duration    `json:"interval"`
	LatestVersion string           `json:"latest_version"`
	LastCrawledAt *string          `json:"last_crawled_at"`
}

// ErrCrawlInProgress is returned when a crawl is already running for a project.
var ErrCrawlInProgress = errors.New("crawl already in progress")

// FetchFunc is the new two-track fetch entry point. Given a project's name,
// source type, and params, it returns release assets. It is satisfied by
// internal/source.Registry.Fetch (wired in init.go) and lets CrawlManager route
// through the new abstraction WITHOUT importing internal/source (which imports
// this package via LegacyAdapter — avoiding an import cycle).
//
// When set (SetFetchFunc), the manager prefers it; otherwise it falls back to
// the legacy Scheduler.GetCrawler path. This enables incremental migration.
type FetchFunc func(ctx context.Context, name, sourceType string, params map[string]string) ([]model.ReleaseAsset, error)

// CrawlManager orchestrates fetching releases, filtering new ones, and
// triggering downloads.
type CrawlManager struct {
	db          *sql.DB
	scheduler   *Scheduler
	fileService *service.FileService
	projectRepo *dbrepo.ProjectRepo
	fileRepo    *dbrepo.FileRepo
	crawlRepo   *dbrepo.CrawlLogRepo
	config      *config.Config
	logger      *slog.Logger
	metrics     *metrics.Metrics

	// fetchFunc is the new two-track fetch entry point (optional). When nil, the
	// manager uses the legacy Scheduler.GetCrawler path.
	fetchFunc FetchFunc

	// retryCfg is the resolved per-crawl retry/timeout policy (built from config
	// in NewCrawlManager). See internal/crawler/retry.go.
	retryCfg retryConfig

	// Per-project crawl locks to prevent concurrent crawls of the same project.
	crawlMu sync.Map // string -> *sync.Mutex
}

// SetFetchFunc wires the new two-track fetch path. Optional; call before Start.
func (m *CrawlManager) SetFetchFunc(f FetchFunc) { m.fetchFunc = f }

// NewCrawlManager creates a new CrawlManager. Call Start() to begin crawling.
func NewCrawlManager(db *sql.DB, fileService *service.FileService, cfg *config.Config, logger *slog.Logger, m *metrics.Metrics) *CrawlManager {
	return &CrawlManager{
		db:          db,
		scheduler:   NewScheduler(logger),
		fileService: fileService,
		projectRepo: dbrepo.NewProjectRepo(db),
		fileRepo:    dbrepo.NewFileRepo(db),
		crawlRepo:   dbrepo.NewCrawlLogRepo(db),
		config:      cfg,
		logger:      logger,
		metrics:     m,
		retryCfg:    retryConfigFromCfg(cfg),
	}
}

// retryConfigFromCfg translates the config.CrawlerConfig knobs into a resolved
// retry policy. Missing/zero values fall back to safe defaults so a partial or
// old config still enables retry+timeout (the whole point of #23).
func retryConfigFromCfg(cfg *config.Config) retryConfig {
	if cfg == nil {
		return defaultRetryConfig()
	}
	rc := retryConfig{sleeper: sleepContext}
	if d, err := time.ParseDuration(cfg.Crawler.FetchTimeout); err == nil && d > 0 {
		rc.timeout = d
	}
	if cfg.Crawler.MaxRetries > 0 {
		rc.maxRetries = cfg.Crawler.MaxRetries
	}
	if d, err := time.ParseDuration(cfg.Crawler.RetryInitialBackoff); err == nil && d > 0 {
		rc.initialBackoff = d
	}
	return rc
}

// Start begins scheduled crawling for all enabled projects from the database.
func (m *CrawlManager) Start() error {
	return m.startProjects(context.Background())
}

// startProjects reads enabled projects from DB and starts scheduling.
func (m *CrawlManager) startProjects(ctx context.Context) error {
	projects, err := m.projectRepo.ListEnabled(ctx)
	if err != nil {
		return fmt.Errorf("listing enabled projects: %w", err)
	}
	for _, proj := range projects {
		if err := m.scheduleProject(ctx, proj); err != nil {
			m.logger.Error("failed to schedule project", "project", proj.Name, "error", err)
		}
	}
	return nil
}

// scheduleProject starts scheduling a single project using its DB settings.
func (m *CrawlManager) scheduleProject(ctx context.Context, proj *dbrepo.Project) error {
	interval := m.parseCrawlInterval(ctx, proj)
	projectName := proj.Name
	m.scheduler.StartProject(projectName, interval, func(ctx context.Context) error {
		_, err := m.TriggerCrawl(ctx, projectName)
		return err
	})
	return nil
}

// parseCrawlInterval reads crawl interval from project settings, falling back to default.
func (m *CrawlManager) parseCrawlInterval(ctx context.Context, proj *dbrepo.Project) time.Duration {
	settings, err := m.projectRepo.GetSettings(ctx, proj.ID)
	if err != nil {
		m.logger.Warn("failed to read project settings, using default interval", "project", proj.Name, "error", err)
	}
	if settings != nil && settings.CrawlInterval > 0 {
		return time.Duration(settings.CrawlInterval) * time.Second
	}
	defaultInterval, err := time.ParseDuration(m.config.Crawler.DefaultInterval)
	if err != nil {
		return 6 * time.Hour
	}
	return defaultInterval
}

// Stop halts all scheduled crawlers.
func (m *CrawlManager) Stop() {
	m.scheduler.StopAll()
}

// Scheduler returns the underlying scheduler for registering crawlers.
func (m *CrawlManager) Scheduler() *Scheduler {
	return m.scheduler
}

// TriggerCrawl performs a single crawl for the named project.
// It fetches releases, filters out existing files, downloads new ones,
// and updates the database.
func (m *CrawlManager) TriggerCrawl(ctx context.Context, projectName string) (*model.CrawlResult, error) {
	// Acquire per-project lock.
	mu := m.getProjectMutex(projectName)
	if !mu.TryLock() {
		return &model.CrawlResult{
			ProjectName: projectName,
			Status:      model.CrawlStatusError,
			Error:       ErrCrawlInProgress,
		}, ErrCrawlInProgress
	}
	defer mu.Unlock()

	proj, crwl, logID, err := m.resolveCrawlSetup(ctx, projectName)
	if err != nil {
		return nil, err
	}

	releases, errResult, err := m.fetchReleases(ctx, proj, crwl, logID, projectName)
	if err != nil {
		return errResult, err
	}

	newAssets, downloaded := m.processAssets(ctx, proj, releases, projectName)

	return m.finalizeCrawl(ctx, proj, logID, releases, newAssets, downloaded, projectName), nil
}

// resolveCrawlSetup looks up the project, resolves its crawler, and creates a crawl log entry.
// When the new fetchFunc is set, a legacy crawler is not required (the Registry
// handles routing); otherwise the legacy Scheduler crawler must be registered.
func (m *CrawlManager) resolveCrawlSetup(ctx context.Context, projectName string) (*dbrepo.Project, Crawler, int64, error) {
	proj, err := m.projectRepo.GetByName(ctx, projectName)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("finding project %q: %w", projectName, err)
	}
	if proj == nil {
		return nil, nil, 0, fmt.Errorf("project %q not found", projectName)
	}

	var crawler Crawler
	if m.fetchFunc == nil {
		// Legacy path requires a registered crawler.
		var ok bool
		crawler, ok = m.scheduler.GetCrawler(model.SourceType(proj.SourceType))
		if !ok {
			return nil, nil, 0, fmt.Errorf("no crawler registered for source type %q", proj.SourceType)
		}
	}

	crawlLog := &dbrepo.CrawlLog{
		ProjectID: proj.ID,
		StartedAt: time.Now(),
		Status:    string(model.CrawlStatusRunning),
	}
	logID, err := m.crawlRepo.Create(ctx, crawlLog)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("creating crawl log: %w", err)
	}

	return proj, crawler, logID, nil
}

// fetchReleases fetches releases from the upstream source.
// On failure it returns a CrawlResult with error details for the caller to return.
//
// Routing: when the new two-track fetchFunc is set (SetFetchFunc), it is used;
// otherwise the legacy Scheduler crawler (crwl) is used. This is the incremental
// migration seam (design Step 3).
//
// The fetch is run through fetchWithRetry so transient errors (timeout,
// connection reset, 5xx) are retried with bounded backoff, the whole fetch is
// bounded by a per-crawl timeout, and the final error is classified into one
// of: rate_limited, network_error (transient, after retries), or error
// (permanent upstream/config problem). This lets operators distinguish a
// flaky-network moment from a genuinely broken source (#23).
func (m *CrawlManager) fetchReleases(ctx context.Context, proj *dbrepo.Project, crwl Crawler, logID int64, projectName string) ([]model.ReleaseAsset, *model.CrawlResult, error) {
	crawlerName := ""
	if crwl != nil {
		crawlerName = crwl.Name()
	}

	// fetchOnce is the single-attempt fetch for either routing path. It closes
	// over which path is active so fetchWithRetry can retry it uniformly.
	fetchOnce := func(fctx context.Context) ([]model.ReleaseAsset, error) {
		if m.fetchFunc != nil {
			crawlerName = "registry:" + proj.SourceType
			return m.fetchFunc(fctx, projectName, proj.SourceType, m.getParams(proj))
		}
		// Legacy path.
		owner, repo := m.getOwnerRepo(proj)
		return crwl.FetchReleases(fctx, owner, repo)
	}

	releases, err, class := fetchWithRetry(ctx, m.retryCfg, fetchOnce)
	if err != nil {
		status := classToStatus(class)
		errMsg := err.Error()
		m.logger.Error("crawl failed",
			"project", projectName,
			"source_type", proj.SourceType,
			"crawler", crawlerName,
			"status", string(status),
			"error", errMsg,
		)
		_ = m.crawlRepo.UpdateFinished(ctx, logID, string(status), 0, 0, errMsg)
		if m.metrics != nil {
			m.metrics.CrawlTotal.WithLabelValues(proj.SourceType, string(status)).Inc()
		}
		return nil, &model.CrawlResult{
			ProjectName:   projectName,
			Status:        status,
			VersionsFound: 0,
			Error:         err,
		}, err
	}
	return releases, nil, nil
}

// classToStatus maps a retry error class to the crawl status persisted to the DB
// and emitted as a metric label. network_error is distinct from error so a
// transient (post-retry) failure is distinguishable from a permanent one.
func classToStatus(class errorClass) model.CrawlStatus {
	switch class {
	case classRateLimit:
		return model.CrawlStatusRateLimited
	case classTransient:
		return model.CrawlStatusNetworkError
	default:
		return model.CrawlStatusError
	}
}

// processAssets filters out existing files and downloads new ones.
func (m *CrawlManager) processAssets(ctx context.Context, proj *dbrepo.Project, releases []model.ReleaseAsset, projectName string) ([]model.ReleaseAsset, int) {
	var newAssets []model.ReleaseAsset
	downloaded := 0

	for _, asset := range releases {
		existing, err := m.fileRepo.FindExisting(ctx, proj.ID, asset.Filename)
		if err != nil {
			m.logger.Error("checking existing file", "project", projectName, "filename", asset.Filename, "error", err)
			continue
		}
		if existing != nil {
			continue // Already have this file.
		}

		storageSubdir := proj.StorageSubdir
		if storageSubdir == "" {
			storageSubdir = service.DefaultStorageSubdir(projectName)
		}
		localPath := filepath.Join(m.config.Storage.BasePath, storageSubdir, asset.Version, asset.Filename)
		dbFile := &dbrepo.File{
			ProjectID:   proj.ID,
			Version:     asset.Version,
			Filename:    asset.Filename,
			OS:          asset.OS,
			Arch:        asset.Arch,
			Ext:         asset.Ext,
			SizeBytes:   asset.SizeBytes,
			DownloadURL: asset.DownloadURL,
			LocalPath:   localPath,
			Checksum:    asset.Checksum,
			Status:      string(model.FileStatusPending),
		}
		fileID, err := m.fileRepo.Create(ctx, dbFile)
		if err != nil {
			m.logger.Error("creating file entry", "project", projectName, "filename", asset.Filename, "error", err)
			continue
		}

		mFile := &model.File{
			ID:          fileID,
			ProjectID:   int(proj.ID),
			Filename:    asset.Filename,
			DownloadURL: asset.DownloadURL,
			LocalPath:   localPath,
			SizeBytes:   asset.SizeBytes,
			Status:      model.FileStatusPending,
		}

		if err := m.fileService.DownloadFile(ctx, mFile); err != nil {
			m.logger.Error("downloading file", "project", projectName, "filename", asset.Filename, "error", err)
			continue
		}
		downloaded++
		newAssets = append(newAssets, asset)
	}
	return newAssets, downloaded
}

// finalizeCrawl updates project metadata, the crawl log, and metrics.
func (m *CrawlManager) finalizeCrawl(ctx context.Context, proj *dbrepo.Project, logID int64, releases []model.ReleaseAsset, newAssets []model.ReleaseAsset, downloaded int, projectName string) *model.CrawlResult {
	if len(releases) > 0 {
		latest := releases[0].Version
		_ = m.projectRepo.UpdateLatestVersion(ctx, proj.ID, latest)
	}
	_ = m.projectRepo.UpdateLastCrawledAt(ctx, proj.ID)
	_ = m.crawlRepo.UpdateFinished(ctx, logID, string(model.CrawlStatusSuccess), len(releases), downloaded, "")
	if m.metrics != nil {
		m.metrics.CrawlTotal.WithLabelValues(proj.SourceType, "success").Inc()
	}
	return &model.CrawlResult{
		ProjectName:     projectName,
		Status:          model.CrawlStatusSuccess,
		VersionsFound:   len(releases),
		FilesDownloaded: downloaded,
		NewAssets:       newAssets,
	}
}

// TriggerAllCrawls triggers a crawl for every enabled project and returns results.
func (m *CrawlManager) TriggerAllCrawls(ctx context.Context) []model.CrawlResult {
	var results []model.CrawlResult
	projects, err := m.projectRepo.ListEnabled(ctx)
	if err != nil {
		m.logger.Error("listing enabled projects for crawl", "error", err)
		return results
	}
	for _, proj := range projects {
		result, err := m.TriggerCrawl(ctx, proj.Name)
		if err != nil && result == nil {
			result = &model.CrawlResult{
				ProjectName: proj.Name,
				Status:      model.CrawlStatusError,
				Error:       err,
			}
		}
		if result != nil {
			results = append(results, *result)
		}
	}
	return results
}

// GetCrawlStatus returns the crawl status of all enabled projects.
func (m *CrawlManager) GetCrawlStatus() map[string]CrawlStatusInfo {
	statuses := make(map[string]CrawlStatusInfo)
	projects, err := m.projectRepo.ListEnabled(context.Background())
	if err != nil {
		m.logger.Error("listing enabled projects for status", "error", err)
		return statuses
	}
	for _, proj := range projects {
		interval := time.Duration(0)
		m.scheduler.mu.Lock()
		if d, ok := m.scheduler.intervals[proj.Name]; ok {
			interval = d
		}
		m.scheduler.mu.Unlock()

		var lastCrawled *string
		if proj.LastCrawledAt != nil {
			s := proj.LastCrawledAt.Format(time.RFC3339)
			lastCrawled = &s
		}

		statuses[proj.Name] = CrawlStatusInfo{
			ProjectName:   proj.Name,
			SourceType:    model.SourceType(proj.SourceType),
			Running:       m.scheduler.Running(proj.Name),
			Interval:      interval,
			LatestVersion: proj.LatestVersion,
			LastCrawledAt: lastCrawled,
		}
	}
	return statuses
}

// getProjectMutex returns (or creates) a mutex for the given project name.
func (m *CrawlManager) getProjectMutex(name string) *sync.Mutex {
	val, _ := m.crawlMu.LoadOrStore(name, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// getOwnerRepo extracts owner and repo from a project's DB settings.
// For GitHub/Grafana sources, it reads from config JSON; for others, returns empty strings.
func (m *CrawlManager) getOwnerRepo(proj *dbrepo.Project) (string, string) {
	settings, err := m.projectRepo.GetSettings(context.Background(), proj.ID)
	if err != nil || settings == nil {
		return "", ""
	}
	return settings.GitHubOwner, settings.GitHubRepo
}

// getParams builds the source.Params map for the new two-track path from a
// project's DB settings. It generalizes getOwnerRepo: today it carries
// owner/repo (matching the legacy overloading), and can grow new keys as
// sources are migrated to fingerprints (e.g. {"fingerprint":"github"}).
func (m *CrawlManager) getParams(proj *dbrepo.Project) map[string]string {
	settings, err := m.projectRepo.GetSettings(context.Background(), proj.ID)
	if err != nil || settings == nil {
		return map[string]string{}
	}
	return map[string]string{
		"owner": settings.GitHubOwner,
		"repo":  settings.GitHubRepo,
	}
}

// isRateLimitError checks if an error indicates rate limiting.
func isRateLimitError(err error) bool {
	return errors.Is(err, ErrRateLimited)
}

// ErrRateLimited is a sentinel error returned by crawlers when rate limited.
var ErrRateLimited = errors.New("rate limited by upstream API")

// ReloadProjects re-reads all enabled projects from DB and restarts the scheduler.
func (m *CrawlManager) ReloadProjects(ctx context.Context) error {
	m.scheduler.StopAll()
	return m.startProjects(ctx)
}
