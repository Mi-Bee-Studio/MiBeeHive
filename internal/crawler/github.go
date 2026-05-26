package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// GitHub API response types.

type githubRelease struct {
	TagName    string         `json:"tag_name"`
	Prerelease bool           `json:"prerelease"`
	Draft      bool           `json:"draft"`
	Assets     []githubAsset  `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

// GitHubCrawler fetches releases from the GitHub Releases API.
type GitHubCrawler struct {
	httpClient *http.Client
	token      string // optional GitHub token for higher rate limits
	baseURL    string // API base URL, overridable for testing
	logger     *slog.Logger
}

// NewGitHubCrawler creates a new GitHubCrawler.
// If token is non-empty, it is used as a Bearer token for API requests
// (raises rate limit from 60/hr to 5000/hr).
func NewGitHubCrawler(token string, logger *slog.Logger) *GitHubCrawler {
	return &GitHubCrawler {
		httpClient: SharedHTTPClient(),
		token:      token,
		baseURL:    "https://api.github.com",
		logger:     logger,
	}
}

// Name returns the human-readable name of this crawler.
func (c *GitHubCrawler) Name() string { return "github" }

// SourceType returns the type of source this crawler handles.
func (c *GitHubCrawler) SourceType() model.SourceType { return model.SourceTypeGitHub }

// FetchReleases fetches the latest release assets from GitHub for the given owner/repo.
// It skips pre-releases and draft releases, returning only stable release assets.
func (c *GitHubCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", c.baseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching releases: %w", err)
	}
	defer resp.Body.Close()

	// Check rate limit headers.
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		if val, err := strconv.Atoi(remaining); err == nil && val < 5 {
			return nil, fmt.Errorf("rate_limited: GitHub API rate limit near exhaustion (%d remaining): %w", val, ErrRateLimited)
		}
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	var assets []model.ReleaseAsset
	for _, r := range releases {
		if r.Prerelease || r.Draft {
			continue
		}
		version := strings.TrimPrefix(r.TagName, "v")
		for _, a := range r.Assets {
			parsed := parseFilename(a.Name)
			parsed.Version = version
			parsed.Filename = a.Name
			parsed.DownloadURL = a.BrowserDownloadURL
			parsed.SizeBytes = a.Size
			assets = append(assets, parsed)
		}
	}

	return assets, nil
}

// knownOS is the set of recognized operating system identifiers.
var knownOS = map[string]bool{
	"linux":   true,
	"darwin":  true,
	"windows": true,
	"freebsd": true,
}

// knownArch is the set of recognized architecture identifiers.
var knownArch = map[string]bool{
	"amd64":   true,
	"arm64":   true,
	"armv6":   true,
	"armv7":   true,
	"386":     true,
	"s390x":   true,
	"ppc64le": true,
}

// parseFilename extracts os, arch, and ext from a release asset filename.
//
// Supported patterns:
//
//	prometheus-3.11.3.linux-arm64.tar.gz       → linux, arm64, tar.gz
//	victoria-metrics-darwin-amd64-v1.142.0.tar.gz → darwin, amd64, tar.gz
//	consul_1.22.5_linux_arm64.zip               → linux, arm64, zip
//	node_exporter-1.9.0.linux-amd64.tar.gz      → linux, amd64, tar.gz
//	blackbox_exporter-0.25.0.linux-arm64.tar.gz → linux, arm64, tar.gz
func parseFilename(name string) model.ReleaseAsset {
	// Remove extension first.
	ext := fileExt(name)
	base := name
	if ext != "" {
		base = strings.TrimSuffix(name, "."+ext)
	}

	// Split by common delimiters.
	parts := splitFilename(base)

	var osVal, archVal string
	for _, p := range parts {
		p = strings.ToLower(p)
		if knownOS[p] && osVal == "" {
			osVal = p
			continue
		}
		if knownArch[p] && archVal == "" {
			archVal = p
		}
	}

	return model.ReleaseAsset{
		OS:   osVal,
		Arch: archVal,
		Ext:  ext,
	}
}

// fileExt returns the file extension, handling compound extensions like .tar.gz.
func fileExt(name string) string {
	for _, ext := range []string{".tar.gz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(name, ext) {
			return ext[1:] // strip leading dot
		}
	}
	if idx := strings.LastIndex(name, "."); idx >= 0 {
		return name[idx+1:]
	}
	return ""
}

// splitFilename splits a filename by common delimiters (- and _).
func splitFilename(name string) []string {
	// Normalize: replace underscores and dots with hyphens for consistent splitting.
	// Dots appear in version strings like "3.11.3.linux" where OS follows a version.
	name = strings.ReplaceAll(name, "_", "-")
	name = strings.ReplaceAll(name, ".", "-")
	return strings.Split(name, "-")
}
