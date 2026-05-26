package health

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

// HealthHandler serves the GET /health endpoint.
type HealthHandler struct {
	db        *sql.DB
	version   string
	startTime time.Time
}

// NewHealthHandler creates a new HealthHandler.
func NewHealthHandler(db *sql.DB, version string) *HealthHandler {
	return &HealthHandler{
		db:        db,
		version:   version,
		startTime: time.Now(),
	}
}

// healthResponse is the JSON payload returned by the health endpoint.
type healthResponse struct {
	Status  string `json:"status"`
	Uptime  string `json:"uptime"`
	Version string `json:"version"`
}

// ServeHTTP handles GET /health requests.
func (h *HealthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	uptime := time.Since(h.startTime).String()
	status := "ok"
	code := http.StatusOK

	if err := h.db.PingContext(ctx); err != nil {
		status = "degraded"
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(healthResponse{
		Status:  status,
		Uptime:  uptime,
		Version: h.version,
	})
}
