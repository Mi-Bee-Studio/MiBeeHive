package health

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// openMemoryDB opens an in-memory SQLite database for testing.
func openMemoryDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestHealthHandler_Healthy(t *testing.T) {
	db := openMemoryDB(t)
	handler := NewHealthHandler(db, "v1.2.3")

	// Give startTime a moment to tick over for a non-zero uptime
	time.Sleep(time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type application/json, got %q", ct)
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "v1.2.3" {
		t.Errorf("expected version 'v1.2.3', got %q", resp.Version)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
	if !strings.HasSuffix(resp.Uptime, "s") {
		t.Errorf("expected uptime to end with 's', got %q", resp.Uptime)
	}
}

func TestHealthHandler_Degraded(t *testing.T) {
	db := openMemoryDB(t)
	// Close the DB immediately so PingContext fails.
	db.Close()

	handler := NewHealthHandler(db, "v1.2.3")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "degraded" {
		t.Errorf("expected status 'degraded', got %q", resp.Status)
	}
	if resp.Version != "v1.2.3" {
		t.Errorf("expected version 'v1.2.3', got %q", resp.Version)
	}
}

func TestHealthHandler_ContainsVersion(t *testing.T) {
	db := openMemoryDB(t)
	handler := NewHealthHandler(db, "abc123")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Version != "abc123" {
		t.Errorf("expected version 'abc123', got %q", resp.Version)
	}
}

func TestHealthHandler_UptimeFormat(t *testing.T) {
	db := openMemoryDB(t)
	handler := NewHealthHandler(db, "v1.0.0")

	// Sleep to ensure a measurable uptime.
	time.Sleep(5 * time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp healthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Uptime should be parseable as a Go duration.
	d, err := time.ParseDuration(resp.Uptime)
	if err != nil {
		t.Fatalf("uptime %q is not a valid duration: %v", resp.Uptime, err)
	}
	if d <= 0 {
		t.Errorf("expected positive uptime, got %s", resp.Uptime)
	}
}
