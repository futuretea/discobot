package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/store"
)

// ListSkills lists all skills for a project.
// GET /api/projects/{projectId}/skills
func (h *Handler) ListSkills(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	skills, err := h.skillService.ListSkills(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list skills")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"skills": skills})
}

// GetSkill returns a single skill by ID.
// GET /api/projects/{projectId}/skills/{skillId}
func (h *Handler) GetSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	skillID := chi.URLParam(r, "skillId")
	if skillID == "" {
		h.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	skill, err := h.skillService.GetSkill(r.Context(), skillID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get skill")
		return
	}

	if skill.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	h.JSON(w, http.StatusOK, skill)
}

// CreateSkillRequest is the request body for creating a skill.
type CreateSkillRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
}

// CreateSkill creates a new skill.
// POST /api/projects/{projectId}/skills
func (h *Handler) CreateSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req CreateSkillRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	skill, err := h.skillService.CreateSkill(r.Context(), service.CreateSkillParams{
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
	})
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusCreated, skill)
}

// UpdateSkillRequest is the request body for updating a skill.
type UpdateSkillRequest struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Content     string `json:"content,omitempty"`
}

// UpdateSkill updates an existing skill.
// PUT /api/projects/{projectId}/skills/{skillId}
func (h *Handler) UpdateSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	skillID := chi.URLParam(r, "skillId")
	if skillID == "" {
		h.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	// Verify the skill belongs to this project before updating.
	existing, err := h.skillService.GetSkill(r.Context(), skillID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get skill")
		return
	}
	if existing.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	var req UpdateSkillRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	skill, err := h.skillService.UpdateSkill(r.Context(), skillID, service.UpdateSkillParams{
		Name:        req.Name,
		Description: req.Description,
		Content:     req.Content,
	})
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, skill)
}

// DeleteSkill deletes a skill.
// DELETE /api/projects/{projectId}/skills/{skillId}
func (h *Handler) DeleteSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	skillID := chi.URLParam(r, "skillId")
	if skillID == "" {
		h.Error(w, http.StatusBadRequest, "skillId is required")
		return
	}

	// Verify the skill belongs to this project before deleting.
	existing, err := h.skillService.GetSkill(r.Context(), skillID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to get skill")
		return
	}
	if existing.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	if err := h.skillService.DeleteSkill(r.Context(), skillID); err != nil {
		if errors.Is(err, service.ErrSkillInUse) {
			var inUseErr *service.SkillInUseByAgentsError
			if errors.As(err, &inUseErr) {
				h.JSON(w, http.StatusConflict, map[string]any{
					"error":      "Skill is in use by one or more agents",
					"agentIds":   inUseErr.AgentIDs,
					"agentTypes": inUseErr.AgentTypes,
				})
			} else {
				h.JSON(w, http.StatusConflict, map[string]any{
					"error": "Skill is in use by one or more agents",
				})
			}
			return
		}
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete skill")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ImportSkillRequest is the request body for importing a skill from the market cache.
type ImportSkillRequest struct {
	RepoURL    string `json:"repoUrl"`
	Branch     string `json:"branch"`
	SkillsPath string `json:"path"`
	SkillID    string `json:"skillId"`
}

// ImportSkill imports a skill from the local git-clone market cache.
// POST /api/projects/{projectId}/skills/import
func (h *Handler) ImportSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req ImportSkillRequest
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Apply config defaults for missing fields.
	if req.RepoURL == "" {
		req.RepoURL = h.cfg.SkillMarketRepoURL
	}
	if req.Branch == "" {
		req.Branch = h.cfg.SkillMarketRepoBranch
	}
	if req.SkillsPath == "" {
		req.SkillsPath = h.cfg.SkillMarketRepoPath
	}

	skill, err := h.skillService.ImportSkillFromMarket(r.Context(), projectID, service.ImportSkillRequest{
		RepoURL:    req.RepoURL,
		Branch:     req.Branch,
		SkillsPath: req.SkillsPath,
		SkillID:    req.SkillID,
	})
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusCreated, skill)
}

// ListAgentSkills lists skills attached to an agent.
// GET /api/projects/{projectId}/agents/{agentId}/skills
func (h *Handler) ListAgentSkills(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentId")
	if agentID == "" {
		h.Error(w, http.StatusBadRequest, "agentId is required")
		return
	}

	skills, err := h.skillService.GetAgentSkills(r.Context(), agentID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list agent skills")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"skills": skills})
}

// AttachSkill attaches a skill to an agent.
// POST /api/projects/{projectId}/agents/{agentId}/skills/{skillId}
func (h *Handler) AttachSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	agentID := chi.URLParam(r, "agentId")
	skillID := chi.URLParam(r, "skillId")
	if agentID == "" || skillID == "" {
		h.Error(w, http.StatusBadRequest, "agentId and skillId are required")
		return
	}

	// Verify skill belongs to this project.
	existing, err := h.skillService.GetSkill(r.Context(), skillID)
	if err != nil || existing.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	if err := h.skillService.AttachSkill(r.Context(), agentID, skillID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to attach skill")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

// DetachSkill detaches a skill from an agent.
// DELETE /api/projects/{projectId}/agents/{agentId}/skills/{skillId}
func (h *Handler) DetachSkill(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	agentID := chi.URLParam(r, "agentId")
	skillID := chi.URLParam(r, "skillId")
	if agentID == "" || skillID == "" {
		h.Error(w, http.StatusBadRequest, "agentId and skillId are required")
		return
	}

	// Verify skill belongs to this project.
	existing, err := h.skillService.GetSkill(r.Context(), skillID)
	if err != nil || existing.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill not found")
		return
	}

	if err := h.skillService.DetachSkill(r.Context(), agentID, skillID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to detach skill")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

// ListSkillMarket returns a listing of skills from the configured (or provided) git repo.
// GET /api/skill-market?repoUrl=...&branch=...&path=...&reload=true
// Pass reload=true to force a "git pull" before listing; otherwise the local cache is used.
func (h *Handler) ListSkillMarket(w http.ResponseWriter, r *http.Request) {
	repoURL := r.URL.Query().Get("repoUrl")
	if repoURL == "" {
		repoURL = h.cfg.SkillMarketRepoURL
	}
	if repoURL == "" {
		h.Error(w, http.StatusBadRequest, "No skill market repo URL configured")
		return
	}

	branch := r.URL.Query().Get("branch")
	if branch == "" {
		branch = h.cfg.SkillMarketRepoBranch
	}

	skillsPath := r.URL.Query().Get("path")
	if skillsPath == "" {
		skillsPath = h.cfg.SkillMarketRepoPath
	}

	forceUpdate := r.URL.Query().Get("reload") == "true"

	skills, err := h.skillService.ListSkillMarket(r.Context(), repoURL, branch, skillsPath, forceUpdate)
	if err != nil {
		h.Error(w, http.StatusBadGateway, "Failed to fetch skill market: "+err.Error())
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"skills": skills})
}

// --- Skill Market Repo handlers ---

// getSkillMarketRepoForProject fetches a skill market repo and verifies project ownership.
func (h *Handler) getSkillMarketRepoForProject(w http.ResponseWriter, r *http.Request, repoID, projectID string) *service.SkillMarketRepoRecord {
	repo, err := h.skillMarketRepoService.GetSkillMarketRepo(r.Context(), repoID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "Skill market repo not found")
		} else {
			h.Error(w, http.StatusInternalServerError, "Failed to get skill market repo")
		}
		return nil
	}
	if repo.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "Skill market repo not found")
		return nil
	}
	return repo
}

// ListSkillMarketRepos lists all skill market repos for a project.
// GET /api/projects/{projectId}/skill-market-repos
func (h *Handler) ListSkillMarketRepos(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	repos, err := h.skillMarketRepoService.ListSkillMarketRepos(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list skill market repos")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"repos": repos})
}

// CreateSkillMarketRepo creates a new skill market repo for a project.
// POST /api/projects/{projectId}/skill-market-repos
func (h *Handler) CreateSkillMarketRepo(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req service.CreateSkillMarketRepoParams
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ProjectID = projectID

	repo, err := h.skillMarketRepoService.CreateSkillMarketRepo(r.Context(), req)
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusCreated, repo)
}

// UpdateSkillMarketRepo updates an existing skill market repo.
// PUT /api/projects/{projectId}/skill-market-repos/{repoId}
func (h *Handler) UpdateSkillMarketRepo(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	repoID := chi.URLParam(r, "repoId")

	if h.getSkillMarketRepoForProject(w, r, repoID, projectID) == nil {
		return
	}

	var req service.UpdateSkillMarketRepoParams
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	repo, err := h.skillMarketRepoService.UpdateSkillMarketRepo(r.Context(), repoID, req)
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, repo)
}

// DeleteSkillMarketRepo deletes a skill market repo.
// DELETE /api/projects/{projectId}/skill-market-repos/{repoId}
func (h *Handler) DeleteSkillMarketRepo(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	repoID := chi.URLParam(r, "repoId")

	if h.getSkillMarketRepoForProject(w, r, repoID, projectID) == nil {
		return
	}

	if err := h.skillMarketRepoService.DeleteSkillMarketRepo(r.Context(), repoID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to delete skill market repo")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
