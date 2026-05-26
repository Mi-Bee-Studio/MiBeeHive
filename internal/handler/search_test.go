package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

func TestSearchHandler_HandleSearch_ValidQuery(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := service.NewSearchService(database)
	h := NewSearchHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminSearch, h.HandleSearch)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminSearch+"?q=prometheus", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[*model.SearchResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil data")
	}
	if len(resp.Data.Projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(resp.Data.Projects))
	}
	if len(resp.Data.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Data.Files))
	}
}

func TestSearchHandler_HandleSearch_MissingQuery(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := service.NewSearchService(database)
	h := NewSearchHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminSearch, h.HandleSearch)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminSearch, nil)
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
		t.Fatal("expected success=false for missing query")
	}
}

func TestSearchHandler_HandleSearch_WithType(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := service.NewSearchService(database)
	h := NewSearchHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminSearch, h.HandleSearch)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminSearch+"?q=prometheus&type=file", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp model.ApiResponse[*model.SearchResponse]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if !resp.Success {
		t.Fatalf("expected success=true, got message: %s", resp.Message)
	}
	// Should only have files, no projects
	if len(resp.Data.Files) != 3 {
		t.Fatalf("expected 3 files, got %d", len(resp.Data.Files))
	}
	if len(resp.Data.Projects) != 0 {
		t.Fatalf("expected 0 projects with type=file, got %d", len(resp.Data.Projects))
	}
	if resp.Data.Total != 3 {
		t.Fatalf("expected total 3, got %d", resp.Data.Total)
	}
}

func TestSearchHandler_HandleSearch_XTotalCountHeader(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	svc := service.NewSearchService(database)
	h := NewSearchHandler(svc)

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+model.RouteAdminSearch, h.HandleSearch)
	handler := wrapWithAuth(mux)

	req := authedRequest(http.MethodGet, model.RouteAdminSearch+"?q=prometheus", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	totalHeader := rec.Header().Get("X-Total-Count")
	if totalHeader != "4" {
		t.Fatalf("expected X-Total-Count=4 (1 project + 3 files), got %q", totalHeader)
	}
}
