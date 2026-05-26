package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// === DecodeJSON Tests ===

func TestDecodeJSON_Valid(t *testing.T) {
	body := `{"name":"test","value":42}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	err := DecodeJSON(req, &result)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.Name != "test" {
		t.Fatalf("expected name='test', got %q", result.Name)
	}
	if result.Value != 42 {
		t.Fatalf("expected value=42, got %d", result.Value)
	}
}

func TestDecodeJSON_EmptyBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(""))

	var result struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Fatal("expected error for empty body, got nil")
	}
}

func TestDecodeJSON_NilBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	// Force nil Body to test the defensive check.
	req.Body = nil

	var result struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Fatal("expected error for nil body, got nil")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Fatalf("expected error mentioning 'empty', got: %v", err)
	}
}

func TestDecodeJSON_Malformed(t *testing.T) {
	body := `{this is not json}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		Name string `json:"name"`
	}
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestDecodeJSON_OversizedBody(t *testing.T) {
	// Create a body larger than the 1MB limit.
	large := make([]byte, 2<<20) // 2MB
	for i := range large {
		large[i] = 'a'
	}
	body := `{"data":"` + string(large) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var result struct {
		Data string `json:"data"`
	}
	err := DecodeJSON(req, &result)
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
}

func TestDecodeJSON_NullBody(t *testing.T) {
	body := `null`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var result *struct{}
	err := DecodeJSON(req, &result)
	if err != nil {
		t.Fatalf("expected no error for null body, got: %v", err)
	}
	if result != nil {
		t.Fatal("expected nil result for null body")
	}
}

// === ParseIntID Tests ===

func TestParseIntID_Valid(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/42", nil)
	req.SetPathValue("id", "42")

	id, err := ParseIntID(req, "id")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if id != 42 {
		t.Fatalf("expected id=42, got %d", id)
	}
}

func TestParseIntID_MissingParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	// Don't set path value — simulates missing parameter.

	_, err := ParseIntID(req, "id")
	if err == nil {
		t.Fatal("expected error for missing param, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Fatalf("expected error mentioning 'missing', got: %v", err)
	}
}

func TestParseIntID_NonNumeric(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/abc", nil)
	req.SetPathValue("id", "abc")

	_, err := ParseIntID(req, "id")
	if err == nil {
		t.Fatal("expected error for non-numeric id, got nil")
	}
}

func TestParseIntID_Zero(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/0", nil)
	req.SetPathValue("id", "0")

	_, err := ParseIntID(req, "id")
	if err == nil {
		t.Fatal("expected error for zero id, got nil")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected error mentioning 'positive', got: %v", err)
	}
}

func TestParseIntID_Negative(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/-1", nil)
	req.SetPathValue("id", "-1")

	_, err := ParseIntID(req, "id")
	if err == nil {
		t.Fatal("expected error for negative id, got nil")
	}
	if !strings.Contains(err.Error(), "positive") {
		t.Fatalf("expected error mentioning 'positive', got: %v", err)
	}
}

func TestParseIntID_DifferentParam(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test/99", nil)
	req.SetPathValue("project_id", "99")

	id, err := ParseIntID(req, "project_id")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if id != 99 {
		t.Fatalf("expected id=99, got %d", id)
	}
}

// === WriteError Tests ===

func TestWriteError_StatusAndContentType(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusBadRequest, "invalid input")

	result := w.Result()
	if result.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", result.StatusCode)
	}
	ct := result.Header.Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("expected Content-Type=application/json, got %q", ct)
	}
}

func TestWriteError_BodyFormat(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusNotFound, "resource not found")

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
	if resp.Message != "resource not found" {
		t.Fatalf("expected message='resource not found', got %q", resp.Message)
	}
}

func TestWriteError_EmptyMessage(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusInternalServerError, "")

	var resp model.ApiResponse[any]
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Success {
		t.Fatal("expected success=false")
	}
}

func TestWriteError_DifferentStatusCodes(t *testing.T) {
	tests := []struct {
		status int
		msg    string
	}{
		{http.StatusBadRequest, "bad request"},
		{http.StatusUnauthorized, "unauthorized"},
		{http.StatusForbidden, "forbidden"},
		{http.StatusNotFound, "not found"},
		{http.StatusConflict, "conflict"},
		{http.StatusInternalServerError, "internal error"},
		{http.StatusServiceUnavailable, "unavailable"},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			w := httptest.NewRecorder()
			WriteError(w, tt.status, tt.msg)

			result := w.Result()
			if result.StatusCode != tt.status {
				t.Fatalf("expected %d, got %d", tt.status, result.StatusCode)
			}

			var resp model.ApiResponse[any]
			if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
				t.Fatalf("failed to decode response: %v", err)
			}
			if resp.Success {
				t.Fatal("expected success=false")
			}
			if resp.Message != tt.msg {
				t.Fatalf("expected message=%q, got %q", tt.msg, resp.Message)
			}
		})
	}
}

func TestWriteError_ResponseBody(t *testing.T) {
	w := httptest.NewRecorder()
	WriteError(w, http.StatusTeapot, "short and stout")

	body := w.Body.String()
	if !strings.Contains(body, "short and stout") {
		t.Fatalf("response body should contain message, got: %s", body)
	}
	if !strings.Contains(body, "false") {
		t.Fatalf("response body should contain success=false, got: %s", body)
	}
}

// === ParsePagination Tests ===

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	limit, offset := ParsePagination(req)
	if limit != 20 {
		t.Fatalf("expected default limit=20, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected default offset=0, got %d", offset)
	}
}

func TestParsePagination_CustomValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=50&offset=10", nil)

	limit, offset := ParsePagination(req)
	if limit != 50 {
		t.Fatalf("expected limit=50, got %d", limit)
	}
	if offset != 10 {
		t.Fatalf("expected offset=10, got %d", offset)
	}
}

func TestParsePagination_MaxLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=500", nil)

	limit, offset := ParsePagination(req)
	if limit != 100 {
		t.Fatalf("expected limit clamped to 100, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0, got %d", offset)
	}
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=-5", nil)

	limit, offset := ParsePagination(req)
	// Negative limit should be ignored (uses default).
	if limit != 20 {
		t.Fatalf("expected default limit=20 for negative value, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0, got %d", offset)
	}
}

func TestParsePagination_ZeroOffset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?offset=0", nil)

	limit, offset := ParsePagination(req)
	if limit != 20 {
		t.Fatalf("expected limit=20, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0, got %d", offset)
	}
}

func TestParsePagination_NegativeOffset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?offset=-1", nil)

	limit, offset := ParsePagination(req)
	// Negative offset should be ignored (uses default).
	if limit != 20 {
		t.Fatalf("expected limit=20, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0 for negative value, got %d", offset)
	}
}

func TestParsePagination_InvalidValues(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=abc&offset=xyz", nil)

	limit, offset := ParsePagination(req)
	// Invalid values should be ignored (use defaults).
	if limit != 20 {
		t.Fatalf("expected default limit=20 for invalid value, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected default offset=0, got %d", offset)
	}
}

func TestParsePagination_OnlyLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=5", nil)

	limit, offset := ParsePagination(req)
	if limit != 5 {
		t.Fatalf("expected limit=5, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0, got %d", offset)
	}
}

func TestParsePagination_OnlyOffset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?offset=25", nil)

	limit, offset := ParsePagination(req)
	if limit != 20 {
		t.Fatalf("expected limit=20, got %d", limit)
	}
	if offset != 25 {
		t.Fatalf("expected offset=25, got %d", offset)
	}
}

func TestParsePagination_ExactMaxLimit(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?limit=100", nil)

	limit, offset := ParsePagination(req)
	if limit != 100 {
		t.Fatalf("expected limit=100, got %d", limit)
	}
	if offset != 0 {
		t.Fatalf("expected offset=0, got %d", offset)
	}
}

func TestParsePagination_LargeOffset(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test?offset=999999", nil)

	_, offset := ParsePagination(req)
	if offset != 999999 {
		t.Fatalf("expected offset=999999, got %d", offset)
	}
}
