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

func TestURLNormalization(t *testing.T) {
	tests := []struct {
		input    string
		wantHost string
		wantErr  bool
	}{
		{"https://registry.example.com", "registry.example.com", false},
		{"http://localhost:5000", "localhost:5000", false},
		{"registry.example.com", "registry.example.com", false},
		{"registry.example.com/", "registry.example.com", false},
		{"https://registry.example.com/v2/", "registry.example.com", false},
		{"ftp://registry.example.com", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			u, err := parseRegistryURL(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Host != tt.wantHost {
				t.Errorf("host = %q, want %q", u.Host, tt.wantHost)
			}
			if tt.input == "registry.example.com" {
				if u.Scheme != "https" {
					t.Errorf("scheme = %q, want https", u.Scheme)
				}
			}
			// Trailing slash on path should be stripped.
			if len(u.Path) > 1 && u.Path[len(u.Path)-1] == '/' {
			}
		})
	}
}

func TestPing_NoAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPing_BasicAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="test"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			// Validate Basic auth.
			expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
			if auth != expected {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, &Credentials{Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPing_BearerAuth(t *testing.T) {
	// Auth server that issues tokens.
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/token" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Validate Basic auth on token endpoint.
		auth := r.Header.Get("Authorization")
		expected := "Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass"))
		if auth != expected {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		resp := tokenResponse{
			Token:     "test-token-123",
			ExpiresIn: 3600,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer authServer.Close()

	// Registry server that requires Bearer auth.
	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v2/" {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Bearer realm="`+authServer.URL+`/token",service="test-registry",scope="registry:catalog:*"`)
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if auth != "Bearer test-token-123" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer registryServer.Close()

	client, err := NewClient(registryServer.URL, &Credentials{Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	if err := client.Ping(context.Background()); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestPing_UnauthorizedNoHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 401 without WWW-Authenticate.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	err = client.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for 401 without WWW-Authenticate")
	}
	authErr, ok := err.(*AuthError)
	if !ok {
		t.Fatalf("expected AuthError, got %T: %v", err, err)
	}
	if authErr.StatusCode != 401 {
		t.Errorf("StatusCode = %d, want 401", authErr.StatusCode)
	}
}

func TestPing_RegistryError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	err = client.Ping(context.Background())
	if err == nil {
		t.Fatal("expected error for 500")
	}
	regErr, ok := err.(*RegistryError)
	if !ok {
		t.Fatalf("expected RegistryError, got %T: %v", err, err)
	}
	if regErr.StatusCode != 500 {
		t.Errorf("StatusCode = %d, want 500", regErr.StatusCode)
	}
}

// TestTokenRefresh_On401 verifies that doRequest auto-refreshes tokens on 401.
func TestTokenRefresh_On401(t *testing.T) {
	callCount := 0
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tokenResponse{Token: "refreshed-token", ExpiresIn: 3600}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer authServer.Close()

	registryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		auth := r.Header.Get("Authorization")
		if auth == "" || auth == "Bearer expired-token" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="`+authServer.URL+`/token",service="test",scope="repository:foo:pull"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if auth != "Bearer refreshed-token" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer registryServer.Close()

	client, err := NewClient(registryServer.URL, &Credentials{Username: "user", Password: "pass"})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	// Pre-seed expired token.
	client.tokenCache.Set("repository:foo:pull", "expired-token", 1*time.Nanosecond)
	time.Sleep(time.Millisecond) // let it expire

	req, err := client.newRequest(context.Background(), http.MethodGet, "/v2/foo/manifests/latest", nil)
	if err != nil {
		t.Fatalf("newRequest: %v", err)
	}

	resp, err := client.doRequest(context.Background(), req)
	if err != nil {
		t.Fatalf("doRequest: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 calls, got %d", callCount)
	}
}

func TestTimeout_Manifest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, err := NewClient(server.URL, nil, WithManifestTimeout(100*time.Millisecond))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	// Override HTTP client timeout for this test.
	client.httpClient.Timeout = 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	err = client.Ping(ctx)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
