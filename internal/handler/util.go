package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// DecodeJSON decodes JSON from request body with a 1MB size limit.
// Returns an error if the body is empty, malformed, or exceeds the size limit.
func DecodeJSON(r *http.Request, v any) error {
	if r.Body == nil {
		return fmt.Errorf("request body is empty")
	}
	defer r.Body.Close()
	r.Body = http.MaxBytesReader(nil, r.Body, 1<<20) // 1MB limit
	return json.NewDecoder(r.Body).Decode(v)
}

// ParseIntID extracts and validates an integer ID from URL path parameters
// using Go 1.22+ ServeMux path parameters (r.PathValue).
// Returns an error if the parameter is missing, non-numeric, or non-positive.
func ParseIntID(r *http.Request, param string) (int64, error) {
	val := r.PathValue(param)
	if val == "" {
		return 0, fmt.Errorf("missing path parameter: %s", param)
	}
	id, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", param, err)
	}
	if id <= 0 {
		return 0, fmt.Errorf("%s must be positive", param)
	}
	return id, nil
}

// WriteError writes a JSON error response with the given status code and message.
// Uses the standard ApiResponse envelope with Success=false for consistency with
// other handler responses.
func WriteError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, model.ApiResponse[any]{
		Success: false,
		Message: msg,
	})
}

// ParsePagination extracts limit and offset from query parameters.
// Defaults: limit=20, offset=0. Limit is clamped to a maximum of 100.
func ParsePagination(r *http.Request) (limit, offset int) {
	limit = 20
	offset = 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 100 {
		limit = 100
	}

	if o := r.URL.Query().Get("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v >= 0 {
			offset = v
		}
	}
	return
}
