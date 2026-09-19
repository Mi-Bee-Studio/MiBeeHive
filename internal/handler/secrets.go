// Package handler — named secrets admin API (#70 follow-up).
//
// GET    /api/v1/admin/secrets           list names + updated_at (never values)
// PUT    /api/v1/admin/secrets/{name}    create/replace (write-only, body {"value": "..."})
// DELETE /api/v1/admin/secrets/{name}    remove
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// SecretHandler handles the secrets admin API.
type SecretHandler struct {
	svc *service.SecretService
}

// NewSecretHandler creates a SecretHandler.
func NewSecretHandler(svc *service.SecretService) *SecretHandler {
	return &SecretHandler{svc: svc}
}

func (h *SecretHandler) secretError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, service.ErrScriptNotFound):
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{Success: false, Message: "secret not found"})
	case errors.Is(err, service.ErrSecretValidation):
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: err.Error()})
	default:
		slog.Error("secret handler error", "action", action, "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{Success: false, Message: "internal error: " + action})
	}
}

// List handles GET /api/v1/admin/secrets.
func (h *SecretHandler) List(w http.ResponseWriter, r *http.Request) {
	metas, err := h.svc.ListSecrets(r.Context())
	if err != nil {
		h.secretError(w, err, "list")
		return
	}
	if metas == nil {
		metas = []*service.SecretMeta{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: metas})
}

// Put handles PUT /api/v1/admin/secrets/{name} — one-time write-only input.
func (h *SecretHandler) Put(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	var body struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid request body"})
		return
	}
	meta, err := h.svc.SetSecret(r.Context(), name, body.Value)
	if err != nil {
		h.secretError(w, err, "set")
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: meta})
}

// Delete handles DELETE /api/v1/admin/secrets/{name}.
func (h *SecretHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.svc.DeleteSecret(r.Context(), r.PathValue("name")); err != nil {
		h.secretError(w, err, "delete")
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: nil})
}
