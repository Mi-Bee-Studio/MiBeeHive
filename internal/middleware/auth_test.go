package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestAuthMiddleware_ValidToken(t *testing.T) {
	secret := "test-secret-key-12345"

	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(9999999999),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := AuthMiddleware(secret)(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_InvalidToken(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware("correct-secret")(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rec.Code)
	}
}

func TestAuthMiddleware_WrongSecret(t *testing.T) {
	secret1 := "secret-one"
	secret2 := "secret-two"

	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(9999999999),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret1))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware(secret2)(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for wrong secret, got %d", rec.Code)
	}
}

func TestAuthMiddleware_LoginSkipped(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mw := AuthMiddleware("any-secret")(handler)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("login should skip auth, expected 200, got %d", rec.Code)
	}
}

func TestAuthMiddleware_MissingAuth(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware("any-secret")(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing auth, got %d", rec.Code)
	}
}

func TestAuthMiddleware_ClaimsExtracted(t *testing.T) {
	secret := "test-secret"

	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(9999999999),
		"role": "superuser",
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	var extractedClaims jwt.MapClaims
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		extractedClaims, _ = GetTokenClaims(r)
		w.WriteHeader(http.StatusOK)
	})

	mw := AuthMiddleware(secret)(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if extractedClaims == nil {
		t.Fatal("claims should be extracted into context")
	}
	if extractedClaims["sub"] != "admin" {
		t.Errorf("expected sub=admin, got %v", extractedClaims["sub"])
	}
	if extractedClaims["role"] != "superuser" {
		t.Errorf("expected role=superuser, got %v", extractedClaims["role"])
	}
}

func TestAuthMiddleware_TokenInvalidatedAfterPasswordChange(t *testing.T) {
	secret := "test-secret-key"

	// Create a token issued 1 hour ago.
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
		"iat": float64(time.Now().Add(-1 * time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Password was changed 30 minutes ago — token issued before that should be rejected.
	passwordChangedAt := time.Now().Add(-30 * time.Minute)
	mw := AuthMiddleware(secret, func() time.Time {
		return passwordChangedAt
	})(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for token issued before password change, got %d", rec.Code)
	}
}

func TestAuthMiddleware_TokenValidAfterPasswordChange(t *testing.T) {
	secret := "test-secret-key"

	// Create a token issued just now (after password change).
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
		"iat": float64(time.Now().Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Password was changed 30 minutes ago — token issued after that should be valid.
	passwordChangedAt := time.Now().Add(-30 * time.Minute)
	mw := AuthMiddleware(secret, func() time.Time {
		return passwordChangedAt
	})(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for token issued after password change, got %d", rec.Code)
	}
}

func TestAuthMiddleware_NoIatClaim_AllowedWhenPasswordChanged(t *testing.T) {
	secret := "test-secret-key"

	// Token without iat claim (legacy token).
	claims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Password was changed but token has no iat — should be allowed through.
	passwordChangedAt := time.Now().Add(-30 * time.Minute)
	mw := AuthMiddleware(secret, func() time.Time {
		return passwordChangedAt
	})(handler)

	req := httptest.NewRequest("GET", "/api/v1/projects", nil)
	req.Header.Set("Authorization", "Bearer "+tokenString)
	rec := httptest.NewRecorder()
	mw.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 for token without iat, got %d", rec.Code)
	}
}

func TestAuthMiddleware_RefreshSkipped(t *testing.T) {
  handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
  })

  mw := AuthMiddleware("any-secret")(handler)

  req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
  rec := httptest.NewRecorder()
  mw.ServeHTTP(rec, req)

  if rec.Code != http.StatusOK {
    t.Errorf("refresh should skip auth, expected 200, got %d", rec.Code)
  }
}
