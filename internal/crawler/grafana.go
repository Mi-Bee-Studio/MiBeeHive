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

// GrafanaCrawler fetches the latest Grafana release from GitHub.
// It uses the GitHub Releases API as its primary source, falling back
// to constructing download URLs from the dl.grafana.com pattern.
type GrafanaCrawler struct {
	httpClient *http.Client
	token      string // optional GitHub token for higher rate limits
	baseURL    string // API base URL, overridable for testing
	logger     *slog.Logger
}

// NewGrafanaCrawler creates a new GrafanaCrawler.
func NewGrafanaCrawler(logger *slog.Logger) *GrafanaCrawler {
	return &GrafanaCrawler{
		httpClient: SharedHTTPClient(),
		baseURL:    "https://api.github.com",
		logger:     logger,
	}
}

// Name returns the human-readable name of this crawler.
func (c *GrafanaCrawler) Name() string { return "grafana" }

// SourceType returns the type of source this crawler handles.
func (c *GrafanaCrawler) SourceType() model.SourceType { return model.SourceTypeGrafana }

// FetchReleases fetches the latest Grafana release assets.
// The owner and repo parameters are ignored — the crawler always targets grafana/grafana.
// Only the latest stable release is returned.
func (c *GrafanaCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	url := fmt.Sprintf("%s/repos/grafana/grafana/releases/latest", c.baseURL)

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
		return nil, fmt.Errorf("fetching grafana release: %w", err)
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

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if release.Prerelease || release.Draft {
		return nil, nil
	}

	version := strings.TrimPrefix(release.TagName, "v")

	var assets []model.ReleaseAsset
	for _, a := range release.Assets {
		parsed := parseFilename(a.Name)
		parsed.Version = version
		parsed.Filename = a.Name
		parsed.DownloadURL = a.BrowserDownloadURL
		parsed.SizeBytes = a.Size
		assets = append(assets, parsed)
	}

	return assets, nil
}
