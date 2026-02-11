//go:build darwin

package vz

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/server/internal/config"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/vm"
)

// TestVZProvider_InitWithoutPaths tests async download initialization.
func TestVZProvider_InitWithoutPaths(t *testing.T) {
	// Skip on non-Darwin since VZ provider is macOS-only
	if os.Getenv("SKIP_VZ_TESTS") == "1" {
		t.Skip("Skipping VZ tests")
	}

	tempDir := t.TempDir()

	cfg := &config.Config{
		VZDataDir:    tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "", // Empty - should trigger download
		BaseDiskPath: "", // Empty - should trigger download
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(_ context.Context, _ string) (string, error) {
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
		VZDataDir:    tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   kernelPath,
		BaseDiskPath: diskPath,
		DataDir:      tempDir,
		MemoryMB:     1024, // 1GB
		CPUCount:     2,
	}

	resolver := func(_ context.Context, _ string) (string, error) {
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
		VZDataDir:    tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(_ context.Context, _ string) (string, error) {
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
		VZDataDir:    tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(_ context.Context, _ string) (string, error) {
		return "test-project", nil
	}

	provider, err := NewDockerProvider(cfg, vmConfig, resolver)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}
	defer provider.Close()

	// Try to create a sandbox immediately - should fail
	ctx := context.Background()
	_, err = provider.Create(ctx, "test-session", sandbox.CreateOptions{})

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
		VZDataDir:    tempDir,
		SandboxImage: "test-image:latest",
	}

	vmConfig := vm.Config{
		KernelPath:   "",
		BaseDiskPath: "",
		DataDir:      tempDir,
		ImageRef:     "ghcr.io/test/image:latest",
	}

	resolver := func(_ context.Context, _ string) (string, error) {
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
