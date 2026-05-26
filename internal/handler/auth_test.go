package handler

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

const testSecret = "test-jwt-secret-key-12345"

func setupTestAuthHandler() *AuthHandler {
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	return NewAuthHandler(string(hash), testSecret)
}

func TestLoginSuccess(t *testing.T) {
	h := setupTestAuthHandler()

	body, _ := json.Marshal(model.LoginRequest{Password: "testpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp model.ApiResponse[model.LoginResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify token is valid
	token, err := jwt.Parse(resp.Data.Token, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("token validation failed: %v", err)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	h := setupTestAuthHandler()

	body, _ := json.Marshal(model.LoginRequest{Password: "wrongpass"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Fatal("expected success=false")
	}
}

func TestLoginMissingBody(t *testing.T) {
	h := setupTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("not json")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Login(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func generateTestToken(secret string, exp time.Time) string {
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": exp.Unix(),
		"iat": time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, _ := token.SignedString([]byte(secret))
	return s
}

func TestAuthMiddlewareValidToken(t *testing.T) {
	tokenStr := generateTestToken(testSecret, time.Now().Add(24*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := middleware.GetTokenClaims(r)
		if !ok {
			t.Fatal("expected claims in context")
		}
		if claims["sub"] != "admin" {
			t.Fatalf("expected sub=admin, got %v", claims["sub"])
		}
		w.WriteHeader(http.StatusOK)
	})

	middleware.AuthMiddleware(testSecret)(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddlewareMissingToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	middleware.AuthMiddleware(testSecret)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareExpiredToken(t *testing.T) {
	tokenStr := generateTestToken(testSecret, time.Now().Add(-1*time.Hour))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	})

	middleware.AuthMiddleware(testSecret)(next).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddlewareSkipsLogin(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	middleware.AuthMiddleware(testSecret)(next).ServeHTTP(rec, req)

	if !called {
		t.Fatal("expected next handler to be called for login path")
	}
}

func TestRefreshTokenSuccess(t *testing.T) {
	h := setupTestAuthHandler()

	// First, get a valid token via login.
	body, _ := json.Marshal(model.LoginRequest{Password: "testpass"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(body))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	h.Login(loginRec, loginReq)

	var loginResp model.ApiResponse[model.LoginResponse]
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	validToken := loginResp.Data.Token

	// Now refresh that token.
	refreshReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	refreshReq.Header.Set("Authorization", "Bearer "+validToken)
	refreshRec := httptest.NewRecorder()

	h.RefreshToken(refreshRec, refreshReq)

	if refreshRec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", refreshRec.Code)
	}

	var refreshResp model.ApiResponse[model.LoginResponse]
	if err := json.NewDecoder(refreshRec.Body).Decode(&refreshResp); err != nil {
		t.Fatalf("failed to decode refresh response: %v", err)
	}

	if !refreshResp.Success {
		t.Fatal("expected success=true")
	}
	if refreshResp.Data.Token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify new token is valid.
	token, err := jwt.Parse(refreshResp.Data.Token, func(token *jwt.Token) (interface{}, error) {
		return []byte(testSecret), nil
	})
	if err != nil || !token.Valid {
		t.Fatalf("refreshed token validation failed: %v", err)
	}

	// Verify new token has fresh iat.
	newClaims := token.Claims.(jwt.MapClaims)
	if iat, ok := newClaims["iat"].(float64); !ok || iat == 0 {
		t.Fatal("expected iat claim in refreshed token")
	}
}
func TestRefreshTokenExpiredRejected(t *testing.T) {
	h := setupTestAuthHandler()

	expiredToken := generateTestToken(testSecret, time.Now().Add(-1*time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+expiredToken)
	rec := httptest.NewRecorder()

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", rec.Code)
	}
}

func TestRefreshTokenNoAuth(t *testing.T) {
	h := setupTestAuthHandler()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	rec := httptest.NewRecorder()

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", rec.Code)
	}
}

func TestRefreshTokenInvalidSignature(t *testing.T) {
	h := setupTestAuthHandler()

	badToken := generateTestToken("wrong-secret", time.Now().Add(24*time.Hour))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+badToken)
	rec := httptest.NewRecorder()

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for wrong secret, got %d", rec.Code)
	}
}

func TestRefreshTokenRejectedAfterPasswordChange(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	pwdChangedAt := time.Now().Add(-30 * time.Minute)
	h := NewAuthHandler(string(hash), testSecret, func() time.Time {
		return pwdChangedAt
	})

	// Token issued before password change.
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
		"iat": float64(time.Now().Add(-1 * time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(testSecret))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for token issued before password change, got %d", rec.Code)
	}
}

func TestRefreshTokenAllowedAfterPasswordChange(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("testpass"), bcrypt.DefaultCost)
	pwdChangedAt := time.Now().Add(-30 * time.Minute)
	h := NewAuthHandler(string(hash), testSecret, func() time.Time {
		return pwdChangedAt
	})

	// Token issued after password change.
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
		"iat": float64(time.Now().Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(testSecret))

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()

	h.RefreshToken(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for token issued after password change, got %d", rec.Code)
	}
}
