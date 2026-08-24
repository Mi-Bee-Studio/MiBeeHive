package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/Mi-Bee-Studio/mibeehive/internal/config"
	"github.com/Mi-Bee-Studio/mibeehive/internal/crawler"
	"github.com/Mi-Bee-Studio/mibeehive/internal/db"
	"github.com/Mi-Bee-Studio/mibeehive/internal/middleware"
	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

func setupConfigHandler(t *testing.T, database *sql.DB) (*ConfigHandler, *config.Config) {
	t.Helper()
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: "$2a$10$abcdefghijklmnopqrstuvwxABCDEFGH",
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: t.TempDir()},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}
	configPath := t.TempDir() + "/config.yaml"
	return NewConfigHandler(cfg, configPath), cfg
}

func registerConfigRoutes(mux *http.ServeMux, h *ConfigHandler) {
	mux.HandleFunc("POST "+model.RouteAdminPasswordChange, h.ChangePassword)
	mux.HandleFunc("GET "+model.RouteAdminConfigMonitor, h.GetMonitorConfig)
	mux.HandleFunc("PUT "+model.RouteAdminConfigMonitor, h.UpdateMonitorConfig)
}

// === Admin Password Change Tests ===

func TestAdminChangePassword_Success(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	// Create a config with a known password hash.
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.DefaultCost)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: string(hash),
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: t.TempDir()},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}
	configPath := t.TempDir() + "/config.yaml"

	h := NewConfigHandler(cfg, configPath)
	mux := http.NewServeMux()
	registerConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass123",
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/password", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify the password was actually changed.
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.Auth.PasswordHash), []byte("newpass123")); err != nil {
		t.Fatal("expected new password to match")
	}
}

func TestAdminChangePassword_WrongOldPassword(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _ := setupConfigHandler(t, database)
	mux := http.NewServeMux()
	registerConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		OldPassword: "wrongoldpass",
		NewPassword: "newpass123",
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/password", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminChangePassword_MissingFields(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _ := setupConfigHandler(t, database)
	mux := http.NewServeMux()
	registerConfigRoutes(mux, h)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.ChangePasswordRequest{
		OldPassword: "oldpass",
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/password", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

// === Password Change Token Invalidation Tests ===

func TestAdminChangePassword_InvalidatesOldToken(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	// Create a config with a known password and password_changed_at in the past.
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.DefaultCost)
	oldPwdChangedAt := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash:      string(hash),
			JWTSecret:         testJWTSecret,
			PasswordChangedAt: oldPwdChangedAt,
		},
		Storage: config.StorageConfig{BasePath: t.TempDir()},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}
	tmpDir := t.TempDir()
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	fileService := service.NewFileService(database, service.NewStorageResolver(&config.Config{Storage: config.StorageConfig{BasePath: tmpDir}}), 2, nil)
	cm := crawler.NewCrawlManager(database, fileService, cfg, logger, nil)

	projectRepo := db.NewProjectRepo(database)
	configPath := t.TempDir() + "/config.yaml"

	configH := NewConfigHandler(cfg, configPath)
	projectH := NewProjectAdminHandler(projectRepo, db.NewFileRepo(database), db.NewCrawlLogRepo(database), cm, cfg)

	// Step 1: Create a token issued 30 minutes ago (after initial password_changed_at, but before the upcoming change).
	oldTokenClaims := jwt.MapClaims{
		"sub": "admin",
		"exp": float64(time.Now().Add(24 * time.Hour).Unix()),
		"iat": float64(time.Now().Add(-30 * time.Minute).Unix()),
	}
	oldToken := jwt.NewWithClaims(jwt.SigningMethodHS256, oldTokenClaims)
	oldTokenString, _ := oldToken.SignedString([]byte(testJWTSecret))

	// Step 2: Verify old token works before password change (token iat is after initial password_changed_at).
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminProjectsList, projectH.ListProjects)
	getPasswordChangedAt := cfg.Auth.GetPasswordChangedAt
	authMw := middleware.AuthMiddleware(testJWTSecret, getPasswordChangedAt)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	req.Header.Set("Authorization", "Bearer "+oldTokenString)
	rec := httptest.NewRecorder()
	authMw(mux).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for old token before password change, got %d", rec.Code)
	}

	// Step 3: Change the password — this sets password_changed_at to now, which is after the token's iat.
	body, _ := json.Marshal(model.ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass123",
	})
	pwdReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/password", bytes.NewReader(body))
	pwdReq.Header.Set("Authorization", "Bearer "+oldTokenString)
	pwdReq.Header.Set("Content-Type", "application/json")
	pwdRec := httptest.NewRecorder()
	authMw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configH.ChangePassword(w, r)
	})).ServeHTTP(pwdRec, pwdReq)

	if pwdRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for password change, got %d: %s", pwdRec.Code, pwdRec.Body.String())
	}

	// Step 4: Verify old token is now rejected (token iat is 30min ago, password_changed_at is now).
	authMwUpdated := middleware.AuthMiddleware(testJWTSecret, getPasswordChangedAt)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	req2.Header.Set("Authorization", "Bearer "+oldTokenString)
	rec2 := httptest.NewRecorder()
	authMwUpdated(mux).ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for old token after password change, got %d", rec2.Code)
	}

	// Step 5: Login with new password and verify new token works.
	authH := NewAuthHandler(cfg.Auth.PasswordHash, testJWTSecret, getPasswordChangedAt)
	loginBody, _ := json.Marshal(model.LoginRequest{Password: "newpass123"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	authH.Login(loginRec, loginReq)

	if loginRec.Code != http.StatusOK {
		t.Fatalf("expected 200 for login with new password, got %d: %s", loginRec.Code, loginRec.Body.String())
	}

	var loginResp model.ApiResponse[model.LoginResponse]
	if err := json.NewDecoder(loginRec.Body).Decode(&loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}

	// Step 6: Verify the new token works with auth middleware.
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/admin/projects", nil)
	req3.Header.Set("Authorization", "Bearer "+loginResp.Data.Token)
	rec3 := httptest.NewRecorder()
	authMwUpdated(mux).ServeHTTP(rec3, req3)

	if rec3.Code != http.StatusOK {
		t.Fatalf("expected 200 for new token after password change, got %d", rec3.Code)
	}
}

// TestAdminChangePassword_SaveFailRollback verifies that when the config file
// cannot be saved, the in-memory password hash is NOT changed (atomicity).
func TestAdminChangePassword_SaveFailRollback(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	// Create a config with a known password hash.
	hash, _ := bcrypt.GenerateFromPassword([]byte("oldpass"), bcrypt.DefaultCost)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			PasswordHash: string(hash),
			JWTSecret:    "test-jwt-secret-key-12345",
		},
		Storage: config.StorageConfig{BasePath: t.TempDir()},
		Crawler: config.CrawlerConfig{MaxConcurrent: 2, DefaultInterval: "6h"},
	}

	// Create a read-only directory so config save fails.
	configDir := t.TempDir()
	configPath := configDir + "/config.yaml"
	// Write initial config so the file exists.
	if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
		t.Fatalf("writing initial config: %v", err)
	}
	// Make file read-only so save fails.
	if err := os.Chmod(configPath, 0444); err != nil {
		t.Fatalf("chmod config: %v", err)
	}

	h := NewConfigHandler(cfg, configPath)
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminPasswordChange, h.ChangePassword)
	handler := wrapWithAuth(mux)

	originalHash := cfg.Auth.PasswordHash

	body, _ := json.Marshal(model.ChangePasswordRequest{
		OldPassword: "oldpass",
		NewPassword: "newpass123",
	})
	req := authedRequest(http.MethodPost, "/api/v1/admin/password", body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Should fail with 500 (config save failure).
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify in-memory hash is unchanged.
	if cfg.Auth.PasswordHash != originalHash {
		t.Error("expected in-memory password hash to be unchanged after save failure")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(cfg.Auth.PasswordHash), []byte("oldpass")); err != nil {
		t.Error("expected old password to still match after failed save")
	}
}

// === Monitor Config Tests ===

func TestMonitorConfig_Get(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, cfg := setupConfigHandler(t, database)
	// Set default monitor config values on the config
	cfg.Monitor = config.MonitorConfig{
		DiskWarningPercent:  90,
		DiskCriticalPercent: 95,
		DiskCheckEnabled:    true,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminConfigMonitor, h.GetMonitorConfig)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminConfigMonitor, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[model.MonitorConfigResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data.DiskWarningPercent != 90 {
		t.Fatalf("expected disk_warning_percent=90, got %d", resp.Data.DiskWarningPercent)
	}
	if resp.Data.DiskCriticalPercent != 95 {
		t.Fatalf("expected disk_critical_percent=95, got %d", resp.Data.DiskCriticalPercent)
	}
	if !resp.Data.DiskCheckEnabled {
		t.Fatal("expected disk_check_enabled=true")
	}
}

func TestMonitorConfig_Put_Valid(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _ := setupConfigHandler(t, database)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT "+model.RouteAdminConfigMonitor, h.UpdateMonitorConfig)
	mux.HandleFunc("GET "+model.RouteAdminConfigMonitor, h.GetMonitorConfig)
	handler := wrapWithAuth(mux)

	// PUT valid config
	body, _ := json.Marshal(model.MonitorConfigRequest{
		DiskWarningPercent:  85,
		DiskCriticalPercent: 95,
		DiskCheckEnabled:    true,
	})
	req := authedRequest(http.MethodPut, model.RouteAdminConfigMonitor, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var putResp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&putResp); err != nil {
		t.Fatalf("failed to decode put response: %v", err)
	}
	if !putResp.Success {
		t.Fatal("expected success=true")
	}

	// Follow with GET to verify values persisted
	req = authedRequest(http.MethodGet, model.RouteAdminConfigMonitor, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET, got %d: %s", rec.Code, rec.Body.String())
	}

	var getResp model.ApiResponse[model.MonitorConfigResponse]
	if err := json.NewDecoder(rec.Body).Decode(&getResp); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getResp.Data.DiskWarningPercent != 85 {
		t.Fatalf("expected disk_warning_percent=85 after update, got %d", getResp.Data.DiskWarningPercent)
	}
	if getResp.Data.DiskCriticalPercent != 95 {
		t.Fatalf("expected disk_critical_percent=95 after update, got %d", getResp.Data.DiskCriticalPercent)
	}
}

func TestMonitorConfig_Put_InvalidRange(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _ := setupConfigHandler(t, database)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT "+model.RouteAdminConfigMonitor, h.UpdateMonitorConfig)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.MonitorConfigRequest{
		DiskWarningPercent:  0,
		DiskCriticalPercent: 95,
		DiskCheckEnabled:    true,
	})
	req := authedRequest(http.MethodPut, model.RouteAdminConfigMonitor, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}

func TestMonitorConfig_Put_WarningExceedsCritical(t *testing.T) {
	database := setupAdminTestDB(t)
	defer database.Close()

	h, _ := setupConfigHandler(t, database)

	mux := http.NewServeMux()
	mux.HandleFunc("PUT "+model.RouteAdminConfigMonitor, h.UpdateMonitorConfig)
	handler := wrapWithAuth(mux)

	body, _ := json.Marshal(model.MonitorConfigRequest{
		DiskWarningPercent:  95,
		DiskCriticalPercent: 90,
		DiskCheckEnabled:    true,
	})
	req := authedRequest(http.MethodPut, model.RouteAdminConfigMonitor, body)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}
