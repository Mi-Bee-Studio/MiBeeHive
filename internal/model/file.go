package model

type FileStatus string

const (
	FileStatusPending         FileStatus = "pending"
	FileStatusDownloading     FileStatus = "downloading"
	FileStatusComplete        FileStatus = "complete"
	FileStatusError           FileStatus = "error"
	FileStatusImported        FileStatus = "imported"
	FileStatusFailedPermanent FileStatus = "failed_permanent"
)

type File struct {
	ID            int64      `json:"id"`
	ProjectID     int        `json:"project_id"`
	Version       string     `json:"version"`
	Filename      string     `json:"filename"`
	OS            string     `json:"os"`
	Arch          string     `json:"arch"`
	Ext           string     `json:"ext"`
	SizeBytes     int64      `json:"size_bytes"`
	DownloadURL   string     `json:"download_url"`
	LocalPath     string     `json:"local_path"`
	Checksum      string     `json:"checksum"`
	Status        FileStatus `json:"status"`
	ErrorMessage  string     `json:"error_message"`
	CreatedAt     string     `json:"created_at"`
	RetryCount    int        `json:"retry_count"`
	LastAttemptAt *string    `json:"last_attempt_at,omitempty"`
}

type FileFilter struct {
	ProjectID int    `json:"project_id,omitempty"`
	Version   string `json:"version,omitempty"`
	OS        string `json:"os,omitempty"`
	Arch      string `json:"arch,omitempty"`
	Query     string `json:"query,omitempty"`
}
