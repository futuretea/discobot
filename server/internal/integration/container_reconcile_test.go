package integration

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/anthropics/octobot/server/internal/config"
	"github.com/anthropics/octobot/server/internal/container"
	"github.com/anthropics/octobot/server/internal/container/docker"
	"github.com/anthropics/octobot/server/internal/database"
	"github.com/anthropics/octobot/server/internal/model"
	"github.com/anthropics/octobot/server/internal/service"
	"github.com/anthropics/octobot/server/internal/store"
)

// Small, fast images for testing
const (
	testImageOld = "busybox:1.36"
	testImageNew = "busybox:1.37"
)

// skipIfNoDocker skips the test if Docker is not available
func skipIfNoDocker(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker not found in PATH, skipping test")
	}

	// Check if Docker daemon is running
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		t.Skip("Docker daemon not running, skipping test")
	}
}

// pullImage ensures an image is available locally
func pullImage(t *testing.T, image string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", "pull", image)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to pull image %s: %v\nOutput: %s", image, err, output)
	}
}

// testContainerSetup holds test resources
type testContainerSetup struct {
	provider   *docker.Provider
	store      *store.Store
	db         *database.DB
	cfg        *config.Config
	workingDir string
}

// newTestContainerSetup creates a new test setup with real Docker
func newTestContainerSetup(t *testing.T) *testContainerSetup {
	t.Helper()
	skipIfNoDocker(t)

	// Pull test images first
	t.Log("Pulling test images...")
	pullImage(t, testImageOld)
	pullImage(t, testImageNew)

	workingDir := t.TempDir()
	dbDir := t.TempDir()

	cfg := &config.Config{
		DatabaseDSN:          fmt.Sprintf("sqlite3://%s/test.db", dbDir),
		DatabaseDriver:       "sqlite",
		ContainerImage:       testImageNew, // Expected image
		ContainerIdleTimeout: 5 * time.Minute,
		WorkspaceDir:         workingDir,
	}

	db, err := database.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}

	if err := db.Migrate(); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	s := store.New(db.DB)

	provider, err := docker.NewProvider(cfg)
	if err != nil {
		db.Close()
		t.Fatalf("Failed to create Docker provider: %v", err)
	}

	setup := &testContainerSetup{
		provider:   provider,
		store:      s,
		db:         db,
		cfg:        cfg,
		workingDir: workingDir,
	}

	t.Cleanup(func() {
		// Clean up any containers created during the test
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		containers, _ := provider.List(ctx)
		for _, c := range containers {
			_ = provider.Remove(ctx, c.SessionID)
		}

		provider.Close()
		db.Close()
	})

	return setup
}

// createTestProject creates a project for testing
func (s *testContainerSetup) createTestProject(t *testing.T) *model.Project {
	t.Helper()
	project := &model.Project{
		Name: "test-project",
		Slug: fmt.Sprintf("test-project-%d", time.Now().UnixNano()),
	}
	if err := s.store.CreateProject(context.Background(), project); err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}
	return project
}

// createTestWorkspace creates a workspace for testing
func (s *testContainerSetup) createTestWorkspace(t *testing.T, project *model.Project) *model.Workspace {
	t.Helper()
	workspace := &model.Workspace{
		ProjectID:  project.ID,
		Path:       s.workingDir,
		SourceType: "local",
		Status:     model.WorkspaceStatusReady,
	}
	if err := s.store.CreateWorkspace(context.Background(), workspace); err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	return workspace
}

// createTestSession creates a session for testing
func (s *testContainerSetup) createTestSession(t *testing.T, workspace *model.Workspace, name string) *model.Session {
	t.Helper()
	session := &model.Session{
		ProjectID:   workspace.ProjectID,
		WorkspaceID: workspace.ID,
		Name:        name,
		Status:      model.SessionStatusRunning,
	}
	if err := s.store.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("Failed to create session: %v", err)
	}
	return session
}

// createContainerWithImage creates a container with a specific image
func (s *testContainerSetup) createContainerWithImage(t *testing.T, sessionID, image string) *container.Container {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	opts := container.CreateOptions{
		Image:   image,
		WorkDir: "/",
		Cmd:     []string{"sleep", "3600"}, // Keep container running
		Labels: map[string]string{
			"octobot.session.id": sessionID,
		},
	}

	c, err := s.provider.Create(ctx, sessionID, opts)
	if err != nil {
		t.Fatalf("Failed to create container: %v", err)
	}

	if err := s.provider.Start(ctx, sessionID); err != nil {
		t.Fatalf("Failed to start container: %v", err)
	}

	return c
}

func TestReconcileContainers_ReplacesOutdatedImage(t *testing.T) {
	setup := newTestContainerSetup(t)
	ctx := context.Background()

	// Create test data
	project := setup.createTestProject(t)
	workspace := setup.createTestWorkspace(t, project)
	session := setup.createTestSession(t, workspace, "test-session-1")

	// Create a container with the OLD image
	t.Logf("Creating container with old image: %s", testImageOld)
	setup.createContainerWithImage(t, session.ID, testImageOld)

	// Verify container exists with old image
	c, err := setup.provider.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get container: %v", err)
	}
	if c.Image != testImageOld {
		t.Fatalf("Expected container image %s, got %s", testImageOld, c.Image)
	}
	if c.Status != container.StatusRunning {
		t.Fatalf("Expected container status running, got %s", c.Status)
	}
	oldContainerID := c.ID
	t.Logf("Container created with ID: %s", oldContainerID)

	// Create container service with NEW image as expected
	containerSvc := service.NewContainerService(setup.store, setup.provider, setup.cfg)

	// Run reconciliation
	t.Log("Running container reconciliation...")
	if err := containerSvc.ReconcileContainers(ctx); err != nil {
		t.Fatalf("ReconcileContainers failed: %v", err)
	}

	// Verify container was recreated with new image
	c, err = setup.provider.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get container after reconciliation: %v", err)
	}

	if c.Image != testImageNew {
		t.Errorf("Expected container image %s after reconciliation, got %s", testImageNew, c.Image)
	}

	if c.ID == oldContainerID {
		t.Errorf("Container ID should have changed after recreation, still %s", c.ID)
	}

	t.Logf("Container recreated with new ID: %s, image: %s", c.ID, c.Image)
}

func TestReconcileContainers_SkipsCorrectImage(t *testing.T) {
	setup := newTestContainerSetup(t)
	ctx := context.Background()

	// Create test data
	project := setup.createTestProject(t)
	workspace := setup.createTestWorkspace(t, project)
	session := setup.createTestSession(t, workspace, "test-session-correct")

	// Create a container with the CORRECT (new) image
	t.Logf("Creating container with correct image: %s", testImageNew)
	setup.createContainerWithImage(t, session.ID, testImageNew)

	// Get container info before reconciliation
	c, err := setup.provider.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get container: %v", err)
	}
	originalID := c.ID
	t.Logf("Container created with ID: %s", originalID)

	// Create container service
	containerSvc := service.NewContainerService(setup.store, setup.provider, setup.cfg)

	// Run reconciliation
	t.Log("Running container reconciliation...")
	if err := containerSvc.ReconcileContainers(ctx); err != nil {
		t.Fatalf("ReconcileContainers failed: %v", err)
	}

	// Verify container was NOT recreated
	c, err = setup.provider.Get(ctx, session.ID)
	if err != nil {
		t.Fatalf("Failed to get container after reconciliation: %v", err)
	}

	if c.ID != originalID {
		t.Errorf("Container should NOT have been recreated, ID changed from %s to %s", originalID, c.ID)
	}

	if c.Image != testImageNew {
		t.Errorf("Container image should still be %s, got %s", testImageNew, c.Image)
	}

	t.Log("Container correctly skipped (already using correct image)")
}

func TestReconcileContainers_RemovesOrphanedContainers(t *testing.T) {
	setup := newTestContainerSetup(t)
	ctx := context.Background()

	// Create a container WITHOUT a corresponding session in the database
	orphanSessionID := "orphan-session-id"
	t.Logf("Creating orphaned container (no session in DB)")
	setup.createContainerWithImage(t, orphanSessionID, testImageOld)

	// Verify container exists
	c, err := setup.provider.Get(ctx, orphanSessionID)
	if err != nil {
		t.Fatalf("Failed to get orphaned container: %v", err)
	}
	t.Logf("Orphaned container created with ID: %s", c.ID)

	// Create container service
	containerSvc := service.NewContainerService(setup.store, setup.provider, setup.cfg)

	// Run reconciliation
	t.Log("Running container reconciliation...")
	if err := containerSvc.ReconcileContainers(ctx); err != nil {
		t.Fatalf("ReconcileContainers failed: %v", err)
	}

	// Verify orphaned container was removed
	_, err = setup.provider.Get(ctx, orphanSessionID)
	if err != container.ErrNotFound {
		t.Errorf("Expected orphaned container to be removed, got error: %v", err)
	}

	t.Log("Orphaned container correctly removed")
}

func TestReconcileContainers_MultipleContainers(t *testing.T) {
	setup := newTestContainerSetup(t)
	ctx := context.Background()

	// Create test data
	project := setup.createTestProject(t)
	workspace := setup.createTestWorkspace(t, project)

	// Create 3 sessions: 2 with old image, 1 with new image
	session1 := setup.createTestSession(t, workspace, "session-old-1")
	session2 := setup.createTestSession(t, workspace, "session-old-2")
	session3 := setup.createTestSession(t, workspace, "session-new")

	t.Log("Creating containers...")
	setup.createContainerWithImage(t, session1.ID, testImageOld)
	setup.createContainerWithImage(t, session2.ID, testImageOld)
	setup.createContainerWithImage(t, session3.ID, testImageNew)

	// Get original container IDs
	c1, _ := setup.provider.Get(ctx, session1.ID)
	c2, _ := setup.provider.Get(ctx, session2.ID)
	c3, _ := setup.provider.Get(ctx, session3.ID)

	originalIDs := map[string]string{
		session1.ID: c1.ID,
		session2.ID: c2.ID,
		session3.ID: c3.ID,
	}

	t.Logf("Original container IDs: %v", originalIDs)

	// Create container service
	containerSvc := service.NewContainerService(setup.store, setup.provider, setup.cfg)

	// Run reconciliation
	t.Log("Running container reconciliation...")
	if err := containerSvc.ReconcileContainers(ctx); err != nil {
		t.Fatalf("ReconcileContainers failed: %v", err)
	}

	// Verify results
	// Session 1 and 2 should have new containers
	for _, sessionID := range []string{session1.ID, session2.ID} {
		c, err := setup.provider.Get(ctx, sessionID)
		if err != nil {
			t.Errorf("Failed to get container for session %s: %v", sessionID, err)
			continue
		}
		if c.Image != testImageNew {
			t.Errorf("Session %s: expected image %s, got %s", sessionID, testImageNew, c.Image)
		}
		if c.ID == originalIDs[sessionID] {
			t.Errorf("Session %s: container should have been recreated", sessionID)
		}
	}

	// Session 3 should have the same container
	c3After, err := setup.provider.Get(ctx, session3.ID)
	if err != nil {
		t.Fatalf("Failed to get container for session3: %v", err)
	}
	if c3After.ID != originalIDs[session3.ID] {
		t.Errorf("Session 3: container should NOT have been recreated, ID changed from %s to %s",
			originalIDs[session3.ID], c3After.ID)
	}

	t.Log("Multiple container reconciliation completed successfully")
}

func TestReconcileContainers_NoContainers(t *testing.T) {
	setup := newTestContainerSetup(t)
	ctx := context.Background()

	// Create container service
	containerSvc := service.NewContainerService(setup.store, setup.provider, setup.cfg)

	// Run reconciliation with no containers
	t.Log("Running container reconciliation with no containers...")
	if err := containerSvc.ReconcileContainers(ctx); err != nil {
		t.Fatalf("ReconcileContainers failed: %v", err)
	}

	// Verify no containers exist
	containers, err := setup.provider.List(ctx)
	if err != nil {
		t.Fatalf("Failed to list containers: %v", err)
	}

	if len(containers) != 0 {
		t.Errorf("Expected 0 containers, got %d", len(containers))
	}

	t.Log("Empty container reconciliation completed successfully")
}
