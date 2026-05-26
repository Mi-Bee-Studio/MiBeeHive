package middleware

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
)

// WriteError logs the full error (if provided) and writes a sanitized JSON error response.
func WriteError(w http.ResponseWriter, status int, code, userMsg string, err error) {
	if err != nil {
		slog.Error("API error", "code", code, "user_message", userMsg, "error", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(model.ApiResponse[any]{
		Success:   false,
		ErrorCode: code,
		Message:   userMsg,
	})
}
