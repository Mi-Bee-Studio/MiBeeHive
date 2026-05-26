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

// PyPI JSON API response types.

// pypiAPIResponse mirrors the PyPI JSON API response structure.
// See: https://warehouse.pypi.org/api-reference/json/
type pypiAPIResponse struct {
	Info     pypiInfo                    `json:"info"`
	Releases map[string][]pypiFileEntry  `json:"releases"`
}

// pypiInfo contains package metadata.
type pypiInfo struct {
	Version string `json:"version"`
	Name    string `json:"name"`
}

// pypiFileEntry represents a single file in PyPI's releases map.
type pypiFileEntry struct {
	Filename      string `json:"filename"`
	URL           string `json:"url"`
	Size          int64  `json:"size"`
	PackageType   string `json:"packagetype"`
	PythonVersion string `json:"python_version"`
	SHA256Digest  string `json:"sha256_digest,omitempty"`
	MD5Digest     string `json:"md5_digest,omitempty"`
}

// PyPICrawler fetches releases from the PyPI JSON API.
type PyPICrawler struct {
	httpClient  *http.Client
	token       string // optional API token for authenticated requests
	baseURL     string // overridable for testing
	logger      *slog.Logger
	maxVersions int // maximum number of versions to return
}

// NewPyPICrawler creates a new PyPICrawler.
// The token parameter is optional (used for private PyPI registries).
func NewPyPICrawler(token string, logger *slog.Logger) *PyPICrawler {
	return &PyPICrawler{
		httpClient:  SharedHTTPClient(),
		token:       token,
		baseURL:     "https://pypi.org",
		logger:      logger,
		maxVersions: 5,
	}
}

// Name returns the human-readable name of this crawler.
func (c *PyPICrawler) Name() string { return "pypi" }

// SourceType returns the type of source this crawler handles.
func (c *PyPICrawler) SourceType() model.SourceType { return model.SourceTypePyPI }

// FetchReleases fetches the latest release assets from PyPI for the given package.
// The owner parameter is ignored. The repo parameter is the package name.
// It filters pre-release versions, prefers platform-independent wheels,
// and returns assets for up to maxVersions (default 5) latest stable versions.
func (c *PyPICrawler) FetchReleases(ctx context.Context, owner, repo string) ([]model.ReleaseAsset, error) {
	url := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating pypi request: %w", err)
	}
	req.Header.Set("User-Agent", UserAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching pypi package %s: %w", repo, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("pypi API returned status %d for %s: %s", resp.StatusCode, repo, string(body))
	}

	var apiResp pypiAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("decoding pypi response: %w", err)
	}

	if len(apiResp.Releases) == 0 {
		return nil, nil
	}

	// Collect stable versions (skip pre-releases).
	var stableVersions []string
	for ver := range apiResp.Releases {
		if isPyPIPreRelease(ver) {
			continue
		}
		stableVersions = append(stableVersions, ver)
	}

	if len(stableVersions) == 0 {
		return nil, nil
	}

	// Sort versions using PEP 440-inspired comparison (simplified: reverse lexicographic).
	sort.Sort(sort.Reverse(sort.StringSlice(stableVersions)))

	// Limit to maxVersions.
	limit := c.maxVersions
	if limit <= 0 {
		limit = 5
	}
	if len(stableVersions) > limit {
		stableVersions = stableVersions[:limit]
	}

	var assets []model.ReleaseAsset
	for _, ver := range stableVersions {
		files := apiResp.Releases[ver]
		if len(files) == 0 {
			continue
		}

		// Sort files: prefer bdist_wheel over sdist.
		sort.SliceStable(files, func(i, j int) bool {
			return pypiFileTypePriority(files[i].PackageType) < pypiFileTypePriority(files[j].PackageType)
		})

		for _, f := range files {
			if !c.shouldIncludeFile(f) {
				continue
			}

			parsed := pypiParseFile(f)
			parsed.Version = ver
			assets = append(assets, parsed)
		}
	}

	return assets, nil
}

// shouldIncludeFile determines if a PyPI file entry should be included in results.
// It skips platform-specific wheels that don't match linux/arm64, and includes
// all platform-independent wheels and sdist files.
func (c *PyPICrawler) shouldIncludeFile(f pypiFileEntry) bool {
	// Always include sdist.
	if f.PackageType == "sdist" {
		return true
	}

	// For wheels, check platform compatibility.
	if f.PackageType == "bdist_wheel" {
		platform := pypiExtractPlatform(f.Filename)
		// Platform "any" is always compatible.
		if platform == "any" {
			return true
		}
		// Check for linux aarch64/arm64 compatibility.
		return isLinuxARM64Platform(platform)
	}

	// Include other types (bdist_egg, etc.) by default.
	return true
}

// pypiParseFile converts a PyPI file entry into a ReleaseAsset.
func pypiParseFile(f pypiFileEntry) model.ReleaseAsset {
	osVal, archVal := pypiParseWheelPlatform(f.Filename)

	checksum := f.SHA256Digest
	if checksum == "" {
		checksum = f.MD5Digest
	}

	return model.ReleaseAsset{
		Filename:     f.Filename,
		OS:           osVal,
		Arch:         archVal,
		Ext:          pypiFileExt(f.Filename),
		DownloadURL:  f.URL,
		SizeBytes:    f.Size,
		Checksum:     checksum,
	}
}

// pypiFileTypePriority returns a sort priority for package types.
// Lower value = higher priority (appears first).
func pypiFileTypePriority(pkgType string) int {
	switch pkgType {
	case "bdist_wheel":
		return 0
	case "sdist":
		return 1
	default:
		return 2
	}
}

// isPyPIPreRelease checks if a version string indicates a pre-release.
// Pre-releases contain "a", "b", or "rc" followed by a digit.
func isPyPIPreRelease(version string) bool {
	lower := strings.ToLower(version)
	// Match patterns like "1.0.0a1", "1.0.0b2", "1.0.0rc1"
	// Also match "1.0.0.alpha", "1.0.0.beta", "1.0.0.rc"
	for _, suffix := range []string{"a", "b", "rc"} {
		// Check for suffix followed by digit (e.g., "rc1", "a0")
		for i := 0; i < len(lower); i++ {
			if strings.HasPrefix(lower[i:], suffix) {
				rest := lower[i+len(suffix):]
				if rest == "" || (len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9') {
					return true
				}
				// Also match dot-separated: "1.0.alpha", "1.0.rc"
				if len(rest) > 0 && rest[0] == '.' {
					return true
				}
			}
		}
	}
	return false
}

// pypiExtractPlatform extracts the platform tag from a wheel filename.
// Wheel format: {name}-{version}(-{build})?-{python}-{abi}-{platform}.whl
// Returns "any" for platform-independent wheels.
func pypiExtractPlatform(filename string) string {
	if !strings.HasSuffix(filename, ".whl") {
		return ""
	}
	base := strings.TrimSuffix(filename, ".whl")
	parts := strings.Split(base, "-")
	if len(parts) < 5 {
		return ""
	}
	// The last part is the platform tag (could be compound like "manylinux_2_17_aarch64.manylinux2014_aarch64")
	return parts[len(parts)-1]
}

// pypiParseWheelPlatform extracts OS and arch from a wheel filename.
// Returns ("any", "any") for platform-independent wheels.
// Returns ("linux", "arm64") for linux aarch64 wheels.
func pypiParseWheelPlatform(filename string) (string, string) {
	if !strings.HasSuffix(filename, ".whl") {
		// sdist — no platform info.
		return "", ""
	}

	platform := pypiExtractPlatform(filename)
	if platform == "" {
		return "", ""
	}

	if platform == "any" {
		return "any", "any"
	}

	// Parse compound platform tags (dot-separated).
	// e.g., "manylinux_2_17_aarch64.manylinux2014_aarch64"
	osVal, archVal := "", ""
	platforms := strings.Split(platform, ".")
	for _, p := range platforms {
		lower := strings.ToLower(p)
		// Detect OS.
		if strings.Contains(lower, "linux") && osVal == "" {
			osVal = "linux"
		} else if strings.Contains(lower, "macos") || strings.Contains(lower, "darwin") {
			if osVal == "" {
				osVal = "darwin"
			}
		} else if strings.Contains(lower, "win") {
			if osVal == "" {
				osVal = "windows"
			}
		}
		// Detect arch.
		if strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64") {
			if archVal == "" {
				archVal = "arm64"
			}
		} else if strings.Contains(lower, "x86_64") || strings.Contains(lower, "amd64") {
			if archVal == "" {
				archVal = "amd64"
			}
		} else if strings.Contains(lower, "i686") || strings.Contains(lower, "i386") || strings.Contains(lower, "x86") {
			if archVal == "" {
				archVal = "386"
			}
		}
	}

	return osVal, archVal
}

// isLinuxARM64Platform checks if a platform tag is compatible with linux/arm64.
func isLinuxARM64Platform(platform string) bool {
	lower := strings.ToLower(platform)
	return strings.Contains(lower, "linux") &&
		(strings.Contains(lower, "aarch64") || strings.Contains(lower, "arm64"))
}

// pypiFileExt returns the file extension, handling compound extensions like .tar.gz.
func pypiFileExt(filename string) string {
	if strings.HasSuffix(filename, ".tar.gz") {
		return "tar.gz"
	}
	if strings.HasSuffix(filename, ".tar.bz2") {
		return "tar.bz2"
	}
	if strings.HasSuffix(filename, ".tar.xz") {
		return "tar.xz"
	}
	if idx := strings.LastIndex(filename, "."); idx >= 0 {
		return filename[idx+1:]
	}
	return ""
}
