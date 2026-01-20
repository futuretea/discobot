package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/octobot/server/internal/middleware"
	"github.com/obot-platform/octobot/server/internal/sandbox/sandboxapi"
)

// GetFile returns a single file with diff content
func (h *Handler) GetFile(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement - this will use git service
	h.Error(w, http.StatusNotImplemented, "Not yet implemented")
}

// GetSuggestions returns autocomplete suggestions
func (h *Handler) GetSuggestions(w http.ResponseWriter, _ *http.Request) {
	// TODO: Implement path/repo suggestions
	h.JSON(w, http.StatusOK, map[string]any{"suggestions": []any{}})
}

// ============================================================================
// Session File Endpoints
// ============================================================================

// ListSessionFiles lists directory contents for a session's workspace.
// GET /api/projects/{projectId}/sessions/{sessionId}/files?path=.&hidden=true
func (h *Handler) ListSessionFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	// Path defaults to "." (root)
	path := r.URL.Query().Get("path")
	if path == "" {
		path = "."
	}

	// Parse hidden flag
	includeHidden := r.URL.Query().Get("hidden") == "true"

	result, err := h.chatService.ListFiles(ctx, projectID, sessionID, path, includeHidden)
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

// ReadSessionFile reads a file from a session's workspace.
// GET /api/projects/{projectId}/sessions/{sessionId}/files/read?path=...
func (h *Handler) ReadSessionFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" {
		h.Error(w, http.StatusBadRequest, "path query parameter required")
		return
	}

	result, err := h.chatService.ReadFile(ctx, projectID, sessionID, path)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "Invalid path") {
			status = http.StatusBadRequest
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// WriteSessionFile writes a file to a session's workspace.
// PUT /api/projects/{projectId}/sessions/{sessionId}/files/write
func (h *Handler) WriteSessionFile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	var req sandboxapi.WriteFileRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Path == "" {
		h.Error(w, http.StatusBadRequest, "path is required")
		return
	}

	result, err := h.chatService.WriteFile(ctx, projectID, sessionID, &req)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "Invalid path") {
			status = http.StatusBadRequest
		} else if strings.Contains(err.Error(), "Permission denied") {
			status = http.StatusForbidden
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}

// GetSessionDiff returns diff information for a session's workspace.
// GET /api/projects/{projectId}/sessions/{sessionId}/diff?format=files&path=...
func (h *Handler) GetSessionDiff(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	projectID := middleware.GetProjectID(ctx)
	sessionID := chi.URLParam(r, "sessionId")

	if sessionID == "" {
		h.Error(w, http.StatusBadRequest, "sessionId is required")
		return
	}

	path := r.URL.Query().Get("path")
	format := r.URL.Query().Get("format")

	result, err := h.chatService.GetDiff(ctx, projectID, sessionID, path, format)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			status = http.StatusNotFound
		} else if strings.Contains(err.Error(), "Invalid path") {
			status = http.StatusBadRequest
		}
		h.Error(w, status, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, result)
}
