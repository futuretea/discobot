package service

import (
	"context"
	"fmt"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

// SkillMarketRepoRecord is the API representation of a skill market repository.
type SkillMarketRepoRecord struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	RepoURL   string `json:"repoUrl"`
	Branch    string `json:"branch,omitempty"`
	Path      string `json:"path,omitempty"`
}

// CreateSkillMarketRepoParams is the input for creating a skill market repository.
type CreateSkillMarketRepoParams struct {
	ProjectID string `json:"-"`
	Name      string `json:"name"`
	RepoURL   string `json:"repoUrl"`
	Branch    string `json:"branch,omitempty"`
	Path      string `json:"path,omitempty"`
}

// UpdateSkillMarketRepoParams is the partial-update input for a skill market repository.
// Empty string fields are ignored (no change).
type UpdateSkillMarketRepoParams struct {
	Name    string `json:"name,omitempty"`
	RepoURL string `json:"repoUrl,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Path    string `json:"path,omitempty"`
}

// SkillMarketRepoService handles skill market repository CRUD.
type SkillMarketRepoService struct {
	store *store.Store
}

func NewSkillMarketRepoService(s *store.Store) *SkillMarketRepoService {
	return &SkillMarketRepoService{store: s}
}

func (s *SkillMarketRepoService) ListSkillMarketRepos(ctx context.Context, projectID string) ([]*SkillMarketRepoRecord, error) {
	rows, err := s.store.ListSkillMarketRepos(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("list skill market repos: %w", err)
	}
	records := make([]*SkillMarketRepoRecord, len(rows))
	for i, r := range rows {
		records[i] = mapSkillMarketRepo(r)
	}
	return records, nil
}

func (s *SkillMarketRepoService) GetSkillMarketRepo(ctx context.Context, id string) (*SkillMarketRepoRecord, error) {
	row, err := s.store.GetSkillMarketRepoByID(ctx, id)
	if err != nil {
		return nil, err // ErrNotFound passes through unchanged
	}
	return mapSkillMarketRepo(row), nil
}

func (s *SkillMarketRepoService) CreateSkillMarketRepo(ctx context.Context, p CreateSkillMarketRepoParams) (*SkillMarketRepoRecord, error) {
	if p.Name == "" {
		return nil, fmt.Errorf("name is required")
	}
	if p.RepoURL == "" {
		return nil, fmt.Errorf("repoUrl is required")
	}
	row := &model.SkillMarketRepo{
		ProjectID: p.ProjectID,
		Name:      p.Name,
		RepoURL:   p.RepoURL,
		Branch:    p.Branch,
		Path:      p.Path,
	}
	if err := s.store.CreateSkillMarketRepo(ctx, row); err != nil {
		return nil, fmt.Errorf("create skill market repo: %w", err)
	}
	return mapSkillMarketRepo(row), nil
}

func (s *SkillMarketRepoService) UpdateSkillMarketRepo(ctx context.Context, id string, p UpdateSkillMarketRepoParams) (*SkillMarketRepoRecord, error) {
	row, err := s.store.GetSkillMarketRepoByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Name != "" {
		row.Name = p.Name
	}
	if p.RepoURL != "" {
		row.RepoURL = p.RepoURL
	}
	if p.Branch != "" {
		row.Branch = p.Branch
	}
	if p.Path != "" {
		row.Path = p.Path
	}
	if err := s.store.UpdateSkillMarketRepo(ctx, row); err != nil {
		return nil, fmt.Errorf("update skill market repo: %w", err)
	}
	return mapSkillMarketRepo(row), nil
}

func (s *SkillMarketRepoService) DeleteSkillMarketRepo(ctx context.Context, id string) error {
	return s.store.DeleteSkillMarketRepo(ctx, id)
}

func mapSkillMarketRepo(r *model.SkillMarketRepo) *SkillMarketRepoRecord {
	return &SkillMarketRepoRecord{
		ID:        r.ID,
		ProjectID: r.ProjectID,
		Name:      r.Name,
		RepoURL:   r.RepoURL,
		Branch:    r.Branch,
		Path:      r.Path,
	}
}
