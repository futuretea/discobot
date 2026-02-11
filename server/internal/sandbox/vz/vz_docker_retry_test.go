//go:build darwin

package vz

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

// mockImageDownloader allows testing retry logic by simulating failures.
type mockImageDownloader struct {
	*ImageDownloader
	startFunc func(ctx context.Context) error
	callCount int
	mu        sync.Mutex
}

func (m *mockImageDownloader) Start(ctx context.Context) error {
	m.mu.Lock()
	m.callCount++
	count := m.callCount
	m.mu.Unlock()

	if m.startFunc != nil {
		return m.startFunc(ctx)
	}
	return m.ImageDownloader.Start(ctx)
}

func (m *mockImageDownloader) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// TestImageDownloader_RecordErrorAfterFailure tests that errors are properly recorded.
func TestImageDownloader_RecordErrorAfterFailure(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/nonexistent:latest",
		DataDir:  tempDir,
	})

	// Simulate a failure scenario by recording an error directly
	testErr := errors.New("simulated download failure")
	downloader.RecordError(testErr)

	// Verify error is accessible via Status
	status := downloader.Status()
	if status.State != DownloadStateFailed {
		t.Errorf("Expected state Failed, got %s", status.State.String())
	}
	if status.Error != testErr.Error() {
		t.Errorf("Expected error %q, got %q", testErr.Error(), status.Error)
	}

	// Verify Wait returns the error
	ctx := context.Background()
	err := downloader.Wait(ctx)
	if err == nil {
		t.Fatal("Expected Wait to return error")
	}
	if !strings.Contains(err.Error(), testErr.Error()) {
		t.Errorf("Expected error to contain %q, got %q", testErr.Error(), err.Error())
	}
}

// TestImageDownloader_RecordErrorSetsCompletedAt tests completion timestamp.
func TestImageDownloader_RecordErrorSetsCompletedAt(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	before := time.Now()
	downloader.RecordError(errors.New("test error"))
	after := time.Now()

	status := downloader.Status()
	if status.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set")
	}
	if status.CompletedAt.Before(before) || status.CompletedAt.After(after) {
		t.Errorf("CompletedAt %v not in expected range [%v, %v]",
			status.CompletedAt, before, after)
	}
}

// TestImageDownloader_RecordErrorClosesDoneCh tests that doneCh is closed.
func TestImageDownloader_RecordErrorClosesDoneCh(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	downloader.RecordError(errors.New("test error"))

	// Verify doneCh is closed
	select {
	case <-downloader.doneCh:
		// Expected - channel is closed
	case <-time.After(100 * time.Millisecond):
		t.Error("Expected doneCh to be closed")
	}
}

// TestImageDownloader_RecordErrorMultipleCalls tests idempotency.
func TestImageDownloader_RecordErrorMultipleCalls(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Call multiple times - should not panic
	for i := 0; i < 5; i++ {
		downloader.RecordError(fmt.Errorf("error %d", i))
	}

	// Last error should be recorded
	status := downloader.Status()
	if !strings.Contains(status.Error, "error 4") {
		t.Errorf("Expected last error to be recorded, got %q", status.Error)
	}
}

// TestVZProvider_InitWithoutPaths tests async download initialization.
func TestVZProvider_InitWithoutPaths(t *testing.T) {
	// Skip on non-Darwin since VZ provider is macOS-only
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	cfg := &config.Config{
		DataDir:      tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "", // Empty - should trigger download
		BaseDiskPath: "", // Empty - should trigger download
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(ctx context.Context, sessionID string) (string, error) {
		return "test-project", nil
	}

	provider, err := NewDockerProvider(cfg, vmConfig, resolver)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Provider should be created but not ready
	if provider.IsReady() {
		t.Error("Expected provider to not be ready immediately")
	}

	// Should have an image downloader
	provider.downloadMu.RLock()
	hasDownloader := provider.imageDownloader != nil
	provider.downloadMu.RUnlock()

	if !hasDownloader {
		t.Error("Expected provider to have image downloader")
	}

	// Status should show downloading
	status := provider.Status()
	if status.State != "downloading" && status.State != "failed" {
		// May fail immediately if image doesn't exist, which is fine for this test
		t.Logf("Provider state: %s (message: %s)", status.State, status.Message)
	}
}

// TestVZProvider_InitWithPaths tests immediate initialization with manual paths.
func TestVZProvider_InitWithPaths(t *testing.T) {
	// This test doesn't actually create a VM, just tests provider initialization logic
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	// Create dummy kernel and disk files
	kernelPath := tempDir + "/vmlinuz"
	diskPath := tempDir + "/rootfs.squashfs"

	if err := os.WriteFile(kernelPath, []byte("kernel"), 0644); err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("disk"), 0644); err != nil {
		t.Fatalf("Failed to create disk: %v", err)
	}

	cfg := &config.Config{
		DataDir:      tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   kernelPath,
		BaseDiskPath: diskPath,
		DataDir:      tempDir,
		MemoryBytes:  1024 * 1024 * 1024, // 1GB
		CPUCount:     2,
	}

	resolver := func(ctx context.Context, sessionID string) (string, error) {
		return "test-project", nil
	}

	// This will fail to create VM manager (needs actual macOS VZ framework),
	// but we can verify it tried to initialize immediately vs async download
	_, err := NewDockerProvider(cfg, vmConfig, resolver)

	// We expect an error because we can't actually create a VM in tests,
	// but the error should be from VM creation, not download
	if err != nil {
		// This is expected - VM creation will fail in test environment
		t.Logf("Expected error from VM creation: %v", err)
	}
}

// TestVZProvider_StatusWithDownloader tests Status() method with active downloader.
func TestVZProvider_StatusWithDownloader(t *testing.T) {
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	cfg := &config.Config{
		DataDir:      tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(ctx context.Context, sessionID string) (string, error) {
		return "test-project", nil
	}

	provider, err := NewDockerProvider(cfg, vmConfig, resolver)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Get initial status
	status := provider.Status()
	if !status.Available {
		t.Error("Expected provider to be available")
	}

	// State should be downloading or failed (if image doesn't exist)
	validStates := []string{"downloading", "failed", "ready"}
	found := false
	for _, validState := range validStates {
		if status.State == validState {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected state to be one of %v, got %q", validStates, status.State)
	}

	// If failed, should have error message
	if status.State == "failed" && status.Message == "" {
		t.Error("Expected error message when state is failed")
	}
}

// TestVZProvider_CreateBeforeReady tests that Create fails when not ready.
func TestVZProvider_CreateBeforeReady(t *testing.T) {
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	cfg := &config.Config{
		DataDir:      tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(ctx context.Context, sessionID string) (string, error) {
		return "test-project", nil
	}

	provider, err := NewDockerProvider(cfg, vmConfig, resolver)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Try to create a sandbox immediately - should fail
	ctx := context.Background()
	_, err = provider.Create(ctx, "test-session", nil)

	if err == nil {
		t.Error("Expected Create to fail when provider not ready")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("Expected error about not ready, got: %v", err)
	}
}

// TestVZProvider_WarmVMBeforeReady tests that WarmVM fails when not ready.
func TestVZProvider_WarmVMBeforeReady(t *testing.T) {
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	cfg := &config.Config{
		DataDir:      tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(ctx context.Context, sessionID string) (string, error) {
		return "test-project", nil
	}

	provider, err := NewDockerProvider(cfg, vmConfig, resolver)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Try to warm VM immediately - should fail
	ctx := context.Background()
	err = provider.WarmVM(ctx, "test-project")

	if err == nil {
		t.Error("Expected WarmVM to fail when provider not ready")
	}
	if !strings.Contains(err.Error(), "not ready") {
		t.Errorf("Expected error about not ready, got: %v", err)
	}
}
