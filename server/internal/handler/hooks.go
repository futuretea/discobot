package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
)

// GetHooksStatus returns hook evaluation status for a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/status
func (h *Handler) GetHooksStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	result, err := h.chatService.GetHooksStatus(ctx, projectID, sessionID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetHookOutput returns the output log for a specific hook in a session's sandbox.
// GET /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/output
func (h *Handler) GetHookOutput(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	result, err := h.chatService.GetHookOutput(ctx, projectID, sessionID, hookID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// RerunHook manually reruns a specific hook in a session's sandbox.
// POST /api/projects/{projectId}/sessions/{sessionId}/hooks/{hookId}/rerun
func (h *Handler) RerunHook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")
	hookID := chi.URLParam(r, "hookId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	result, err := h.chatService.RerunHook(ctx, projectID, sessionID, hookID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}
