// Package model defines the database models used throughout the application.
// These models work with both PostgreSQL and SQLite via GORM.
package model

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User represents an authenticated user.
type User struct {
	ID         string    `gorm:"primaryKey;type:text" json:"id"`
	Email      string    `gorm:"uniqueIndex;not null;type:text" json:"email"`
	Name       *string   `gorm:"type:text" json:"name,omitempty"`
	AvatarURL  *string   `gorm:"column:avatar_url;type:text" json:"avatar_url,omitempty"`
	Provider   string    `gorm:"not null;type:text" json:"provider"`
	ProviderID string    `gorm:"column:provider_id;not null;type:text" json:"provider_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (User) TableName() string { return "users" }

func (u *User) BeforeCreate(_ *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.New().String()
	}
	return nil
}

// UserSession represents an authentication session (cookie-based).
type UserSession struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;type:text;index" json:"user_id"`
	TokenHash string    `gorm:"column:token_hash;uniqueIndex;not null;type:text" json:"token_hash"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null;index" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (UserSession) TableName() string { return "user_sessions" }

func (s *UserSession) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// Project represents a multi-tenant container.
type Project struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	Name      string    `gorm:"not null;type:text" json:"name"`
	Slug      string    `gorm:"uniqueIndex;not null;type:text" json:"slug"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Members    []ProjectMember `gorm:"foreignKey:ProjectID" json:"-"`
	Workspaces []Workspace     `gorm:"foreignKey:ProjectID" json:"-"`
	Agents     []Agent         `gorm:"foreignKey:ProjectID" json:"-"`
}

func (Project) TableName() string { return "projects" }

func (p *Project) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// ProjectMember represents a user's membership in a project.
type ProjectMember struct {
	ID         string     `gorm:"primaryKey;type:text" json:"id"`
	ProjectID  string     `gorm:"column:project_id;not null;type:text;uniqueIndex:idx_project_user" json:"project_id"`
	UserID     string     `gorm:"column:user_id;not null;type:text;uniqueIndex:idx_project_user;index" json:"user_id"`
	Role       string     `gorm:"not null;type:text;default:member" json:"role"`
	InvitedBy  *string    `gorm:"column:invited_by;type:text" json:"invited_by,omitempty"`
	InvitedAt  *time.Time `gorm:"column:invited_at" json:"invited_at,omitempty"`
	AcceptedAt *time.Time `gorm:"column:accepted_at" json:"accepted_at,omitempty"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
	User    *User    `gorm:"foreignKey:UserID" json:"-"`
}

func (ProjectMember) TableName() string { return "project_members" }

func (m *ProjectMember) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// ProjectInvitation represents a pending invitation to join a project.
type ProjectInvitation struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"column:project_id;not null;type:text;uniqueIndex:idx_project_email" json:"project_id"`
	Email     string    `gorm:"not null;type:text;uniqueIndex:idx_project_email" json:"email"`
	Role      string    `gorm:"not null;type:text;default:member" json:"role"`
	InvitedBy *string   `gorm:"column:invited_by;type:text" json:"invited_by,omitempty"`
	Token     string    `gorm:"uniqueIndex;not null;type:text" json:"token"`
	ExpiresAt time.Time `gorm:"column:expires_at;not null" json:"expires_at"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (ProjectInvitation) TableName() string { return "project_invitations" }

func (i *ProjectInvitation) BeforeCreate(_ *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.New().String()
	}
	return nil
}

// Agent represents an AI agent configuration.
type Agent struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"column:project_id;not null;type:text;index" json:"project_id"`
	AgentType string    `gorm:"column:agent_type;not null;type:text" json:"agent_type"`
	IsDefault bool      `gorm:"column:is_default;default:false" json:"is_default"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (Agent) TableName() string { return "agents" }

func (a *Agent) BeforeCreate(_ *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// Workspace status constants representing the lifecycle of a workspace
const (
	WorkspaceStatusInitializing = "initializing" // Workspace just created, starting setup
	WorkspaceStatusCloning      = "cloning"      // Cloning git repository
	WorkspaceStatusReady        = "ready"        // Workspace is ready for use
	WorkspaceStatusError        = "error"        // Something failed during setup
)

// Workspace provider constants representing how sessions are executed.
// When a workspace has no provider set (empty string), the platform default is used
// at runtime: "vz" on macOS, "docker" on other platforms.
const (
	WorkspaceProviderVZ     = "vz"     // Run in Virtualization.framework VMs (macOS only)
	WorkspaceProviderDocker = "docker" // Run in Docker containers
	WorkspaceProviderLocal  = "local"  // Run in local directory without isolation
)

// Workspace represents a working directory (local folder or git repo).
type Workspace struct {
	ID           string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID    string    `gorm:"column:project_id;not null;type:text;index" json:"projectId"`
	Path         string    `gorm:"not null;type:text" json:"path"`
	DisplayName  *string   `gorm:"column:display_name;type:text" json:"displayName,omitempty"`
	SourceType   string    `gorm:"column:source_type;not null;type:text" json:"sourceType"`
	Provider     string    `gorm:"type:text;default:''" json:"provider,omitempty"`
	Status       string    `gorm:"not null;type:text;default:initializing" json:"status"`
	ErrorMessage *string   `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	Project  *Project  `gorm:"foreignKey:ProjectID" json:"-"`
	Sessions []Session `gorm:"foreignKey:WorkspaceID" json:"-"`
}

func (Workspace) TableName() string { return "workspaces" }

func (w *Workspace) BeforeCreate(_ *gorm.DB) error {
	if w.ID == "" {
		w.ID = uuid.New().String()
	}
	return nil
}

// Session status constants representing the lifecycle of a session
const (
	SessionStatusInitializing    = "initializing"     // Session just created, starting setup
	SessionStatusReinitializing  = "reinitializing"   // Recreating sandbox after it was deleted
	SessionStatusCloning         = "cloning"          // Cloning git repository
	SessionStatusPullingImage    = "pulling_image"    // Pulling runtime image
	SessionStatusCreatingSandbox = "creating_sandbox" // Creating sandbox environment
	SessionStatusReady           = "ready"            // Session is ready for use
	SessionStatusRunning         = "running"          // Session has an active chat completion in progress
	SessionStatusStopped         = "stopped"          // Sandbox is stopped, will restart on demand
	SessionStatusError           = "error"            // Something failed during setup
	SessionStatusRemoving        = "removing"         // Session is being deleted
	SessionStatusRemoved         = "removed"          // Session has been deleted
)

// Commit status constants representing the commit state of a session (orthogonal to session status)
const (
	CommitStatusNone       = ""           // No commit in progress (default)
	CommitStatusPending    = "pending"    // Commit requested, waiting to start
	CommitStatusCommitting = "committing" // Commit in progress
	CommitStatusCompleted  = "completed"  // Commit completed successfully
	CommitStatusFailed     = "failed"     // Commit failed
)

// Session represents a chat thread within a workspace.
type Session struct {
	ID              string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID       string    `gorm:"column:project_id;not null;type:text;index" json:"projectId"`
	WorkspaceID     string    `gorm:"column:workspace_id;not null;type:text;index" json:"workspaceId"`
	AgentID         *string   `gorm:"column:agent_id;type:text;index" json:"agentId,omitempty"`
	Name            string    `gorm:"not null;type:text" json:"name"`
	DisplayName     *string   `gorm:"column:display_name;type:text" json:"displayName,omitempty"`
	Description     *string   `gorm:"type:text" json:"description,omitempty"`
	Status          string    `gorm:"not null;type:text;default:initializing" json:"status"`
	CommitStatus    string    `gorm:"column:commit_status;type:text;default:''" json:"commitStatus"`
	CommitError     *string   `gorm:"column:commit_error;type:text" json:"commitError,omitempty"`
	BaseCommit      *string   `gorm:"column:base_commit;type:text" json:"baseCommit,omitempty"`
	AppliedCommit   *string   `gorm:"column:applied_commit;type:text" json:"appliedCommit,omitempty"`
	ErrorMessage    *string   `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`
	WorkspacePath   *string   `gorm:"column:workspace_path;type:text" json:"workspacePath,omitempty"`
	WorkspaceCommit *string   `gorm:"column:workspace_commit;type:text" json:"workspaceCommit,omitempty"`
	Model           *string   `gorm:"column:model;type:text" json:"model,omitempty"`
	Reasoning       *string   `gorm:"column:reasoning;type:text" json:"reasoning,omitempty"`
	Mode            *string   `gorm:"column:mode;type:text" json:"mode,omitempty"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updatedAt"`

	Project   *Project   `gorm:"foreignKey:ProjectID" json:"-"`
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"-"`
	Agent     *Agent     `gorm:"foreignKey:AgentID" json:"-"`
	Messages  []Message  `gorm:"foreignKey:SessionID" json:"-"`
}

func (Session) TableName() string { return "sessions" }

func (s *Session) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// Message represents a chat message in a session.
// Stored in UIMessage format compatible with AI SDK.
type Message struct {
	ID        string          `gorm:"primaryKey;type:text" json:"id"`
	SessionID string          `gorm:"column:session_id;not null;type:text;index" json:"sessionId"`
	Role      string          `gorm:"not null;type:text" json:"role"`
	Parts     json.RawMessage `gorm:"type:text;not null" json:"parts"`
	CreatedAt time.Time       `gorm:"autoCreateTime" json:"createdAt"`

	Session *Session `gorm:"foreignKey:SessionID" json:"-"`
}

func (Message) TableName() string { return "messages" }

func (m *Message) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// TextPart represents a text part in a UIMessage.
type TextPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// NewTextParts creates a JSON parts array with a single text part.
func NewTextParts(text string) json.RawMessage {
	parts := []TextPart{{Type: "text", Text: text}}
	data, _ := json.Marshal(parts)
	return data
}

// Credential represents stored credentials for AI providers.
type Credential struct {
	ID            string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID     string    `gorm:"column:project_id;not null;type:text;uniqueIndex:idx_project_provider" json:"project_id"`
	Provider      string    `gorm:"not null;type:text;uniqueIndex:idx_project_provider" json:"provider"`
	Name          string    `gorm:"not null;type:text" json:"name"`
	AuthType      string    `gorm:"column:auth_type;not null;type:text" json:"auth_type"`
	EncryptedData []byte    `gorm:"column:encrypted_data" json:"-"`
	IsConfigured  bool      `gorm:"column:is_configured;default:false" json:"is_configured"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (Credential) TableName() string { return "credentials" }

func (c *Credential) BeforeCreate(_ *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// TerminalHistory represents a terminal command/output entry.
type TerminalHistory struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	SessionID string    `gorm:"column:session_id;not null;type:text;index" json:"session_id"`
	EntryType string    `gorm:"column:entry_type;not null;type:text" json:"entry_type"`
	Content   string    `gorm:"not null;type:text" json:"content"`
	ExitCode  *int      `gorm:"column:exit_code" json:"exit_code,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`

	Session *Session `gorm:"foreignKey:SessionID" json:"-"`
}

func (TerminalHistory) TableName() string { return "terminal_history" }

func (t *TerminalHistory) BeforeCreate(_ *gorm.DB) error {
	if t.ID == "" {
		t.ID = uuid.New().String()
	}
	return nil
}

// Skill represents a reusable skill (SKILL.md) at the project level.
type Skill struct {
	ID          string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID   string    `gorm:"column:project_id;not null;type:text;index" json:"project_id"`
	Name        string    `gorm:"not null;type:text" json:"name"`
	Description string    `gorm:"type:text" json:"description,omitempty"`
	Content     string    `gorm:"not null;type:text" json:"content"`
	// SourceURL tracks where this skill was imported from (empty = manually created)
	SourceURL string    `gorm:"column:source_url;type:text" json:"source_url,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (Skill) TableName() string { return "skills" }

func (s *Skill) BeforeCreate(_ *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return nil
}

// MCPServer represents a project-level MCP server configuration.
type MCPServer struct {
	ID          string          `gorm:"primaryKey;type:text" json:"id"`
	ProjectID   string          `gorm:"column:project_id;not null;type:text;index" json:"project_id"`
	Name        string          `gorm:"not null;type:text" json:"name"`
	Description string          `gorm:"type:text" json:"description,omitempty"`
	Type        string          `gorm:"not null;type:text" json:"type"` // "stdio" or "http"
	Command     string          `gorm:"type:text" json:"command,omitempty"`
	Args        json.RawMessage `gorm:"type:text" json:"args,omitempty"`
	Env         json.RawMessage `gorm:"type:text" json:"env,omitempty"`
	URL         string          `gorm:"type:text" json:"url,omitempty"`
	Headers     json.RawMessage `gorm:"type:text" json:"headers,omitempty"`
	CreatedAt   time.Time       `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time       `gorm:"autoUpdateTime" json:"updated_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (MCPServer) TableName() string { return "mcp_servers" }

func (m *MCPServer) BeforeCreate(_ *gorm.DB) error {
	if m.ID == "" {
		m.ID = uuid.New().String()
	}
	return nil
}

// AgentSkill is the junction table between Agent and Skill.
type AgentSkill struct {
	AgentID string `gorm:"primaryKey;type:text;column:agent_id" json:"agent_id"`
	SkillID string `gorm:"primaryKey;type:text;column:skill_id" json:"skill_id"`

	Agent *Agent `gorm:"foreignKey:AgentID" json:"-"`
	Skill *Skill `gorm:"foreignKey:SkillID" json:"-"`
}

func (AgentSkill) TableName() string { return "agent_skills" }

// AgentMCPServer is the junction table between Agent and MCPServer.
type AgentMCPServer struct {
	AgentID     string `gorm:"primaryKey;type:text;column:agent_id" json:"agent_id"`
	MCPServerID string `gorm:"primaryKey;type:text;column:mcp_server_id" json:"mcp_server_id"`

	Agent     *Agent     `gorm:"foreignKey:AgentID" json:"-"`
	MCPServer *MCPServer `gorm:"foreignKey:MCPServerID" json:"-"`
}

func (AgentMCPServer) TableName() string { return "agent_mcp_servers" }

// SkillMarketRepo stores a per-project skill market repository configuration.
type SkillMarketRepo struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	ProjectID string    `gorm:"column:project_id;not null;type:text;index" json:"project_id"`
	Name      string    `gorm:"not null;type:text" json:"name"`
	RepoURL   string    `gorm:"column:repo_url;not null;type:text" json:"repo_url"`
	Branch    string    `gorm:"type:text" json:"branch,omitempty"`
	Path      string    `gorm:"type:text" json:"path,omitempty"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (SkillMarketRepo) TableName() string { return "skill_market_repos" }

func (r *SkillMarketRepo) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}

// Event type constants
const (
	EventTypeSessionUpdated = "session_updated"
)

// ProjectEvent represents a persisted event for a project.
// Events are used for SSE streaming to clients.
type ProjectEvent struct {
	ID        string          `gorm:"primaryKey;type:text" json:"id"`
	Seq       int64           `gorm:"column:seq;autoIncrement;uniqueIndex" json:"seq"`
	ProjectID string          `gorm:"column:project_id;not null;type:text;index:idx_project_seq,priority:1" json:"projectId"`
	Type      string          `gorm:"not null;type:text" json:"type"`
	Data      json.RawMessage `gorm:"type:text;not null" json:"data"`
	CreatedAt time.Time       `gorm:"autoCreateTime;index:idx_project_seq,priority:2" json:"createdAt"`

	Project *Project `gorm:"foreignKey:ProjectID" json:"-"`
}

func (ProjectEvent) TableName() string { return "project_events" }

func (e *ProjectEvent) BeforeCreate(_ *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}

// UserPreference represents a user preference (key/value store scoped to user).
type UserPreference struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	UserID    string    `gorm:"column:user_id;not null;type:text;uniqueIndex:idx_user_key" json:"user_id"`
	Key       string    `gorm:"not null;type:text;uniqueIndex:idx_user_key" json:"key"`
	Value     string    `gorm:"not null;type:text" json:"value"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`

	User *User `gorm:"foreignKey:UserID" json:"-"`
}

func (UserPreference) TableName() string { return "user_preferences" }

func (p *UserPreference) BeforeCreate(_ *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}

// AllModels returns all model types for migration.
func AllModels() []interface{} {
	return []interface{}{
		&User{},
		&UserSession{},
		&Project{},
		&ProjectMember{},
		&ProjectInvitation{},
		&Agent{},
		&Workspace{},
		&Session{},
		&Message{},
		&Credential{},
		&TerminalHistory{},
		&Skill{},
		&MCPServer{},
		&AgentSkill{},
		&AgentMCPServer{},
		&SkillMarketRepo{},
		&ProjectEvent{},
		&Job{},
		&DispatcherLeader{},
		&UserPreference{},
	}
}
