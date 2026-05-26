// Package service defines service interfaces for documentation and future testability.
// These interfaces describe the public contracts of each service without
// changing any existing handler code to use them.
package service

import (
	"context"
	"net/http"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// FileServiceContract describes the contract for file download and streaming operations.
type FileServiceContract interface {
	DownloadFile(ctx context.Context, file *model.File) error
	StreamFile(w http.ResponseWriter, file *model.File) error
	VerifyIntegrity(localPath string) error
	GetDiskUsage(basePath string) (total, used, avail int64, err error)
	CheckDiskSpace(requiredBytes int64) error
	GetActiveProgress() map[int64]*DownloadProgress
	Shutdown()
}

// ISOServiceContract describes the contract for ISO download and management operations.
type ISOServiceContract interface {
	DownloadISO(ctx context.Context, filename string, rawURL string, expectedSHA256 string) error
	ListISOs() ([]ISOInfo, error)
	DeleteISO(filename string) error
	DiskAvailable() (uint64, error)
	StreamISO(w http.ResponseWriter, filename string) error
	ResetStaleDownloads(ctx context.Context, entries []StaleISOCheck, resetFn func(id int64, status string) error) (int, error)
	GetActiveProgress() map[string]*DownloadProgress
	Shutdown()
}

// ISOCatalogServiceContract describes the contract for ISO catalog management.
type ISOCatalogServiceContract interface {
	ListCatalog(ctx context.Context) ([]model.ISOCatalogEntry, error)
	GetCatalogEntry(ctx context.Context, id int64) (*model.ISOCatalogEntry, error)
	CreateCatalogEntry(ctx context.Context, req model.ISOCatalogCreateRequest) (int64, error)
	UpdateCatalogEntry(ctx context.Context, id int64, req model.ISOCatalogUpdateRequest) error
	DeleteCatalogEntry(ctx context.Context, id int64) error
	CheckVersion(ctx context.Context, id int64) (*model.ISOCatalogCheckResponse, error)
	DownloadFromCatalog(ctx context.Context, id int64) error
	RetryCatalogDownload(ctx context.Context, id int64) error
	CancelDownload(ctx context.Context, id int64) error
	CheckAllAutoUpdate(ctx context.Context) error
	QueueDownloadAll(ctx context.Context) error
	GetQueueStats(ctx context.Context) (*model.ISOQueueStats, error)
	GetQueueList(ctx context.Context) ([]model.ISOQueueItem, error)
	StartVersionChecker(ctx context.Context, interval time.Duration)
	StartQueueProcessor(ctx context.Context)
}

// OsTemplateServiceContract describes the contract for OS template generation.
type OsTemplateServiceContract interface {
	Generate(osType string, params model.OsInstallParams) (string, error)
}

// ContainerServiceContract describes the contract for Docker container lifecycle management.
type ContainerServiceContract interface {
	List(ctx context.Context) ([]model.Container, error)
	Get(ctx context.Context, id string) (*model.Container, error)
	Create(ctx context.Context, req model.CreateContainerRequest) (*model.Container, error)
	Start(ctx context.Context, id string) error
	Stop(ctx context.Context, id string, timeout int) error
	Restart(ctx context.Context, id string, timeout int) error
	Remove(ctx context.Context, id string, force bool) error
}

// SearchServiceContract describes the contract for global search operations.
type SearchServiceContract interface {
	Search(ctx context.Context, query string, searchType string) (*model.SearchResponse, error)
	SearchPaginated(ctx context.Context, query string, searchType string, limit, offset int) (*model.SearchResponse, error)
}

// RegistryServiceContract describes the contract for container registry management.
type RegistryServiceContract interface {
	ListRegistries(ctx context.Context) ([]model.Registry, error)
	GetRegistry(ctx context.Context, id int64) (*model.Registry, error)
	CreateRegistry(ctx context.Context, req model.CreateRegistryRequest) (*model.Registry, error)
	UpdateRegistry(ctx context.Context, id int64, req model.UpdateRegistryRequest) (*model.Registry, error)
	DeleteRegistry(ctx context.Context, id int64) error
	TestConnection(ctx context.Context, id int64) (*model.TestConnectionResponse, error)
	BrowseCatalog(ctx context.Context, id int64, n int, last string) ([]string, error)
	BrowseTags(ctx context.Context, id int64, repo string, n int, last string) ([]string, error)
	GetTagDetail(ctx context.Context, id int64, repo, tag string) (*model.RegistryTag, *model.ManifestDetail, error)
	DeleteTag(ctx context.Context, id int64, repo, tag string) error
}

// SyncServiceContract describes the contract for cross-registry image synchronization.
type SyncServiceContract interface {
	CreateSync(ctx context.Context, req model.SyncRequest) (*model.SyncTask, error)
	ExecuteSync(ctx context.Context, taskID int64) error
	ListSyncTasks(ctx context.Context, status string) ([]model.SyncTask, error)
	GetSyncTask(ctx context.Context, id int64) (*model.SyncTask, error)
	CancelSync(ctx context.Context, id int64) error
}

// RetentionServiceContract describes the contract for retention policy management and execution.
type RetentionServiceContract interface {
	CreatePolicy(ctx context.Context, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error)
	UpdatePolicy(ctx context.Context, id int64, req model.CreateRetentionPolicyRequest) (*model.RetentionPolicy, error)
	DeletePolicy(ctx context.Context, id int64) error
	ListPolicies(ctx context.Context) ([]model.RetentionPolicy, error)
	GetPolicy(ctx context.Context, id int64) (*model.RetentionPolicy, error)
	ExecutePolicy(ctx context.Context, policyID int64) (int, error)
}

// Compile-time interface satisfaction checks.
var (
	_ FileServiceContract        = (*FileService)(nil)
	_ ISOServiceContract         = (*ISOService)(nil)
	_ ISOCatalogServiceContract  = (*ISOCatalogService)(nil)
	_ OsTemplateServiceContract  = (*OsTemplateService)(nil)
	_ ContainerServiceContract   = (*ContainerService)(nil)
	_ SearchServiceContract      = (*SearchService)(nil)
	_ RegistryServiceContract    = (*RegistryService)(nil)
	_ SyncServiceContract        = (*SyncService)(nil)
	_ RetentionServiceContract   = (*RetentionService)(nil)
)
