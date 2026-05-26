package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

// Catalog lists repositories in the registry.
//
// Parameters n and last control pagination: n is the page size and last is the
// last repository name from the previous page. Pass 0 for n to use the server
// default. If the returned slice has fewer items than n, there are no more pages.
//
// Note: Docker Hub does NOT support /v2/_catalog — the method returns an error
// with a helpful message if the registry responds with 404 or 403.
func (c *RegistryClient) Catalog(ctx context.Context, n int, last string) ([]string, error) {
	path := "/v2/_catalog"
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("create catalog request: %w", err)
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
		return nil, fmt.Errorf("catalog request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden {
		_ = drainBody(resp.Body)
		return nil, fmt.Errorf("catalog: registry does not support /v2/_catalog endpoint (status %d); Docker Hub and some hosted registries do not expose this endpoint",
			resp.StatusCode)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusCodeError(resp)
	}

	var result struct {
		Repositories []string `json:"repositories"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1024*1024)).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode catalog response: %w", err)
	}

	return result.Repositories, nil
}

// drainBody reads and discards the remaining response body.
func drainBody(body io.ReadCloser) error {
	_, err := io.Copy(io.Discard, io.LimitReader(body, 512*1024))
	return err
}
