package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestBasicAuthMiddleware(t *testing.T) {
	password := "testpass123"
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("failed to generate bcrypt hash: %v", err)
	}
	hashStr := string(hash)

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mw := BasicAuthMiddleware(hashStr)(next)

	tests := []struct {
		name        string
		method      string
		setupAuth   bool
		authUser    string
		authPass    string
		wantStatus  int
		wantWWWAuth bool
	}{
		{"Anonymous GET passes", "GET", false, "", "", http.StatusOK, false},
		{"Anonymous HEAD passes", "HEAD", false, "", "", http.StatusOK, false},
		{"Anonymous PROPFIND passes", "PROPFIND", false, "", "", http.StatusOK, false},
		{"OPTIONS passes without auth", "OPTIONS", false, "", "", http.StatusOK, false},
		{"Anonymous PUT blocked", "PUT", false, "", "", http.StatusUnauthorized, true},
		{"Anonymous DELETE blocked", "DELETE", false, "", "", http.StatusUnauthorized, true},
		{"Anonymous MKCOL blocked", "MKCOL", false, "", "", http.StatusUnauthorized, true},
		{"Correct auth PUT passes", "PUT", true, "admin", password, http.StatusOK, false},
		{"Wrong password PUT blocked", "PUT", true, "admin", "wrongpass", http.StatusUnauthorized, true},
		{"Wrong username PUT blocked", "PUT", true, "notadmin", password, http.StatusUnauthorized, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.setupAuth {
				req.SetBasicAuth(tt.authUser, tt.authPass)
			}
			rec := httptest.NewRecorder()
			mw.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if tt.wantWWWAuth {
				if got := rec.Header().Get("WWW-Authenticate"); got == "" {
					t.Error("expected WWW-Authenticate header, got empty")
				}
			}
		})
	}
}
