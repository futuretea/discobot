package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

// ErrSkillInUse is returned when attempting to delete a skill that is still attached to one or more agents.
var ErrSkillInUse = errors.New("skill is in use by one or more agents")

// SkillInUseByAgentsError carries which agents are blocking deletion.
type SkillInUseByAgentsError struct {
	AgentIDs   []string
	AgentTypes []string
}

func (e *SkillInUseByAgentsError) Error() string {
	return ErrSkillInUse.Error()
}

func (e *SkillInUseByAgentsError) Is(target error) bool {
	return target == ErrSkillInUse
}

// Skill represents a skill for API responses.
type Skill struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"projectId"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Content     string    `json:"content"`
	SourceURL   string    `json:"sourceUrl,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// SkillService handles skill operations.
type SkillService struct {
	store *store.Store
	// skillsDir is the base directory on the host where skill packages are stored.
	// Each skill occupies a sub-directory named after its slugified Name field.
	skillsDir string
	// marketCache manages git clones for skill market repos.
	marketCache *SkillMarketCache
}

// NewSkillService creates a new skill service.
func NewSkillService(s *store.Store, skillsDir string, marketCache *SkillMarketCache) *SkillService {
	if skillsDir == "" {
		log.Printf("[SkillService] empty skillsDir, skill imports will fail")
	}
	return &SkillService{
		store:       s,
		skillsDir:   skillsDir,
		marketCache: marketCache,
	}
}

// SkillDir returns the absolute host path to the skill's directory, derived from its name.
func (s *SkillService) SkillDir(name string) string {
	return filepath.Join(s.skillsDir, skillDirName(name))
}

// ListSkills returns all skills for a project.
func (s *SkillService) ListSkills(ctx context.Context, projectID string) ([]*Skill, error) {
	dbSkills, err := s.store.ListSkillsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list skills: %w", err)
	}

	skills := make([]*Skill, len(dbSkills))
	for i, sk := range dbSkills {
		skills[i] = mapSkill(sk)
	}
	return skills, nil
}

// GetSkill returns a single skill by ID.
func (s *SkillService) GetSkill(ctx context.Context, id string) (*Skill, error) {
	sk, err := s.store.GetSkillByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}
	return mapSkill(sk), nil
}

// CreateSkillParams holds the input for creating a skill.
type CreateSkillParams struct {
	ProjectID   string
	Name        string
	Description string
	Content     string
	SourceURL   string
}

// CreateSkill creates a new skill.
func (s *SkillService) CreateSkill(ctx context.Context, p CreateSkillParams) (*Skill, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("skill name is required")
	}
	if strings.TrimSpace(p.Content) == "" {
		return nil, fmt.Errorf("skill content is required")
	}

	sk := &model.Skill{
		ProjectID:   p.ProjectID,
		Name:        p.Name,
		Description: p.Description,
		Content:     p.Content,
		SourceURL:   p.SourceURL,
	}
	if err := s.store.CreateSkill(ctx, sk); err != nil {
		return nil, fmt.Errorf("failed to create skill: %w", err)
	}

	// Materialize the skill directory on disk under skillsDir/<slug(name)>/SKILL.md
	// so that sessions can mount it into the sandbox.
	if s.skillsDir != "" {
		skillDir := filepath.Join(s.skillsDir, skillDirName(sk.Name))
		if err := os.MkdirAll(skillDir, 0o755); err != nil {
			log.Printf("[SkillService] failed to create skill directory %s: %v", skillDir, err)
		} else {
			skillFile := filepath.Join(skillDir, "SKILL.md")
			if err := os.WriteFile(skillFile, []byte(p.Content), 0o644); err != nil {
				log.Printf("[SkillService] failed to write SKILL.md for skill %s: %v", sk.ID, err)
			}
		}
	}

	return mapSkill(sk), nil
}

// UpdateSkillParams holds the partial-update input for a skill.
// Empty strings mean "no change".
type UpdateSkillParams struct {
	Name        string
	Description string
	Content     string
}

// UpdateSkill updates name/description/content for a skill.
// Empty strings mean "no change" for name/description/content.
func (s *SkillService) UpdateSkill(ctx context.Context, id string, p UpdateSkillParams) (*Skill, error) {
	sk, err := s.store.GetSkillByID(ctx, id)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, store.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get skill: %w", err)
	}

	oldName := sk.Name
	if p.Name != "" {
		sk.Name = p.Name
	}
	if p.Description != "" {
		sk.Description = p.Description
	}
	contentChanged := p.Content != "" && p.Content != sk.Content
	if p.Content != "" {
		sk.Content = p.Content
	}

	if err := s.store.UpdateSkill(ctx, sk); err != nil {
		return nil, fmt.Errorf("failed to update skill: %w", err)
	}

	// Keep on-disk state in sync.
	if s.skillsDir != "" {
		oldDir := s.SkillDir(oldName)
		newDir := s.SkillDir(sk.Name)

		// Rename the directory when the slug changes.
		if oldDir != newDir {
			if err := os.Rename(oldDir, newDir); err != nil && !os.IsNotExist(err) {
				log.Printf("[SkillService] failed to rename skill directory %s → %s: %v", oldDir, newDir, err)
			}
		}

		// Rewrite SKILL.md when content changes.
		if contentChanged {
			if err := os.MkdirAll(newDir, 0o755); err != nil {
				log.Printf("[SkillService] failed to ensure skill directory %s: %v", newDir, err)
			} else {
				skillFile := filepath.Join(newDir, "SKILL.md")
				if err := os.WriteFile(skillFile, []byte(sk.Content), 0o644); err != nil {
					log.Printf("[SkillService] failed to rewrite SKILL.md for skill %s: %v", sk.ID, err)
				}
			}
		}
	}

	return mapSkill(sk), nil
}

// DeleteSkill deletes a skill by ID.
func (s *SkillService) DeleteSkill(ctx context.Context, id string) error {
	// Check if any agents are currently using this skill.
	agents, err := s.store.ListAgentsBySkillID(ctx, id)
	if err != nil {
		return fmt.Errorf("failed to check agents for skill: %w", err)
	}
	if len(agents) > 0 {
		inUseErr := &SkillInUseByAgentsError{}
		for _, a := range agents {
			inUseErr.AgentIDs = append(inUseErr.AgentIDs, a.ID)
			inUseErr.AgentTypes = append(inUseErr.AgentTypes, a.AgentType)
		}
		return inUseErr
	}

	// Fetch before deleting so we can clean up the on-disk directory by name.
	sk, _ := s.store.GetSkillByID(ctx, id)

	if err := s.store.DeleteSkill(ctx, id); err != nil {
		return fmt.Errorf("failed to delete skill: %w", err)
	}

	// Remove the on-disk skill directory to avoid accumulating orphaned files.
	if s.skillsDir != "" && sk != nil {
		skillDir := filepath.Join(s.skillsDir, skillDirName(sk.Name))
		if err := os.RemoveAll(skillDir); err != nil {
			log.Printf("[SkillService] failed to remove skill directory %s: %v", skillDir, err)
		}
	}

	return nil
}

// GetAgentSkills returns skills attached to an agent.
func (s *SkillService) GetAgentSkills(ctx context.Context, agentID string) ([]*Skill, error) {
	dbSkills, err := s.store.GetAgentSkills(ctx, agentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get agent skills: %w", err)
	}

	skills := make([]*Skill, len(dbSkills))
	for i, sk := range dbSkills {
		skills[i] = mapSkill(sk)
	}
	return skills, nil
}

// AttachSkill attaches a skill to an agent.
func (s *SkillService) AttachSkill(ctx context.Context, agentID, skillID string) error {
	if err := s.store.AttachSkillToAgent(ctx, agentID, skillID); err != nil {
		return fmt.Errorf("failed to attach skill to agent: %w", err)
	}
	return nil
}

// DetachSkill detaches a skill from an agent.
func (s *SkillService) DetachSkill(ctx context.Context, agentID, skillID string) error {
	if err := s.store.DetachSkillFromAgent(ctx, agentID, skillID); err != nil {
		return fmt.Errorf("failed to detach skill from agent: %w", err)
	}
	return nil
}

// ImportSkillRequest carries the parameters needed to import a skill from the
// market cache into the project's skill store.
type ImportSkillRequest struct {
	// RepoURL is the git clone URL of the market repository.
	RepoURL string
	// Branch is the branch to use; defaults to "main".
	Branch string
	// SkillsPath is the relative path inside the repo that contains skill sub-directories.
	SkillsPath string
	// SkillID is the name of the sub-directory inside SkillsPath that represents the skill.
	SkillID string
	// ForceUpdate, when true, runs "git pull" before reading files.
	// Normally false — the existing local clone is used as-is.
	ForceUpdate bool
}

// ImportSkillFromMarket installs a single skill from the local market cache into
// the project's skill store.  The repo is cloned (or updated) on demand.
func (s *SkillService) ImportSkillFromMarket(ctx context.Context, projectID string, req ImportSkillRequest) (*Skill, error) {
	if s.marketCache == nil {
		return nil, fmt.Errorf("skill market cache is not configured")
	}
	if s.skillsDir == "" {
		return nil, fmt.Errorf("skillsDir is not configured")
	}
	if req.SkillID == "" {
		return nil, fmt.Errorf("skillId is required")
	}

	branch := req.Branch
	if branch == "" {
		branch = "main"
	}
	skillsPath := req.SkillsPath
	if skillsPath == "" {
		skillsPath = "skills"
	}

	repo := s.marketCache.Get(req.RepoURL, branch, skillsPath)
	if err := repo.EnsureCloned(ctx, req.ForceUpdate); err != nil {
		return nil, fmt.Errorf("failed to prepare skill market repo: %w", err)
	}

	// Read SKILL.md from the clone.
	skillMDPath := filepath.Join(repo.SkillsDir(), req.SkillID, "SKILL.md")
	skillContent, err := os.ReadFile(skillMDPath)
	if err != nil {
		return nil, fmt.Errorf("SKILL.md not found for skill %q: %w", req.SkillID, err)
	}

	name := req.SkillID
	description := parseSkillMDDescription(skillContent)

	// Build a human-readable source URL pointing to the skill sub-directory.
	sourceURL := req.RepoURL
	if sourceURL != "" {
		sourceURL = strings.TrimSuffix(sourceURL, ".git") +
			"/tree/" + branch + "/" + path.Join(skillsPath, req.SkillID)
	}

	// Create the DB record, then copy files.
	// name = req.SkillID keeps the human-readable market directory name (e.g. "pptx").
	sk, err := s.CreateSkill(ctx, CreateSkillParams{
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		Content:     string(skillContent),
		SourceURL:   sourceURL,
	})
	if err != nil {
		return nil, err
	}

	// Copy all files from the clone sub-directory into skillsDir/<slug(name)>/.
	skillDir := filepath.Join(s.skillsDir, skillDirName(sk.Name))
	if err := repo.CopySkillDir(req.SkillID, skillDir); err != nil {
		// Best-effort rollback: remove the DB record and partially copied dir.
		_ = s.store.DeleteSkill(ctx, sk.ID)
		_ = os.RemoveAll(skillDir)
		return nil, fmt.Errorf("failed to copy skill files: %w", err)
	}

	return sk, nil
}

// SkillMarketEntry represents a skill discovered in the skill market repository.
type SkillMarketEntry struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	SourceURL   string   `json:"sourceUrl"`
	Tags        []string `json:"tags"`
}

// ListSkillMarket lists all skills available in the given git repository.
// When forceUpdate is true the local clone is refreshed via "git pull" first.
func (s *SkillService) ListSkillMarket(ctx context.Context, repoURL, branch, skillsPath string, forceUpdate bool) ([]SkillMarketEntry, error) {
	if s.marketCache == nil {
		return nil, fmt.Errorf("skill market cache is not configured")
	}
	if branch == "" {
		branch = "main"
	}
	if skillsPath == "" {
		skillsPath = "skills"
	}

	repo := s.marketCache.Get(repoURL, branch, skillsPath)
	if err := repo.EnsureCloned(ctx, forceUpdate); err != nil {
		return nil, fmt.Errorf("failed to prepare skill market repo: %w", err)
	}

	entries, err := os.ReadDir(repo.SkillsDir())
	if err != nil {
		return nil, fmt.Errorf("failed to read skill market directory: %w", err)
	}

	baseSourceURL := strings.TrimSuffix(repoURL, ".git") + "/tree/" + branch + "/" + skillsPath

	var skills []SkillMarketEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		skillID := entry.Name()

		skillName := strings.ReplaceAll(skillID, "-", " ")

		sourceURL := baseSourceURL + "/" + skillID

		// Read SKILL.md for description (local IO — no network).
		skillMDPath := filepath.Join(repo.SkillsDir(), skillID, "SKILL.md")
		description := ""
		if data, err := os.ReadFile(skillMDPath); err == nil {
			description = parseSkillMDDescription(data)
		}

		skills = append(skills, SkillMarketEntry{
			ID:          skillID,
			Name:        skillName,
			Description: description,
			SourceURL:   sourceURL,
			Tags:        []string{},
		})
	}

	return skills, nil
}

// mapSkill converts a model.Skill to a service.Skill.
func mapSkill(sk *model.Skill) *Skill {
	if sk == nil {
		return nil
	}
	return &Skill{
		ID:          sk.ID,
		ProjectID:   sk.ProjectID,
		Name:        sk.Name,
		Description: sk.Description,
		Content:     sk.Content,
		SourceURL:   sk.SourceURL,
		CreatedAt:   sk.CreatedAt,
		UpdatedAt:   sk.UpdatedAt,
	}
}

// skillDirName returns a filesystem-safe directory name derived from the skill name.
// e.g. "PPTX Tools" → "pptx-tools", "code review" → "code-review"
func skillDirName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var b strings.Builder
	prevHyphen := false
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevHyphen = false
		} else if !prevHyphen {
			b.WriteByte('-')
			prevHyphen = true
		}
	}
	result := strings.Trim(b.String(), "-")
	if result == "" {
		return "skill"
	}
	return result
}

// parseSkillMDDescription extracts the "description:" value from a SKILL.md
// YAML front matter block (--- ... ---) without any external dependency.
func parseSkillMDDescription(content []byte) string {
	s := strings.TrimSpace(string(content))
	if !strings.HasPrefix(s, "---") {
		return ""
	}
	// Find closing ---
	rest := s[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return ""
	}
	frontMatter := rest[:end]
	for _, line := range strings.Split(frontMatter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "description:"))
			// Strip surrounding quotes if present
			if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}
