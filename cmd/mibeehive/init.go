package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	webpkg "github.com/Mi-Bee-Studio/mibeehive"
	"github.com/Mi-Bee-Studio/mibeehive/internal/backup"
	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/docker"
	"github.com/Mi-Bee-Studio/mibeehive/internal/handler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/health"
	"github.com/Mi-Bee-Studio/mibeehive/internal/metrics"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/monitor"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"github.com/Mi-Bee-Studio/mibeehive/internal/source"
	"github.com/Mi-Bee-Studio/mibeehive/internal/supply"
	"github.com/Mi-Bee-Studio/mibeehive/internal/eventbus"
	webdavpkg "github.com/Mi-Bee-Studio/mibeehive/internal/webdav"
	"gopkg.in/natefinch/lumberjack.v2"
)

// appServices holds all initialized services, repositories, and background task handles.
type appServices struct {
	// Database pools
	readDB *sql.DB

	// Metrics
	// Metrics
	appMetrics *metrics.Metrics

	// Repositories
	projectRepo     *db.ProjectRepo
	fileRepo        *db.FileRepo
	crawlLogRepo    *db.CrawlLogRepo
	credRepo        *db.SourceCredentialRepo
	osConfigRepo    *db.OsInstallConfigRepo
	catalogRepo     *db.ISOCatalogRepo
	appTemplateRepo *db.AppTemplateRepo
	statsRepo       *db.SystemStatsRepo

	// Core services
	fileService    *service.FileService
	isoService     *service.ISOService
	catalogService *service.ISOCatalogService
	crawlManager   *crawler.CrawlManager

	// Docker (optional)
	dockerClient    *docker.Client
	containerSvc    *service.ContainerService
	imageSvc        *service.ImageService
	containerLogSvc *service.ContainerLogService

	// Additional services
	logService    *service.LogService
	taskService   *service.TaskService
	searchService *service.SearchService

	// Registry (optional)
	registrySvc        *service.RegistryService
	syncSvc            *service.SyncService
	retentionSvc       *service.RetentionService
	retentionScheduler *service.RetentionScheduler

	// Storage resolver
	storageResolver *service.StorageResolver
	migrationSvc    *service.MigrationService
	// Virtual index
	virtualIndexSvc *service.VirtualIndexService
	// Event bus
	eventbus       *eventbus.Bus
	// Cancel funcs for background goroutines started during init
	diskCancel     context.CancelFunc
	retryCancel    context.CancelFunc
	registryCancel context.CancelFunc
	globalCancel   context.CancelFunc
}

// appHandlers holds all initialized HTTP handlers.
type appHandlers struct {
	auth          *handler.AuthHandler
	project       *handler.ProjectHandler
	file          *handler.FileHandler
	crawl         *handler.CrawlHandler
	system        *handler.SystemHandler
	osInstall     *handler.OSInstallHandler
	projectAdmin  *handler.ProjectAdminHandler
	crawlControl  *handler.CrawlControlHandler
	configHandler *handler.ConfigHandler
	webdavAdmin   *handler.WebDAVAdminHandler
	iso           *handler.ISOHandler
	backupH       *handler.BackupHandler
	dashboard     *handler.DashboardHandler
	log           *handler.LogHandler
	task          *handler.TaskHandler
	search        *handler.SearchHandler
	appTemplate   *handler.AppTemplateHandler
	container     *handler.ContainerHandler
	registry      *handler.RegistryHandler // nil if remote registry disabled
	storageConfig *handler.StorageConfigHandler
	supply        *supply.Handler    // public ops-tool supply endpoints (#3)
	aptRepo       *supply.AptHandler // public APT repository over /apt/ (deb supply)
	pypiRepo      *supply.PyPIHandler // public PyPI Simple repository over /simple/ (#24)
	download      *handler.DownloadHandler // public file download by token
	shareLink     *handler.ShareLinkHandler // share link admin and public download
	adminInternal *handler.AdminInternalHandler // admin-only internal file details (exposes local_path)
	fileCenter   *handler.FileCenterHandler // cross-project file listing with filtering
	cacheMetrics  *handler.CacheMetricsHandler // admin cache metrics endpoint
	virtualAdmin  *handler.VirtualAdminHandler // virtual index admin API
	toolCatalog   *handler.ToolCatalogHandler // built-in tool catalog + one-click enable
	}
// loadConfig initializes the logger, loads or generates the config file,
// loadConfig initializes the logger, loads or generates the config file,
// and sets up log rotation via lumberjack.
func loadConfig(configPath string) *config.Config {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Load config, or generate default if not exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		slog.Info("config file not found, generating default", "path", configPath)
		if err := config.GenerateDefault(configPath); err != nil {
			slog.Error("failed to generate default config", "error", err)
			os.Exit(1)
		}
		slog.Info("default config written", "path", configPath)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	// Replace bootstrap logger with lumberjack-backed logger for log rotation.
	lumberjackWriter := &lumberjack.Logger{
		Filename:   cfg.Logging.Filename,
		MaxSize:    cfg.Logging.MaxSize,
		MaxBackups: cfg.Logging.MaxBackups,
		MaxAge:     cfg.Logging.MaxAge,
		Compress:   cfg.Logging.Compress,
		LocalTime:  cfg.Logging.LocalTime,
	}

	// Verify the log file can be opened, fall back to stdout only if it fails.
	if f, err := os.OpenFile(cfg.Logging.Filename, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644); err != nil {
		slog.Warn("log file unavailable, falling back to stdout only", "filename", cfg.Logging.Filename, "error", err)
	} else {
		f.Close()
		multiWriter := io.MultiWriter(os.Stdout, lumberjackWriter)
		logger = slog.New(slog.NewTextHandler(multiWriter, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}))
		slog.SetDefault(logger)
	}

	slog.Info("config loaded",
		"projects", len(cfg.Projects),
		"port", cfg.Server.Port,
		"bind_addr", cfg.Server.BindAddr,
		"db_path", cfg.Database.Path,
		"storage_path", cfg.Storage.BasePath,
		"max_concurrent", cfg.Crawler.MaxConcurrent,
		"default_interval", cfg.Crawler.DefaultInterval,
	)

	if cfg.Auth.IsDefaultPassword() {
		slog.Warn("default password detected - change immediately")
	}
	for _, p := range cfg.Projects {
		slog.Info("project configured",
			"name", p.Name,
			"display_name", p.DisplayName,
			"source_type", p.SourceType,
			"source_url", p.SourceURL,
		)
	}

	return cfg
}

// initDatabase opens the SQLite database (write + read pools) and runs pending
// migrations. It returns the write pool for backward compatibility and the
// read pool for concurrent read workloads.
func initDatabase(cfg *config.Config) (*sql.DB, *sql.DB) {
	database, err := db.Open(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}

	if err := db.Migrate(database); err != nil {
		slog.Error("failed to migrate database", "error", err)
		os.Exit(1)
	}

	// Verify WAL journal mode is active.
	var journalMode string
	if err := database.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		slog.Error("failed to verify journal_mode", "error", err)
		os.Exit(1)
	}
	if journalMode != "wal" {
		slog.Error("journal_mode is not wal", "mode", journalMode)
		os.Exit(1)
	}
	slog.Info("database pragmas verified", "journal_mode", journalMode)

	readDB, err := db.OpenReadDB(cfg.Database.Path)
	if err != nil {
		slog.Error("failed to open read database", "error", err)
		os.Exit(1)
	}

	slog.Info("database initialized", "path", cfg.Database.Path)

	return database, readDB
}

// initServices creates all repositories, services, and starts background goroutines
// that need to run before HTTP handlers are registered.
func initServices(cfg *config.Config, database *sql.DB, readDB *sql.DB) *appServices {
	s := &appServices{}
	s.readDB = readDB

	// Initialize Prometheus metrics.
	s.appMetrics = metrics.NewMetrics()
	slog.Info("prometheus metrics initialized")

	// Initialize disk monitor.
	if cfg.Monitor.DiskCheckEnabled {
		diskCheckInterval := time.Duration(cfg.Monitor.SampleInterval) * time.Second
		diskMonitor := monitor.NewDiskMonitor(
			cfg.Storage.BasePath,
			cfg.Monitor.DiskWarningPercent,
			cfg.Monitor.DiskCriticalPercent,
			diskCheckInterval,
		)
		diskCtx, dc := context.WithCancel(context.Background())
		s.diskCancel = dc
		go diskMonitor.Start(diskCtx)
		slog.Info("disk monitor started",
			"warning_pct", cfg.Monitor.DiskWarningPercent,
			"critical_pct", cfg.Monitor.DiskCriticalPercent,
		)
	}

	// Seed projects from config if DB is empty.
	s.projectRepo = db.NewProjectRepo(database)
	seedProjects := config.SeedProjects()
	if len(seedProjects) > 0 {
		seedCount, err := db.SeedProjectsFromConfig(context.Background(), s.projectRepo, seedProjects)
		if err != nil {
			slog.Warn("failed to seed projects from config", "error", err)
		} else if seedCount > 0 {
			slog.Info("seeded projects from config", "count", seedCount)
		}
	}

	// Seed default OS install configs if not already present.
	s.osConfigRepo = db.NewOsInstallConfigRepo(database)
	if seedCount, err := db.SeedOSInstallConfigs(context.Background(), s.osConfigRepo); err != nil {
		slog.Warn("failed to seed OS install configs", "error", err)
	} else if seedCount > 0 {
		slog.Info("seeded default OS install configs", "count", seedCount)
	}

	// Reset zombie downloads (status='downloading' but file doesn't exist on disk).
	s.fileRepo = db.NewFileRepo(database)
	resetCount, err := s.fileRepo.ResetZombieDownloads(context.Background())
	if err != nil {
		slog.Warn("failed to reset zombie downloads", "error", err)
	} else if resetCount > 0 {
		slog.Info("reset zombie downloads", "count", resetCount)
	}

	// Create global context for graceful shutdown of active downloads.
	_, s.globalCancel = context.WithCancel(context.Background())

	// Ensure storage directories exist.
	os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "webdav"), 0o755)
	os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "os-install"), 0o755)
	s.storageResolver = service.NewStorageResolver(cfg)
	s.fileService = service.NewFileService(database, s.storageResolver, cfg.Crawler.MaxConcurrent, s.appMetrics)

	// Read API tokens from DB for crawlers that need them.
	s.credRepo = db.NewSourceCredentialRepo(database)
	initCtx := context.Background()
	githubToken := ""
	if cred, err := s.credRepo.GetBySourceType(initCtx, "github"); err != nil {
		slog.Warn("failed to read github credential", "error", err)
	} else if cred != nil {
		githubToken = cred.Token
	}
	hashicorpToken := ""
	if cred, err := s.credRepo.GetBySourceType(initCtx, "hashicorp"); err != nil {
		slog.Warn("failed to read hashicorp credential", "error", err)
	} else if cred != nil {
		hashicorpToken = cred.Token
	}

	// Initialize crawl manager with registered crawlers.
	logger := slog.Default()
	s.crawlManager = crawler.NewCrawlManager(database, s.fileService, cfg, logger, s.appMetrics)
	s.crawlManager.Scheduler().Register(crawler.NewGitHubCrawler(githubToken, logger))
	s.crawlManager.Scheduler().Register(crawler.NewGoCrawler(logger))
	s.crawlManager.Scheduler().Register(crawler.NewHashiCorpCrawler(hashicorpToken, logger))
	s.crawlManager.Scheduler().Register(crawler.NewGrafanaCrawler(logger))

	// Read NPM token if available.
	npmToken := ""
	if cred, err := s.credRepo.GetBySourceType(initCtx, "npm"); err != nil {
		slog.Warn("failed to read npm credential", "error", err)
	} else if cred != nil {
		npmToken = cred.Token
	}
	// Read PyPI token if available.
	pypiToken := ""
	if cred, err := s.credRepo.GetBySourceType(initCtx, "pypi"); err != nil {
		slog.Warn("failed to read pypi credential", "error", err)
	} else if cred != nil {
		pypiToken = cred.Token
	}

	// Register new crawlers.
	s.crawlManager.Scheduler().Register(crawler.NewNPMCrawler(npmToken, logger))
	s.crawlManager.Scheduler().Register(crawler.NewPyPICrawler(pypiToken, logger))
	s.crawlManager.Scheduler().Register(crawler.NewCratesCrawler("", logger))

	// Wire the two-track source.Registry (design Step 3). GitHub is served by a
	// YAML fingerprint (RuleFetcher); the rest are wrapped as LegacyAdapters so
	// their behavior is unchanged. The Registry.Fetch matches CrawlManager's
	// FetchFunc signature. When set, the manager prefers the Registry and falls
	// back to the Scheduler crawlers above only if fetchFunc were nil.
	reg := source.NewRegistry()
	ruleFetcher, err := source.NewRuleFetcher()
	if err != nil {
		slog.Warn("failed to load rule fingerprints; github/grafana will use legacy crawlers", "error", err)
		// Fallback: serve github + grafana via legacy crawlers.
		reg.Register(source.NewLegacyAdapter(crawler.NewGitHubCrawler(githubToken, logger)))
		reg.Register(source.NewLegacyAdapter(crawler.NewGrafanaCrawler(logger)))
	} else {
		// github + grafana served by fingerprints. RuleFetcher.Sources() reports
		// the builtin fingerprint names, so these types route to it.
		// Wire token resolution so fingerprint requests carry API tokens (read
		// from source_credentials at fetch time) — keeps secrets out of YAML and
		// avoids GitHub's 60/hr unauthenticated rate limit.
		ruleFetcher.SetTokenResolver(func(sourceType string) string {
			cred, err := s.credRepo.GetBySourceType(initCtx, sourceType)
			if err != nil || cred == nil {
				return ""
			}
			return cred.Token
		})
		reg.Register(ruleFetcher)
	}
	// Wrap the remaining crawlers as LegacyAdapters. These sources (go/npm/pypi/
	// crates/hashicorp) have logic too complex for a declarative fingerprint
	// (scoped packages, version filtering/sorting, per-file OS/Arch from the
	// response, multi-step) — they stay code adapters per the #1 boundary.
	reg.Register(source.NewLegacyAdapter(crawler.NewGoCrawler(logger)))
	reg.Register(source.NewLegacyAdapter(crawler.NewHashiCorpCrawler(hashicorpToken, logger)))
	reg.Register(source.NewLegacyAdapter(crawler.NewNPMCrawler(npmToken, logger)))
	reg.Register(source.NewLegacyAdapter(crawler.NewPyPICrawler(pypiToken, logger)))
	reg.Register(source.NewLegacyAdapter(crawler.NewCratesCrawler("", logger)))
	s.crawlManager.SetFetchFunc(reg.Fetch)
	slog.Info("source registry wired", "types", reg.Types())

	// Start background download retry worker with periodic zombie cleanup.
	const maxDownloadRetries = 3
	retryCtx, retryCancel := context.WithCancel(context.Background())
	s.retryCancel = retryCancel
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				retryFailedDownloads(retryCtx, s.fileRepo, s.fileService, maxDownloadRetries)
				// Periodically reset zombie downloads stuck in 'downloading' status.
				if resetCount, err := s.fileRepo.ResetZombieDownloads(retryCtx); err == nil && resetCount > 0 {
					slog.Info("reset zombie downloads", "count", resetCount)
				}
			case <-retryCtx.Done():
				return
			}
		}
	}()
	slog.Info("download retry worker started", "interval", "5m", "max_retries", maxDownloadRetries)

	// Initialize ISO service for OS install provisioning.
	s.isoService = service.NewISOService(s.storageResolver, 2, s.appMetrics)

	// Initialize ISO catalog service.
	s.catalogRepo = db.NewISOCatalogRepo(database)
	s.catalogService = service.NewISOCatalogService(s.catalogRepo, s.isoService, logger, s.appMetrics)
	if err := os.MkdirAll(filepath.Join(cfg.Storage.BasePath, "os-install"), 0755); err != nil {
		slog.Error("failed to create os-install directory", "error", err)
		os.Exit(1)
	}
	// Initialize storage migration service.
	migrationTaskRepo := db.NewMigrationTaskRepo(database)
	s.migrationSvc = service.NewMigrationService(migrationTaskRepo, s.fileRepo, logger)
	if err := s.migrationSvc.ResetStaleMigrations(context.Background()); err != nil {
		slog.Warn("resetting stale migrations", "error", err)
	}

	// Initialize Docker client for container management (optional).
	if cfg.Container.Local.Enabled {
		dc, err := docker.NewClient(cfg.Container.Local.DockerHost)
		if err != nil {
			slog.Warn("docker client creation failed, container management disabled", "error", err)
		} else if !dc.IsAvailable() {
			slog.Warn("docker daemon unavailable, container management disabled")
			dc.Close()
		} else {
			s.dockerClient = dc
			slog.Info("docker client initialized", "socket", cfg.Container.Local.DockerHost)
		}
	}

	// Container management services (optional — requires Docker).
	if s.dockerClient != nil {
		s.containerSvc = service.NewContainerService(s.dockerClient.DockerClient(), logger)
		s.imageSvc = service.NewImageService(s.dockerClient.DockerClient(), logger)
		s.containerLogSvc = service.NewContainerLogService(s.dockerClient.DockerClient())
	}

	// Additional services.
	s.logService = service.NewLogService(database, cfg.Logging.Filename)
	s.taskService = service.NewTaskService(database)
	s.searchService = service.NewSearchService(database)
	s.appTemplateRepo = db.NewAppTemplateRepo(database)
	s.crawlLogRepo = db.NewCrawlLogRepo(database)
	s.statsRepo = db.NewSystemStatsRepo(database)

	// Registry module (optional — requires remote registry enabled).
	if cfg.Container.Remote.Enabled {
		registryRepo := db.NewRegistryRepo(database, cfg.Auth.JWTSecret)
		syncTaskRepo := db.NewSyncTaskRepo(database)
		retentionPolicyRepo := db.NewRetentionPolicyRepo(database)

		s.registrySvc = service.NewRegistryService(registryRepo)
		s.syncSvc = service.NewSyncService(syncTaskRepo, registryRepo, cfg.Container.Remote.SyncConcurrency)
		s.retentionSvc = service.NewRetentionService(retentionPolicyRepo, registryRepo)

		interval := time.Hour
		if cfg.Container.Remote.RetentionCheckInterval != "" {
			if d, err := time.ParseDuration(cfg.Container.Remote.RetentionCheckInterval); err == nil && d > 0 {
				interval = d
			}
		}
		s.retentionScheduler = service.NewRetentionScheduler(s.retentionSvc, interval)
		var regCtx context.Context
		regCtx, s.registryCancel = context.WithCancel(context.Background())
		go s.retentionScheduler.Start(regCtx)
		slog.Info("registry module initialized", "retention_interval", interval.String())
	} else {
		slog.Info("remote registry management disabled")
	}

	// Initialize event bus for virtual index events.
	s.eventbus = eventbus.NewBus(100)
	// Initialize virtual index service.
	virtualRepo := db.NewVirtualRepo(database)
	s.virtualIndexSvc = service.NewVirtualIndexService(virtualRepo, s.eventbus, logger)
	slog.Info("virtual index service initialized")

	return s

	return s
}

// initHandlers creates all HTTP handlers.
func initHandlers(cfg *config.Config, svcs *appServices, database *sql.DB, configPath string, requestShutdown func()) *appHandlers {
	h := &appHandlers{}

	jwtSecret := cfg.Auth.JWTSecret
	h.auth = handler.NewAuthHandler(cfg.Auth.PasswordHash, jwtSecret, cfg.Auth.GetPasswordChangedAt)
	h.project = handler.NewProjectHandler(database)
	h.file = handler.NewFileHandler(database, svcs.fileService, jwtSecret)
	// Supply layer (Issue #3): public ops-tool repo index + download. Reuses
	// FileService.StreamFile; built from the same FileRepo/FileService as admin.
	h.supply = supply.NewHandler(db.NewFileRepo(database), svcs.fileService)
	// APT repository: serve collected .deb files as a Debian repo under /apt/.
	// basePath is where .deb files live on disk (for control-metadata parsing).
	h.aptRepo = supply.NewAptHandler(db.NewFileRepo(database), svcs.fileService, cfg.Storage.BasePath, "stable")
	// PyPI Simple repository (#24): serve collected wheels/sdists as a PEP 503
	// index under /simple/ so external Python hosts can `pip install` from it.
	h.pypiRepo = supply.NewPyPIHandler(db.NewFileRepo(database), db.NewProjectRepo(database), svcs.fileService)
	h.crawl = handler.NewCrawlHandler(svcs.crawlManager, db.NewCrawlLogRepo(database), db.NewProjectRepo(database))
	h.system = handler.NewSystemHandler(database, svcs.fileService, cfg.Storage.BasePath, version, cfg.Monitor.NodeExporterURL)
	h.osInstall = handler.NewOSInstallHandler(db.NewOsInstallConfigRepo(database), service.NewOsTemplateService(), cfg.Storage.BasePath)
	h.projectAdmin = handler.NewProjectAdminHandler(db.NewProjectRepo(database), db.NewFileRepo(database), svcs.crawlManager, cfg)
	h.webdavAdmin = handler.NewWebDAVAdminHandler(cfg, svcs.storageResolver)
	h.configHandler = handler.NewConfigHandler(cfg, configPath)
	migrationAdapter := service.NewMigrationAdapter(svcs.migrationSvc, slog.Default())
	h.storageConfig = handler.NewStorageConfigHandler(cfg, configPath, svcs.storageResolver, migrationAdapter, slog.Default())
	h.crawlControl = handler.NewCrawlControlHandler(db.NewProjectRepo(database), db.NewSourceCredentialRepo(database), svcs.crawlManager)
	h.iso = handler.NewISOHandler(svcs.isoService, svcs.catalogService, jwtSecret)
	h.backupH = handler.NewBackupHandler(cfg.Backup.LocalPath, cfg.Database.Path, requestShutdown)
	dashSvc := service.NewDashboardService(svcs.fileService, db.NewProjectRepo(database), svcs.fileRepo, db.NewCrawlLogRepo(database), svcs.osConfigRepo, svcs.catalogRepo, cfg, version)
	h.dashboard = handler.NewDashboardHandler(dashSvc)

	h.log = handler.NewLogHandler(svcs.logService)
	h.task = handler.NewTaskHandler(svcs.taskService)
	h.search = handler.NewSearchHandler(svcs.searchService)
	h.appTemplate = handler.NewAppTemplateHandler(svcs.appTemplateRepo)
	h.container = handler.NewContainerHandler(svcs.containerSvc, svcs.imageSvc, svcs.containerLogSvc, slog.Default())

	// Registry handler (optional — only if remote registry enabled).
	if cfg.Container.Remote.Enabled && svcs.registrySvc != nil {
		h.registry = handler.NewRegistryHandler(svcs.registrySvc, svcs.syncSvc, svcs.retentionSvc, true)
	}

	// Download handler: public file download by public_token (no auth).
	h.download = handler.NewDownloadHandler(database, cfg.Storage.BasePath)
	// Share link handler: admin CRUD + public download.
	h.shareLink = handler.NewShareLinkHandler(database, svcs.readDB, cfg.Storage.BasePath)
	// Admin-only internal file details endpoint (exposes physical local_path).
	h.adminInternal = handler.NewAdminInternalHandler(svcs.readDB)
	h.cacheMetrics = handler.NewCacheMetricsHandler()
	// File center handler: cross-project file listing with filtering (requires auth).
	h.fileCenter = handler.NewFileCenterHandler(svcs.readDB)
	// Virtual index admin handler.
	h.virtualAdmin = handler.NewVirtualAdminHandler(svcs.virtualIndexSvc)
	// Tool catalog handler: built-in catalog + one-click enable.
	h.toolCatalog = handler.NewToolCatalogHandler(service.NewToolCatalogService(), db.NewProjectRepo(database))

	return h
	return h
}

// buildRouter creates the HTTP ServeMux with all routes registered and
// middleware applied. Returns (httpHandler, httpsHandler) where httpHandler
// rejects WebDAV requests (WebDAV is HTTPS-only).
func buildRouter(cfg *config.Config, h *appHandlers, svcs *appServices, database *sql.DB) (http.Handler, http.Handler) {
	mux := http.NewServeMux()

	// Auth routes (no middleware).
	mux.HandleFunc("POST /api/v1/auth/login", h.auth.Login)

	// PXE routes (public, no auth — PXE clients can't authenticate).
	mux.HandleFunc("GET "+model.RoutePXEConfig, h.osInstall.ServePXEConfig)

	// Health and metrics endpoints (public, no auth).
	healthHandler := health.NewHealthHandler(database, version)
	mux.HandleFunc(model.RouteHealth, healthHandler.ServeHTTP)
	mux.Handle(model.RouteMetrics, svcs.appMetrics.Handler())

	// File download route (public — does its own JWT validation from header or ?token= query param).
	mux.HandleFunc("GET /api/v1/files/{id}/download", h.file.Download)

	// Public file download by token (no auth — token-based access).
	mux.HandleFunc("GET "+model.RouteFileDownloadByToken, h.download.ServeDownload)
	// Public share link download (no auth — token-based access).
	mux.HandleFunc(model.RouteShareDownload, h.shareLink.ShareDownload)
	// Public ISO endpoints (list is public, download does its own JWT validation).
	mux.HandleFunc("GET "+model.RouteFileDownloadByToken, h.download.ServeDownload)
	// Public ISO endpoints (list is public, download does its own JWT validation).
	mux.HandleFunc("GET "+model.RouteISOsList, h.iso.PublicListISOs)
	mux.HandleFunc("GET "+model.RouteISODownload, h.iso.DownloadISO)

	// Supply layer (Issue #3): public ops-tool repository index + download.
	// External servers discover and pull collected tools from here without auth.
	if h.supply != nil {
		mux.HandleFunc("GET /repo/index", h.supply.ServeIndex)
		mux.HandleFunc("GET /repo/files/{id}", h.supply.ServeFile)
	}
	// APT repository (deb supply): metadata + pool download. Public (no auth) so
	// external Debian/Ubuntu hosts can `apt update` / `apt install` from it.
	if h.aptRepo != nil {
		mux.HandleFunc("GET /apt/{rest...}", h.aptRepo.Serve)
	}
	// PyPI Simple repository (#24): PEP 503 index of collected wheels/sdists.
	// Public (no auth) so external Python hosts can `pip install --index-url
	// http://<host>/simple/ <pkg>` from it.
	if h.pypiRepo != nil {
		mux.HandleFunc("GET /simple/{rest...}", h.pypiRepo.Serve)
	}

	// API routes (protected by auth middleware).
	apiMux := http.NewServeMux()
	apiMux.HandleFunc("GET "+model.RoutePasswordStatus, h.auth.PasswordStatus)
	apiMux.HandleFunc("POST "+model.RouteAuthRefresh, h.auth.RefreshToken)
	apiMux.HandleFunc("GET /api/v1/projects", h.project.List)
	apiMux.HandleFunc("GET /api/v1/projects/{id}", h.project.GetByID)
	apiMux.HandleFunc("GET /api/v1/projects/{id}/files", h.file.ListByProject)
	apiMux.HandleFunc("GET /api/v1/files/search", h.file.Search)
	apiMux.HandleFunc("GET /api/v1/crawl/status", h.crawl.Status)
	apiMux.HandleFunc("POST /api/v1/crawl/trigger", h.crawl.Trigger)
	apiMux.HandleFunc("GET "+model.RouteCrawlLogs, h.crawl.ListLogs)
	apiMux.HandleFunc("GET "+model.RouteSystemStats, h.system.Stats)
	apiMux.HandleFunc("GET /api/v1/system/info", h.system.Info)
	apiMux.HandleFunc("GET "+model.RouteSystemStatsHistory, h.system.StatsHistory)
	apiMux.HandleFunc("GET "+model.RouteFileQueue, h.file.ListQueue)
	apiMux.HandleFunc("GET "+model.RouteFileQueueStats, h.file.QueueStats)
	apiMux.HandleFunc("GET "+model.RouteFileQueueProgress, h.file.QueueProgress)

	// OS install config routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminOSInstallList, h.osInstall.ListConfigs)
	apiMux.HandleFunc("POST "+model.RouteAdminOSInstallCreate, h.osInstall.CreateConfig)
	apiMux.HandleFunc("GET "+model.RouteAdminOSInstallGet, h.osInstall.GetConfig)
	apiMux.HandleFunc("PUT "+model.RouteAdminOSInstallUpdate, h.osInstall.UpdateConfig)
	apiMux.HandleFunc("DELETE "+model.RouteAdminOSInstallDelete, h.osInstall.DeleteConfig)
	apiMux.HandleFunc("POST "+model.RouteAdminOSInstallPreview, h.osInstall.PreviewConfig)

	apiMux.HandleFunc("GET "+model.RouteAdminProjectsList, h.projectAdmin.ListProjects)
	apiMux.HandleFunc("GET "+model.RouteAdminProjectsGet, h.projectAdmin.GetProject)
	apiMux.HandleFunc("POST "+model.RouteAdminProjectsCreate, h.projectAdmin.CreateProject)
	apiMux.HandleFunc("PUT "+model.RouteAdminProjectsUpdate, h.projectAdmin.UpdateProject)
	apiMux.HandleFunc("DELETE "+model.RouteAdminProjectsDelete, h.projectAdmin.DeleteProject)
	apiMux.HandleFunc("PATCH "+model.RouteAdminProjectsToggle, h.projectAdmin.ToggleProject)
	apiMux.HandleFunc("POST "+model.RouteAdminCrawlTrigger, h.crawlControl.TriggerCrawl)
	apiMux.HandleFunc("POST "+model.RouteAdminCrawlTriggerAll, h.crawlControl.TriggerAllCrawls)
	apiMux.HandleFunc("POST "+model.RouteAdminCrawlPause, h.crawlControl.PauseProject)
	apiMux.HandleFunc("POST "+model.RouteAdminCrawlResume, h.crawlControl.ResumeProject)
	apiMux.HandleFunc("GET "+model.RouteAdminCrawlStatus, h.crawlControl.GetCrawlStatus)
	apiMux.HandleFunc("GET "+model.RouteAdminCredentialsList, h.crawlControl.ListCredentials)
	apiMux.HandleFunc("PUT "+model.RouteAdminCredentialsUpsert, h.crawlControl.UpsertCredential)
	apiMux.HandleFunc("POST "+model.RouteAdminPasswordChange, h.configHandler.ChangePassword)
	apiMux.HandleFunc("GET "+model.RouteAdminWebDAVStatus, h.webdavAdmin.WebDAVStatus)
	apiMux.HandleFunc("GET "+model.RouteAdminWebDAVList, h.webdavAdmin.WebDAVFileList)
	apiMux.HandleFunc("GET "+model.RouteAdminConfigMonitor, h.configHandler.GetMonitorConfig)
	apiMux.HandleFunc("PUT "+model.RouteAdminConfigMonitor, h.configHandler.UpdateMonitorConfig)
	// Storage config routes (admin).
	apiMux.HandleFunc("GET "+model.RouteStorageConfig, h.storageConfig.GetStorageConfig)
	apiMux.HandleFunc("PUT "+model.RouteStorageConfig, h.storageConfig.UpdateStorageConfig)
	apiMux.HandleFunc("GET "+model.RouteStorageMigrations, h.storageConfig.ListMigrations)
	apiMux.HandleFunc("GET "+model.RouteStorageMigrationByID+"{id}", h.storageConfig.GetMigration)
	apiMux.HandleFunc("POST "+model.RouteStorageMigrationByID+"{id}/cancel", h.storageConfig.CancelMigration)

	// ISO management routes (admin).
	apiMux.HandleFunc("POST "+model.RouteAdminISODownload, h.iso.TriggerDownload)
	apiMux.HandleFunc("GET "+model.RouteAdminISOsList, h.iso.ListISOs)
	apiMux.HandleFunc("DELETE "+model.RouteAdminISODelete, h.iso.DeleteISO)
	// ISO catalog routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminISOCatalogList, h.iso.ISOCatalogList)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogCreate, h.iso.ISOCatalogCreate)
	apiMux.HandleFunc("PUT "+model.RouteAdminISOCatalogUpdate, h.iso.ISOCatalogUpdate)
	apiMux.HandleFunc("DELETE "+model.RouteAdminISOCatalogDelete, h.iso.ISOCatalogDelete)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogCheck, h.iso.ISOCatalogCheck)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogDownload, h.iso.ISOCatalogDownload)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogRetry, h.iso.ISOCatalogRetry)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogCancel, h.iso.ISOCatalogCancel)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogCheckAll, h.iso.ISOCatalogCheckAll)
	// ISO queue routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminISOCatalogQueue, h.iso.ISOCatalogQueue)
	apiMux.HandleFunc("POST "+model.RouteAdminISOCatalogDownloadAll, h.iso.ISOCatalogDownloadAll)
	apiMux.HandleFunc("GET "+model.RouteAdminISOQueueProgress, h.iso.ISOQueueProgress)
	apiMux.HandleFunc(model.RouteAdminISOCatalogProfiles, h.iso.ISOCatalogProfiles)
	// File management routes (admin).
	apiMux.HandleFunc("POST "+model.RouteAdminFileRetry, h.file.Retry)
	apiMux.HandleFunc("GET "+model.RouteFileInternal, h.adminInternal.GetFileInternal)
	apiMux.HandleFunc("GET "+model.RouteFiles, h.fileCenter.ServeFileCenter)
	// File management routes (admin).
	apiMux.HandleFunc("POST "+model.RouteAdminFileRetry, h.file.Retry)
	// Share link routes (admin).
	apiMux.HandleFunc("POST "+model.RouteShareLinkCreate, h.shareLink.Create)
	apiMux.HandleFunc("GET "+model.RouteShareLinks, h.shareLink.List)
	apiMux.HandleFunc("DELETE "+model.RouteShareLinkRevoke, h.shareLink.Revoke)
	
	apiMux.HandleFunc("POST "+model.RouteAdminFileRetry, h.file.Retry)

	// Container management routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminContainerList, h.container.HandleContainerList)
	apiMux.HandleFunc("POST "+model.RouteAdminContainerCreate, h.container.HandleContainerCreate)
	apiMux.HandleFunc("POST "+model.RouteAdminContainerStart, h.container.HandleContainerStart)
	apiMux.HandleFunc("POST "+model.RouteAdminContainerStop, h.container.HandleContainerStop)
	apiMux.HandleFunc("POST "+model.RouteAdminContainerRestart, h.container.HandleContainerRestart)
	apiMux.HandleFunc("DELETE "+model.RouteAdminContainerDelete, h.container.HandleContainerDelete)
	apiMux.HandleFunc("GET "+model.RouteAdminContainerLogs, h.container.HandleContainerLogs)
	apiMux.HandleFunc("GET "+model.RouteAdminContainerStats, h.container.HandleContainerStats)
	// Image management routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminImageList, h.container.HandleImageList)
	apiMux.HandleFunc("POST "+model.RouteAdminImagePull, h.container.HandleImagePull)
	apiMux.HandleFunc("DELETE "+model.RouteAdminImageDelete, h.container.HandleImageDelete)
	// Application template routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminTemplateList, h.appTemplate.HandleTemplateList)
	apiMux.HandleFunc("POST "+model.RouteAdminTemplateCreate, h.appTemplate.HandleTemplateCreate)
	apiMux.HandleFunc("GET "+model.RouteAdminTemplateGet, h.appTemplate.HandleTemplateGet)
	apiMux.HandleFunc("DELETE "+model.RouteAdminTemplateDelete, h.appTemplate.HandleTemplateDelete)
	// Log aggregation, task center, global search (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminLogs, h.log.HandleLogList)
	apiMux.HandleFunc("GET "+model.RouteAdminTasks, h.task.HandleTaskList)
	apiMux.HandleFunc("GET "+model.RouteAdminSearch, h.search.HandleSearch)
	// Backup management routes (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminBackupList, h.backupH.ListBackups)
	apiMux.HandleFunc("POST "+model.RouteAdminBackupRestore, h.backupH.RestoreBackup)
	// Dashboard summary (admin).
	apiMux.HandleFunc("GET "+model.RouteAdminDashboardSummary, h.dashboard.Summary)
	apiMux.HandleFunc("GET "+model.RouteAdminCacheMetrics, h.cacheMetrics.CacheMetrics)
	// Virtual index admin routes.
	apiMux.HandleFunc("POST "+model.RouteChannels, h.virtualAdmin.CreateChannel)
	apiMux.HandleFunc("GET "+model.RouteChannels, h.virtualAdmin.ListChannels)
	apiMux.HandleFunc("GET "+model.RouteChannelGet, h.virtualAdmin.GetChannel)
	apiMux.HandleFunc("PUT "+model.RouteChannelGet, h.virtualAdmin.UpdateChannel)
	apiMux.HandleFunc("DELETE "+model.RouteChannelGet, h.virtualAdmin.DeleteChannel)
	apiMux.HandleFunc("POST "+model.RouteViews, h.virtualAdmin.CreateView)
	apiMux.HandleFunc("GET "+model.RouteViews, h.virtualAdmin.ListViews)
	apiMux.HandleFunc("GET "+model.RouteViewGet, h.virtualAdmin.GetView)
	apiMux.HandleFunc("PUT "+model.RouteViewGet, h.virtualAdmin.UpdateView)
	apiMux.HandleFunc("DELETE "+model.RouteViewGet, h.virtualAdmin.DeleteView)
	apiMux.HandleFunc("POST "+model.RouteNodes, h.virtualAdmin.CreateNode)
	apiMux.HandleFunc("GET "+model.RouteViewTree, h.virtualAdmin.GetViewTree)
	apiMux.HandleFunc("PUT "+model.RouteNodeUpdate, h.virtualAdmin.UpdateNode)
	apiMux.HandleFunc("DELETE "+model.RouteNodeDelete, h.virtualAdmin.DeleteNode)
	// Tool catalog routes (admin).
	apiMux.HandleFunc("GET "+model.RouteToolCatalog, h.toolCatalog.ListCatalog)
	apiMux.HandleFunc("POST "+model.RouteToolCatalogEnable, h.toolCatalog.EnableTool)

	// Registry management routes (admin).
	if h.registry != nil {
		apiMux.HandleFunc(model.RouteRegistryList, h.registry.ListRegistries)
		apiMux.HandleFunc(model.RouteRegistryCreate, h.registry.CreateRegistry)
		apiMux.HandleFunc(model.RouteRegistryGet, h.registry.GetRegistry)
		apiMux.HandleFunc(model.RouteRegistryUpdate, h.registry.UpdateRegistry)
		apiMux.HandleFunc(model.RouteRegistryDelete, h.registry.DeleteRegistry)
		apiMux.HandleFunc(model.RouteRegistryTestConnection, h.registry.TestConnection)
		apiMux.HandleFunc(model.RouteRegistryCatalog, h.registry.BrowseCatalog)
		apiMux.HandleFunc(model.RouteRegistryTags, h.registry.BrowseTags)
		apiMux.HandleFunc(model.RouteRegistryTagDetail, h.registry.GetTagDetail)
		apiMux.HandleFunc(model.RouteRegistryTagDelete, h.registry.DeleteTag)
		apiMux.HandleFunc(model.RouteSyncCreate, h.registry.CreateSync)
		apiMux.HandleFunc(model.RouteSyncTaskList, h.registry.ListSyncTasks)
		apiMux.HandleFunc(model.RouteSyncTaskGet, h.registry.GetSyncTask)
		apiMux.HandleFunc(model.RouteSyncTaskCancel, h.registry.CancelSync)
		apiMux.HandleFunc(model.RouteRetentionList, h.registry.ListPolicies)
		apiMux.HandleFunc(model.RouteRetentionCreate, h.registry.CreatePolicy)
		apiMux.HandleFunc(model.RouteRetentionUpdate, h.registry.UpdatePolicy)
		apiMux.HandleFunc(model.RouteRetentionDelete, h.registry.DeletePolicy)
		apiMux.HandleFunc(model.RouteRetentionExecute, h.registry.ExecutePolicy)
	}

	jwtSecret := cfg.Auth.JWTSecret
	authMiddleware := middleware.AuthMiddleware(jwtSecret, cfg.Auth.GetPasswordChangedAt)
	mux.Handle("/api/v1/", authMiddleware(apiMux))

	// Create WebDAV handler with Basic Auth.
	webdavStoragePath := filepath.Join(cfg.Storage.BasePath, "webdav")
	webdavHandler := webdavpkg.NewHandler(webdavStoragePath, "/webdav")
	webdavMux := middleware.BasicAuthMiddleware(cfg.Auth.PasswordHash)(http.StripPrefix("/webdav", webdavHandler))
	// Legacy /webdav/ root redirects to the new default view; sub-paths pass through.
	mux.Handle("GET /webdav/", handler.WebDAVRedirectHandler(webdavMux))
	mux.Handle("/webdav/", webdavMux)
	mux.Handle("/webdav/", webdavMux)
	slog.Info("WebDAV enabled", "path", webdavStoragePath)

	// Serve embedded frontend for all other routes.
	webSub, err := webpkg.FS()
	if err != nil {
		slog.Error("failed to load embedded frontend", "error", err)
		os.Exit(1)
	}
	fileServer := middleware.CacheAndGzip(http.FileServer(http.FS(webSub)))
	mux.Handle("/", fileServer)

	// HTTP handler rejects WebDAV (WebDAV is HTTPS-only).
	var httpHandler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/webdav") {
			http.Error(w, "WebDAV is only available over HTTPS", http.StatusNotFound)
			return
		}
		mux.ServeHTTP(w, r)
	})

	// Wrap with JSON not-found handler, security headers and rate limiting middleware.
	httpHandler = apiNotFoundMiddleware(httpHandler)
	httpHandler = middleware.SecurityHeaders(httpHandler)
	httpHandler = middleware.RateLimit(5, time.Minute, 15*time.Minute)(httpHandler)
	httpHandler = middleware.EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)(httpHandler)

	// HTTPS handler serves everything (including WebDAV).
	var httpsHandler http.Handler = mux
	httpsHandler = apiNotFoundMiddleware(httpsHandler)
	httpsHandler = middleware.SecurityHeaders(httpsHandler)
	httpsHandler = middleware.RateLimit(5, time.Minute, 15*time.Minute)(httpsHandler)
	httpsHandler = middleware.EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)(httpsHandler)

	return httpHandler, httpsHandler
}

// runServers starts all background goroutines, HTTP/HTTPS servers, and handles
// graceful shutdown on SIGINT/SIGTERM.
func runServers(cfg *config.Config, httpHandler, httpsHandler http.Handler, svcs *appServices, database *sql.DB, quit chan os.Signal, configPath string) {
	// Start background system stats sampling.
	monitorCtx, monitorCancel := context.WithCancel(context.Background())
	defer monitorCancel()
	go func() {
		interval := time.Duration(cfg.Monitor.SampleInterval) * time.Second
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		// First sample immediately.
		handler.SampleAndStore(monitorCtx, svcs.statsRepo, cfg.Monitor.RetentionDays, cfg.Monitor.NodeExporterURL)
		for {
			select {
			case <-ticker.C:
				handler.SampleAndStore(monitorCtx, svcs.statsRepo, cfg.Monitor.RetentionDays, cfg.Monitor.NodeExporterURL)
			case <-monitorCtx.Done():
				return
			}
		}
	}()
	slog.Info("system monitor started",
		"sample_interval", cfg.Monitor.SampleInterval,
		"retention_days", cfg.Monitor.RetentionDays,
	)

	// Start HTTP server.
	addr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.Port)
	srv := &http.Server{
		Addr:    addr,
		Handler: httpHandler,
	}

	go func() {
		slog.Info("MiBeeHive server starting",
			"addr", addr,
			"projects", len(cfg.Projects),
		)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	// Start HTTPS server if configured.
	var httpsSrv *http.Server
	if cfg.Server.HTTPSPort > 0 {
		certPath := cfg.Server.CertPath
		keyPath := cfg.Server.KeyPath
		if certPath == "" || keyPath == "" {
			configDir := filepath.Dir(configPath)
			if certPath == "" {
				certPath = filepath.Join(configDir, "server.crt")
			}
			if keyPath == "" {
				keyPath = filepath.Join(configDir, "server.key")
			}
		}

		if err := middleware.EnsureTLSCert(certPath, keyPath, cfg.Server.TLSIPAddresses, cfg.Server.TLSDNSNames); err != nil {
			slog.Error("failed to generate TLS certificate", "error", err)
			os.Exit(1)
		}

		tlsCfg, err := middleware.LoadTLSConfig(certPath, keyPath)
		if err != nil {
			slog.Error("failed to load TLS config", "error", err)
			os.Exit(1)
		}

		httpsAddr := fmt.Sprintf("%s:%d", cfg.Server.BindAddr, cfg.Server.HTTPSPort)
		httpsSrv = &http.Server{
			Addr:      httpsAddr,
			Handler:   httpsHandler,
			TLSConfig: tlsCfg,
		}

		go func() {
			slog.Info("MiBeeHive HTTPS server starting", "addr", httpsAddr)
			if err := httpsSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				slog.Error("HTTPS server error", "error", err)
				os.Exit(1)
			}
		}()
	}

	// Start crawl scheduler.
	if err := svcs.crawlManager.Start(); err != nil {
		slog.Error("failed to start crawl manager", "error", err)
		os.Exit(1)
	}
	slog.Info("crawl scheduler started")

	// Start ISO catalog version checker.
	catalogCtx, catalogCancel := context.WithCancel(context.Background())
	defer catalogCancel()
	svcs.catalogService.StartVersionChecker(catalogCtx, 24*time.Hour)
	slog.Info("ISO catalog version checker started", "interval", "24h")

	// Start ISO download queue processor.
	svcs.catalogService.StartQueueProcessor(catalogCtx)
	slog.Info("ISO download queue processor started")

	// Start backup scheduler (if enabled).
	var backupCancel context.CancelFunc
	if cfg.Backup.Enabled {
		backupSvc := backup.NewBackupService(database, cfg.Database.Path, configPath, backup.Config{
			LocalPath:      cfg.Backup.LocalPath,
			Retention:      cfg.Backup.Retention,
			Schedule:       cfg.Backup.Schedule,
			RemoteURL:      cfg.Backup.RemoteURL,
			RemoteUsername: cfg.Backup.RemoteUsername,
			RemotePassword: cfg.Backup.RemotePassword,
		})
		backupCtx, bc := context.WithCancel(context.Background())
		backupCancel = bc
		go func() {
			if err := backupSvc.Start(backupCtx); err != nil {
				slog.Error("backup scheduler stopped with error", "error", err)
			}
		}()
		slog.Info("backup scheduler started", "schedule", cfg.Backup.Schedule)
	}

	// Wait for shutdown signal (from OS signal or backup restore handler).
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("received shutdown signal", "signal", sig)

	// Graceful HTTP server shutdown.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	} else {
		slog.Info("HTTP server stopped")
	}
	if httpsSrv != nil {
		if err := httpsSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("HTTPS server shutdown error", "error", err)
		} else {
			slog.Info("HTTPS server stopped")
		}
	}

	// Cancel background goroutines.
	if svcs.diskCancel != nil {
		svcs.diskCancel()
	}
	if svcs.registryCancel != nil {
		svcs.registryCancel()
	}
	if svcs.retentionScheduler != nil {
		svcs.retentionScheduler.Stop()
		slog.Info("retention scheduler stopped")
	}

	// Cancel global context to abort active downloads.
	svcs.globalCancel()

	// Wait for active downloads to finish (bounded by 10s shutdown timeout).
	downloadDone := make(chan struct{})
	go func() {
		svcs.fileService.Shutdown()
		svcs.isoService.Shutdown()
		close(downloadDone)
	}()
	select {
	case <-downloadDone:
		slog.Info("all active downloads stopped")
	case <-shutdownCtx.Done():
		slog.Warn("timed out waiting for active downloads to stop")
	}

	svcs.crawlManager.Stop()

	if svcs.dockerClient != nil {
		svcs.dockerClient.Close()
		slog.Info("docker client closed")
	}

	// Cancel remaining background goroutines.
	monitorCancel()
	catalogCancel()
	if backupCancel != nil {
		backupCancel()
	}
	svcs.retryCancel()

	slog.Info("MiBeeHive stopped")
}
