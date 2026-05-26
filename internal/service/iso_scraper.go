package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"sort"
	"time"
)

// linkRegex matches href attributes in HTML anchor tags.
var linkRegex = regexp.MustCompile(`(?i)<a[^>]+href="([^"]+)"`)

// ScrapeLatestISO fetches a directory listing page at checkURL, finds all links matching
// the filenamePattern regex, and returns the URL of the latest matching ISO file.
// Returns empty string with nil error if no match is found.
func ScrapeLatestISO(ctx context.Context, checkURL string, filenamePattern string) (string, error) {
	re, err := regexp.Compile(filenamePattern)
	if err != nil {
		return "", fmt.Errorf("invalid filename pattern: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, checkURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", "MiBeeHive/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching directory listing: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching %s: HTTP %d", checkURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	if err != nil {
		return "", fmt.Errorf("reading response body: %w", err)
	}

	base, err := url.Parse(checkURL)
	if err != nil {
		return "", fmt.Errorf("parsing check URL: %w", err)
	}

	matches := linkRegex.FindAllStringSubmatch(string(body), -1)
	type fileMatch struct {
		filename string
		fullURL  string
	}
	var files []fileMatch
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		href := m[1]
		if !re.MatchString(href) {
			continue
		}
		linkURL, err := url.Parse(href)
		if err != nil {
			continue
		}
		resolved := base.ResolveReference(linkURL)
		fn := path.Base(resolved.Path)
		files = append(files, fileMatch{filename: fn, fullURL: resolved.String()})
	}

	if len(files) == 0 {
		return "", nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].filename < files[j].filename
	})
	return files[len(files)-1].fullURL, nil
}
