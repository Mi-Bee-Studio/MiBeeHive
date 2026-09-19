// Package handler — scheduled scripts admin API (#70).
//
// Routes (all behind admin auth middleware):
//
//	GET    /api/v1/admin/scripts            list tasks
//	POST   /api/v1/admin/scripts            create task
//	GET    /api/v1/admin/scripts/{id}       task detail + latest runs
//	PUT    /api/v1/admin/scripts/{id}       update task (fields optional)
//	DELETE /api/v1/admin/scripts/{id}       delete task + history
//	POST   /api/v1/admin/scripts/{id}/run   trigger a manual run (async)
//	GET    /api/v1/admin/scripts/{id}/runs  run history (paged)
//	GET    /api/v1/admin/scripts/{id}/content   read script body
//	PUT    /api/v1/admin/scripts/{id}/content   write script body
package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// ScriptHandler handles scheduled script admin endpoints.
type ScriptHandler struct {
	svc       *service.ScriptService
	scheduler *service.ScriptScheduler
}

// NewScriptHandler creates a ScriptHandler.
func NewScriptHandler(svc *service.ScriptService, scheduler *service.ScriptScheduler) *ScriptHandler {
	return &ScriptHandler{svc: svc, scheduler: scheduler}
}

func (h *ScriptHandler) scriptError(w http.ResponseWriter, err error, action string) {
	switch {
	case errors.Is(err, service.ErrScriptNotFound):
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{
			Success: false, Message: "script not found",
		})
	case errors.Is(err, service.ErrValidation):
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false, Message: err.Error(),
		})
	default:
		slog.Error("script handler error", "action", action, "error", err)
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false, Message: "internal error: " + action,
		})
	}
}

// List handles GET /api/v1/admin/scripts.
func (h *ScriptHandler) List(w http.ResponseWriter, r *http.Request) {
	scripts, err := h.svc.ListScripts(r.Context())
	if err != nil {
		h.scriptError(w, err, "list")
		return
	}
	if scripts == nil {
		scripts = []*service.ScheduledScript{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Data: map[string]any{
			"scripts":     scripts,
			"scripts_dir": h.svc.ScriptsDir(),
		},
	})
}

// Create handles POST /api/v1/admin/scripts.
func (h *ScriptHandler) Create(w http.ResponseWriter, r *http.Request) {
	var in service.ScriptInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{
			Success: false, Message: "invalid request body",
		})
		return
	}
	sc, err := h.svc.CreateScript(r.Context(), in)
	if err != nil {
		h.scriptError(w, err, "create")
		return
	}
	h.scheduler.Reschedule(sc)
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: sc})
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// Get handles GET /api/v1/admin/scripts/{id} (detail + latest runs).
func (h *ScriptHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	sc, err := h.svc.GetScript(r.Context(), id)
	if err != nil {
		h.scriptError(w, err, "get")
		return
	}
	runs, err := h.svc.ListRuns(r.Context(), id, 10, 0)
	if err != nil {
		h.scriptError(w, err, "get runs")
		return
	}
	if runs == nil {
		runs = []*service.ScriptRun{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{
		Success: true,
		Data:    map[string]any{"script": sc, "recent_runs": runs},
	})
}

// Update handles PUT /api/v1/admin/scripts/{id}.
func (h *ScriptHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	var in service.ScriptInput
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid request body"})
		return
	}
	sc, err := h.svc.UpdateScript(r.Context(), id, in)
	if err != nil {
		h.scriptError(w, err, "update")
		return
	}
	h.scheduler.Reschedule(sc)
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: sc})
}

// Delete handles DELETE /api/v1/admin/scripts/{id}.
func (h *ScriptHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	if err := h.svc.DeleteScript(r.Context(), id); err != nil {
		h.scriptError(w, err, "delete")
		return
	}
	h.scheduler.Remove(id)
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: nil})
}

// Run handles POST /api/v1/admin/scripts/{id}/run — starts an async run and
// returns the fresh "running" run row.
func (h *ScriptHandler) Run(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	run, err := h.svc.StartRun(r.Context(), id, service.ScriptTriggerManual)
	if err != nil {
		h.scriptError(w, err, "run")
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: run})
}

// Runs handles GET /api/v1/admin/scripts/{id}/runs?limit=&offset=.
func (h *ScriptHandler) Runs(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	runs, err := h.svc.ListRuns(r.Context(), id, limit, offset)
	if err != nil {
		h.scriptError(w, err, "list runs")
		return
	}
	if runs == nil {
		runs = []*service.ScriptRun{}
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: runs})
}

// GetContent handles GET /api/v1/admin/scripts/{id}/content (text/plain).
func (h *ScriptHandler) GetContent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	content, err := h.svc.GetScriptContent(r.Context(), id)
	if errors.Is(err, service.ErrScriptNotFound) {
		writeJSON(w, http.StatusNotFound, model.ApiResponse[any]{Success: false, Message: "script file not uploaded yet"})
		return
	}
	if err != nil {
		h.scriptError(w, err, "get content")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(content))
}

// PutContent handles PUT /api/v1/admin/scripts/{id}/content (raw body).
func (h *ScriptHandler) PutContent(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, model.ApiResponse[any]{Success: false, Message: "invalid id"})
		return
	}
	if err := h.svc.PutScriptContent(r.Context(), id, r.Body); err != nil {
		h.scriptError(w, err, "put content")
		return
	}
	writeJSON(w, http.StatusOK, model.ApiResponse[any]{Success: true, Data: nil})
}
