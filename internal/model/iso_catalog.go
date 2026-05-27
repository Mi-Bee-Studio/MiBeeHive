package model

// ISOCatalogEntry represents a catalog entry for auto-discoverable ISO images.
type ISOCatalogEntry struct {
	ID                 int    `json:"id"`
	Name               string `json:"name"`
	Distro             string `json:"distro"`
	Variant            string `json:"variant"`
	Arch               string `json:"arch"`
	CheckURL           string `json:"check_url"`
	FilenamePattern    string `json:"filename_pattern"`
	BaseURL            string `json:"base_url"`
	VersionDirPattern  string `json:"version_dir_pattern"`
	ISOPathTemplate    string `json:"iso_path_template"`
	CurrentURL         string `json:"current_url"`
	AutoUpdate         bool   `json:"auto_update"`
	CheckIntervalHours int    `json:"check_interval_hours"`
	LastChecked        string `json:"last_checked"`
	LastError          string `json:"last_error"`
	Status             string `json:"status"`
	DownloadStatus      string `json:"download_status"`
SHA256             string `json:"sha256"`
	CreatedAt          string `json:"created_at"`
	UpdatedAt          string `json:"updated_at"`
}

// ISOCatalogCreateRequest is the request body for creating a catalog entry.
type ISOCatalogCreateRequest struct {
	Name               string `json:"name"`
	Distro             string `json:"distro"`
	Variant            string `json:"variant"`
	Arch               string `json:"arch"`
	CheckURL           string `json:"check_url"`
	FilenamePattern    string `json:"filename_pattern"`
	BaseURL            string `json:"base_url"`
	VersionDirPattern  string `json:"version_dir_pattern"`
	ISOPathTemplate    string `json:"iso_path_template"`
	AutoUpdate         bool   `json:"auto_update"`
	CheckIntervalHours int    `json:"check_interval_hours"`
SHA256             string `json:"sha256"`
}

// ISOCatalogUpdateRequest is the request body for updating a catalog entry.
// Pointer fields allow distinguishing "not provided" from "set to zero value".
type ISOCatalogUpdateRequest struct {
	Name               *string `json:"name,omitempty"`
	Distro             *string `json:"distro,omitempty"`
	Variant            *string `json:"variant,omitempty"`
	Arch               *string `json:"arch,omitempty"`
	CheckURL           *string `json:"check_url,omitempty"`
	FilenamePattern    *string `json:"filename_pattern,omitempty"`
	BaseURL            *string `json:"base_url,omitempty"`
	VersionDirPattern  *string `json:"version_dir_pattern,omitempty"`
	ISOPathTemplate    *string `json:"iso_path_template,omitempty"`
	AutoUpdate         *bool   `json:"auto_update,omitempty"`
	CheckIntervalHours *int    `json:"check_interval_hours,omitempty"`
SHA256             *string `json:"sha256,omitempty"`
}

// ISOCatalogCheckResponse is the response for a version check.
type ISOCatalogCheckResponse struct {
	FoundURL string `json:"found_url"`
	Status   string `json:"status"`
}

// ISOQueueStats holds counts for ISO download queue states.
type ISOQueueStats struct {
	Pending     int `json:"pending"`
	Downloading int `json:"downloading"`
	Downloaded  int `json:"downloaded"`
	Available   int `json:"available"`
	Error       int `json:"error"`
	Total       int `json:"total"`
}

// ISOQueueItem represents an entry in the ISO download queue.
type ISOQueueItem struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Distro         string `json:"distro"`
	Arch           string `json:"arch"`
	CurrentURL     string `json:"current_url"`
	DownloadStatus string `json:"download_status"`
	Status         string `json:"status"`
}
