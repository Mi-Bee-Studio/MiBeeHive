package model

type CrawlStatus string

const (
	CrawlStatusRunning       CrawlStatus = "running"
	CrawlStatusSuccess       CrawlStatus = "success"
	CrawlStatusError         CrawlStatus = "error"
	CrawlStatusRateLimited   CrawlStatus = "rate_limited"
	CrawlStatusNetworkError  CrawlStatus = "network_error" // transient fetch failure after retries (timeout, reset, 5xx)
)

type CrawlLog struct {
	ID              int64       `json:"id"`
	ProjectID       int         `json:"project_id"`
	StartedAt       string      `json:"started_at"`
	FinishedAt      *string     `json:"finished_at"`
	Status          CrawlStatus `json:"status"`
	VersionsFound   int         `json:"versions_found"`
	FilesDownloaded int         `json:"files_downloaded"`
	ErrorMessage    string      `json:"error_message"`
}

type ReleaseAsset struct {
	Version     string
	Filename    string
	OS          string
	Arch        string
	Ext         string
	DownloadURL string
	SizeBytes   int64
	Checksum    string
}

type CrawlResult struct {
	ProjectName     string
	Status          CrawlStatus
	VersionsFound   int
	FilesDownloaded int
	NewAssets       []ReleaseAsset
	Error           error
}
