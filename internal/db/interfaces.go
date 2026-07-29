// Package db defines repository interfaces for documentation and future testability.
// These interfaces describe the public contracts of each repository without
// changing any existing handler or service code to use them.
package db

import (
	"context"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// ProjectRepository describes the contract for project persistence.
type ProjectRepository interface {
	Create(ctx context.Context, name, displayName, sourceType, sourceURL string) (*Project, error)
	GetByID(ctx context.Context, id int64) (*Project, error)
	GetByName(ctx context.Context, name string) (*Project, error)
	List(ctx context.Context) ([]*Project, error)
	ListEnabled(ctx context.Context) ([]*Project, error)
	ListAll(ctx context.Context) ([]*Project, error)
	UpdateLatestVersion(ctx context.Context, id int64, version string) error
	UpdateLastCrawledAt(ctx context.Context, id int64) error
	CreateWithSettings(ctx context.Context, name, displayName, sourceType, sourceURL string, settings model.ProjectSettings) (*Project, error)
	UpdateProject(ctx context.Context, id int64, name, displayName, sourceType, sourceURL string, settings model.ProjectSettings) error
	GetSettings(ctx context.Context, projectID int64) (*model.ProjectSettings, error)
	SetEnabled(ctx context.Context, projectID int64, enabled bool) error
	Delete(ctx context.Context, projectID int64) error
}

// FileRepository describes the contract for file persistence and queue operations.
type FileRepository interface {
	Create(ctx context.Context, f *File) (int64, error)
	GetByID(ctx context.Context, id int64) (*File, error)
	ListByProject(ctx context.Context, projectID int64) ([]*File, error)
	ListByProjectPaginated(ctx context.Context, projectID int64, limit, offset int) ([]*File, int, error)
	FindExisting(ctx context.Context, projectID int64, filename string) (*File, error)
	UpdateStatus(ctx context.Context, id int64, status, errorMsg string) error
	CountByProject(ctx context.Context, projectID int64) (int, error)
	SearchByFilename(ctx context.Context, pattern string) ([]*File, error)
	CountAll(ctx context.Context) (int, error)
	ListQueue(ctx context.Context) ([]*File, error)
	ResetZombieDownloads(ctx context.Context) (int, error)
	IncrementRetryCount(ctx context.Context, fileID int64, lastError string) (int, error)
	MarkFailedPermanent(ctx context.Context, fileID int64) error
	ResetRetry(ctx context.Context, fileID int64) error
	ListRetryable(ctx context.Context, maxRetries int) ([]*File, error)
	GetQueueStats(ctx context.Context) (*QueueStats, error)
	UpdateLocalPath(ctx context.Context, id int64, newPath string) error
}

// CrawlLogRepository describes the contract for crawl log persistence.
type CrawlLogRepository interface {
	Create(ctx context.Context, log *CrawlLog) (int64, error)
	UpdateFinished(ctx context.Context, id int64, status string, versionsFound, filesDownloaded int, errorMsg string) error
	ListByProject(ctx context.Context, projectID int64, limit int) ([]*CrawlLog, error)
	ListRecent(ctx context.Context, limit int) ([]*CrawlLog, error)
}

// SystemStatsRepository describes the contract for system stat persistence.
type SystemStatsRepository interface {
	Insert(ctx context.Context, s *SystemStat) error
	QueryHistory(ctx context.Context, since time.Time) ([]*SystemStat, error)
	PurgeOlderThan(ctx context.Context, before time.Time) (int64, error)
}

// SourceCredentialRepository describes the contract for source credential persistence.
type SourceCredentialRepository interface {
	GetBySourceType(ctx context.Context, sourceType string) (*SourceCredential, error)
	Upsert(ctx context.Context, sourceType, token string) error
	List(ctx context.Context) ([]*SourceCredential, error)
}

// OsInstallConfigRepository describes the contract for OS install config persistence.
type OsInstallConfigRepository interface {
	List(ctx context.Context) ([]*model.OsInstallConfig, error)
	ListEnabled(ctx context.Context) ([]*model.OsInstallConfig, error)
	GetByID(ctx context.Context, id int64) (*model.OsInstallConfig, error)
	GetByName(ctx context.Context, name string) (*model.OsInstallConfig, error)
	Create(ctx context.Context, name, configName, osType, config string) (*model.OsInstallConfig, error)
	Update(ctx context.Context, id int64, name, configName, osType, config string) error
	Delete(ctx context.Context, id int64) error
}

// ISOCatalogRepository describes the contract for ISO catalog and queue persistence.
type ISOCatalogRepository interface {
	List(ctx context.Context) ([]ISOCatalogDBEntry, error)
	GetByID(ctx context.Context, id int64) (*ISOCatalogDBEntry, error)
	Create(ctx context.Context, e *ISOCatalogDBEntry) (int64, error)
	Update(ctx context.Context, id int64, e *ISOCatalogDBEntry) error
	Delete(ctx context.Context, id int64) error
	ListAutoUpdate(ctx context.Context) ([]ISOCatalogDBEntry, error)
	UpdateAfterCheck(ctx context.Context, id int64, currentURL, status, lastError string) error
	ListDownloadQueue(ctx context.Context) ([]ISOCatalogDBEntry, error)
	UpdateDownloadStatus(ctx context.Context, id int64, status string) error
	ListByDownloadStatuses(ctx context.Context, statuses []string) ([]ISOCatalogDBEntry, error)
}

// RegistryRepository describes the contract for container registry persistence.
type RegistryRepository interface {
	List(ctx context.Context) ([]model.Registry, error)
	GetByID(ctx context.Context, id int64) (*model.Registry, error)
	GetByURL(ctx context.Context, url string) (*model.Registry, error)
	Create(ctx context.Context, reg *model.Registry) (int64, error)
	Update(ctx context.Context, reg *model.Registry) error
	Delete(ctx context.Context, id int64) error
	DecryptPassword(ctx context.Context, id int64) (string, error)
}

// SyncTaskRepository describes the contract for sync task persistence with status transitions.
type SyncTaskRepository interface {
	Create(ctx context.Context, task *model.SyncTask) (int64, error)
	GetByID(ctx context.Context, id int64) (*model.SyncTask, error)
	ListByStatus(ctx context.Context, status model.SyncTaskStatus) ([]model.SyncTask, error)
	GetActiveByTarget(ctx context.Context, registryID int64, repo, tag string) (*model.SyncTask, error)
	Start(ctx context.Context, id int64) error
	UpdateProgress(ctx context.Context, id int64, progressBytes, totalBytes int64) error
	Complete(ctx context.Context, id int64) error
	Fail(ctx context.Context, id int64, errMsg string) error
	Cancel(ctx context.Context, id int64) error
}

// RetentionPolicyRepository describes the contract for retention policy persistence.
type RetentionPolicyRepository interface {
	List(ctx context.Context) ([]model.RetentionPolicy, error)
	GetByID(ctx context.Context, id int64) (*model.RetentionPolicy, error)
	ListEnabled(ctx context.Context) ([]model.RetentionPolicy, error)
	Create(ctx context.Context, policy *model.RetentionPolicy) (int64, error)
	Update(ctx context.Context, policy *model.RetentionPolicy) error
	Delete(ctx context.Context, id int64) error
	UpdateLastExecuted(ctx context.Context, id int64, t time.Time) error
}

// AppTemplateRepository describes the contract for application template persistence.
type AppTemplateRepository interface {
	List(ctx context.Context) ([]model.AppTemplate, error)
	ListAll(ctx context.Context) ([]model.AppTemplate, error)
	GetByID(ctx context.Context, id int64) (*model.AppTemplate, error)
	Create(ctx context.Context, t *model.AppTemplate) error
	Delete(ctx context.Context, id int64) error
}

// Compile-time interface satisfaction checks.
var (
	_ ProjectRepository          = (*ProjectRepo)(nil)
	_ FileRepository             = (*FileRepo)(nil)
	_ CrawlLogRepository         = (*CrawlLogRepo)(nil)
	_ SystemStatsRepository      = (*SystemStatsRepo)(nil)
	_ SourceCredentialRepository = (*SourceCredentialRepo)(nil)
	_ OsInstallConfigRepository  = (*OsInstallConfigRepo)(nil)
	_ ISOCatalogRepository       = (*ISOCatalogRepo)(nil)
	_ RegistryRepository         = (*RegistryRepo)(nil)
	_ SyncTaskRepository         = (*SyncTaskRepo)(nil)
	_ RetentionPolicyRepository  = (*RetentionPolicyRepo)(nil)
	_ AppTemplateRepository      = (*AppTemplateRepo)(nil)
)
