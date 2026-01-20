// Package jobs defines job types and payloads for background job processing.
package jobs

// JobType represents the type of job.
type JobType string

const (
	JobTypeSessionInit   JobType = "session_init"
	JobTypeSessionDelete JobType = "session_delete"
	JobTypeSessionCommit JobType = "session_commit"
	JobTypeWorkspaceInit JobType = "workspace_init"
)

// SessionInitPayload is the payload for session_init jobs.
type SessionInitPayload struct {
	ProjectID   string `json:"projectId"`
	SessionID   string `json:"sessionId"`
	WorkspaceID string `json:"workspaceId"`
	AgentID     string `json:"agentId"`
}

// WorkspaceInitPayload is the payload for workspace_init jobs.
type WorkspaceInitPayload struct {
	ProjectID   string `json:"projectId"`
	WorkspaceID string `json:"workspaceId"`
}

// SessionDeletePayload is the payload for session_delete jobs.
type SessionDeletePayload struct {
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId"`
}

// SessionCommitPayload is the payload for session_commit jobs.
type SessionCommitPayload struct {
	ProjectID string `json:"projectId"`
	SessionID string `json:"sessionId"`
}
