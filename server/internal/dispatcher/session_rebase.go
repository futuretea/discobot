package dispatcher

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/service"
)

// SessionRebaseExecutor handles session_rebase jobs.
type SessionRebaseExecutor struct {
	sessionService *service.SessionService
}

// NewSessionRebaseExecutor creates a new session rebase executor.
func NewSessionRebaseExecutor(sessionSvc *service.SessionService) *SessionRebaseExecutor {
	return &SessionRebaseExecutor{sessionService: sessionSvc}
}

// Type returns the job type this executor handles.
func (e *SessionRebaseExecutor) Type() jobs.JobType {
	return jobs.JobTypeSessionRebase
}

// Execute processes the job.
func (e *SessionRebaseExecutor) Execute(ctx context.Context, job *model.Job) error {
	if e.sessionService == nil {
		return fmt.Errorf("session service not available")
	}

	var payload jobs.SessionRebasePayload
	if err := json.Unmarshal(job.Payload, &payload); err != nil {
		return fmt.Errorf("invalid payload: %w", err)
	}

	if payload.SessionID == "" {
		return fmt.Errorf("sessionId is required")
	}
	if payload.ProjectID == "" {
		return fmt.Errorf("projectId is required")
	}

	return e.sessionService.PerformRebase(ctx, payload.ProjectID, payload.SessionID)
}
