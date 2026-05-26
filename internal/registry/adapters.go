package registry

import (
	"fmt"
	"net/http"
	"strings"
)

// Registry type constants identify the registry backend.
const (
	RegistryTypeDockerHub = "dockerhub"
	RegistryTypeGHCR     = "ghcr"
	RegistryTypeACR      = "acr"
	RegistryTypeTCR      = "tcr"
	RegistryTypeQuay     = "quay"
	RegistryTypeUnknown  = "unknown"
)

// RateLimitInfo holds parsed rate limit headers from a registry response.
type RateLimitInfo struct {
	Limit     int
	Remaining int
}

// RegistryAdapter provides per-registry customization for auth and URL handling.
type RegistryAdapter struct {
	// Type identifies the registry backend.
	Type string
	// Description is a human-readable name for the registry.
	Description string
	// DefaultAuthRealm is the default token endpoint URL for this registry.
	DefaultAuthRealm string
	// DefaultAuthService is the service identifier expected by the auth server.
	DefaultAuthService string

	// AdjustChallenge modifies the parsed WWW-Authenticate challenge before
	// using it for token negotiation. Used by Docker Hub to fix the DNS mismatch
	// (registry-1.docker.io API → registry.docker.io auth audience).
	AdjustChallenge func(challenge *AuthChallenge) *AuthChallenge

	// PrepareCredentials transforms user-provided credentials into the format
	// expected by this registry's auth endpoint. GHCR sends empty username + PAT,
	// Quay sends empty username + token.
	PrepareCredentials func(creds *Credentials) *Credentials

	// ScopeForAction maps an action (pull/push/delete) to the registry-specific
	// scope string. Default is "repository:image:action".
	ScopeForAction func(action string) string

	// NormalizeImageName adjusts an image name for this registry.
	// Docker Hub needs "library/" prefix for official images.
	NormalizeImageName func(image string) string

	// ParseRateLimit extracts rate limit info from response headers.
	ParseRateLimit func(resp *http.Response) RateLimitInfo
}

// RegistryAdapterInfo is returned by AdapterForURL and contains both the
// adapter and the normalized URL for the detected registry.
type RegistryAdapterInfo struct {
	Type          string
	Description   string
	NormalizedURL string
	Adapter       *RegistryAdapter
}

// --- Adapter constructors ---

func dockerHubAdapter() *RegistryAdapter {
	return &RegistryAdapter{
		Type:               RegistryTypeDockerHub,
		Description:        "Docker Hub",
		DefaultAuthRealm:   "https://auth.docker.io/token",
		DefaultAuthService: "registry.docker.io",
		AdjustChallenge: func(challenge *AuthChallenge) *AuthChallenge {
			adjusted := *challenge
			// Docker Hub API host is registry-1.docker.io but auth audience
			// must be registry.docker.io.
			adjusted.Service = strings.ReplaceAll(adjusted.Service, "registry-1.docker.io", "registry.docker.io")
			return &adjusted
		},
		PrepareCredentials: func(creds *Credentials) *Credentials {
			// Docker Hub passes credentials as-is.
			return creds
		},
		ScopeForAction: func(action string) string {
			// Docker Hub uses standard repository scope format.
			return "repository:{image}:" + action
		},
		NormalizeImageName: func(image string) string {
			// Official images (no slash) need "library/" prefix.
			if !strings.Contains(image, "/") {
				return "library/" + image
			}
			return image
		},
		ParseRateLimit: func(resp *http.Response) RateLimitInfo {
			return RateLimitInfo{
				Limit:     parseIntHeader(resp, "X-RateLimit-Limit", 0),
				Remaining: parseIntHeader(resp, "X-RateLimit-Remaining", 0),
			}
		},
	}
}

func ghcrAdapter() *RegistryAdapter {
	return &RegistryAdapter{
		Type:               RegistryTypeGHCR,
		Description:        "GitHub Container Registry",
		DefaultAuthRealm:   "https://ghcr.io/token",
		DefaultAuthService: "ghcr.io",
		AdjustChallenge: func(challenge *AuthChallenge) *AuthChallenge {
			return challenge
		},
		PrepareCredentials: func(creds *Credentials) *Credentials {
			// GHCR uses a placeholder username + GitHub PAT as password.
			// The underscore placeholder ensures Basic auth is sent (authenticateBearer
			// skips auth when username is empty). GHCR ignores the username field.
			return &Credentials{
				Username: "_",
				Password: creds.Password,
			}
		},
		ScopeForAction: func(action string) string {
			switch action {
			case "pull", "get":
				return "package:read"
			case "push", "delete":
				return "package:write"
			default:
				return "package:read"
			}
		},
		NormalizeImageName: func(image string) string {
			return image
		},
		ParseRateLimit: func(resp *http.Response) RateLimitInfo {
			return RateLimitInfo{
				Limit:     parseIntHeader(resp, "X-RateLimit-Limit", 0),
				Remaining: parseIntHeader(resp, "X-RateLimit-Remaining", 0),
			}
		},
	}
}

func acrAdapter() *RegistryAdapter {
	return &RegistryAdapter{
		Type:               RegistryTypeACR,
		Description:        "Alibaba Cloud Container Registry",
		DefaultAuthRealm:   "",
		DefaultAuthService: "",
		AdjustChallenge: func(challenge *AuthChallenge) *AuthChallenge {
			return challenge
		},
		PrepareCredentials: func(creds *Credentials) *Credentials {
			return creds
		},
		ScopeForAction: func(action string) string {
			return "repository:{image}:" + action
		},
		NormalizeImageName: func(image string) string {
			return image
		},
		ParseRateLimit: func(resp *http.Response) RateLimitInfo {
			return RateLimitInfo{}
		},
	}
}

func tcrAdapter() *RegistryAdapter {
	return &RegistryAdapter{
		Type:               RegistryTypeTCR,
		Description:        "Tencent Cloud Container Registry",
		DefaultAuthRealm:   "",
		DefaultAuthService: "",
		AdjustChallenge: func(challenge *AuthChallenge) *AuthChallenge {
			return challenge
		},
		PrepareCredentials: func(creds *Credentials) *Credentials {
			return creds
		},
		ScopeForAction: func(action string) string {
			return "repository:{image}:" + action
		},
		NormalizeImageName: func(image string) string {
			return image
		},
		ParseRateLimit: func(resp *http.Response) RateLimitInfo {
			return RateLimitInfo{}
		},
	}
}

func quayAdapter() *RegistryAdapter {
	return &RegistryAdapter{
		Type:               RegistryTypeQuay,
		Description:        "Quay.io",
		DefaultAuthRealm:   "https://quay.io/v2/auth",
		DefaultAuthService: "quay.io",
		AdjustChallenge: func(challenge *AuthChallenge) *AuthChallenge {
			return challenge
		},
		PrepareCredentials: func(creds *Credentials) *Credentials {
			// Quay uses a placeholder username + token/OAuth as password.
			return &Credentials{
				Username: "$oauthtoken",
				Password: creds.Password,
			}
		},
		ScopeForAction: func(action string) string {
			return "repository:{image}:" + action
		},
		NormalizeImageName: func(image string) string {
			return image
		},
		ParseRateLimit: func(resp *http.Response) RateLimitInfo {
			return RateLimitInfo{
				Limit:     parseIntHeader(resp, "X-RateLimit-Limit", 0),
				Remaining: parseIntHeader(resp, "X-RateLimit-Remaining", 0),
			}
		},
	}
}

// --- URL Detection ---

// detectRegistryType identifies the registry type from a URL host.
func detectRegistryType(host string) string {
	host = strings.ToLower(host)

	// Docker Hub variants.
	if host == "docker.io" ||
		host == "registry.hub.docker.com" ||
		host == "registry-1.docker.io" ||
		host == "registry.docker.io" ||
		host == "index.docker.io" {
		return RegistryTypeDockerHub
	}

	// GHCR.
	if host == "ghcr.io" {
		return RegistryTypeGHCR
	}

	// Alibaba Cloud (ACR).
	if strings.HasSuffix(host, ".aliyuncs.com") {
		return RegistryTypeACR
	}

	// Tencent Cloud (TCR).
	if host == "ccr.ccs.tencentyun.com" {
		return RegistryTypeTCR
	}

	// Quay.io.
	if host == "quay.io" {
		return RegistryTypeQuay
	}

	return RegistryTypeUnknown
}

// getAdapter returns the adapter for a given registry type.
func getAdapter(registryType string) *RegistryAdapter {
	switch registryType {
	case RegistryTypeDockerHub:
		return dockerHubAdapter()
	case RegistryTypeGHCR:
		return ghcrAdapter()
	case RegistryTypeACR:
		return acrAdapter()
	case RegistryTypeTCR:
		return tcrAdapter()
	case RegistryTypeQuay:
		return quayAdapter()
	default:
		return nil
	}
}

// AdapterForURL detects the registry type from a URL and returns the
// appropriate adapter info with normalized URL.
func AdapterForURL(rawURL string) *RegistryAdapterInfo {
	// Extract host for detection.
	host := extractHost(rawURL)
	registryType := detectRegistryType(host)
	normalizedURL := NormalizeRegistryURL(rawURL, registryType)

	adapter := getAdapter(registryType)
	if adapter == nil {
		adapter = &RegistryAdapter{
			Type:               RegistryTypeUnknown,
			Description:        "Unknown",
			AdjustChallenge:    func(c *AuthChallenge) *AuthChallenge { return c },
			PrepareCredentials: func(c *Credentials) *Credentials { return c },
			ScopeForAction:     func(a string) string { return "repository:{image}:" + a },
			NormalizeImageName: func(i string) string { return i },
			ParseRateLimit:     func(_ *http.Response) RateLimitInfo { return RateLimitInfo{} },
		}
	}

	return &RegistryAdapterInfo{
		Type:          registryType,
		Description:   adapter.Description,
		NormalizedURL: normalizedURL,
		Adapter:       adapter,
	}
}

// extractHost extracts the hostname from a raw URL string.
// Handles both URLs with scheme and bare hostnames.
func extractHost(rawURL string) string {
	// If no scheme, add one temporarily for parsing.
	s := rawURL
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}

	// Split off any path.
	parts := strings.SplitN(s, "/", 4)
	hostPart := parts[2] // host:port

	// Strip port.
	host := hostPart
	if idx := strings.LastIndex(hostPart, ":"); idx != -1 {
		host = hostPart[:idx]
	}

	return strings.ToLower(host)
}

// NormalizeRegistryURL normalizes a registry URL for the given registry type.
// Handles per-registry quirks (Docker Hub DNS alias, scheme defaults,
// /v2/ suffix stripping).
func NormalizeRegistryURL(rawURL, registryType string) string {
	s := strings.TrimSpace(rawURL)

	// Strip /v2/ or /v2 suffix from path.
	s = stripV2Suffix(s)

	switch registryType {
	case RegistryTypeDockerHub:
		return normalizeDockerHubURL(s)
	case RegistryTypeGHCR:
		return normalizeWithScheme(s, "ghcr.io")
	case RegistryTypeACR:
		return normalizeACRURL(s)
	case RegistryTypeTCR:
		return normalizeWithScheme(s, "ccr.ccs.tencentyun.com")
	case RegistryTypeQuay:
		return normalizeWithScheme(s, "quay.io")
	default:
		return normalizeGenericURL(s)
	}
}

// normalizeDockerHubURL handles Docker Hub's DNS aliases.
func normalizeDockerHubURL(s string) string {
	host := extractHost(s)
	switch host {
	case "docker.io", "registry.hub.docker.com", "index.docker.io", "registry.docker.io", "registry-1.docker.io":
		// All Docker Hub hosts resolve to registry-1.docker.io.
	default:
		// Not a Docker Hub host; return as-is with scheme.
		return ensureScheme(s)
	}
	return "https://registry-1.docker.io"
}

// normalizeWithScheme ensures the URL has https:// and uses the canonical host.
func normalizeWithScheme(s, canonicalHost string) string {
	host := extractHost(s)
	if host == canonicalHost || host == "" {
		return "https://" + canonicalHost
	}
	return ensureScheme(s)
}

// normalizeACRURL ensures ACR URLs have the correct registry prefix.
func normalizeACRURL(s string) string {
	host := extractHost(s)
	if strings.HasSuffix(host, ".aliyuncs.com") {
		// Check if it already has the "registry." prefix.
		if !strings.HasPrefix(host, "registry.") {
			host = "registry." + host
		}
		return "https://" + host
	}
	return ensureScheme(s)
}

// normalizeGenericURL adds https:// if no scheme and strips trailing slashes.
func normalizeGenericURL(s string) string {
	return ensureScheme(s)
}

// ensureScheme adds https:// if no scheme is present.
func ensureScheme(s string) string {
	if !strings.Contains(s, "://") {
		return "https://" + s
	}
	return s
}

// stripV2Suffix removes trailing /v2/ or /v2 from the URL path.
func stripV2Suffix(s string) string {
	if !strings.Contains(s, "://") {
		// Bare hostname — nothing to strip.
		if !strings.Contains(s, "/") {
			return s
		}
	}

	// Strip /v2/ at the end of path.
	s = strings.TrimSuffix(s, "/v2/")
	s = strings.TrimSuffix(s, "/v2")
	return s
}

// FormatScope builds a scope string for an image and action using the
// adapter's scope format. Falls back to the standard repository scope
// format for unknown registries.
func FormatScope(adapter *RegistryAdapter, image, action string) string {
	if adapter == nil || adapter.ScopeForAction == nil {
		return fmt.Sprintf("repository:%s:%s", image, action)
	}
	scope := adapter.ScopeForAction(action)
	// Replace {image} placeholder if present.
	if strings.Contains(scope, "{image}") {
		return strings.ReplaceAll(scope, "{image}", image)
	}
	return scope
}
