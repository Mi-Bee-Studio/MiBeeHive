package service

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"
)

// linkRegex matches href attributes in HTML anchor tags.
var linkRegex = regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"`)

// ScrapeLatestISO discovers the latest ISO file URL by scraping HTML directory listings.
//
// Two-level scraping when versionDirPattern is non-empty:
//  1. HTTP GET baseURL → extract links matching versionDirPattern regex
//  2. Parse version numbers using ParseVersion, sort with CompareVersion, pick latest
//  3. Construct URL: baseURL + replaced isoPathTemplate ({version} and {arch} placeholders)
//  4. HTTP GET constructed URL → extract links matching filenamePattern
//  5. Sort ISO files by version and pick latest → return full URL
//
// Single-level fallback when versionDirPattern is empty:
//  1. Construct URL: baseURL + replaced isoPathTemplate ({arch} placeholder only)
//  2. HTTP GET → extract links matching filenamePattern
//  3. Sort and pick latest → return full URL
//
// Returns empty string with nil error if no match is found.
func ScrapeLatestISO(ctx context.Context, baseURL, versionDirPattern, isoPathTemplate, filenamePattern, arch string) (string, error) {
	fileRe, err := regexp.Compile(filenamePattern)
	if err != nil {
		return "", fmt.Errorf("invalid filename pattern: %w", err)
	}

	mirrorArch := MirrorArch(arch)

	if versionDirPattern != "" {
		return scrapeTwoLevel(ctx, baseURL, versionDirPattern, isoPathTemplate, fileRe, mirrorArch)
	}
	return scrapeSingleLevel(ctx, baseURL, isoPathTemplate, fileRe, mirrorArch)
}

// scrapeTwoLevel performs two-level scraping: discover version directories, then find ISO files.
func scrapeTwoLevel(ctx context.Context, baseURL, versionDirPattern string, isoPathTemplate string, fileRe *regexp.Regexp, mirrorArch string) (string, error) {
	dirRe, err := regexp.Compile(versionDirPattern)
	if err != nil {
		return "", fmt.Errorf("invalid version directory pattern: %w", err)
	}

	body, err := fetchPage(ctx, baseURL)
	if err != nil {
		return "", fmt.Errorf("fetching version directory listing: %w", err)
	}

	// Extract version directories from the base URL listing.
	matches := linkRegex.FindAllStringSubmatch(string(body), -1)
	type versionDir struct {
		dirName string
		version []int
	}
	var dirs []versionDir
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		href := m[1]
		if !dirRe.MatchString(href) {
			continue
		}
		// Strip trailing slash for version extraction.
		clean := strings.TrimSuffix(href, "/")
		v := ParseVersion(clean)
		if v == nil {
			continue
		}
		dirs = append(dirs, versionDir{dirName: clean, version: v})
	}

	if len(dirs) == 0 {
		return "", nil
	}

	// Sort dirs by version descending (latest first).
	sort.Slice(dirs, func(i, j int) bool {
		return CompareVersion(dirs[i].version, dirs[j].version) > 0
	})

	// Try each version directory from latest to oldest.
	// This handles cases where the latest version dir exists but has no
	// ISO files yet (e.g., Alpine v3.24 has no releases/ subdir).
	for _, d := range dirs {
		isoURL := joinURL(baseURL, replacePlaceholders(isoPathTemplate, d.dirName, mirrorArch))
		result, err := scrapeISOFiles(ctx, isoURL, fileRe)
		if err == nil && result != "" {
			return result, nil
		}
		if err != nil {
			slog.Debug("version dir has no ISO files, trying next", "dir", d.dirName, "error", err)
		}
	}
	return "", nil
}

// scrapeSingleLevel performs single-level scraping: directly find ISO files.
func scrapeSingleLevel(ctx context.Context, baseURL, isoPathTemplate string, fileRe *regexp.Regexp, mirrorArch string) (string, error) {
	isoURL := joinURL(baseURL, replacePlaceholders(isoPathTemplate, "", mirrorArch))
	return scrapeISOFiles(ctx, isoURL, fileRe)
}

// scrapeISOFiles fetches a directory listing and returns the latest matching ISO file URL.
func scrapeISOFiles(ctx context.Context, listingURL string, fileRe *regexp.Regexp) (string, error) {
	body, err := fetchPage(ctx, listingURL)
	if err != nil {
		return "", fmt.Errorf("fetching ISO listing: %w", err)
	}

	base, err := url.Parse(listingURL)
	if err != nil {
		return "", fmt.Errorf("parsing listing URL: %w", err)
	}

	matches := linkRegex.FindAllStringSubmatch(string(body), -1)
	type fileMatch struct {
		filename string
		version  []int
		fullURL  string
	}
	var files []fileMatch
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		href := m[1]
		if !fileRe.MatchString(href) {
			continue
		}
		linkURL, err := url.Parse(href)
		if err != nil {
			continue
		}
		resolved := base.ResolveReference(linkURL)
		fn := path.Base(resolved.Path)
		v := ParseVersion(fn)
		files = append(files, fileMatch{filename: fn, version: v, fullURL: resolved.String()})
	}

	if len(files) == 0 {
		return "", nil
	}

	// Sort by parsed version using CompareVersion. Fall back to string comparison for nil versions.
	for i := 1; i < len(files); i++ {
		for j := i; j > 0; j-- {
			var less bool
			vj := files[j].version
			vjPrev := files[j-1].version
			if vj != nil && vjPrev != nil {
				less = CompareVersion(vj, vjPrev) < 0
			} else if vj != nil {
				less = false
			} else if vjPrev != nil {
				less = true
			} else {
				less = files[j].filename < files[j-1].filename
			}
			if !less {
				break
			}
			files[j], files[j-1] = files[j-1], files[j]
		}
	}

	return files[len(files)-1].fullURL, nil
}

// fetchPage performs an HTTP GET with retry and returns the body.
func fetchPage(ctx context.Context, pageURL string) ([]byte, error) {
	const maxRetries = 3
	const browserUA = "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt) * 2 * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
		client := &http.Client{Timeout: 30 * time.Second}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		req.Header.Set("User-Agent", browserUA)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("fetching %s: %w", pageURL, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("fetching %s: HTTP %d", pageURL, resp.StatusCode)
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("reading response body: %w", err)
		}
		return body, nil
	}
	return nil, lastErr
}

// replacePlaceholders replaces {version} and {arch} placeholders in the template.
func replacePlaceholders(template, version, arch string) string {
	s := template
	if version != "" {
		s = strings.ReplaceAll(s, "{version}", version)
	}
	s = strings.ReplaceAll(s, "{arch}", arch)
	return s
}
// joinURL joins a base URL with a relative path, ensuring proper separation.
func joinURL(base, rel string) string {
	if rel == "" {
		return base
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return base + rel
	}
	relURL, err := url.Parse(rel)
	if err != nil {
		return base + rel
	}
	return baseURL.ResolveReference(relURL).String()
}
