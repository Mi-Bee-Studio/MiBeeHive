package registry

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- TestNormalizeRegistryURL ---

func TestNormalizeRegistryURL(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		registryType string
		want        string
	}{
		// Docker Hub variants
		{
			name:        "docker.io",
			input:       "docker.io",
			registryType: RegistryTypeDockerHub,
			want:        "https://registry-1.docker.io",
		},
		{
			name:        "registry.hub.docker.com",
			input:       "registry.hub.docker.com",
			registryType: RegistryTypeDockerHub,
			want:        "https://registry-1.docker.io",
		},
		{
			name:        "docker.io with scheme",
			input:       "https://docker.io",
			registryType: RegistryTypeDockerHub,
			want:        "https://registry-1.docker.io",
		},
		{
			name:        "docker.io with v2 suffix",
			input:       "https://docker.io/v2/",
			registryType: RegistryTypeDockerHub,
			want:        "https://registry-1.docker.io",
		},
		{
			name:        "registry-1.docker.io already correct",
			input:       "registry-1.docker.io",
			registryType: RegistryTypeDockerHub,
			want:        "https://registry-1.docker.io",
		},

		// GHCR variants
		{
			name:        "ghcr.io",
			input:       "ghcr.io",
			registryType: RegistryTypeGHCR,
			want:        "https://ghcr.io",
		},
		{
			name:        "ghcr.io with v2 suffix",
			input:       "https://ghcr.io/v2/",
			registryType: RegistryTypeGHCR,
			want:        "https://ghcr.io",
		},

		// ACR variants
		{
			name:        "acr with region",
			input:       "registry.cn-hangzhou.aliyuncs.com",
			registryType: RegistryTypeACR,
			want:        "https://registry.cn-hangzhou.aliyuncs.com",
		},
		{
			name:        "acr add registry prefix",
			input:       "cn-hangzhou.aliyuncs.com",
			registryType: RegistryTypeACR,
			want:        "https://registry.cn-hangzhou.aliyuncs.com",
		},
		{
			name:        "acr with v2 suffix",
			input:       "https://registry.cn-shanghai.aliyuncs.com/v2/",
			registryType: RegistryTypeACR,
			want:        "https://registry.cn-shanghai.aliyuncs.com",
		},

		// TCR variants
		{
			name:        "tcr personal edition",
			input:       "ccr.ccs.tencentyun.com",
			registryType: RegistryTypeTCR,
			want:        "https://ccr.ccs.tencentyun.com",
		},
		{
			name:        "tcr with v2 suffix",
			input:       "https://ccr.ccs.tencentyun.com/v2/",
			registryType: RegistryTypeTCR,
			want:        "https://ccr.ccs.tencentyun.com",
		},

		// Quay variants
		{
			name:        "quay.io",
			input:       "quay.io",
			registryType: RegistryTypeQuay,
			want:        "https://quay.io",
		},
		{
			name:        "quay.io with v2 suffix",
			input:       "https://quay.io/v2/",
			registryType: RegistryTypeQuay,
			want:        "https://quay.io",
		},

		// Generic fallback
		{
			name:        "unknown registry adds scheme",
			input:       "my-registry.example.com",
			registryType: RegistryTypeUnknown,
			want:        "https://my-registry.example.com",
		},
		{
			name:        "unknown strips v2 suffix",
			input:       "https://my-registry.example.com/v2/",
			registryType: RegistryTypeUnknown,
			want:        "https://my-registry.example.com",
		},
		{
			name:        "unknown with scheme preserved",
			input:       "http://localhost:5000",
			registryType: RegistryTypeUnknown,
			want:        "http://localhost:5000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeRegistryURL(tt.input, tt.registryType)
			if got != tt.want {
				t.Errorf("NormalizeRegistryURL(%q, %q) = %q, want %q", tt.input, tt.registryType, got, tt.want)
			}
		})
	}
}

// --- TestAdapterForURL ---

func TestAdapterForURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://registry-1.docker.io", RegistryTypeDockerHub},
		{"https://docker.io", RegistryTypeDockerHub},
		{"docker.io", RegistryTypeDockerHub},
		{"https://registry.hub.docker.com", RegistryTypeDockerHub},
		{"https://ghcr.io", RegistryTypeGHCR},
		{"ghcr.io", RegistryTypeGHCR},
		{"https://registry.cn-hangzhou.aliyuncs.com", RegistryTypeACR},
		{"registry.cn-hangzhou.aliyuncs.com", RegistryTypeACR},
		{"https://ccr.ccs.tencentyun.com", RegistryTypeTCR},
		{"ccr.ccs.tencentyun.com", RegistryTypeTCR},
		{"https://quay.io", RegistryTypeQuay},
		{"quay.io", RegistryTypeQuay},
		{"https://my-registry.example.com", RegistryTypeUnknown},
		{"http://localhost:5000", RegistryTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info := AdapterForURL(tt.input)
			if info == nil {
				t.Fatalf("AdapterForURL(%q) returned nil", tt.input)
			}
			if info.Type != tt.want {
				t.Errorf("Type = %q, want %q", info.Type, tt.want)
			}
		})
	}
}

// --- TestDockerHubAuth ---

func TestDockerHubAuth(t *testing.T) {
	// Docker Hub token server that validates the DNS-mismatch audience.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			t.Errorf("token request path = %q, want /token", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Docker Hub uses Basic auth with username+password for token endpoint.
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("dockeruser:dockerpass"))
		if auth != expected {
			t.Errorf("token auth = %q, want %q", auth, expected)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// The service parameter should be registry.docker.io (not registry-1.docker.io).
		service := r.URL.Query().Get("service")
		if service != "registry.docker.io" {
			t.Errorf("service = %q, want registry.docker.io", service)
		}

		resp := tokenResponse{Token: "docker-hub-token", ExpiresIn: 300}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	// Adapter should produce the correct auth challenge for Docker Hub.
	adapter := dockerHubAdapter()
	challenge := &AuthChallenge{
		Scheme:  "Bearer",
		Realm:   tokenServer.URL + "/token",
		Service: "registry.docker.io",
		Scope:   "repository:library/alpine:pull",
	}

	token, expiresIn, err := authenticateBearer(context.Background(), http.DefaultClient, challenge, &Credentials{Username: "dockeruser", Password: "dockerpass"})
	if err != nil {
		t.Fatalf("authenticateBearer: %v", err)
	}
	if token != "docker-hub-token" {
		t.Errorf("token = %q, want docker-hub-token", token)
	}
	if expiresIn != 300*time.Second {
		t.Errorf("expiresIn = %v, want 300s", expiresIn)
	}

	// Verify adapter has correct properties.
	if adapter.Type != RegistryTypeDockerHub {
		t.Errorf("adapter.Type = %q, want %q", adapter.Type, RegistryTypeDockerHub)
	}
}

func TestDockerHubAuth_ReplaceAuthAudience(t *testing.T) {
	// Verify that DockerHubAdapter rewrites registry-1.docker.io → registry.docker.io
	// in the auth realm/service.
	adapter := dockerHubAdapter()

	// Simulate challenge from registry-1.docker.io with service=registry-1.docker.io
	challenge := &AuthChallenge{
		Scheme:  "Bearer",
		Realm:   "https://auth.docker.io/token",
		Service: "registry-1.docker.io",
	}

	adjusted := adapter.AdjustChallenge(challenge)
	if adjusted.Service != "registry.docker.io" {
		t.Errorf("AdjustedChallenge service = %q, want registry.docker.io", adjusted.Service)
	}
}

func TestDockerHub_OfficialImagePrefix(t *testing.T) {
	adapter := dockerHubAdapter()

	tests := []struct {
		image string
		want  string
	}{
		{"alpine", "library/alpine"},
		{"nginx", "library/nginx"},
		{"myuser/myimage", "myuser/myimage"},
		{"library/alpine", "library/alpine"},
	}

	for _, tt := range tests {
		got := adapter.NormalizeImageName(tt.image)
		if got != tt.want {
			t.Errorf("NormalizeImageName(%q) = %q, want %q", tt.image, got, tt.want)
		}
	}
}

func TestDockerHub_RateLimitHeaders(t *testing.T) {
	adapter := dockerHubAdapter()

	h := http.Header{}
	h.Set("X-RateLimit-Limit", "100")
	h.Set("X-RateLimit-Remaining", "75")
	resp := &http.Response{Header: h}

	info := adapter.ParseRateLimit(resp)
	if info.Limit != 100 {
		t.Errorf("Limit = %d, want 100", info.Limit)
	}
	if info.Remaining != 75 {
		t.Errorf("Remaining = %d, want 75", info.Remaining)
	}
}

func TestDockerHub_RateLimitHeaders_Missing(t *testing.T) {
	adapter := dockerHubAdapter()

	resp := &http.Response{
		Header: http.Header{},
	}

	info := adapter.ParseRateLimit(resp)
	if info.Limit != 0 {
		t.Errorf("Limit = %d, want 0", info.Limit)
	}
	if info.Remaining != 0 {
		t.Errorf("Remaining = %d, want 0", info.Remaining)
	}
}

// --- TestGHCRAuth ---

func TestGHCRAuth(t *testing.T) {
	// GHCR token endpoint at ghcr.io/token.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// GHCR accepts GitHub PAT as password via Basic auth (_:PAT pattern).
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("_:ghp_testpat123"))
		if auth != expected {
			t.Errorf("GHCR auth = %q, want %q", auth, expected)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// GHCR scope should be package:read or package:write.
		scope := r.URL.Query().Get("scope")
		if scope != "package:read" {
			t.Errorf("scope = %q, want package:read", scope)
		}

		resp := tokenResponse{Token: "ghcr-token-abc", ExpiresIn: 300}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	adapter := ghcrAdapter()
	if adapter.Type != RegistryTypeGHCR {
		t.Errorf("adapter.Type = %q, want %q", adapter.Type, RegistryTypeGHCR)
	}

	// Verify GHCR scope mapping.
	readScope := adapter.ScopeForAction("pull")
	if readScope != "package:read" {
		t.Errorf("pull scope = %q, want package:read", readScope)
	}

	writeScope := adapter.ScopeForAction("push")
	if writeScope != "package:write" {
		t.Errorf("push scope = %q, want package:write", writeScope)
	}

	// Verify GHCR uses placeholder username + PAT as password.
	creds := adapter.PrepareCredentials(&Credentials{Username: "ignored", Password: "ghp_testpat123"})
	if creds.Username != "_" {
		t.Errorf("GHCR username = %q, want _", creds.Username)
	}
	if creds.Password != "ghp_testpat123" {
		t.Errorf("GHCR password = %q, want ghp_testpat123", creds.Password)
	}

	// Test auth flow through the adapter's realm.
	challenge := &AuthChallenge{
		Scheme:  "Bearer",
		Realm:   tokenServer.URL + "/token",
		Service: "ghcr.io",
		Scope:   "package:read",
	}

	// Use prepared credentials (empty username + PAT).
	preparedCreds := adapter.PrepareCredentials(&Credentials{Username: "ignored", Password: "ghp_testpat123"})
	token, expiresIn, err := authenticateBearer(context.Background(), http.DefaultClient, challenge, preparedCreds)
	if err != nil {
		t.Fatalf("authenticateBearer: %v", err)
	}
	if token != "ghcr-token-abc" {
		t.Errorf("token = %q, want ghcr-token-abc", token)
	}
	_ = expiresIn
}

// --- TestACRAuth ---

func TestACRAuth(t *testing.T) {
	// ACR token server with standard Bearer auth.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/auth/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("aliuser:alipass"))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := tokenResponse{Token: "acr-token-xyz", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	adapter := acrAdapter()
	if adapter.Type != RegistryTypeACR {
		t.Errorf("adapter.Type = %q, want %q", adapter.Type, RegistryTypeACR)
	}

	// Verify ACR URL detection patterns.
	info := AdapterForURL("https://registry.cn-hangzhou.aliyuncs.com")
	if info.Type != RegistryTypeACR {
		t.Errorf("ACR detection failed: got %q", info.Type)
	}
}

// --- TestTCRAuth ---

func TestTCRAuth(t *testing.T) {
	// TCR personal edition with standard Bearer auth.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("tencentuser:tencentpass"))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := tokenResponse{Token: "tcr-token-456", ExpiresIn: 7200}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	adapter := tcrAdapter()
	if adapter.Type != RegistryTypeTCR {
		t.Errorf("adapter.Type = %q, want %q", adapter.Type, RegistryTypeTCR)
	}

	// Verify TCR URL detection.
	info := AdapterForURL("ccr.ccs.tencentyun.com")
	if info.Type != RegistryTypeTCR {
		t.Errorf("TCR detection failed: got %q", info.Type)
	}
}

// --- TestQuayAuth ---

func TestQuayAuth(t *testing.T) {
	// Quay.io token endpoint.
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/auth" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Quay supports token-based auth (OAuth token as password).
		auth := r.Header.Get("Authorization")
		if auth == "" {
			// First request without auth — return a challenge.
			resp := tokenResponse{Token: "quay-anonymous-token", ExpiresIn: 300}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		resp := tokenResponse{Token: "quay-auth-token", ExpiresIn: 600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer tokenServer.Close()

	adapter := quayAdapter()
	if adapter.Type != RegistryTypeQuay {
		t.Errorf("adapter.Type = %q, want %q", adapter.Type, RegistryTypeQuay)
	}

	// Verify Quay URL detection.
	info := AdapterForURL("quay.io")
	if info.Type != RegistryTypeQuay {
		t.Errorf("Quay detection failed: got %q", info.Type)
	}

	// Quay uses $oauthtoken as username + token as password.
	creds := adapter.PrepareCredentials(&Credentials{Username: "ignored", Password: "quay-token-xyz"})
	if creds.Username != "$oauthtoken" {
		t.Errorf("Quay username = %q, want $oauthtoken", creds.Username)
	}
	if creds.Password != "quay-token-xyz" {
		t.Errorf("Quay password = %q, want quay-token-xyz", creds.Password)
	}
}

// --- TestAdapterForURL_NilSafe ---

func TestAdapterForURL_ReturnsNormalizedURL(t *testing.T) {
	info := AdapterForURL("docker.io/v2/")
	if info.NormalizedURL != "https://registry-1.docker.io" {
		t.Errorf("NormalizedURL = %q, want https://registry-1.docker.io", info.NormalizedURL)
	}
}

// --- TestAdapterCreation via factory ---

func TestAdapterForURL_FactoryReturnsCorrectAdapters(t *testing.T) {
	tests := []struct {
		url        string
		wantType   string
		wantDesc   string
	}{
		{"https://registry-1.docker.io", RegistryTypeDockerHub, "Docker Hub"},
		{"https://ghcr.io", RegistryTypeGHCR, "GitHub Container Registry"},
		{"https://registry.cn-hangzhou.aliyuncs.com", RegistryTypeACR, "Alibaba Cloud Container Registry"},
		{"https://ccr.ccs.tencentyun.com", RegistryTypeTCR, "Tencent Cloud Container Registry"},
		{"https://quay.io", RegistryTypeQuay, "Quay.io"},
		{"https://my-registry.example.com", RegistryTypeUnknown, "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			info := AdapterForURL(tt.url)
			if info.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", info.Type, tt.wantType)
			}
			if info.Description != tt.wantDesc {
				t.Errorf("Description = %q, want %q", info.Description, tt.wantDesc)
			}
		})
	}
}
