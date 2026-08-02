package model

import "encoding/json"

type LoginRequest struct {
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string `json:"token"`
}

type PasswordStatusResponse struct {
	IsDefault     bool `json:"is_default"`
	RequireChange bool `json:"require_change"`
}

type CrawlTriggerRequest struct {
	ProjectName string `json:"project_name,omitempty"`
}

type ApiResponse[T any] struct {
	Success   bool   `json:"success"`
	Message   string `json:"message,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Data      T      `json:"data"`
}

type ProjectResponse struct {
	ID            int        `json:"id"`
	Name          string     `json:"name"`
	DisplayName   string     `json:"display_name"`
	SourceType    SourceType `json:"source_type"`
	SourceURL     string     `json:"source_url"`
	LatestVersion string     `json:"latest_version"`
	LastCrawledAt *string    `json:"last_crawled_at"`
	CreatedAt     string     `json:"created_at"`
	FileCount     int        `json:"file_count"`
}

type FileResponse struct {
	ID           int64      `json:"id"`
	ProjectID    int        `json:"project_id"`
	Version      string     `json:"version"`
	Filename     string     `json:"filename"`
	OS           string     `json:"os"`
	Arch         string     `json:"arch"`
	Ext          string     `json:"ext"`
	SizeBytes    int64      `json:"size_bytes"`
	DownloadURL  string     `json:"download_url"`
	LocalPath    string     `json:"local_path"`
	Checksum     string     `json:"checksum"`
	Status       FileStatus `json:"status"`
	ErrorMessage string     `json:"error_message"`
	CreatedAt    string     `json:"created_at"`
	PublicToken  string     `json:"public_token"`
	SourceType   string     `json:"source_type"`
	Category     string     `json:"category"`
}

type SystemInfoResponse struct {
	DiskTotal    int64  `json:"disk_total"`
	DiskUsed     int64  `json:"disk_used"`
	DiskAvail    int64  `json:"disk_avail"`
	FileCount    int    `json:"file_count"`
	ProjectCount int    `json:"project_count"`
	LastCrawlAt  string `json:"last_crawl_at"`
	Version      string `json:"version"`
}

type CrawlLogResponse struct {
	ID              int64   `json:"id"`
	ProjectID       int64   `json:"project_id"`
	ProjectName     string  `json:"project_name"`
	StartedAt       string  `json:"started_at"`
	FinishedAt      *string `json:"finished_at,omitempty"`
	Status          string  `json:"status"`
	VersionsFound   int     `json:"versions_found"`
	FilesDownloaded int     `json:"files_downloaded"`
}

type SystemStatsResponse struct {
	CpuUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryTotalBytes   uint64  `json:"memory_total_bytes"`
	MemoryUsedBytes    uint64  `json:"memory_used_bytes"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	NetworkRxBytes     uint64  `json:"network_rx_bytes"`
	NetworkTxBytes     uint64  `json:"network_tx_bytes"`
}

type SystemStatsHistoryPoint struct {
	Timestamp          string  `json:"timestamp"`
	CpuUsagePercent    float64 `json:"cpu_usage_percent"`
	MemoryUsagePercent float64 `json:"memory_usage_percent"`
	NetworkRxBytes     uint64  `json:"network_rx_bytes"`
	NetworkTxBytes     uint64  `json:"network_tx_bytes"`
}

type QueueStatsResponse struct {
	Pending         int `json:"pending"`
	Downloading     int `json:"downloading"`
	Complete        int `json:"complete"`
	Error           int `json:"error"`
	FailedPermanent int `json:"failed_permanent"`
}

type DownloadProgressResponse struct {
	BytesRead int64 `json:"bytes_read"`
	Total     int64 `json:"total_bytes"`
	Percent   int   `json:"percent"`
	Speed     int64 `json:"speed"`
	ETA       int64 `json:"eta"`
}

// Admin request/response types.

type CreateProjectRequest struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	SourceType  SourceType      `json:"source_type"`
	SourceURL   string          `json:"source_url"`
	Settings    ProjectSettings `json:"settings"`
}

type UpdateProjectRequest struct {
	Name        string          `json:"name"`
	DisplayName string          `json:"display_name"`
	SourceType  SourceType      `json:"source_type"`
	SourceURL   string          `json:"source_url"`
	Settings    ProjectSettings `json:"settings"`
}

type AdminProjectResponse struct {
	ID             int64           `json:"id"`
	Name           string          `json:"name"`
	DisplayName    string          `json:"display_name"`
	SourceType     string          `json:"source_type"`
	SourceURL      string          `json:"source_url"`
	Enabled        bool            `json:"enabled"`
	LatestVersion  string          `json:"latest_version"`
	LastCrawledAt  *string         `json:"last_crawled_at,omitempty"`
	CreatedAt      string          `json:"created_at"`
	Config         json.RawMessage `json:"config"`
	FileCount      int             `json:"file_count"`
	VersionPattern string          `json:"version_pattern,omitempty"`
}
type UpsertCredentialRequest struct {
	SourceType string `json:"source_type"`
	Token      string `json:"token"`
}

type CredentialResponse struct {
	ID         int64  `json:"id"`
	SourceType string `json:"source_type"`
	Token      string `json:"token"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type ChangePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// WebDAVStatusResponse holds WebDAV service status information.
type WebDAVStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	HTTPURL     string `json:"http_url"`
	HTTPSURL    string `json:"https_url,omitempty"`
	StoragePath string `json:"storage_path"`
}

// DashboardSummaryResponse aggregates all module statistics for the dashboard.
type DashboardSummaryResponse struct {
	System   SystemModuleStats `json:"system"`
	Files    FilesModuleStats  `json:"files"`
	Deploy   DeployModuleStats `json:"deploy"`
	Share    SharedModuleStats `json:"share"`
	Activity []ActivityEvent   `json:"activity"`
}

// SystemModuleStats holds system resource usage stats.
type SystemModuleStats struct {
	Version           string  `json:"version"`
	Uptime            string  `json:"uptime"`
	CpuUsage          float64 `json:"cpu_usage_percent"`
	MemUsage          float64 `json:"memory_usage_percent"`
	MemTotal          uint64  `json:"memory_total_bytes"`
	MemUsed           uint64  `json:"memory_used_bytes"`
	DiskTotal         uint64  `json:"disk_total_bytes"`
	DiskUsed          uint64  `json:"disk_used_bytes"`
	DiskUsagePercent  float64 `json:"disk_usage_percent"`
	ContainersEnabled bool    `json:"containers_enabled"`
	ContainerCount    int     `json:"container_count"`
	ContainerRunning  int     `json:"container_running"`
}

// FilesModuleStats holds the Foraging module stats.
type FilesModuleStats struct {
	ProjectCount     int `json:"project_count"`
	TotalFiles       int `json:"total_files"`
	QueuePending     int `json:"queue_pending"`
	QueueDownloading int `json:"queue_downloading"`
	QueueComplete    int `json:"queue_complete"`
	QueueError       int `json:"queue_error"`
}

// DeployModuleStats holds the Provisioning module stats.
type DeployModuleStats struct {
	ConfigCount   int `json:"config_count"`
	IsoCount      int `json:"iso_count"`
	IsoPending    int `json:"iso_pending"`
	IsoDownloaded int `json:"iso_downloaded"`
}

// SharedModuleStats holds the Sharing module stats.
type SharedModuleStats struct {
	FileCount  int    `json:"file_count"`
	TotalSize  string `json:"total_size"`
	TotalBytes uint64 `json:"total_bytes"`
}

// ActivityEvent represents a single recent activity entry for the dashboard.
type ActivityEvent struct {
	ID        string `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Subtitle  string `json:"subtitle"`
	Timestamp string `json:"timestamp"`
}

// MonitorConfigRequest holds monitor threshold values for PUT requests.
type MonitorConfigRequest struct {
	DiskWarningPercent  int  `json:"disk_warning_percent"`
	DiskCriticalPercent int  `json:"disk_critical_percent"`
	DiskCheckEnabled    bool `json:"disk_check_enabled"`
}

// MonitorConfigResponse holds monitor threshold values for GET responses.
type MonitorConfigResponse struct {
	DiskWarningPercent  int  `json:"disk_warning_percent"`
	DiskCriticalPercent int  `json:"disk_critical_percent"`
	DiskCheckEnabled    bool `json:"disk_check_enabled"`
}

// StorageConfigResponse returns current storage path configuration.
type StorageConfigResponse struct {
	OSS               string `json:"oss"`
	OSInstall         string `json:"os_install"`
	ISO               string `json:"iso"`
	OSSFallback       string `json:"oss_fallback"`
	OSInstallFallback string `json:"os_install_fallback"`
	ISOFallback       string `json:"iso_fallback"`
}

// StorageConfigUpdateRequest updates storage module paths.
type StorageConfigUpdateRequest struct {
	OSS       *string `json:"oss,omitempty"`
	OSInstall *string `json:"os_install,omitempty"`
	ISO       *string `json:"iso,omitempty"`
}

// StorageConfigUpdateResponse returns migration task IDs triggered by the update.
type StorageConfigUpdateResponse struct {
	MigrationIDs []int64 `json:"migration_ids"`
}

// MigrationTaskResponse represents a migration task for the API.
type MigrationTaskResponse struct {
	ID            int64  `json:"id"`
	Module        string `json:"module"`
	OldPath       string `json:"old_path"`
	NewPath       string `json:"new_path"`
	Status        string `json:"status"`
	Progress      int    `json:"progress"`
	TotalFiles    int    `json:"total_files"`
	MigratedFiles int    `json:"migrated_files"`
	TotalBytes    int64  `json:"total_bytes"`
	MigratedBytes int64  `json:"migrated_bytes"`
	StartedAt     string `json:"started_at,omitempty"`
	CompletedAt   string `json:"completed_at,omitempty"`
	ErrorMessage  string `json:"error_message,omitempty"`
	CreatedAt     string `json:"created_at"`
}
