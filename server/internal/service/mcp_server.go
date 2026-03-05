package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

// MCPServer represents an MCP server configuration for API responses.
type MCPServer struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Type        string    `json:"type"` // "stdio" or "http"
	Command     string    `json:"command,omitempty"`
	Args        []string  `json:"args,omitempty"`
	Env         []string  `json:"env,omitempty"`
	URL         string    `json:"url,omitempty"`
	Headers     []string  `json:"headers,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// MCPServerService handles MCP server operations.
type MCPServerService struct {
	store *store.Store
}

// NewMCPServerService creates a new MCP server service.
func NewMCPServerService(s *store.Store) *MCPServerService {
	return &MCPServerService{store: s}
}

// ListMCPServers returns all MCP servers for a project.
func (s *MCPServerService) ListMCPServers(ctx context.Context, projectID string) ([]*MCPServer, error) {
	dbServers, err := s.store.ListMCPServersByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list MCP servers: %w", err)
	}

	servers := make([]*MCPServer, len(dbServers))
	for i, srv := range dbServers {
		servers[i] = mapMCPServer(srv)
	}
	return servers, nil
}

// GetMCPServer returns a single MCP server by ID.
func (s *MCPServerService) GetMCPServer(ctx context.Context, id string) (*MCPServer, error) {
	srv, err := s.store.GetMCPServerByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}
	return mapMCPServer(srv), nil
}

// CreateMCPServerParams holds the input for creating an MCP server.
type CreateMCPServerParams struct {
	ProjectID   string `json:"-"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type"` // "stdio" or "http"
	// stdio fields
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`
	Env     []string `json:"env,omitempty"`
	// http fields
	URL     string   `json:"url,omitempty"`
	Headers []string `json:"headers,omitempty"`
}

// UpdateMCPServerParams holds the partial-update input for an MCP server.
// Pointer fields are only applied when non-nil; slice fields are applied when non-nil.
type UpdateMCPServerParams struct {
	Name        string   `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Type        *string  `json:"type,omitempty"`
	Command     *string  `json:"command,omitempty"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
	URL         *string  `json:"url,omitempty"`
	Headers     []string `json:"headers,omitempty"`
}

// marshalStringSlice marshals a string slice to JSON for storage.
// Returns nil (no-op) when the slice is empty.
func marshalStringSlice(label string, values []string) (json.RawMessage, error) {
	if len(values) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal %s: %w", label, err)
	}
	return data, nil
}

// CreateMCPServer creates a new MCP server.
func (s *MCPServerService) CreateMCPServer(ctx context.Context, p CreateMCPServerParams) (*MCPServer, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.Type == "" {
		return nil, fmt.Errorf("type is required")
	}

	srv := &model.MCPServer{
		ProjectID:   p.ProjectID,
		Name:        p.Name,
		Description: p.Description,
		Type:        p.Type,
	}

	switch p.Type {
	case "stdio":
		if p.Command == "" {
			return nil, fmt.Errorf("command is required for stdio MCP server")
		}
		srv.Command = p.Command
		if data, err := marshalStringSlice("args", p.Args); err != nil {
			return nil, err
		} else {
			srv.Args = data
		}
		if data, err := marshalStringSlice("env", p.Env); err != nil {
			return nil, err
		} else {
			srv.Env = data
		}
	case "http":
		if p.URL == "" {
			return nil, fmt.Errorf("url is required for http MCP server")
		}
		srv.URL = p.URL
		if data, err := marshalStringSlice("headers", p.Headers); err != nil {
			return nil, err
		} else {
			srv.Headers = data
		}
	default:
		return nil, fmt.Errorf("invalid MCP server type: %s", p.Type)
	}

	if err := s.store.CreateMCPServer(ctx, srv); err != nil {
		return nil, fmt.Errorf("failed to create MCP server: %w", err)
	}

	return mapMCPServer(srv), nil
}

// UpdateMCPServer applies a partial update to an existing MCP server.
func (s *MCPServerService) UpdateMCPServer(ctx context.Context, id string, p UpdateMCPServerParams) (*MCPServer, error) {
	srv, err := s.store.GetMCPServerByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get MCP server: %w", err)
	}

	if p.Name != "" {
		srv.Name = p.Name
	}
	if p.Description != nil {
		srv.Description = *p.Description
	}
	if p.Type != nil {
		srv.Type = *p.Type
	}

	switch srv.Type {
	case "stdio":
		if p.Command != nil {
			srv.Command = *p.Command
		}
		if p.Args != nil {
			if data, err := marshalStringSlice("args", p.Args); err != nil {
				return nil, err
			} else {
				srv.Args = data
			}
		}
		if p.Env != nil {
			if data, err := marshalStringSlice("env", p.Env); err != nil {
				return nil, err
			} else {
				srv.Env = data
			}
		}
	case "http":
		if p.URL != nil {
			srv.URL = *p.URL
		}
		if p.Headers != nil {
			if data, err := marshalStringSlice("headers", p.Headers); err != nil {
				return nil, err
			} else {
				srv.Headers = data
			}
		}
	default:
		return nil, fmt.Errorf("invalid MCP server type: %s", srv.Type)
	}

	if err := s.store.UpdateMCPServer(ctx, srv); err != nil {
		return nil, fmt.Errorf("failed to update MCP server: %w", err)
	}

	return mapMCPServer(srv), nil
}

// DeleteMCPServer deletes an MCP server by ID.
func (s *MCPServerService) DeleteMCPServer(ctx context.Context, id string) error {
	if err := s.store.DeleteMCPServer(ctx, id); err != nil {
		return fmt.Errorf("failed to delete MCP server: %w", err)
	}
	return nil
}

// GetAgentMCPServers returns MCP servers attached to an agent.
func (s *MCPServerService) GetAgentMCPServers(ctx context.Context, agentID string) ([]*MCPServer, error) {
	dbServers, err := s.store.GetAgentMCPServers(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent MCP servers: %w", err)
	}

	servers := make([]*MCPServer, len(dbServers))
	for i, srv := range dbServers {
		servers[i] = mapMCPServer(srv)
	}
	return servers, nil
}

// AttachMCPServer attaches an MCP server to an agent.
func (s *MCPServerService) AttachMCPServer(ctx context.Context, agentID, mcpServerID string) error {
	if err := s.store.AttachMCPServerToAgent(ctx, agentID, mcpServerID); err != nil {
		return fmt.Errorf("failed to attach MCP server to agent: %w", err)
	}
	return nil
}

// DetachMCPServer detaches an MCP server from an agent.
func (s *MCPServerService) DetachMCPServer(ctx context.Context, agentID, mcpServerID string) error {
	if err := s.store.DetachMCPServerFromAgent(ctx, agentID, mcpServerID); err != nil {
		return fmt.Errorf("failed to detach MCP server from agent: %w", err)
	}
	return nil
}

// mapMCPServer converts a model.MCPServer to a service.MCPServer.
func mapMCPServer(srv *model.MCPServer) *MCPServer {
	if srv == nil {
		return nil
	}

	var args []string
	if len(srv.Args) > 0 {
		_ = json.Unmarshal(srv.Args, &args)
	}

	var env []string
	if len(srv.Env) > 0 {
		_ = json.Unmarshal(srv.Env, &env)
	}

	var headers []string
	if len(srv.Headers) > 0 {
		_ = json.Unmarshal(srv.Headers, &headers)
	}

	return &MCPServer{
		ID:          srv.ID,
		ProjectID:   srv.ProjectID,
		Name:        srv.Name,
		Description: srv.Description,
		Type:        srv.Type,
		Command:     srv.Command,
		Args:        args,
		Env:         env,
		URL:         srv.URL,
		Headers:     headers,
		CreatedAt:   srv.CreatedAt,
		UpdatedAt:   srv.UpdatedAt,
	}
}
