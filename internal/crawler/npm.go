package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// npmRegistryResponse represents the response from the npm registry API.
type npmRegistryResponse struct {
	Name     string                     `json:"name"`
	Versions map[string]npmVersionEntry `json:"versions"`
}

// npmVersionEntry represents a single version entry in the npm registry response.
type npmVersionEntry struct {
	Dist npmDist `json:"dist"`
}

// npmDist contains download metadata for an npm package version.
type npmDist struct {
	Tarball  string `json:"tarball"`
	Size     int64  `json:"size,omitempty"`
	Checksum string `json:"shasum,omitempty"`
}

// NPMCrawler fetches releases from the npm registry API.
type NPMCrawler struct {
	httpClient  *http.Client
	baseURL     string // overridable for testing
	logger      *slog.Logger
	token       string
	maxVersions int // maximum number of versions to return (0 = default 5)
}

// NewNPMCrawler creates a new NPMCrawler.
// The token parameter is optional (used for private registries).
func NewNPMCrawler(token string, logger *slog.Logger) *NPMCrawler {
	return &NPMCrawler{
		httpClient:  SharedHTTPClient(),
		baseURL:     "https://registry.npmjs.org",
		logger:      logger,
		token:       token,
		maxVersions: 5,
	}
}

// Name returns the human-readable name of this crawler.
func (c *NPMCrawler) Name() string { return "npm" }

// SourceType returns the type of source this crawler handles.
func (c *NPMCrawler) SourceType() model.SourceType { return model.SourceTypeNPM }

// FetchReleases fetches npm package versions from the registry.
// For scoped packages, owner is the scope (e.g. "@types") and repo is the package name (e.g. "node").
// For unscoped packages, owner is ignored and repo is the package name.
func (c *NPMCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	var packageName string
	if owner != "" {
		// Scoped package: @scope/name
		packageName = owner + "/" + repo
	} else {
		packageName = repo
	}

	url := c.baseURL + "/" + packageName

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating npm request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching npm package %s: %w", packageName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm registry returned status %d for %s: %s", resp.StatusCode, packageName, string(body))
	}

	var registryResp npmRegistryResponse
	if err := json.NewDecoder(resp.Body).Decode(&registryResp); err != nil {
		return nil, fmt.Errorf("decoding npm registry response: %w", err)
	}

	if len(registryResp.Versions) == 0 {
		return nil, nil
	}

	// Collect stable versions (skip pre-release with "-")
	var stableVersions []string
	for ver := range registryResp.Versions {
		if strings.Contains(ver, "-") {
			continue
		}
		stableVersions = append(stableVersions, ver)
	}

	if len(stableVersions) == 0 {
		return nil, nil
	}

	// Sort versions descending (simple lexicographic — semver-compatible for standard versions)
	sort.Sort(sort.Reverse(sort.StringSlice(stableVersions)))

	// Limit to maxVersions
	limit := c.maxVersions
	if limit <= 0 {
		limit = 5
	}
	if len(stableVersions) > limit {
		stableVersions = stableVersions[:limit]
	}

	var assets []model.ReleaseAsset
	for _, ver := range stableVersions {
		entry, ok := registryResp.Versions[ver]
		if !ok {
			continue
		}

		// Extract filename from tarball URL
		tarball := entry.Dist.Tarball
		filename := tarballFilename(tarball)

		assets = append(assets, model.ReleaseAsset{
			Version:     ver,
			Filename:    filename,
			OS:          "", // NPM packages are platform-independent
			Arch:        "",
			Ext:         ".tgz",
			DownloadURL: tarball,
			SizeBytes:   entry.Dist.Size,
			Checksum:    entry.Dist.Checksum,
		})
	}

	return assets, nil
}

// tarballFilename extracts the filename portion from a tarball URL.
// e.g. "https://registry.npmjs.org/express/-/express-4.18.2.tgz" → "express-4.18.2.tgz"
func tarballFilename(url string) string {
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}
