package middleware

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

func TestWriteError_BasicStructure(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusNotFound, "NOT_FOUND", "resource not found", nil)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
	if resp.ErrorCode != "NOT_FOUND" {
		t.Errorf("expected error code 'NOT_FOUND', got %q", resp.ErrorCode)
	}
	if resp.Message != "resource not found" {
		t.Errorf("expected message 'resource not found', got %q", resp.Message)
	}
}

func TestWriteError_InternalServerError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusInternalServerError, "INTERNAL_ERROR", "something went wrong", errors.New("db connection failed"))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Error("expected success=false")
	}
}

func TestWriteError_Unauthorized(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusUnauthorized, "UNAUTHORIZED", "invalid credentials", nil)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.ErrorCode != "UNAUTHORIZED" {
		t.Errorf("expected error code 'UNAUTHORIZED', got %q", resp.ErrorCode)
	}
}

func TestWriteError_BadRequest(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusBadRequest, "VALIDATION", "invalid input", nil)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", rec.Code)
	}

	var resp model.ApiResponse[any]
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.ErrorCode != "VALIDATION" {
		t.Errorf("expected error code 'VALIDATION', got %q", resp.ErrorCode)
	}
	if resp.Message != "invalid input" {
		t.Errorf("expected message 'invalid input', got %q", resp.Message)
	}
}

func TestWriteError_WithNilError(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusForbidden, "FORBIDDEN", "access denied", nil)

	var resp model.ApiResponse[any]
	json.NewDecoder(rec.Body).Decode(&resp)

	if resp.ErrorCode != "FORBIDDEN" {
		t.Errorf("expected error code 'FORBIDDEN', got %q", resp.ErrorCode)
	}
	if resp.Message != "access denied" {
		t.Errorf("expected message 'access denied', got %q", resp.Message)
	}
}

func TestWriteError_WithDataField(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteError(rec, http.StatusConflict, "DUPLICATE", "item already exists", nil)

	var resp model.ApiResponse[any]
	json.NewDecoder(rec.Body).Decode(&resp)

	// Data field should be nil/zero for error responses.
	if resp.Data != nil {
		t.Errorf("expected nil data for error response, got %v", resp.Data)
	}
}
