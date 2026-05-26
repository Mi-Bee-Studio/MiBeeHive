package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

// Tags lists tags for a repository, returning a single page.
//
// n controls page size (0 for server default). last is the tag name to start
// after (for pagination). If the returned slice has fewer items than n, there
// are no more pages.
func (c *RegistryClient) Tags(ctx context.Context, repo string, n int, last string) ([]string, error) {
	result, err := c.fetchTagsPage(ctx, repo, n, last)
	if err != nil {
		return nil, err
	}
	return result.tags, nil
}

// TagsWithPagination fetches ALL tags for a repository by following pagination.
func (c *RegistryClient) TagsWithPagination(ctx context.Context, repo string) ([]string, error) {
	var allTags []string
	const pageSize = 100
	last := ""

	for {
		result, err := c.fetchTagsPage(ctx, repo, pageSize, last)
		if err != nil {
			return nil, err
		}
		allTags = append(allTags, result.tags...)

		if !result.hasMore || len(result.tags) == 0 {
			break
		}

		// Use nextLast from Link header, or fall back to last tag name.
		if result.nextLast != "" {
			last = result.nextLast
		} else {
			last = result.tags[len(result.tags)-1]
		}
	}

	return allTags, nil
}

// tagsPageResult holds the result of a single tags list page request.
type tagsPageResult struct {
	tags    []string
	hasMore bool
	nextLast string
}

// fetchTagsPage fetches one page of tags and returns pagination metadata.
func (c *RegistryClient) fetchTagsPage(ctx context.Context, repo string, n int, last string) (*tagsPageResult, error) {
	path := fmt.Sprintf("/v2/%s/tags/list", repo)
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create tags request: %w", err)
	}

	q := req.URL.Query()
	if n > 0 {
		q.Set("n", strconv.Itoa(n))
	}
	if last != "" {
		q.Set("last", last)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tags request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, statusCodeError(resp)
	}

	var body struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode tags response: %w", err)
	}

	// Determine pagination from Link header.
	hasMore := false
	nextLast := ""
	if link := resp.Header.Get("Link"); link != "" {
		if nextURL, ok := parseLinkNext(link); ok {
			hasMore = true
			// Extract last parameter from the Link URL.
			if idx := strings.Index(nextURL, "last="); idx != -1 {
				rem := nextURL[idx+5:]
				if ampIdx := strings.Index(rem, "&"); ampIdx != -1 {
					rem = rem[:ampIdx]
				}
				if gtIdx := strings.Index(rem, ">"); gtIdx != -1 {
					rem = rem[:gtIdx]
				}
				nextLast, _ = urlUnescape(rem)
			}
		}
	}

	// If Link header was present, trust it for pagination.
	// Some registries return fewer tags than requested on the last page.
	// Only disable pagination if there was no Link header AND count < n.
	if n > 0 && len(body.Tags) < n && !hasMore {
		hasMore = false
	}

	return &tagsPageResult{
		tags:     body.Tags,
		hasMore:  hasMore,
		nextLast: nextLast,
	}, nil
}

// parseLinkNext extracts the URL from a Link header with rel="next".
func parseLinkNext(link string) (string, bool) {
	start := strings.Index(link, "<")
	end := strings.Index(link, ">")
	if start == -1 || end == -1 || end <= start {
		return "", false
	}
	urlPart := link[start+1 : end]
	rest := link[end+1:]
	if !strings.Contains(rest, `rel="next"`) {
		return "", false
	}
	return urlPart, true
}
