package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// HashiCorp Releases API response types.

type hashiCorpRelease struct {
	Version string          `json:"version"`
	Builds  []hashiCorpBuild `json:"builds"`
}

type hashiCorpBuild struct {
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	URL      string `json:"url"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

// HashiCorpCrawler fetches releases from the HashiCorp Releases API.
type HashiCorpCrawler struct {
	httpClient *http.Client
	apiToken   string // optional API token for authenticated requests
	baseURL    string // overridable for testing
	logger     *slog.Logger
}

// NewHashiCorpCrawler creates a new HashiCorpCrawler.
// If token is non-empty, it is used as a Bearer token for API requests.
func NewHashiCorpCrawler(token string, logger *slog.Logger) *HashiCorpCrawler {
	return &HashiCorpCrawler{
		httpClient: SharedHTTPClient(),
		apiToken:   token,
		baseURL:    "https://api.releases.hashicorp.com",
		logger:     logger,
	}
}

// Name returns the human-readable name of this crawler.
func (c *HashiCorpCrawler) Name() string { return "hashicorp" }

// SourceType returns the type of source this crawler handles.
func (c *HashiCorpCrawler) SourceType() model.SourceType { return model.SourceTypeHashiCorp }

// FetchReleases fetches the latest release assets from HashiCorp for the given product.
// The owner parameter is the product name (e.g. "consul", "packer", "vagrant", "nomad").
// The repo parameter is ignored.
// Only the latest version (first entry in the API response) is returned.
func (c *HashiCorpCrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	// An empty product name produces /v1/releases/ which the API rejects with
	// a misleading 403 ("authorization failure") — fail fast with a clear
	// config error instead (issue #60).
	if owner == "" {
		return nil, fmt.Errorf("hashicorp product name is empty — set the project's owner to the product (e.g. consul, packer) or fix its source URL")
	}

	url := fmt.Sprintf("%s/v1/releases/%s", c.baseURL, owner)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if c.apiToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiToken)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching hashicorp releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		if c.logger != nil {
			c.logger.Warn("HashiCorp API returned 403 — API token may be required",
				"product", owner,
				"has_token", c.apiToken != "",
				"body", string(body),
			)
		}
		if c.apiToken == "" {
			return nil, fmt.Errorf("HashiCorp API token required for %s — configure in Settings > API Tokens", owner)
		}
		return nil, fmt.Errorf("HashiCorp API returned 403 for %s (token may be invalid or revoked): %s", owner, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("hashicorp API returned status %d for %s: %s", resp.StatusCode, owner, string(body))
	}

	var releases []hashiCorpRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}

	if len(releases) == 0 {
		return nil, nil
	}

	// Only return the latest version (first entry).
	latest := releases[0]

	var assets []model.ReleaseAsset
	for _, b := range latest.Builds {
		filename := b.Filename
		if filename == "" {
			filename = path.Base(b.URL)
		}
		parsed := parseFilename(filename)
		parsed.Version = latest.Version
		parsed.Filename = filename
		parsed.DownloadURL = b.URL
		parsed.Checksum = b.SHA256
		parsed.OS = b.OS
		parsed.Arch = b.Arch
		assets = append(assets, parsed)
	}

	return assets, nil
}
