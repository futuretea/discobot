package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
)

// createEnvSetRequest is the request body for creating or updating an env set.
type createEnvSetRequest struct {
	Name    string            `json:"name"`
	EnvVars map[string]string `json:"envVars"`
}

// ListEnvSets returns all env sets for a project (metadata only, no secrets).
func (h *Handler) ListEnvSets(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	envSets, err := h.envSetService.List(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list env sets")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"envSets": envSets})
}

// CreateEnvSet creates a new env set.
func (h *Handler) CreateEnvSet(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req createEnvSetRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		h.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.EnvVars == nil {
		req.EnvVars = map[string]string{}
	}

	envSet, err := h.envSetService.Create(r.Context(), projectID, req.Name, req.EnvVars)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to create env set")
		return
	}

	h.JSON(w, http.StatusCreated, envSet)
}

// GetEnvSet returns a single env set with its decrypted env vars.
func (h *Handler) GetEnvSet(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	envSetID := chi.URLParam(r, "envSetId")

	envSet, err := h.envSetService.Get(r.Context(), projectID, envSetID)
	if err != nil {
		if errors.Is(err, service.ErrEnvSetNotFound) {
			h.Error(w, http.StatusNotFound, "Env set not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get env set")
		return
	}

	h.JSON(w, http.StatusOK, envSet)
}

// UpdateEnvSet updates an existing env set's name and env vars.
func (h *Handler) UpdateEnvSet(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	envSetID := chi.URLParam(r, "envSetId")

	var req createEnvSetRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Name == "" {
		h.Error(w, http.StatusBadRequest, "name is required")
		return
	}

	if req.EnvVars == nil {
		req.EnvVars = map[string]string{}
	}

	envSet, err := h.envSetService.Update(r.Context(), projectID, envSetID, req.Name, req.EnvVars)
	if err != nil {
		if errors.Is(err, service.ErrEnvSetNotFound) {
			h.Error(w, http.StatusNotFound, "Env set not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to update env set")
		return
	}

	h.JSON(w, http.StatusOK, envSet)
}

// DeleteEnvSet removes an env set.
func (h *Handler) DeleteEnvSet(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	envSetID := chi.URLParam(r, "envSetId")

	if err := h.envSetService.Delete(r.Context(), projectID, envSetID); err != nil {
		if errors.Is(err, service.ErrEnvSetNotFound) {
			h.Error(w, http.StatusNotFound, "Env set not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete env set")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}

// setSessionActiveEnvSetsRequest is the request body for setting the active env sets on a session.
type setSessionActiveEnvSetsRequest struct {
	// EnvSetIDs is the list of env set IDs to activate. Pass an empty array to clear all.
	EnvSetIDs []string `json:"envSetIds"`
}

// SetSessionActiveEnvSet sets the active env sets for a session.
// Ownership of the env set IDs is enforced by middleware.EnvSetsOwnedByProject
// before this handler is reached.
func (h *Handler) SetSessionActiveEnvSet(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "sessionId")

	var req setSessionActiveEnvSetsRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.EnvSetIDs == nil {
		req.EnvSetIDs = []string{}
	}

	if err := h.envSetService.SetSessionActiveEnvSets(r.Context(), sessionID, req.EnvSetIDs); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to set active env sets")
		return
	}

	h.JSON(w, http.StatusOK, map[string]bool{"success": true})
}
