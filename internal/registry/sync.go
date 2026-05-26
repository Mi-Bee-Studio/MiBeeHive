package registry

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

// RawManifest fetches the raw manifest body for a given repo and ref.
// For multi-arch images, it resolves to the linux/arm64 variant (same as Manifest).
// Returns the raw JSON body, content digest, media type, and any error.
func (c *RegistryClient) RawManifest(ctx context.Context, repo, ref string) ([]byte, string, string, error) {
	result, err := c.fetchManifest(ctx, repo, ref, 0)
	if err != nil {
		return nil, "", "", err
	}
	return result.rawBody, result.digest, result.contentType, nil
}

// PushManifest pushes a raw manifest body to a repository tag.
// The mediaType must match the manifest type
// (e.g. application/vnd.docker.distribution.manifest.v2+json).
func (c *RegistryClient) PushManifest(ctx context.Context, repo, tag string, body []byte, mediaType string) error {
	u := *c.baseURL
	u.Path = u.Path + fmt.Sprintf("/v2/%s/manifests/%s", repo, tag)

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u.String(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create push manifest request: %w", err)
	}
	req.Header.Set("Content-Type", mediaType)
	req.ContentLength = int64(len(body))

	resp, err := c.doRequest(ctx, req)
	if err != nil {
		return fmt.Errorf("push manifest request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return statusCodeError(resp)
	}

	return nil
}
