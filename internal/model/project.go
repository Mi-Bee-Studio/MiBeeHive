package model

type SourceType string

const (
    SourceTypeGitHub    SourceType = "github"
    SourceTypeGo        SourceType = "go"
    SourceTypeHashiCorp SourceType = "hashicorp"
	SourceTypeGrafana   SourceType = "grafana"
	SourceTypeNPM       SourceType = "npm"
	SourceTypePyPI      SourceType = "pypi"
	SourceTypeCrates    SourceType = "crates"
)

type Project struct {
    ID            int        `json:"id"`
    Name          string     `json:"name"`
    DisplayName   string     `json:"display_name"`
    SourceType    SourceType `json:"source_type"`
    SourceURL     string     `json:"source_url"`
    LatestVersion string     `json:"latest_version"`
    LastCrawledAt *string    `json:"last_crawled_at"`
    CreatedAt     string     `json:"created_at"`
}

// ProjectSettings stores per-project configuration in the projects.config JSON column.
type ProjectSettings struct {
	CrawlInterval    int      `json:"crawl_interval,omitempty"`
	GitHubOwner      string   `json:"github_owner,omitempty"`
	GitHubRepo       string   `json:"github_repo,omitempty"`
	FilterPatterns   []string `json:"filter_patterns,omitempty"`
	StorageSubpath   string   `json:"storage_subpath,omitempty"`
	DownloadAll      bool     `json:"download_all_assets,omitempty"`
	VersionPattern string `json:"version_pattern,omitempty"`
}
