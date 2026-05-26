package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func setupLogHandler(t *testing.T) (*LogHandler, func()) {
	t.Helper()
	database := setupTestDB(t)
	svc := service.NewLogService(database, "")
	h := NewLogHandler(svc)
	return h, func() { database.Close() }
}

func TestLogHandler_HandleLogList_CrawlType(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=crawl", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
	// No crawl logs seeded in setupTestDB, so should be empty but non-nil.
	if resp.Data == nil {
		t.Fatal("expected non-nil data")
	}
}

func TestLogHandler_HandleLogList_DownloadType(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
	// setupTestDB seeds 3 files.
	if len(resp.Data) != 3 {
		t.Fatalf("expected 3 download entries, got %d", len(resp.Data))
	}
}

func TestLogHandler_HandleLogList_AppType(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=app", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
	// No log file configured — should return empty.
	if len(resp.Data) != 0 {
		t.Fatalf("expected 0 app entries, got %d", len(resp.Data))
	}
}

func TestLogHandler_HandleLogList_InvalidType(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=invalid", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLogHandler_HandleLogList_DefaultParams(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	// No type param — should default to "crawl".
	req := authedRequest(http.MethodGet, model.RouteAdminLogs, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success, got message: %s", resp.Message)
	}
}

func TestLogHandler_HandleLogList_WithLimit(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=download&limit=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 entry with limit=1, got %d", len(resp.Data))
	}
}

func TestLogHandler_HandleLogList_XTotalCount(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	// Download type has 3 seeded files.
	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=download", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	totalHeader := rec.Header().Get("X-Total-Count")
	if totalHeader != "3" {
		t.Fatalf("expected X-Total-Count=3, got %q", totalHeader)
	}
}

func TestLogHandler_HandleLogList_XTotalCountWithLimit(t *testing.T) {
	h, cleanup := setupLogHandler(t)
	defer cleanup()

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminLogs, h.HandleLogList)
	handler := wrapWithAuth(mux)

	// limit=1 should still return total=3 in header.
	req := authedRequest(http.MethodGet, model.RouteAdminLogs+"?type=download&limit=1", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	totalHeader := rec.Header().Get("X-Total-Count")
	if totalHeader != "3" {
		t.Fatalf("expected X-Total-Count=3, got %q", totalHeader)
	}
	var resp model.ApiResponse[[]model.LogEntry]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 entry with limit=1, got %d", len(resp.Data))
	}
}
