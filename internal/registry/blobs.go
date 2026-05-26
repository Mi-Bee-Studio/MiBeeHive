package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// BlobExists checks whether a blob exists in the repository.
// Returns true if the blob exists (HTTP 200), false if not found (HTTP 404).
func (c *RegistryClient) BlobExists(ctx context.Context, repo, digest string) (bool, error) {
	path := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	req, err := c.newRequest(ctx, http.MethodHead, path, nil)
	if err != nil {
		return false, fmt.Errorf("create blob head request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return false, fmt.Errorf("blob head request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, statusCodeError(resp)
	}
}

// PullBlob downloads a blob and streams it to the provided writer.
// Uses blobTimeout (30 min) for the transfer. Does NOT buffer in memory.
func (c *RegistryClient) PullBlob(ctx context.Context, repo, digest string, writer io.Writer) error {
	blobCtx, cancel := context.WithTimeout(ctx, c.blobTimeout)
	defer cancel()

	path := fmt.Sprintf("/v2/%s/blobs/%s", repo, digest)
	req, err := c.newRequest(blobCtx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("create pull blob request: %w", err)
	}

	resp, err := c.doRequest(blobCtx, req)
	if err != nil {
		return fmt.Errorf("pull blob request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return statusCodeError(resp)
	}

	if _, err := io.Copy(writer, resp.Body); err != nil {
		return fmt.Errorf("copy blob data: %w", err)
	}

	return nil
}

// PushBlob uploads a blob to the repository in a two-step process:
//  1. POST /v2/{name}/blobs/uploads/ to get an upload URL
//  2. PUT {upload_url}?digest={digest} with the blob data
//
// The reader is streamed directly — no in-memory buffering.
func (c *RegistryClient) PushBlob(ctx context.Context, repo string, reader io.Reader, digest string, size int64) error {
	// Step 1: Initiate upload.
	path := fmt.Sprintf("/v2/%s/blobs/uploads/", repo)
	req, err := c.newRequest(ctx, http.MethodPost, path, nil)
	if err != nil {
		return fmt.Errorf("create upload init request: %w", err)
	}

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("upload init request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return statusCodeError(resp)
	}

	location := resp.Header.Get("Location")
	if location == "" {
		return fmt.Errorf("upload init: no Location header in response")
	}

	// Step 2: PUT blob data to the upload URL.
	uploadURL, err := url.Parse(location)
	if err != nil {
		return fmt.Errorf("parse upload URL: %w", err)
	}
	q := uploadURL.Query()
	q.Set("digest", digest)
	uploadURL.RawQuery = q.Encode()

	blobCtx, cancel := context.WithTimeout(ctx, c.blobTimeout)
	defer cancel()

	putReq, err := http.NewRequestWithContext(blobCtx, http.MethodPut, uploadURL.String(), reader)
	if err != nil {
		return fmt.Errorf("create upload put request: %w", err)
	}
	putReq.Header.Set("Content-Type", "application/octet-stream")
	if size > 0 {
		putReq.ContentLength = size
	}

	// Attach auth to the PUT request.
	c.attachAuth(putReq, repo)

	putResp, err := c.httpClient.Do(putReq)
	if err != nil {
		return fmt.Errorf("upload put request: %w", err)
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusCreated {
		return statusCodeError(putResp)
	}

	return nil
}

// attachAuth sets the Authorization header on a request using cached tokens
// or credentials. Used for requests that bypass doRequest (e.g. blob uploads
// to external storage URLs).
func (c *RegistryClient) attachAuth(req *http.Request, repo string) {
	// Try to find a cached token for this repo.
	scope := scopeFromPath("/v2/" + repo)
	if cached, ok := c.tokenCache.Get(scope); ok {
		req.Header.Set("Authorization", "Bearer "+cached)
		return
	}
	// Fall back to basic auth if credentials are set.
	if c.credentials != nil {
		req.Header.Set("Authorization", authenticateBasic(c.credentials))
	}
}

// urlUnescape is a test-safe wrapper for url.QueryUnescape.
func urlUnescape(s string) (string, error) {
	return url.QueryUnescape(s)
}

// isValidDigest checks if a string looks like a content digest (algorithm:hex).
func isValidDigest(s string) bool {
	return strings.Contains(s, ":") && strings.HasPrefix(s, "sha")
}
