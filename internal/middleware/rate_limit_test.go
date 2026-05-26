package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func testHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
}

func TestRateLimit_RequestsUnderLimitPass(t *testing.T) {
	rl := RateLimit(5, time.Minute, 15*time.Minute)
	handler := rl(testHandler())

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_SixthRequestReturns429(t *testing.T) {
	rl := RateLimit(5, time.Minute, 15*time.Minute)
	handler := rl(testHandler())

	// Exhaust the 5 attempts.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// 6th request should be rejected.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if h := rec.Header().Get("Retry-After"); h == "" {
		t.Error("expected Retry-After header")
	}
	body := rec.Body.String()
	if body == "" {
		t.Error("expected response body")
	}
	want := `{"success":false,"message":"too many requests"}`
	if body != want+"\n" && body != want {
		t.Errorf("unexpected body: %q", body)
	}
}

func TestRateLimit_LockoutExpires(t *testing.T) {
	rl := RateLimit(5, time.Minute, 100*time.Millisecond)
	handler := rl(testHandler())

	// Exhaust 5 attempts.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Confirm locked.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429 during lockout, got %d", rec.Code)
	}

	// Wait for lockout to expire.
	time.Sleep(150 * time.Millisecond)

	// Should work again.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 after lockout expiry, got %d", rec.Code)
	}
}

func TestRateLimit_DifferentIPsTrackedIndependently(t *testing.T) {
	rl := RateLimit(5, time.Minute, 15*time.Minute)
	handler := rl(testHandler())

	// Exhaust for IP 1.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP 2 should still work.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "5.6.7.8:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for different IP, got %d", rec.Code)
	}

	// IP 1 should be blocked.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for original IP, got %d", rec.Code)
	}
}

func TestRateLimit_NonLoginPathsNotRateLimited(t *testing.T) {
	rl := RateLimit(1, time.Minute, 15*time.Minute)
	handler := rl(testHandler())

	paths := []struct {
		path   string
		method string
	}{
		{"/api/v1/auth/login", http.MethodGet},
		{"/api/v1/admin/projects", http.MethodPost},
		{"/api/v1/admin/projects", http.MethodGet},
		{"/webdav/file.txt", http.MethodPut},
		{"/static/style.css", http.MethodGet},
	}

	for _, p := range paths {
		req := httptest.NewRequest(p.method, p.path, nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("path=%s method=%s: expected 200, got %d", p.path, p.method, rec.Code)
		}
	}
}

func TestRateLimit_WindowReset(t *testing.T) {
	rl := RateLimit(5, 100*time.Millisecond, 15*time.Minute)
	handler := rl(testHandler())

	// Use 3 attempts.
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Wait for window to expire.
	time.Sleep(150 * time.Millisecond)

	// Should have 5 fresh attempts.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d after window reset: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestRateLimit_MapPruning(t *testing.T) {
	rl := RateLimit(5, 50*time.Millisecond, 50*time.Millisecond)
	handler := rl(testHandler())

	// Create entries for multiple IPs.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = fmt.Sprintf("1.2.3.%d:1234", i)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Wait for entries to expire.
	time.Sleep(100 * time.Millisecond)

	// Send 100 requests with a new IP to trigger pruning.
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Verify 10.0.0.1 is locked out (100 requests, 5 max, so locked).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 after 100+ attempts, got %d", rec.Code)
	}

	// Old IPs should be gone — new request from them should work.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.0:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for previously pruned IP, got %d", rec.Code)
	}
}

func TestRateLimit_MaxIPsCap(t *testing.T) {
	rl := RateLimit(5, time.Minute, 15*time.Minute)
	handler := rl(testHandler())

	// Fill map to maxIPs (100).
	for i := 0; i < maxIPs; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = fmt.Sprintf("192.168.%d.%d:1234", i/256, i%256)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("request %d: expected 200, got %d", i, rec.Code)
		}
	}

	// One more IP should still work (eviction should happen).
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for new IP after map full, got %d", rec.Code)
	}
}

func TestRateLimit_ExtractIP(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"1.2.3.4:1234", "1.2.3.4"},
		{"[::1]:8080", "::1"},
		{"no-port", "no-port"},
	}
	for _, tt := range tests {
		got := extractIP(tt.input)
		if got != tt.want {
			t.Errorf("extractIP(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- EndpointRateLimit tests for refresh endpoint ---

func TestEndpointRateLimit_RefreshUnderLimitPass(t *testing.T) {
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)
	handler := rl(testHandler())

	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, rec.Code)
		}
	}
}

func TestEndpointRateLimit_Refresh21stRequestReturns429(t *testing.T) {
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)
	handler := rl(testHandler())

	// Exhaust 20 attempts.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// 21st should be rejected.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", rec.Code)
	}
	if h := rec.Header().Get("Retry-After"); h == "" {
		t.Error("expected Retry-After header on 429 response")
	}
}

func TestEndpointRateLimit_RefreshDoesNotAffectOtherPaths(t *testing.T) {
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 1, time.Minute, 5*time.Minute)
	handler := rl(testHandler())

	// Send many requests to a different path — all should pass.
	for i := 0; i < 50; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d to /login: expected 200, got %d", i+1, rec.Code)
		}
	}

	// GET to /refresh should also pass (only POST is limited).
	req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("GET /refresh: expected 200, got %d", rec.Code)
	}
}

func TestEndpointRateLimit_DifferentIPsIndependent(t *testing.T) {
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)
	handler := rl(testHandler())

	// Exhaust 20 for IP 1.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// IP 2 should still work.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "5.6.7.8:5678"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for different IP, got %d", rec.Code)
	}

	// IP 1 should be blocked.
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for original IP, got %d", rec.Code)
	}
}

func TestEndpointRateLimit_RetryAfterHeader(t *testing.T) {
	lockout := 5 * time.Minute
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 2, 100*time.Millisecond, lockout)
	handler := rl(testHandler())

	// Exhaust 2 attempts.
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Next request should get 429 with Retry-After ~ lockout seconds.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Fatal("expected Retry-After header")
	}
	// Should be approximately lockout duration in seconds (300).
	if retryAfter != "300" {
		t.Errorf("expected Retry-After=300, got %s", retryAfter)
	}
}

func TestEndpointRateLimit_WindowReset(t *testing.T) {
	rl := EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, 100*time.Millisecond, 5*time.Minute)
	handler := rl(testHandler())

	// Use 10 attempts.
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
	}

	// Wait for window to expire.
	time.Sleep(150 * time.Millisecond)

	// Should have 20 fresh attempts.
	for i := 0; i < 20; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("request %d after window reset: expected 200, got %d", i+1, rec.Code)
		}
	}
}

// --- Combined: login and refresh rate limits can coexist ---

func TestEndpointRateLimit_LoginAndRefreshIndependent(t *testing.T) {
	loginRL := RateLimit(5, time.Minute, 15*time.Minute)
	refreshRL := EndpointRateLimit("/api/v1/auth/refresh", "POST", 20, time.Minute, 5*time.Minute)

	handler := refreshRL(loginRL(testHandler()))

	// Exhaust login for IP.
	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
		req.RemoteAddr = "1.2.3.4:1234"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("login request %d: expected 200, got %d", i+1, rec.Code)
		}
	}

	// Login should be blocked.
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429 for login, got %d", rec.Code)
	}

	// Refresh should still work (not affected by login limiter).
	req = httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for refresh (separate limiter), got %d", rec.Code)
	}
}
