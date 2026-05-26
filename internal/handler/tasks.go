package handler

import (
	"fmt"
	"net/http"

	"github.com/Mi-Bee-Studio/mibeehive/internal/model"
	"github.com/Mi-Bee-Studio/mibeehive/internal/service"
)

// TaskHandler handles task center API endpoints.
type TaskHandler struct {
	taskService *service.TaskService
}

// NewTaskHandler creates a new TaskHandler.
func NewTaskHandler(svc *service.TaskService) *TaskHandler {
	return &TaskHandler{taskService: svc}
}

// HandleTaskList handles GET /api/v1/admin/tasks.
func (h *TaskHandler) HandleTaskList(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.taskService.GetAllTasks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, model.ApiResponse[any]{
			Success: false,
			Message: fmt.Sprintf("failed to get tasks: %v", err),
		})
		return
	}

	if tasks == nil {
		tasks = []model.Task{}
	}

	writeJSON(w, http.StatusOK, model.ApiResponse[[]model.Task]{
		Success: true,
		Data:    tasks,
	})
}
