package registry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Credentials holds basic authentication credentials for registry access.
type Credentials struct {
	Username string
	Password string
}

// ClientOption applies a configuration option to a RegistryClient.
type ClientOption func(*RegistryClient)

// WithManifestTimeout sets the timeout for manifest and small API requests.
func WithManifestTimeout(d time.Duration) ClientOption {
	return func(c *RegistryClient) {
		c.manifestTimeout = d
	}
}

// WithBlobTimeout sets the timeout for blob download requests.
func WithBlobTimeout(d time.Duration) ClientOption {
	return func(c *RegistryClient) {
		c.blobTimeout = d
	}
}

// WithHTTPClient sets a custom HTTP client (e.g. for custom transport).
func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *RegistryClient) {
		c.httpClient = hc
	}
}

// RegistryClient is a V2 Docker/OCI registry client that handles
// authentication negotiation (Basic and Bearer token) with caching.
type RegistryClient struct {
	baseURL    *url.URL
	httpClient *http.Client
	credentials *Credentials
	tokenCache *TokenCache

	manifestTimeout time.Duration
	blobTimeout     time.Duration
}

// NewClient creates a new V2 registry client.
//
// baseURL is the registry endpoint (e.g. "https://registry-1.docker.io").
// The URL is normalized: trailing slashes are stripped, and "https" is
// used as default scheme if none is provided.
//
// creds may be nil for anonymous access.
func NewClient(baseURL string, creds *Credentials, opts ...ClientOption) (*RegistryClient, error) {
	u, err := parseRegistryURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse registry URL: %w", err)
	}

	c := &RegistryClient{
		baseURL:    u,
		credentials: creds,
		tokenCache: NewTokenCache(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		manifestTimeout: 30 * time.Second,
		blobTimeout:     30 * time.Minute,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c, nil
}

// parseRegistryURL normalizes a registry URL.
func parseRegistryURL(raw string) (*url.URL, error) {
	// Add scheme if missing.
	if !stringsContainsScheme(raw) {
		raw = "https://" + raw
	}

	u, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid URL %q: %w", raw, err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme %q (use http or https)", u.Scheme)
	}

	// Strip trailing slash.
	u.Path = stripTrailingSlash(u.Path)

	return u, nil
}

// stringsContainsScheme checks if a URL string contains a scheme prefix.
func stringsContainsScheme(s string) bool {
	return strings.Contains(s, "://")
}

// stripTrailingSlash removes a trailing '/' from a path, preserving root "/".
func stripTrailingSlash(path string) string {
	if len(path) > 1 && path[len(path)-1] == '/' {
		return path[:len(path)-1]
	}
	return path
}

// Ping checks connectivity to the registry by calling GET /v2/.
// If the registry requires authentication, it negotiates auth automatically.
func (c *RegistryClient) Ping(ctx context.Context) error {
	req, err := c.newRequest(ctx, http.MethodGet, "/v2/", nil)
	if err != nil {
		return fmt.Errorf("create ping request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("ping request: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		if wwwAuth == "" {
			return NewAuthError(resp.StatusCode, "401 with no WWW-Authenticate header")
		}

		authHeader, err := resolveAuth(ctx, c.httpClient, wwwAuth, c.credentials, c.tokenCache)
		if err != nil {
			return fmt.Errorf("auth negotiation: %w", err)
		}

		// Retry with auth.
		retryReq, err := c.newRequest(ctx, http.MethodGet, "/v2/", nil)
		if err != nil {
			return fmt.Errorf("create retry ping request: %w", err)
		}
		retryReq.Header.Set("Authorization", authHeader)

		retryResp, err := c.httpClient.Do(retryReq)
		if err != nil {
			return fmt.Errorf("retry ping request: %w", err)
		}
		defer retryResp.Body.Close()

		if retryResp.StatusCode != http.StatusOK {
			return statusCodeError(retryResp)
		}
		return nil
	default:
		return statusCodeError(resp)
	}
}

// newRequest creates an HTTP request against the registry base URL.
// path is appended to the base URL path.
func (c *RegistryClient) newRequest(ctx context.Context, method, path string, _ interface{}) (*http.Request, error) {
	u := *c.baseURL
	u.Path = u.Path + path
	return http.NewRequestWithContext(ctx, method, u.String(), nil)
}

// doRequest executes an HTTP request with automatic auth retry on 401.
// The caller is responsible for closing the response body.
func (c *RegistryClient) doRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	// Attach cached auth token if available.
	scope := scopeFromPath(req.URL.Path)
	if cached, ok := c.tokenCache.Get(scope); ok {
		req.Header.Set("Authorization", "Bearer "+cached)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized {
		wwwAuth := resp.Header.Get("WWW-Authenticate")
		resp.Body.Close()

		if wwwAuth == "" {
			return nil, NewAuthError(resp.StatusCode, "401 with no WWW-Authenticate header")
		}

		authHeader, err := resolveAuth(ctx, c.httpClient, wwwAuth, c.credentials, c.tokenCache)
		if err != nil {
			return nil, fmt.Errorf("auth refresh: %w", err)
		}

		// Rebuild request (can't reuse the old body, but we handle nil body only).
		retryReq, err := http.NewRequestWithContext(ctx, req.Method, req.URL.String(), nil)
		if err != nil {
			return nil, fmt.Errorf("create retry request: %w", err)
		}
		retryReq.Header = req.Header.Clone()
		retryReq.Header.Set("Authorization", authHeader)

		retryResp, err := c.httpClient.Do(retryReq)
		if err != nil {
			return nil, fmt.Errorf("retry request: %w", err)
		}
		return retryResp, nil
	}

	return resp, nil
}

// scopeFromPath derives a token cache scope from the request path.
// For /v2/ ping requests, returns empty string.
func scopeFromPath(path string) string {
	if path == "/v2/" || path == "/v2" {
		return ""
	}
	return path
}
