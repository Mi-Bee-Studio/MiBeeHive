// Package rulesrc implements a declarative "rule fingerprint" crawl engine.
//
// This is a prototype (Issue #1) validating whether a YAML fingerprint can
// describe single-page sources (JSON APIs, HTML listing pages, static dirs)
// well enough to replace per-source Go code. It is intentionally minimal:
// enough to cover the validation sources, not pre-designed beyond them.
//
// The output type is the existing model.ReleaseAsset, so collected artifacts
// flow into the unchanged download/store pipeline. See REPORT.md for findings.
package rulesrc

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Format is the response format a source returns.
type Format string

const (
	FormatJSON Format = "json" // structured JSON: GitHub API, Crates.io, NPM, etc.
	FormatHTML Format = "html" // HTML listing: Apache/nginx autoindex, static dirs
)

// Spec is a single source fingerprint. One YAML file = one source.
type Spec struct {
	// APIVersion pins the fingerprint schema version.
	APIVersion string `yaml:"apiVersion"`
	// Kind is always "Source".
	Kind string `yaml:"kind"`
	// Name identifies the source (maps to a project name).
	Name string `yaml:"name"`

	Request Request `yaml:"request"`
	List    List    `yaml:"list"`
	Extract Extract `yaml:"extract"`
	Filter  Filter  `yaml:"filter,omitempty"`
}

// Request describes how to fetch one page.
type Request struct {
	URL    string  `yaml:"url"`
	Format Format  `yaml:"format"`
	Header []Field `yaml:"header,omitempty"` // optional request headers, e.g. Authorization
}

// List describes how to find the iteration units in the response.
// For JSON: Path is a dot-path to the array (e.g. "[]" or "versions[]" or ".assets[]").
// For HTML: Selector is a CSS-like tag/class selector; only tag names + class are
// supported by the minimal picker (e.g. "a", "a.file", "tr.file a").
type List struct {
	Path     string   `yaml:"path,omitempty"`     // JSON array path
	Selector string   `yaml:"selector,omitempty"` // HTML selector
	Skip     []string `yaml:"skip,omitempty"`     // truthy JSON paths to skip an item (e.g. ".prerelease")
}

// Extract describes how to pull fields out of each iteration unit.
type Extract struct {
	// For JSON: per-field dot-paths. For HTML: each maps to an attribute/text.
	Version     string `yaml:"version,omitempty"`
	Filename    string `yaml:"filename,omitempty"`
	DownloadURL string `yaml:"download_url,omitempty"`
	Size        string `yaml:"size,omitempty"`
	Checksum    string `yaml:"checksum,omitempty"`

	// Assets is used when assets nest inside a release (GitHub Releases:
	// each release has a version and an []assets). When set, the engine
	// iterates the assets array inside each unit and pairs the unit's
	// Version with each asset.
	Assets *AssetExtract `yaml:"assets,omitempty"`

	// Classify, when "filename-parser", runs os/arch/ext detection on the
	// extracted filename (mirrors internal/crawler.parseFilename).
	Classify string `yaml:"classify,omitempty"`

	// VersionStrip, when non-empty, strips this prefix from the extracted version
	// (e.g. "v" turns "v3.11.3" into "3.11.3").
	VersionStrip string `yaml:"version_strip,omitempty"`

	// --- HTML-only extraction controls ---
	// Attr names the attribute to read for Filename/DownloadURL when format is html
	// (defaults to "href"). When set, DownloadURL is read from this attribute.
	Attr string `yaml:"attr,omitempty"`
	// Basename, when true, strips the directory from an extracted href for Filename.
	Basename bool `yaml:"basename,omitempty"`
	// Resolve, when "url", resolves a relative href against the request URL.
	Resolve string `yaml:"resolve,omitempty"`
}

// AssetExtract describes a nested assets array (GitHub Releases style).
type AssetExtract struct {
	Path        string `yaml:"path"`           // JSON array path inside the unit, e.g. ".assets[]"
	Filename    string `yaml:"filename"`       // dot-path within each asset
	DownloadURL string `yaml:"download_url"`   // dot-path within each asset
	Size        string `yaml:"size,omitempty"` // dot-path within each asset
	Checksum    string `yaml:"checksum,omitempty"`
}

// Filter optionally restricts which extracted items are kept.
type Filter struct {
	Include []string     `yaml:"include,omitempty"` // glob patterns the filename must match (any)
	Exclude []string     `yaml:"exclude,omitempty"` // glob patterns to drop
	Version VersionRegex `yaml:"version,omitempty"` // regex extracting version from filename
}

// VersionRegex extracts a version from a filename via one capture group.
type VersionRegex struct {
	Regex string `yaml:"regex"`
}

// Field is a simple key/value used for headers.
type Field struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// LoadSpec reads and parses a fingerprint YAML file.
func LoadSpec(path string) (*Spec, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read spec %s: %w", path, err)
	}
	return ParseSpec(data)
}

// ParseSpec parses fingerprint YAML bytes.
func ParseSpec(data []byte) (*Spec, error) {
	var s Spec
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse spec yaml: %w", err)
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func (s *Spec) validate() error {
	if s.Name == "" {
		return fmt.Errorf("spec: name is required")
	}
	if s.Request.URL == "" {
		return fmt.Errorf("spec %q: request.url is required", s.Name)
	}
	switch s.Request.Format {
	case FormatJSON, FormatHTML:
	default:
		return fmt.Errorf("spec %q: request.format must be %q or %q", s.Name, FormatJSON, FormatHTML)
	}
	if s.Request.Format == FormatJSON && s.List.Path == "" {
		return fmt.Errorf("spec %q: list.path is required for json format", s.Name)
	}
	if s.Request.Format == FormatHTML && s.List.Selector == "" {
		return fmt.Errorf("spec %q: list.selector is required for html format", s.Name)
	}
	return nil
}
