package vz

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestRecordError verifies that RecordError properly updates state and progress.
func TestRecordError(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Record an error
	testErr := errors.New("test error message")
	downloader.RecordError(testErr)

	// Verify state is failed
	downloader.stateMu.RLock()
	state := downloader.state
	downloader.stateMu.RUnlock()

	if state != DownloadStateFailed {
		t.Errorf("Expected state Failed, got %s", state.String())
	}

	// Verify progress has error
	progress := downloader.Status()
	if progress.State != DownloadStateFailed {
		t.Errorf("Expected progress state Failed, got %s", progress.State.String())
	}
	if progress.Error != testErr.Error() {
		t.Errorf("Expected error %q, got %q", testErr.Error(), progress.Error)
	}
	if progress.CompletedAt.IsZero() {
		t.Error("Expected CompletedAt to be set")
	}

	// Verify doneCh is closed
	select {
	case <-downloader.doneCh:
		// Expected - channel is closed
	default:
		t.Error("Expected doneCh to be closed")
	}

	// Verify Wait returns error
	ctx := context.Background()
	err := downloader.Wait(ctx)
	if err == nil {
		t.Error("Expected Wait to return error, got nil")
	}
	if !contains(err.Error(), testErr.Error()) {
		t.Errorf("Expected Wait error to contain %q, got %q", testErr.Error(), err.Error())
	}
}

// TestRecordError_MultipleCallsSafe verifies RecordError can be called multiple times safely.
func TestRecordError_MultipleCallsSafe(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Call RecordError multiple times
	downloader.RecordError(errors.New("first error"))
	downloader.RecordError(errors.New("second error"))
	downloader.RecordError(errors.New("third error"))

	// Should not panic and should have the last error
	progress := downloader.Status()
	if progress.Error != "third error" {
		t.Errorf("Expected last error, got %q", progress.Error)
	}
}

// TestGetPaths_NotReady verifies GetPaths returns false when download not complete.
func TestGetPaths_NotReady(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Should return false when not ready
	_, _, ok := downloader.GetPaths()
	if ok {
		t.Error("Expected GetPaths to return false when not ready")
	}

	// Even after recording error
	downloader.RecordError(errors.New("test error"))
	_, _, ok = downloader.GetPaths()
	if ok {
		t.Error("Expected GetPaths to return false after error")
	}
}

// TestGetPaths_Ready verifies GetPaths returns paths when download complete.
func TestGetPaths_Ready(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Manually set state to ready and populate paths
	testKernelPath := "/test/kernel"
	testDiskPath := "/test/disk"

	downloader.kernelPath = testKernelPath
	downloader.baseDiskPath = testDiskPath
	downloader.updateState(DownloadStateReady)

	// Should return paths
	kernelPath, diskPath, ok := downloader.GetPaths()
	if !ok {
		t.Fatal("Expected GetPaths to return true when ready")
	}
	if kernelPath != testKernelPath {
		t.Errorf("Expected kernel path %q, got %q", testKernelPath, kernelPath)
	}
	if diskPath != testDiskPath {
		t.Errorf("Expected disk path %q, got %q", testDiskPath, diskPath)
	}
}

// TestWait_Success verifies Wait returns nil when download succeeds.
func TestWait_Success(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Simulate successful completion in background
	go func() {
		time.Sleep(50 * time.Millisecond)
		downloader.updateState(DownloadStateReady)
		close(downloader.doneCh)
	}()

	// Wait should return nil
	ctx := context.Background()
	err := downloader.Wait(ctx)
	if err != nil {
		t.Errorf("Expected Wait to return nil on success, got %v", err)
	}
}

// TestWait_ContextCanceled verifies Wait respects context cancellation.
func TestWait_ContextCanceled(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Wait should return context error
	err := downloader.Wait(ctx)
	if err == nil {
		t.Error("Expected Wait to return error when context canceled")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected context.DeadlineExceeded, got %v", err)
	}
}

// TestStatus_InitialState verifies initial status.
func TestStatus_InitialState(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	progress := downloader.Status()
	if progress.State != DownloadStateNotStarted {
		t.Errorf("Expected state NotStarted, got %s", progress.State.String())
	}
	if progress.BytesDownloaded != 0 {
		t.Errorf("Expected BytesDownloaded=0, got %d", progress.BytesDownloaded)
	}
	if progress.TotalBytes != 0 {
		t.Errorf("Expected TotalBytes=0, got %d", progress.TotalBytes)
	}
	if progress.Error != "" {
		t.Errorf("Expected empty error, got %q", progress.Error)
	}
}

// TestCheckCache_NoCache verifies checkCache returns false when no cache exists.
func TestCheckCache_NoCache(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	cached, _, _ := downloader.checkCache()
	if cached {
		t.Error("Expected checkCache to return false when no cache exists")
	}
}

// TestCheckCache_WithCache verifies checkCache returns true when cache exists.
func TestCheckCache_WithCache(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Create cache directory and files
	digest := downloader.computeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	kernelPath := filepath.Join(cacheDir, "vmlinuz")
	diskPath := filepath.Join(cacheDir, "discobot-rootfs.squashfs")

	// Write dummy files with content
	if err := os.WriteFile(kernelPath, []byte("kernel data"), 0644); err != nil {
		t.Fatalf("Failed to write kernel: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("disk data"), 0644); err != nil {
		t.Fatalf("Failed to write disk: %v", err)
	}

	// Should find cache
	cached, foundKernel, foundDisk := downloader.checkCache()
	if !cached {
		t.Error("Expected checkCache to return true when cache exists")
	}
	if foundKernel != kernelPath {
		t.Errorf("Expected kernel path %q, got %q", kernelPath, foundKernel)
	}
	if foundDisk != diskPath {
		t.Errorf("Expected disk path %q, got %q", diskPath, foundDisk)
	}
}

// TestCheckCache_EmptyFiles verifies checkCache returns false for empty cached files.
func TestCheckCache_EmptyFiles(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Create cache directory with empty files
	digest := downloader.computeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	kernelPath := filepath.Join(cacheDir, "vmlinuz")
	diskPath := filepath.Join(cacheDir, "discobot-rootfs.squashfs")

	// Write empty files
	if err := os.WriteFile(kernelPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write kernel: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to write disk: %v", err)
	}

	// Should not consider empty files as valid cache
	cached, _, _ := downloader.checkCache()
	if cached {
		t.Error("Expected checkCache to return false for empty cached files")
	}
}

// TestComputeDigest_Stability verifies digest computation is deterministic.
func TestComputeDigest_Stability(t *testing.T) {
	tempDir := t.TempDir()

	imageRef := "ghcr.io/test/image:v1.2.3"

	d1 := NewImageDownloader(DownloadConfig{
		ImageRef: imageRef,
		DataDir:  tempDir,
	})
	d2 := NewImageDownloader(DownloadConfig{
		ImageRef: imageRef,
		DataDir:  tempDir,
	})

	digest1 := d1.computeDigest()
	digest2 := d2.computeDigest()

	if digest1 != digest2 {
		t.Errorf("Digest mismatch: %q != %q", digest1, digest2)
	}

	// Verify digest format
	if len(digest1) != 19 {
		t.Errorf("Expected digest length 19, got %d", len(digest1))
	}
	if digest1[:7] != "sha256-" {
		t.Errorf("Expected digest to start with 'sha256-', got %q", digest1[:7])
	}
}

// TestComputeDigest_DifferentImages verifies different images produce different digests.
func TestComputeDigest_DifferentImages(t *testing.T) {
	tempDir := t.TempDir()

	d1 := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:v1",
		DataDir:  tempDir,
	})
	d2 := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:v2",
		DataDir:  tempDir,
	})

	digest1 := d1.computeDigest()
	digest2 := d2.computeDigest()

	if digest1 == digest2 {
		t.Error("Expected different digests for different image refs")
	}
}

// TestStart_UsesCachedImage verifies Start returns immediately when image is cached.
func TestStart_UsesCachedImage(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Create valid cache
	digest := downloader.computeDigest()
	cacheDir := filepath.Join(tempDir, "images", digest)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	kernelPath := filepath.Join(cacheDir, "vmlinuz")
	diskPath := filepath.Join(cacheDir, "discobot-rootfs.squashfs")

	if err := os.WriteFile(kernelPath, []byte("kernel"), 0644); err != nil {
		t.Fatalf("Failed to write kernel: %v", err)
	}
	if err := os.WriteFile(diskPath, []byte("disk"), 0644); err != nil {
		t.Fatalf("Failed to write disk: %v", err)
	}

	// Start should return immediately
	start := time.Now()
	ctx := context.Background()
	err := downloader.Start(ctx)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Expected Start to succeed with cached image, got %v", err)
	}

	// Should be very fast (< 1 second)
	if elapsed > time.Second {
		t.Errorf("Start took too long with cache: %v (expected < 1s)", elapsed)
	}

	// Verify state is ready
	progress := downloader.Status()
	if progress.State != DownloadStateReady {
		t.Errorf("Expected state Ready, got %s", progress.State.String())
	}

	// Verify paths are set
	k, d, ok := downloader.GetPaths()
	if !ok {
		t.Error("Expected GetPaths to return true after cached Start")
	}
	if k != kernelPath {
		t.Errorf("Expected kernel path %q, got %q", kernelPath, k)
	}
	if d != diskPath {
		t.Errorf("Expected disk path %q, got %q", diskPath, d)
	}

	// Verify doneCh is closed
	select {
	case <-downloader.doneCh:
		// Expected
	default:
		t.Error("Expected doneCh to be closed after Start")
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
