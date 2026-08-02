package db

import "time"

// Project represents a crawled project.
type Project struct {
	ID            int64
	Name          string
	DisplayName   string
	SourceType    string
	SourceURL     string
	Config        string // JSON
	LatestVersion string
	LastCrawledAt *time.Time
	CreatedAt     time.Time
	Enabled       bool
}

// File represents a downloaded release file.
type File struct {
	ID            int64
	ProjectID     int64
	Version       string
	Filename      string
	OS            string
	Arch          string
	Ext           string
	SizeBytes     int64
	DownloadURL   string
	LocalPath     string
	Checksum      string
	Status        string
	ErrorMessage  string
	CreatedAt     time.Time
	RetryCount    int
	LastAttemptAt *time.Time
	SourceType    string
	Category      string
	StorageSubdir string
	PublicToken   string
}

// CrawlLog records a crawl attempt for a project.
type CrawlLog struct {
	ID              int64
	ProjectID       int64
	StartedAt       time.Time
	FinishedAt      *time.Time
	Status          string
	VersionsFound   int
	FilesDownloaded int
	ErrorMessage    string
}

// QueueStats holds file counts grouped by status.
type QueueStats struct {
	Pending         int `json:"pending"`
	Downloading     int `json:"downloading"`
	Complete        int `json:"complete"`
	Error           int `json:"error"`
	FailedPermanent int `json:"failed_permanent"`
}

// SourceCredential stores an API token for a crawl source type.
type SourceCredential struct {
	ID         int64  `json:"id"`
	SourceType string `json:"source_type"`
	Token      string `json:"token"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// Channel represents a distribution channel in the virtual index (public,
// internal, token).
type Channel struct {
	ID          int64
	Slug        string
	Name        string
	AuthMode    string
	Description string
	CreatedAt   time.Time
}

// View represents a logical directory tree within a channel.
type View struct {
	ID        int64
	Slug      string
	Name      string
	ChannelID int64
	Mode      string
	Writable  bool
	SortOrder int
	CreatedAt time.Time
}

// Node represents a node in the virtual tree (folder, file reference, or
// rule folder). Status is 'visible' or 'hidden' (soft-deleted).
type Node struct {
	ID         int64
	ViewID     int64
	ParentID   *int64
	Name       string
	NodeType   string
	FileID     *int64
	RuleConfig *string
	SortOrder  int
	Status     string
	CreatedAt  time.Time
}

// FileSummaryDTO is a lightweight file representation for the file center API.
// Excludes sensitive paths (local_path, storage_subdir) but includes public_token.
type FileSummaryDTO struct {
	ID          int64  `json:"id"`
	ProjectID   int64  `json:"project_id"`
	Version     string `json:"version"`
	Filename    string `json:"filename"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	SizeBytes   int64  `json:"size_bytes"`
	SourceType  string `json:"source_type"`
	Category    string `json:"category"`
	PublicToken string `json:"public_token"`
	Status      string `json:"status"`
	DownloadURL string `json:"download_url"`
	Checksum    string `json:"checksum"`
	CreatedAt   string `json:"created_at"`
}
