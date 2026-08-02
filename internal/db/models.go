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
