package crawler

import (
	"regexp"
	"strings"
)

// semverFromFilename extracts a semver-like version (X.Y.Z or X.Y) from a filename.
var semverFromFilename = regexp.MustCompile(`(\d+\.\d+(?:\.\d+)?(?:-(?:alpha|beta|rc|dev|pre|patch|build|post)\.?[a-zA-Z0-9.]*)?)`)

// ExtractVersionFromFilename extracts a version string from a filename.
// Returns empty string if no version found.
func ExtractVersionFromFilename(filename string) string {
	// Strip 'go' prefix if filename starts with 'go' followed by a digit
	stripped := filename
	if strings.HasPrefix(filename, "go") && len(filename) > 2 && filename[2] >= '0' && filename[2] <= '9' {
		stripped = filename[2:]
	}

	// Find first semver-like match
	matches := semverFromFilename.FindStringSubmatch(stripped)
	if len(matches) < 2 {
		return ""
	}
	version := matches[1]

	// A valid version must have at least one dot
	if !strings.Contains(version, ".") {
		return ""
	}

	return version
}

// ParseVersionGroup returns a grouping key like "1.22.x" from a version string.
// pattern: "semver" (default), "gover" (strip 'go' prefix first), or "" (auto).
func ParseVersionGroup(version, pattern string) string {
	if version == "" {
		return ""
	}

	v := version
	if pattern == "gover" {
		v = strings.TrimPrefix(v, "go")
	}

	// Handle pre-release suffix: "1.9.0-rc1" → take "1.9.0" part
	if idx := strings.Index(v, "-"); idx > 0 {
		v = v[:idx]
	}

	parts := strings.SplitN(v, ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1] + ".x"
}

// VersionGroupKey is a convenience wrapper using default semver pattern.
func VersionGroupKey(version string) string {
	return ParseVersionGroup(version, "semver")
}

// SourceTypeVersionPattern returns the default version pattern for a source type.
func SourceTypeVersionPattern(sourceType string) string {
	switch sourceType {
	case "go":
		return "gover"
	case "hashicorp", "grafana", "npm", "pypi", "crates", "github":
		return "semver"
	default:
		return ""
	}
}
