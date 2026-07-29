package rulesrc

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// Fetcher turns a Spec into a slice of ReleaseAssets. It is the engine core.
// HTTP is abstracted behind httpGetter so tests can inject fixtures offline.
type Fetcher struct {
	client httpGetter
}

// httpGetter fetches the body and final URL for a request. The final URL is
// needed to resolve relative hrefs in HTML listings.
type httpGetter interface {
	Get(ctx context.Context, req Request) (body io.ReadCloser, finalURL string, err error)
}

// defaultGetter uses net/http.
type defaultGetter struct{ hc *http.Client }

func (d defaultGetter) Get(ctx context.Context, req Request) (io.ReadCloser, string, error) {
	hr, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, http.NoBody)
	if err != nil {
		return nil, req.URL, err
	}
	hr.Header.Set("User-Agent", "MiBeeHive-rulesrc/1.0")
	for _, h := range req.Header {
		hr.Header.Set(h.Name, h.Value)
	}
	resp, err := d.hc.Do(hr)
	if err != nil {
		return nil, req.URL, err
	}
	if resp.StatusCode >= 400 {
		resp.Body.Close()
		return nil, req.URL, fmt.Errorf("source %s: HTTP %d", req.URL, resp.StatusCode)
	}
	return resp.Body, resp.Request.URL.String(), nil
}

// NewFetcher builds a Fetcher using the default http.Client.
func NewFetcher() *Fetcher { return &Fetcher{client: defaultGetter{hc: http.DefaultClient}} }

// newFetcherWith allows tests to inject a stub getter.
func newFetcherWith(g httpGetter) *Fetcher { return &Fetcher{client: g} }

// Fetch resolves a Spec into ReleaseAssets. The request URL is used as-is
// (no template substitution). For parametric sources use FetchWithParams.
func (f *Fetcher) Fetch(ctx context.Context, spec *Spec) ([]model.ReleaseAsset, error) {
	return f.FetchWithParams(ctx, spec, nil)
}

// FetchWithParams resolves a Spec into ReleaseAssets, substituting {key}
// placeholders in the request URL with values from params. E.g. a spec URL of
// "https://api.github.com/repos/{owner}/{repo}/releases" with params
// {"owner":"x","repo":"y"} fetches ".../repos/x/y/releases". Unknown
// placeholders are left intact. This lets one fingerprint serve many sources.
func (f *Fetcher) FetchWithParams(ctx context.Context, spec *Spec, params map[string]string) ([]model.ReleaseAsset, error) {
	req := spec.Request
	if len(params) > 0 {
		req.URL = interpolate(req.URL, params)
	}
	body, finalURL, err := f.client.Get(ctx, req)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	switch spec.Request.Format {
	case FormatJSON:
		return parseJSONSource(body, spec)
	case FormatHTML:
		return parseHTMLSource(body, spec, finalURL)
	}
	return nil, fmt.Errorf("unsupported format %q", spec.Request.Format)
}

// interpolate replaces {key} tokens in s with params[key], if present.
func interpolate(s string, params map[string]string) string {
	r := strings.NewReplacer(tokenize(params)...)
	return r.Replace(s)
}

// tokenize flattens params into {"{k1}", v1, "{k2}", v2, ...} for strings.Replacer.
func tokenize(params map[string]string) []string {
	out := make([]string, 0, len(params)*2)
	for k, v := range params {
		out = append(out, "{"+k+"}", v)
	}
	return out
}

// parseJSONSource handles GitHub-style and structured-JSON sources.
func parseJSONSource(r io.Reader, spec *Spec) ([]model.ReleaseAsset, error) {
	var root jsonNode
	if err := jsonDecode(r, &root); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	var out []model.ReleaseAsset
	// Iterate the top-level list units (e.g. each release).
	if err := iterArray(root, spec.List.Path, func(unit jsonNode) error {
		for _, skip := range spec.List.Skip {
			if truthy(unit, skip) {
				return nil // skip prerelease/draft
			}
		}
		version := asString(mustGet(unit, spec.Extract.Version))

		if spec.Extract.Assets != nil {
			// Nested assets (GitHub Releases): pair unit version with each asset.
			return iterArray(unit, spec.Extract.Assets.Path, func(asset jsonNode) error {
				a := buildAsset(asset, spec.Extract.Assets.Filename, spec.Extract.Assets.DownloadURL, spec.Extract.Assets.Size, spec.Extract.Assets.Checksum, version)
				out = append(out, finalizeAsset(a, spec))
				return nil
			})
		}
		// Flat structure (Crates.io versions[]): each unit is its own asset.
		a := buildAsset(unit, spec.Extract.Filename, spec.Extract.DownloadURL, spec.Extract.Size, spec.Extract.Checksum, version)
		out = append(out, finalizeAsset(a, spec))
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// parseHTMLSource handles directory-listing / static-dir sources.
func parseHTMLSource(r io.Reader, spec *Spec, baseURL string) ([]model.ReleaseAsset, error) {
	items, err := pickHTML(r, spec.List.Selector, spec.Extract.Attr)
	if err != nil {
		return nil, err
	}
	base, _ := url.Parse(baseURL)
	var out []model.ReleaseAsset
	for _, it := range items {
		href := it.attr
		if href == "" || strings.HasPrefix(href, "?") || href == baseURL {
			continue
		}
		filename := href
		if spec.Extract.Basename {
			filename = path.Base(strings.TrimRight(href, "/"))
		}
		dl := href
		if spec.Extract.Resolve == "url" && base != nil {
			if ref, err := base.Parse(href); err == nil {
				dl = ref.String()
			}
		}
		a := model.ReleaseAsset{Filename: filename, DownloadURL: dl}
		if spec.Extract.Version == "" && spec.Filter.Version.Regex != "" {
			a.Version = matchVersion(filename, spec.Filter.Version.Regex)
		} else {
			a.Version = it.text
		}
		out = append(out, finalizeAsset(a, spec))
	}
	return out, nil
}

// buildAsset reads scalar fields by dot-path from a JSON object.
func buildAsset(node jsonNode, fnamePath, urlPath, sizePath, checksumPath, version string) model.ReleaseAsset {
	a := model.ReleaseAsset{Version: version}
	a.Filename = asString(mustGet(node, fnamePath))
	a.DownloadURL = asString(mustGet(node, urlPath))
	a.SizeBytes = asInt(mustGet(node, sizePath))
	a.Checksum = asString(mustGet(node, checksumPath))
	return a
}

// finalizeAsset applies classify + version-strip to an extracted asset.
func finalizeAsset(a model.ReleaseAsset, spec *Spec) model.ReleaseAsset {
	if spec.Extract.VersionStrip != "" {
		a.Version = strings.TrimPrefix(a.Version, spec.Extract.VersionStrip)
	}
	if spec.Extract.Classify == "filename-parser" {
		c := classify(a.Filename)
		a.OS, a.Arch, a.Ext = c.os, c.arch, c.ext
	}
	return a
}

// keepFilter reports whether an asset passes the include/exclude globs.
func keepFilter(a model.ReleaseAsset, spec *Spec) bool {
	f := spec.Filter
	if len(f.Include) > 0 && !matchAny(a.Filename, f.Include) {
		return false
	}
	if len(f.Exclude) > 0 && matchAny(a.Filename, f.Exclude) {
		return false
	}
	return true
}

// ApplyFilter drops assets that do not pass the spec's include/exclude rules.
func ApplyFilter(assets []model.ReleaseAsset, spec *Spec) []model.ReleaseAsset {
	var out []model.ReleaseAsset
	for _, a := range assets {
		if keepFilter(a, spec) {
			out = append(out, a)
		}
	}
	return out
}

// matchAny reports whether name matches any glob pattern (case-insensitive).
func matchAny(name string, patterns []string) bool {
	for _, p := range patterns {
		ok, _ := path.Match(strings.ToLower(p), strings.ToLower(name))
		if ok {
			return true
		}
	}
	return false
}

// matchVersion extracts the first regex capture group from s.
func matchVersion(s, pattern string) string {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return ""
	}
	m := re.FindStringSubmatch(s)
	if len(m) >= 2 {
		return m[1]
	}
	if len(m) == 1 {
		return m[0]
	}
	return ""
}

// mustGet is getPath that returns nil (not an error) when missing.
func mustGet(node jsonNode, p string) jsonNode {
	v, _ := getPath(node, p)
	return v
}
