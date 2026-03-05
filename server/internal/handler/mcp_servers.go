package handler

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/middleware"
	"github.com/obot-platform/discobot/server/internal/service"
	"github.com/obot-platform/discobot/server/internal/store"
)

// getMCPServerForProject fetches an MCP server and verifies it belongs to the
// given project. Writes the appropriate error response and returns nil on failure.
func (h *Handler) getMCPServerForProject(w http.ResponseWriter, r *http.Request, serverID, projectID string) *service.MCPServer {
	srv, err := h.mcpServerService.GetMCPServer(r.Context(), serverID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "MCP server not found")
		} else {
			h.Error(w, http.StatusInternalServerError, "Failed to get MCP server")
		}
		return nil
	}
	if srv.ProjectID != projectID {
		h.Error(w, http.StatusNotFound, "MCP server not found")
		return nil
	}
	return srv
}

// ListMCPServers lists all MCP servers for a project.
// GET /api/projects/{projectId}/mcp-servers
func (h *Handler) ListMCPServers(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	servers, err := h.mcpServerService.ListMCPServers(r.Context(), projectID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list MCP servers")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"servers": servers})
}

// GetMCPServer returns a single MCP server by ID.
// GET /api/projects/{projectId}/mcp-servers/{mcpServerId}
func (h *Handler) GetMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	serverID := chi.URLParam(r, "mcpServerId")
	if serverID == "" {
		h.Error(w, http.StatusBadRequest, "mcpServerId is required")
		return
	}

	server := h.getMCPServerForProject(w, r, serverID, projectID)
	if server == nil {
		return
	}

	h.JSON(w, http.StatusOK, server)
}

// CreateMCPServer creates a new MCP server.
// POST /api/projects/{projectId}/mcp-servers
func (h *Handler) CreateMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())

	var req service.CreateMCPServerParams
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	req.ProjectID = projectID

	server, err := h.mcpServerService.CreateMCPServer(r.Context(), req)
	if err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusCreated, server)
}

// UpdateMCPServer updates an existing MCP server.
// PUT /api/projects/{projectId}/mcp-servers/{mcpServerId}
func (h *Handler) UpdateMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	serverID := chi.URLParam(r, "mcpServerId")
	if serverID == "" {
		h.Error(w, http.StatusBadRequest, "mcpServerId is required")
		return
	}

	if h.getMCPServerForProject(w, r, serverID, projectID) == nil {
		return
	}

	var req service.UpdateMCPServerParams
	if err := h.DecodeJSON(r, &req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	server, err := h.mcpServerService.UpdateMCPServer(r.Context(), serverID, req)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, server)
}

// DeleteMCPServer deletes an MCP server.
// DELETE /api/projects/{projectId}/mcp-servers/{mcpServerId}
func (h *Handler) DeleteMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	serverID := chi.URLParam(r, "mcpServerId")
	if serverID == "" {
		h.Error(w, http.StatusBadRequest, "mcpServerId is required")
		return
	}

	if h.getMCPServerForProject(w, r, serverID, projectID) == nil {
		return
	}

	if err := h.mcpServerService.DeleteMCPServer(r.Context(), serverID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			h.Error(w, http.StatusNotFound, "MCP server not found")
			return
		}
		h.Error(w, http.StatusInternalServerError, "Failed to delete MCP server")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ListAgentMCPServers lists MCP servers attached to an agent.
// GET /api/projects/{projectId}/agents/{agentId}/mcp-servers
func (h *Handler) ListAgentMCPServers(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agentId")
	if agentID == "" {
		h.Error(w, http.StatusBadRequest, "agentId is required")
		return
	}

	servers, err := h.mcpServerService.GetAgentMCPServers(r.Context(), agentID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to list agent MCP servers")
		return
	}

	h.JSON(w, http.StatusOK, map[string]any{"servers": servers})
}

// AttachMCPServer attaches an MCP server to an agent.
// POST /api/projects/{projectId}/agents/{agentId}/mcp-servers/{mcpServerId}
func (h *Handler) AttachMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	agentID := chi.URLParam(r, "agentId")
	serverID := chi.URLParam(r, "mcpServerId")
	if agentID == "" || serverID == "" {
		h.Error(w, http.StatusBadRequest, "agentId and mcpServerId are required")
		return
	}

	if h.getMCPServerForProject(w, r, serverID, projectID) == nil {
		return
	}

	if err := h.mcpServerService.AttachMCPServer(r.Context(), agentID, serverID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to attach MCP server")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

// DetachMCPServer detaches an MCP server from an agent.
// DELETE /api/projects/{projectId}/agents/{agentId}/mcp-servers/{mcpServerId}
func (h *Handler) DetachMCPServer(w http.ResponseWriter, r *http.Request) {
	projectID := middleware.GetProjectID(r.Context())
	agentID := chi.URLParam(r, "agentId")
	serverID := chi.URLParam(r, "mcpServerId")
	if agentID == "" || serverID == "" {
		h.Error(w, http.StatusBadRequest, "agentId and mcpServerId are required")
		return
	}

	if h.getMCPServerForProject(w, r, serverID, projectID) == nil {
		return
	}

	if err := h.mcpServerService.DetachMCPServer(r.Context(), agentID, serverID); err != nil {
		h.Error(w, http.StatusInternalServerError, "Failed to detach MCP server")
		return
	}

	h.JSON(w, http.StatusOK, map[string]string{"status": "detached"})
}
