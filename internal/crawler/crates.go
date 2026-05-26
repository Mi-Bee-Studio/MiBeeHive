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

// crates.io API response types.

// cratesResponse represents the top-level response from the crates.io API.
// See: https://crates.io/api/v1/crates/{crate}
type cratesResponse struct {
	Crate    cratesCrate     `json:"crate"`
	Versions []crateVersion  `json:"versions"`
}

// cratesCrate contains crate-level metadata.
type cratesCrate struct {
	Name          string `json:"name"`
	Repository    string `json:"repository"`
	NewestVersion string `json:"newest_version"`
}

// crateVersion represents a single version entry in the crates.io response.
type crateVersion struct {
	Num     string `json:"num"`
	Yanked  bool   `json:"yanked"`
	Created string `json:"created_at"`
}

// CratesCrawler fetches releases from the crates.io API.
type CratesCrawler struct {
	httpClient    *http.Client
	baseURL       string // overridable for testing
	githubBaseURL string // GitHub API base URL, overridable for testing
	logger        *slog.Logger
	token         string
	maxVersions   int // maximum number of versions to return (0 = default 5)
}

// NewCratesCrawler creates a new CratesCrawler.
// The token parameter is optional.
// crates.io REQUIRES a User-Agent header — the crawler sets it on every request.
func NewCratesCrawler(token string, logger *slog.Logger) *CratesCrawler {
	return &CratesCrawler{
		httpClient:    SharedHTTPClient(),
		baseURL:       "https://crates.io/api/v1",
		githubBaseURL: "https://api.github.com",
		logger:        logger,
		token:         token,
		maxVersions:   5,
	}
}

// Name returns the human-readable name of this crawler.
func (c *CratesCrawler) Name() string { return "crates" }

// SourceType returns the type of source this crawler handles.
func (c *CratesCrawler) SourceType() model.SourceType { return model.SourceTypeCrates }

// FetchReleases fetches crate versions from crates.io.
// The owner parameter is ignored. The repo parameter is the crate name.
// Auto-adapt strategy:
//  1. Fetch crate info from crates.io API → get repository field (GitHub URL)
//  2. If repository is a GitHub URL → fetch GitHub Releases for pre-compiled binaries
//  3. Otherwise → fall back to .crate source download URLs from crates.io
//
// Pre-release versions are included (0.x versions are common in the Rust ecosystem).
// Yanked versions are excluded.
func (c *CratesCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	crateName := repo

	apiURL := c.baseURL + "/crates/" + crateName

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating crates.io request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching crate %s: %w", crateName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("crates.io returned status %d for %s: %s", resp.StatusCode, crateName, string(body))
	}

	var cratesResp cratesResponse
	if err := json.NewDecoder(resp.Body).Decode(&cratesResp); err != nil {
		return nil, fmt.Errorf("decoding crates.io response: %w", err)
	}

	if len(cratesResp.Versions) == 0 {
		return nil, nil
	}

	// Filter out yanked versions, keep all others (including pre-releases).
	var versions []crateVersion
	for _, v := range cratesResp.Versions {
		if v.Yanked {
			continue
		}
		versions = append(versions, v)
	}

	if len(versions) == 0 {
		return nil, nil
	}

	// Sort versions descending (simple lexicographic — semver-compatible for standard versions).
	sort.SliceStable(versions, func(i, j int) bool {
		return versions[i].Num > versions[j].Num
	})

	// Limit to maxVersions.
	limit := c.maxVersions
	if limit <= 0 {
		limit = 5
	}
	if len(versions) > limit {
		versions = versions[:limit]
	}

	// Auto-adapt: if the crate has a GitHub repository, try fetching binary releases.
	if ghOwner, ghRepo := parseGitHubRepoURL(cratesResp.Crate.Repository); ghOwner != "" {
		ghAssets, err := c.fetchGitHubBinaries(ctx, ghOwner, ghRepo)
		if err == nil && len(ghAssets) > 0 {
			return ghAssets, nil
		}
		// Fall through to source .crate downloads if GitHub fetch fails or has no assets.
	}

	// Fallback: return .crate source download URLs.
	var assets []model.ReleaseAsset
	for _, v := range versions {
		downloadURL := fmt.Sprintf("https://crates.io/api/v1/crates/%s/%s/download", crateName, v.Num)
		filename := crateName + "-" + v.Num + ".crate"
		assets = append(assets, model.ReleaseAsset{
			Version:     v.Num,
			Filename:    filename,
			OS:          "",
			Arch:        "",
			Ext:         ".crate",
			DownloadURL: downloadURL,
			SizeBytes:   0,
			Checksum:    "",
		})
	}

	return assets, nil
}

// fetchGitHubBinaries attempts to fetch pre-compiled binary assets from GitHub Releases.
func (c *CratesCrawler) fetchGitHubBinaries(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	githubAPIURL := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", c.githubBaseURL, owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubAPIURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating github request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "token "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching github releases for %s/%s: %w", owner, repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding github releases: %w", err)
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

// parseGitHubRepoURL extracts the owner and repo from a GitHub repository URL.
// Returns empty strings if the URL is not a recognized GitHub URL.
// Supports: https://github.com/owner/repo, git+https://github.com/owner/repo.git, etc.
func parseGitHubRepoURL(rawURL string) (owner, repo string) {
	if rawURL == "" {
		return "", ""
	}

	// Strip common prefixes.
	s := rawURL
	s = strings.TrimPrefix(s, "git+")
	s = strings.TrimPrefix(s, "git://")
	s = strings.TrimPrefix(s, "https://")
	s = strings.TrimPrefix(s, "http://")
	s = strings.TrimPrefix(s, "ssh://git@")
	s = strings.TrimPrefix(s, "git@github.com:")

	// Must start with github.com.
	if !strings.HasPrefix(s, "github.com/") {
		return "", ""
	}
	s = strings.TrimPrefix(s, "github.com/")

	// Strip trailing .git, /, and any query/hash fragments.
	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimRight(s, "/")
	if idx := strings.IndexAny(s, "?#"); idx >= 0 {
		s = s[:idx]
	}

	parts := strings.SplitN(s, "/", 3)
	if len(parts) < 2 {
		return "", ""
	}
	return parts[0], parts[1]
}
