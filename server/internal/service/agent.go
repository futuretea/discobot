package service

import (
	"context"
	"fmt"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

// Agent represents an agent configuration (for API responses)
type Agent struct {
	ID        string `json:"id"`
	AgentType string `json:"agentType"`
	IsDefault bool   `json:"isDefault,omitempty"`
}

// AgentService handles agent operations
type AgentService struct {
	store *store.Store
}

// NewAgentService creates a new agent service
func NewAgentService(s *store.Store) *AgentService {
	return &AgentService{store: s}
}

// ListAgents returns all agents for a project
func (s *AgentService) ListAgents(ctx context.Context, projectID string) ([]*Agent, error) {
	dbAgents, err := s.store.ListAgentsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list agents: %w", err)
	}

	agents := make([]*Agent, len(dbAgents))
	for i, ag := range dbAgents {
		agents[i] = s.mapAgent(ag)
	}
	return agents, nil
}

// GetAgent returns an agent by ID
func (s *AgentService) GetAgent(ctx context.Context, agentID string) (*Agent, error) {
	ag, err := s.store.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	return s.mapAgent(ag), nil
}

// CreateAgent creates a new agent
func (s *AgentService) CreateAgent(ctx context.Context, projectID, agentType string) (*Agent, error) {
	ag := &model.Agent{
		ProjectID: projectID,
		AgentType: agentType,
		IsDefault: false,
	}
	if err := s.store.CreateAgent(ctx, ag); err != nil {
		return nil, fmt.Errorf("failed to create agent: %w", err)
	}

	return s.mapAgent(ag), nil
}

// UpdateAgent updates an agent
func (s *AgentService) UpdateAgent(ctx context.Context, agentID string) (*Agent, error) {
	ag, err := s.store.GetAgentByID(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent: %w", err)
	}

	if err := s.store.UpdateAgent(ctx, ag); err != nil {
		return nil, fmt.Errorf("failed to update agent: %w", err)
	}

	return s.mapAgent(ag), nil
}

// DeleteAgent deletes an agent
func (s *AgentService) DeleteAgent(ctx context.Context, agentID string) error {
	return s.store.DeleteAgent(ctx, agentID)
}

// SetDefaultAgent sets the default agent for a project
func (s *AgentService) SetDefaultAgent(ctx context.Context, projectID, agentID string) error {
	return s.store.SetDefaultAgent(ctx, projectID, agentID)
}

// mapAgent maps a model Agent to a service Agent
func (s *AgentService) mapAgent(ag *model.Agent) *Agent {
	return &Agent{
		ID:        ag.ID,
		AgentType: ag.AgentType,
		IsDefault: ag.IsDefault,
	}
}
