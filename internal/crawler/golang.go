package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// goVersion represents a single Go release entry from the go.dev API.
type goVersion struct {
	Version string      `json:"version"`
	Files   []goFile    `json:"files"`
}

// goFile represents a single file within a Go release.
type goFile struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Kind     string `json:"kind"`
	Filename string `json:"filename"`
	Size     int64  `json:"size"`
	SHA256   string `json:"sha256"`
}

// GoCrawler fetches Go releases from the official go.dev downloads API.
type GoCrawler struct {
	httpClient *http.Client
	baseURL    string // overridable for testing
	logger     *slog.Logger
}

// NewGoCrawler creates a new GoCrawler.
func NewGoCrawler(logger *slog.Logger) *GoCrawler {
	return &GoCrawler{
		httpClient: SharedHTTPClient(),
		baseURL:    "https://go.dev",
		logger:     logger,
	}
}

// Name returns the human-readable name of this crawler.
func (c *GoCrawler) Name() string { return "go" }

// SourceType returns the type of source this crawler handles.
func (c *GoCrawler) SourceType() model.SourceType { return model.SourceTypeGo }

// FetchReleases fetches the latest Go release assets from go.dev.
// The owner and repo parameters are ignored — Go always fetches from go.dev.
// Only the latest stable version is returned (first entry in the API response).
func (c *GoCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	url := c.baseURL + "/dl/?mode=json&include=all"

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching go releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("go.dev API returned status %d: %s", resp.StatusCode, string(body))
	}

	var versions []goVersion
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(versions) == 0 {
		return nil, nil
	}

	// Only return the latest version (first entry).
	latest := versions[0]
	version := strings.TrimPrefix(latest.Version, "go")

	var assets []model.ReleaseAsset
	for _, f := range latest.Files {
		// Skip source archives.
		if f.Kind == "source" {
			continue
		}
		// Skip archive kind that is a source tarball.
		if f.Kind == "archive" && strings.HasSuffix(f.Filename, ".src.tar.gz") {
			continue
		}

		assets = append(assets, model.ReleaseAsset{
			Version:     version,
			Filename:    f.Filename,
			OS:          f.OS,
			Arch:        f.Arch,
			Ext:         fileExt(f.Filename),
			DownloadURL: "https://go.dev/dl/" + f.Filename,
			SizeBytes:   f.Size,
			Checksum:    f.SHA256,
		})
	}

	return assets, nil
}
