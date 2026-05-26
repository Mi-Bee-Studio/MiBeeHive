package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// AuthChallenge represents a parsed WWW-Authenticate header.
type AuthChallenge struct {
	Scheme  string // "Basic" or "Bearer"
	Realm   string
	Service string
	Scope   string
}

// tokenResponse is the JSON response from a token endpoint.
type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
	// Some registries use access_token instead of token.
	AccessToken string `json:"access_token"`
}

// cachedToken holds a cached auth token with its expiry.
type cachedToken struct {
	token   string
	expires time.Time
}

// TokenCache caches authentication tokens keyed by scope.
type TokenCache struct {
	mu     sync.Mutex
	tokens map[string]*cachedToken
}

// NewTokenCache creates an empty token cache.
func NewTokenCache() *TokenCache {
	return &TokenCache{
		tokens: make(map[string]*cachedToken),
	}
}

// Get returns a cached token for the given scope if it hasn't expired.
// Returns ("", false) if not found or expired.
func (c *TokenCache) Get(scope string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ct, ok := c.tokens[scope]
	if !ok {
		return "", false
	}
	if time.Now().After(ct.expires) {
		return "", false
	}
	return ct.token, true
}

// Set stores a token with the given scope and expiry duration.
// A 5-minute buffer is subtracted from the actual expiry to avoid
// edge cases with tokens expiring mid-request.
func (c *TokenCache) Set(scope, token string, expiresIn time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	buffer := 5 * time.Minute
	if expiresIn < buffer {
		buffer = expiresIn / 2
	}
	c.tokens[scope] = &cachedToken{
		token:   token,
		expires: time.Now().Add(expiresIn - buffer),
	}
}

// parseWWWAuthenticate parses a WWW-Authenticate header value into an AuthChallenge.
// Supports:
//
//	Basic realm="..."
//	Bearer realm="...",service="...",scope="..."
func parseWWWAuthenticate(header string) (*AuthChallenge, error) {
	header = strings.TrimSpace(header)

	// Split scheme from the rest.
	spaceIdx := strings.Index(header, " ")
	if spaceIdx == -1 {
		// Scheme only, no parameters (unusual but handle it).
		return &AuthChallenge{Scheme: header}, nil
	}

	challenge := &AuthChallenge{
		Scheme: header[:spaceIdx],
	}

	rest := header[spaceIdx+1:]

	// Parse key="value" pairs.
	for _, pair := range parseKV(rest) {
		switch pair[0] {
		case "realm":
			challenge.Realm = pair[1]
		case "service":
			challenge.Service = pair[1]
		case "scope":
			challenge.Scope = pair[1]
		}
	}

	if challenge.Scheme != "Basic" && challenge.Scheme != "Bearer" {
		return nil, fmt.Errorf("unsupported auth scheme: %s", challenge.Scheme)
	}

	return challenge, nil
}

// parseKV parses key="value" pairs from a comma-separated string.
func parseKV(s string) [][2]string {
	var pairs [][2]string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			continue
		}
		key := part[:eqIdx]
		val := strings.Trim(part[eqIdx+1:], "\"")
		pairs = append(pairs, [2]string{key, val})
	}
	return pairs
}

// authenticateBasic returns a Basic auth header value (base64-encoded user:pass).
func authenticateBasic(creds *Credentials) string {
	raw := creds.Username + ":" + creds.Password
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(raw))
}

// authenticateBearer fetches a Bearer token from the auth realm.
// The request includes Basic auth using the provided credentials.
func authenticateBearer(ctx context.Context, httpClient *http.Client, challenge *AuthChallenge, creds *Credentials) (string, time.Duration, error) {
	u, err := url.Parse(challenge.Realm)
	if err != nil {
		return "", 0, fmt.Errorf("parse auth realm URL: %w", err)
	}

	q := u.Query()
	if challenge.Service != "" {
		q.Set("service", challenge.Service)
	}
	if challenge.Scope != "" {
		q.Set("scope", challenge.Scope)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", 0, fmt.Errorf("create token request: %w", err)
	}

	if creds != nil && creds.Username != "" {
		req.Header.Set("Authorization", authenticateBasic(creds))
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", 0, fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp tokenResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&tokenResp); err != nil {
		return "", 0, fmt.Errorf("decode token response: %w", err)
	}

	token := tokenResp.Token
	if token == "" {
		token = tokenResp.AccessToken
	}
	if token == "" {
		return "", 0, fmt.Errorf("token response contained no token")
	}

	expiresIn := time.Duration(tokenResp.ExpiresIn) * time.Second
	if expiresIn <= 0 {
		expiresIn = 60 * time.Minute // default 1 hour
	}

	return token, expiresIn, nil
}

// resolveAuth negotiates authentication based on a WWW-Authenticate challenge.
// Returns the Authorization header value to use for subsequent requests.
func resolveAuth(ctx context.Context, httpClient *http.Client, wwwAuth string, creds *Credentials, tokenCache *TokenCache) (string, error) {
	challenge, err := parseWWWAuthenticate(wwwAuth)
	if err != nil {
		return "", fmt.Errorf("parse www-authenticate: %w", err)
	}

	switch challenge.Scheme {
	case "Basic":
		if creds == nil {
			return "", NewAuthError(http.StatusUnauthorized, "basic auth required but no credentials provided")
		}
		return authenticateBasic(creds), nil

	case "Bearer":
		// Check cache first.
		cacheKey := challenge.Scope
		if cached, ok := tokenCache.Get(cacheKey); ok {
			return "Bearer " + cached, nil
		}

		token, expiresIn, err := authenticateBearer(ctx, httpClient, challenge, creds)
		if err != nil {
			return "", fmt.Errorf("bearer auth: %w", err)
		}

		tokenCache.Set(cacheKey, token, expiresIn)
		return "Bearer " + token, nil

	default:
		return "", fmt.Errorf("unsupported auth scheme: %s", challenge.Scheme)
	}
}

// statusCodeError maps an HTTP status code to a typed error.
func statusCodeError(resp *http.Response) error {
	msg := resp.Status
	detail := ""
	if body, err := io.ReadAll(io.LimitReader(resp.Body, 512)); err == nil && len(body) > 0 {
		detail = string(body)
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return NewAuthError(resp.StatusCode, msg)
	case http.StatusNotFound:
		return NewNotFoundError(resp.StatusCode, msg)
	case http.StatusTooManyRequests:
		retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"))
		return NewRateLimitError(resp.StatusCode, msg, retryAfter)
	default:
		return &RegistryError{
			StatusCode: resp.StatusCode,
			Message:    msg,
			Detail:     detail,
		}
	}
}

// parseIntHeader parses an integer header value, returning defaultVal on failure.
func parseIntHeader(resp *http.Response, key string, defaultVal int) int {
	val := resp.Header.Get(key)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}

// logger returns a slog.Logger with a registry component tag.
func logger() *slog.Logger {
	return slog.Default().With("component", "registry")
}
