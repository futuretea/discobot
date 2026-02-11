package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
)

// TestCommitSession_WorkspaceLevel tests that CommitSession checks workspace commit status.
func TestCommitSession_WorkspaceLevel(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusNone // Reset to none
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Create a mock job enqueuer to capture the job
	var enqueuedJob jobs.JobPayload
	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, payload jobs.JobPayload) error {
			enqueuedJob = payload
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Commit the session
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err != nil {
		t.Fatalf("CommitSession failed: %v", err)
	}

	// Verify workspace commit status is set to pending
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusPending {
		t.Errorf("Expected workspace commit status to be pending, got %s", ws.CommitStatus)
	}

	// Verify session commit status is also set to pending
	sess, err := env.store.GetSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if sess.CommitStatus != model.CommitStatusPending {
		t.Errorf("Expected session commit status to be pending, got %s", sess.CommitStatus)
	}

	// Verify job was enqueued with correct payload
	if enqueuedJob == nil {
		t.Fatal("Expected job to be enqueued")
	}
	commitPayload, ok := enqueuedJob.(jobs.SessionCommitPayload)
	if !ok {
		t.Fatalf("Expected SessionCommitPayload, got %T", enqueuedJob)
	}
	if commitPayload.WorkspaceID != workspace.ID {
		t.Errorf("Expected workspace ID %s, got %s", workspace.ID, commitPayload.WorkspaceID)
	}
	if commitPayload.SessionID != session.ID {
		t.Errorf("Expected session ID %s, got %s", session.ID, commitPayload.SessionID)
	}
}

// TestCommitSession_WorkspaceAlreadyCommitting tests that CommitSession fails if workspace is already committing.
func TestCommitSession_WorkspaceAlreadyCommitting(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Set workspace to committing status
	workspace.CommitStatus = model.CommitStatusCommitting
	if err := env.store.UpdateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to update workspace: %v", err)
	}

	// Create session
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusNone
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			t.Fatal("Job should not be enqueued when workspace is already committing")
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when workspace is already committing")
	}
	if err.Error() != "commit already in progress for workspace (status: committing)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestCommitSession_WorkspacePending tests that CommitSession fails if workspace is pending.
func TestCommitSession_WorkspacePending(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Set workspace to pending status (another session is committing)
	workspace.CommitStatus = model.CommitStatusPending
	if err := env.store.UpdateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to update workspace: %v", err)
	}

	// Create a different session
	session := &model.Session{
		ID:           "test-session-2",
		ProjectID:    project.ID,
		WorkspaceID:  workspace.ID,
		AgentID:      ptrString(agent.ID),
		Name:         "Test Session 2",
		Status:       model.SessionStatusReady,
		CommitStatus: model.CommitStatusNone,
		BaseCommit:   ptrString(initialCommit),
	}
	if err := env.store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			t.Fatal("Job should not be enqueued when workspace is already pending")
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when workspace is already pending")
	}
	if err.Error() != "commit already in progress for workspace (status: pending)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestPerformCommit_WorkspaceAlreadyCompleted tests that PerformCommit skips if workspace is already completed.
func TestPerformCommit_WorkspaceAlreadyCompleted(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Set workspace to completed
	workspace.CommitStatus = model.CommitStatusCompleted
	if err := env.store.UpdateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to update workspace: %v", err)
	}

	// Create session with pending status
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, nil)

	// Run PerformCommit - should be a no-op
	err := sessionSvc.PerformCommit(context.Background(), project.ID, session.ID)
	if err != nil {
		t.Fatalf("PerformCommit failed: %v", err)
	}

	// Verify workspace status unchanged
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusCompleted {
		t.Errorf("Expected workspace commit status to remain completed, got %s", ws.CommitStatus)
	}
}

// TestPerformCommit_WorkspaceNotPendingOrCommitting tests that PerformCommit skips if workspace is not pending/committing.
func TestPerformCommit_WorkspaceNotPendingOrCommitting(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Set workspace to "none" commit status
	workspace.CommitStatus = model.CommitStatusNone
	if err := env.store.UpdateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to update workspace: %v", err)
	}

	// Create session with pending status (mismatched state)
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, nil)

	// Run PerformCommit - should skip
	err := sessionSvc.PerformCommit(context.Background(), project.ID, session.ID)
	if err != nil {
		t.Fatalf("PerformCommit failed: %v", err)
	}

	// Verify workspace status unchanged
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusNone {
		t.Errorf("Expected workspace commit status to remain none, got %s", ws.CommitStatus)
	}
}

// TestSessionCommitPayload_ResourceKey tests that SessionCommitPayload returns workspace resource.
func TestSessionCommitPayload_ResourceKey(t *testing.T) {
	payload := jobs.SessionCommitPayload{
		ProjectID:   "test-project",
		SessionID:   "test-session",
		WorkspaceID: "test-workspace",
	}

	resourceType, resourceID := payload.ResourceKey()

	if resourceType != jobs.ResourceTypeWorkspace {
		t.Errorf("Expected resource type %s, got %s", jobs.ResourceTypeWorkspace, resourceType)
	}

	if resourceID != "test-workspace" {
		t.Errorf("Expected resource ID test-workspace, got %s", resourceID)
	}
}

// TestPerformCommit_UpdatesWorkspaceToFailed tests that workspace status transitions to failed on error.
func TestPerformCommit_UpdatesWorkspaceToFailed(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, _ := env.createTestWorkspace(t, project.ID)

	// Set workspace to pending
	workspace.CommitStatus = model.CommitStatusPending
	if err := env.store.UpdateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to update workspace: %v", err)
	}

	// Create session with pending status but no baseCommit
	session := &model.Session{
		ID:           "test-session",
		ProjectID:    project.ID,
		WorkspaceID:  workspace.ID,
		AgentID:      ptrString(agent.ID),
		Name:         "Test Session",
		Status:       model.SessionStatusReady,
		CommitStatus: model.CommitStatusPending,
		BaseCommit:   nil, // Missing baseCommit will cause failure
	}
	if err := env.store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, nil)

	// Run PerformCommit - should set workspace to failed
	err := sessionSvc.PerformCommit(context.Background(), project.ID, session.ID)
	if err != nil {
		t.Fatalf("PerformCommit returned error: %v", err)
	}

	// Verify workspace status transitioned to failed
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusFailed {
		t.Errorf("Expected workspace commit status to be failed, got %s", ws.CommitStatus)
	}
	if ws.CommitError == nil {
		t.Error("Expected workspace commit error to be set")
	} else if *ws.CommitError != "No base commit set" {
		t.Errorf("Expected error 'No base commit set', got %s", *ws.CommitError)
	}

	// Verify session status also transitioned to failed
	sess, err := env.store.GetSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if sess.CommitStatus != model.CommitStatusFailed {
		t.Errorf("Expected session commit status to be failed, got %s", sess.CommitStatus)
	}
	if sess.CommitError == nil {
		t.Error("Expected session commit error to be set")
	}
}

// TestCommitSession_EnqueueFailureRevertsWorkspace tests that workspace status is reverted if job enqueue fails.
func TestCommitSession_EnqueueFailureRevertsWorkspace(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusNone
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Create a mock job enqueuer that fails
	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			return jobs.ErrJobAlreadyExists
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when enqueue fails")
	}

	// Verify workspace commit status was reverted to none
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusNone {
		t.Errorf("Expected workspace commit status to be reverted to none, got %s", ws.CommitStatus)
	}

	// Verify session commit status was also reverted
	sess, err := env.store.GetSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if sess.CommitStatus != model.CommitStatusNone {
		t.Errorf("Expected session commit status to be reverted to none, got %s", sess.CommitStatus)
	}
}

// TestCommitSession_SessionAlreadyPending tests that commit is rejected if session is already pending.
func TestCommitSession_SessionAlreadyPending(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session with pending commit status
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusPending
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			t.Fatal("Job should not be enqueued when session is already pending")
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when session is already pending")
	}
	if err.Error() != "commit already in progress for session (status: pending)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestCommitSession_SessionAlreadyCommitting tests that commit is rejected if session is already committing.
func TestCommitSession_SessionAlreadyCommitting(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session with committing status
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusCommitting
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			t.Fatal("Job should not be enqueued when session is already committing")
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when session is already committing")
	}
	if err.Error() != "commit already in progress for session (status: committing)" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestCommitSession_SessionAlreadyCompleted tests that commit is rejected if session already committed.
func TestCommitSession_SessionAlreadyCompleted(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session with completed status
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusCompleted
	session.AppliedCommit = ptrString("abc123")
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			t.Fatal("Job should not be enqueued when session is already completed")
			return nil
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail when session is already completed")
	}
	if err.Error() != "session already committed" {
		t.Errorf("Unexpected error message: %v", err)
	}
}

// TestCommitSession_DeferCleansUpWorkspace tests that defer reverts workspace status on error after workspace update.
func TestCommitSession_DeferCleansUpWorkspace(t *testing.T) {
	env := newTestEnv(t)
	defer env.cleanup()

	project := env.createTestProject(t)
	agent := env.createTestAgent(t, project.ID)
	workspace, initialCommit := env.createTestWorkspace(t, project.ID)

	// Create session
	session := env.createTestSession(t, project.ID, workspace.ID, agent.ID, initialCommit)
	session.CommitStatus = model.CommitStatusNone
	if err := env.store.UpdateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to update session: %v", err)
	}

	// Mock enqueuer that will fail
	mockEnqueuer := &mockJobEnqueuer{
		enqueueFunc: func(_ context.Context, _ jobs.JobPayload) error {
			return fmt.Errorf("simulated enqueue error")
		},
	}

	sessionSvc := NewSessionService(env.store, env.gitService, env.mockSandbox, nil, env.eventBroker, mockEnqueuer)

	// Attempt to commit - should fail
	err := sessionSvc.CommitSession(context.Background(), project.ID, session.ID, mockEnqueuer)
	if err == nil {
		t.Fatal("Expected CommitSession to fail")
	}

	// Verify workspace status was reverted by defer
	ws, err := env.store.GetWorkspaceByID(context.Background(), workspace.ID)
	if err != nil {
		t.Fatalf("Failed to get workspace: %v", err)
	}
	if ws.CommitStatus != model.CommitStatusNone {
		t.Errorf("Expected workspace commit status to be reverted to none by defer, got %s", ws.CommitStatus)
	}

	// Verify session status was also reverted by defer
	sess, err := env.store.GetSessionByID(context.Background(), session.ID)
	if err != nil {
		t.Fatalf("Failed to get session: %v", err)
	}
	if sess.CommitStatus != model.CommitStatusNone {
		t.Errorf("Expected session commit status to be reverted to none by defer, got %s", sess.CommitStatus)
	}
	if sess.BaseCommit != nil {
		t.Errorf("Expected session baseCommit to be cleared by defer, got %v", sess.BaseCommit)
	}
}

// mockJobEnqueuer is a mock implementation of JobEnqueuer for testing.
type mockJobEnqueuer struct {
	enqueueFunc func(ctx context.Context, payload jobs.JobPayload) error
}

func (m *mockJobEnqueuer) Enqueue(ctx context.Context, payload jobs.JobPayload) error {
	if m.enqueueFunc != nil {
		return m.enqueueFunc(ctx, payload)
	}
	return nil
}
