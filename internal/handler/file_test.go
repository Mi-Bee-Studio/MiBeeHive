package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func TestRetry_Success(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Set file ID 3 (seeded with status "pending") to "error" status so retry is valid.
	_, err := database.Exec("UPDATE files SET status = 'error' WHERE id = 3")
	if err != nil {
		t.Fatalf("failed to update file status: %v", err)
	}

	tmpDir := t.TempDir()
	fileService := service.NewFileService(database, tmpDir, 2, nil)
	h := NewFileHandler(database, fileService, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminFileRetry, h.Retry)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/files/3/retry", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[string]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}

func TestRetry_FileNotFound(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminFileRetry, h.Retry)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/files/999/retry", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRetry_InvalidID(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminFileRetry, h.Retry)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/files/abc/retry", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRetry_WrongStatus(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// File ID 1 is seeded with status "complete" — retry should reject it.
	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminFileRetry, h.Retry)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/files/1/retry", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-error status, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRetry_FailedPermanentStatus(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Set file to failed_permanent — retry should accept it.
	_, err := database.Exec("UPDATE files SET status = 'failed_permanent' WHERE id = 3")
	if err != nil {
		t.Fatalf("failed to update file status: %v", err)
	}

	tmpDir := t.TempDir()
	fileService := service.NewFileService(database, tmpDir, 2, nil)
	h := NewFileHandler(database, fileService, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("POST "+model.RouteAdminFileRetry, h.Retry)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodPost, "/api/v1/admin/files/3/retry", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for failed_permanent retry, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestListQueue(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// File 3 is originally pending — it's in the queue.

	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileQueue, h.ListQueue)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteFileQueue, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[[]model.FileResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 file in queue (pending), got %d", len(resp.Data))
	}
}

func TestQueueStats(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	h := NewFileHandler(database, nil, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileQueueStats, h.QueueStats)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteFileQueueStats, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[*model.QueueStatsResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil data")
	}
	// Seeded data: 2 complete, 1 pending → stats should reflect that.
	if resp.Data.Complete != 2 {
		t.Fatalf("expected 2 complete, got %d", resp.Data.Complete)
	}
	if resp.Data.Pending != 1 {
		t.Fatalf("expected 1 pending, got %d", resp.Data.Pending)
	}
}

func TestQueueProgress(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	tmpDir := t.TempDir()
	fileService := service.NewFileService(database, tmpDir, 2, nil)
	h := NewFileHandler(database, fileService, testJWTSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteFileQueueProgress, h.QueueProgress)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteFileQueueProgress, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[map[int64]*model.DownloadProgressResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatal("expected success=true")
	}
}
