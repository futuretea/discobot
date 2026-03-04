package handler

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/obot-platform/discobot/agent-go/mcp"
)

// ListMCPServers handles GET /mcp/servers.
// Returns the connection status of all configured MCP servers, including
// the OAuth authorization URL when a server is awaiting user authorization.
func (h *Handler) ListMCPServers(w http.ResponseWriter, r *http.Request) {
	mgr := h.mcpManager()
	if mgr == nil {
		h.JSON(w, http.StatusOK, []mcp.ServerInfo{})
		return
	}
	h.JSON(w, http.StatusOK, mgr.Status())
}

// GetMCPServerOAuth handles GET /mcp/servers/{name}/oauth.
// Returns the OAuth authorization URL for a server that is waiting for user
// authorization. Returns 404 if the server is not in oauth_required state.
func (h *Handler) GetMCPServerOAuth(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	mgr := h.mcpManager()
	if mgr == nil {
		h.Error(w, http.StatusNotFound, fmt.Sprintf("MCP server %q not found", name))
		return
	}
	url, ok := mgr.PendingOAuthURL(name)
	if !ok {
		h.Error(w, http.StatusNotFound, fmt.Sprintf("MCP server %q is not awaiting OAuth authorization", name))
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"authUrl": url})
}

// PostMCPServerOAuthCode handles POST /mcp/servers/{name}/oauth/code.
// Submits the OAuth authorization code (and state) to complete the OAuth flow
// for a server that is blocked waiting for user authorization.
// Body: { "code": "...", "state": "..." }
func (h *Handler) PostMCPServerOAuthCode(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var body struct {
		Code  string `json:"code"`
		State string `json:"state"`
	}
	if err := h.DecodeJSON(r, &body); err != nil {
		h.Error(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Code == "" {
		h.Error(w, http.StatusBadRequest, "code is required")
		return
	}

	mgr := h.mcpManager()
	if mgr == nil {
		h.Error(w, http.StatusNotFound, fmt.Sprintf("MCP server %q not found", name))
		return
	}

	if err := mgr.SubmitOAuthCode(name, body.Code, body.State); err != nil {
		h.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	h.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// mcpManager returns the MCP manager from the default agent, or nil if unavailable.
func (h *Handler) mcpManager() *mcp.Manager {
	if h.defaultAgent == nil {
		return nil
	}
	return h.defaultAgent.MCPManager()
}
