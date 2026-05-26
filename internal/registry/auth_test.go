package registry

import (
	"encoding/base64"
	"testing"
	"time"
)

func TestParseWWWAuthenticate_Basic(t *testing.T) {
	challenge, err := parseWWWAuthenticate(`Basic realm="test"`)
	if err != nil {
		t.Fatalf("parseWWWAuthenticate: %v", err)
	}
	if challenge.Scheme != "Basic" {
		t.Errorf("Scheme = %q, want Basic", challenge.Scheme)
	}
	if challenge.Realm != "test" {
		t.Errorf("Realm = %q, want test", challenge.Realm)
	}
}

func TestParseWWWAuthenticate_Bearer(t *testing.T) {
	input := `Bearer realm="https://auth.example.com/token",service="registry",scope="repository:foo:pull"`
	challenge, err := parseWWWAuthenticate(input)
	if err != nil {
		t.Fatalf("parseWWWAuthenticate: %v", err)
	}
	if challenge.Scheme != "Bearer" {
		t.Errorf("Scheme = %q, want Bearer", challenge.Scheme)
	}
	if challenge.Realm != "https://auth.example.com/token" {
		t.Errorf("Realm = %q, want https://auth.example.com/token", challenge.Realm)
	}
	if challenge.Service != "registry" {
		t.Errorf("Service = %q, want registry", challenge.Service)
	}
	if challenge.Scope != "repository:foo:pull" {
		t.Errorf("Scope = %q, want repository:foo:pull", challenge.Scope)
	}
}

func TestParseWWWAuthenticate_Unsupported(t *testing.T) {
	_, err := parseWWWAuthenticate("Digest realm=\"test\"")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestParseWWWAuthenticate_NoParams(t *testing.T) {
	challenge, err := parseWWWAuthenticate("Bearer")
	if err != nil {
		t.Fatalf("parseWWWAuthenticate: %v", err)
	}
	if challenge.Scheme != "Bearer" {
		t.Errorf("Scheme = %q, want Bearer", challenge.Scheme)
	}
	if challenge.Realm != "" {
		t.Errorf("Realm = %q, want empty", challenge.Realm)
	}
}

func TestTokenCache_SetGet(t *testing.T) {
	cache := NewTokenCache()
	cache.Set("repo:foo:pull", "token-abc", 60*time.Minute)

	token, ok := cache.Get("repo:foo:pull")
	if !ok {
		t.Fatal("expected token to be found")
	}
	if token != "token-abc" {
		t.Errorf("token = %q, want token-abc", token)
	}
}

func TestTokenCache_Expired(t *testing.T) {
	cache := NewTokenCache()
	// Set with very short duration; buffer = 0.5s, expires after 0.5s.
	cache.Set("repo:foo:pull", "token-abc", 1*time.Second)

	time.Sleep(600 * time.Millisecond)

	_, ok := cache.Get("repo:foo:pull")
	if ok {
		t.Fatal("expected expired token to not be found")
	}
}

func TestTokenCache_DifferentScopes(t *testing.T) {
	cache := NewTokenCache()
	cache.Set("scope-a", "token-a", 60*time.Minute)
	cache.Set("scope-b", "token-b", 60*time.Minute)

	token, ok := cache.Get("scope-a")
	if !ok || token != "token-a" {
		t.Errorf("scope-a: token=%q, ok=%v", token, ok)
	}

	token, ok = cache.Get("scope-b")
	if !ok || token != "token-b" {
		t.Errorf("scope-b: token=%q, ok=%v", token, ok)
	}

	_, ok = cache.Get("scope-c")
	if ok {
		t.Error("expected scope-c to not exist")
	}
}

func TestAuthenticateBasic(t *testing.T) {
	creds := &Credentials{Username: "user", Password: "pass"}
	result := authenticateBasic(creds)

	expected := "Basic " + encodeBase64("user:pass")
	if result != expected {
		t.Errorf("authenticateBasic = %q, want %q", result, expected)
	}
}

func TestErrors(t *testing.T) {
	t.Run("AuthError", func(t *testing.T) {
		err := NewAuthError(401, "unauthorized")
		if err.StatusCode != 401 {
			t.Errorf("StatusCode = %d, want 401", err.StatusCode)
		}
		if err.Error() == "" {
			t.Error("Error() should not be empty")
		}
	})

	t.Run("NotFoundError", func(t *testing.T) {
		err := NewNotFoundError(404, "not found")
		if err.StatusCode != 404 {
			t.Errorf("StatusCode = %d, want 404", err.StatusCode)
		}
	})

	t.Run("RateLimitError", func(t *testing.T) {
		err := NewRateLimitError(429, "rate limited", 30*time.Second)
		if err.StatusCode != 429 {
			t.Errorf("StatusCode = %d, want 429", err.StatusCode)
		}
		if err.RetryAfter != 30*time.Second {
			t.Errorf("RetryAfter = %v, want 30s", err.RetryAfter)
		}
	})

	t.Run("RegistryError with detail", func(t *testing.T) {
		err := &RegistryError{StatusCode: 500, Message: "internal error", Detail: "db timeout"}
		msg := err.Error()
		if msg == "" {
			t.Error("Error() should not be empty")
		}
	})

	t.Run("RegistryError without detail", func(t *testing.T) {
		err := &RegistryError{StatusCode: 500, Message: "internal error"}
		msg := err.Error()
		if msg == "" {
			t.Error("Error() should not be empty")
		}
	})
}

func TestParseRetryAfter(t *testing.T) {
	tests := []struct {
		input string
		want  time.Duration
	}{
		{"30", 30 * time.Second},
		{"0", 0},
		{"", 60 * time.Second},
		{"invalid", 60 * time.Second},
	}
	for _, tt := range tests {
		got := parseRetryAfter(tt.input)
		if got != tt.want {
			t.Errorf("parseRetryAfter(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestTokenCache_BufferLogic(t *testing.T) {
	cache := NewTokenCache()
	// 10 minute token should have 5 min buffer, so expires after 5 min.
	cache.Set("scope", "tok", 10*time.Minute)

	ct, ok := cache.tokens["scope"]
	if !ok {
		t.Fatal("token not cached")
	}

	expectedExpiry := 5 * time.Minute // 10min - 5min buffer = 5 min
	actualExpiry := ct.expires.Sub(time.Now())
	diff := actualExpiry - expectedExpiry
	if diff < -time.Second || diff > time.Second {
		t.Errorf("expiry diff = %v, expected ~0", diff)
	}
}

// encodeBase64 is a test helper.
func encodeBase64(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
