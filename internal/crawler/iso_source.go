package crawler

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"golang.org/x/net/html"
)

// ISOSource fetches ISO files from official distribution sites.
type ISOSource struct {
	httpClient *http.Client
	logger     *slog.Logger
}

// NewISOSource creates a new ISOSource.
func NewISOSource(logger *slog.Logger) *ISOSource {
	return &ISOSource{
		httpClient: SharedHTTPClient(),
		logger:     logger,
	}
}

// Name returns the human-readable name of this crawler.
func (c *ISOSource) Name() string {
	return "iso"
}

// SourceType returns the type of source this crawler handles.
func (c *ISOSource) SourceType() model.SourceType {
	return model.SourceType("iso")
}

// FetchReleases fetches ISO files from the given URL.
// The owner and repo parameters are ignored - only the URL (as repo param) is used.
// It parses the HTML page and extracts links to .iso files.
func (c *ISOSource) FetchReleases(ctx context.Context, owner, url string) ([]model.ReleaseAsset, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching ISO source page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ISO source page returned status %d", resp.StatusCode)
	}

	// Parse HTML and extract .iso links
	var assets []model.ReleaseAsset
	tokenizer := html.NewTokenizer(resp.Body)

	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if err := tokenizer.Err(); err != io.EOF {
				return assets, fmt.Errorf("parsing HTML: %w", err)
			}
			return assets, nil
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			if token.Data == "a" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						href := attr.Val
						// Filter for .iso files only
						if strings.HasSuffix(strings.ToLower(href), ".iso") {
							filename := href
							// Handle relative URLs by constructing full URL
							downloadURL := href
							if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") {
								// Absolute URL - use as is
								downloadURL = href
							} else {
								// Relative URL - construct from base URL
								baseURL := url
								if idx := strings.LastIndex(url, "/"); idx >= 0 {
									baseURL = url[:idx+1]
								}
								downloadURL = baseURL + href
							}

							// Extract version from filename (simple extraction before first .iso)
							version := extractVersionFromFilename(filename)

							// Extract OS and arch from filename
							osVal, archVal := parseISOFilename(filename)

							assets = append(assets, model.ReleaseAsset{
								Version:     version,
								Filename:    filename,
								OS:          osVal,
								Arch:        archVal,
								Ext:         "iso",
								DownloadURL: downloadURL,
								SizeBytes:   0, // Size unknown until download
								Checksum:    "",
							})
						}
					}
				}
			}
		}
	}
}

// extractVersionFromFilename extracts a version string from an ISO filename.
// This is a simple heuristic that looks for version-like patterns.
func extractVersionFromFilename(filename string) string {
	// Remove .iso suffix
	base := strings.TrimSuffix(filename, ".iso")

	// Try to find version pattern (X.Y.Z or similar)
	// For ISOs, we'll use the base filename as version since patterns vary widely
	// This can be enhanced later with more sophisticated parsing
	return base
}

// parseISOFilename extracts OS and architecture from an ISO filename.
func parseISOFilename(filename string) (osVal, archVal string) {
	lower := strings.ToLower(filename)

	// Detect OS
	if strings.Contains(lower, "ubuntu") {
		osVal = "ubuntu"
	} else if strings.Contains(lower, "debian") {
		osVal = "debian"
	} else if strings.Contains(lower, "centos") || strings.Contains(lower, "rhel") || strings.Contains(lower, "rocky") || strings.Contains(lower, "almalinux") {
		osVal = "centos"
	} else if strings.Contains(lower, "fedora") {
		osVal = "fedora"
	} else if strings.Contains(lower, "alpine") {
		osVal = "alpine"
	} else if strings.Contains(lower, "arch") {
		osVal = "arch"
	}

	// Detect architecture
	if strings.Contains(lower, "amd64") || strings.Contains(lower, "x86_64") {
		archVal = "amd64"
	} else if strings.Contains(lower, "arm64") || strings.Contains(lower, "aarch64") {
		archVal = "arm64"
	} else if strings.Contains(lower, "i386") || strings.Contains(lower, "i686") {
		archVal = "386"
	} else if strings.Contains(lower, "armhf") || strings.Contains(lower, "armv7") {
		archVal = "armv7"
	}

	return osVal, archVal
}