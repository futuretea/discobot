package vz

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

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

// TestImageDownloader_RecordErrorPreservesCompletedAt tests that CompletedAt is not overwritten.
func TestImageDownloader_RecordErrorPreservesCompletedAt(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// First error sets CompletedAt
	downloader.RecordError(errors.New("first error"))
	firstCompletedAt := downloader.Status().CompletedAt

	// Wait a bit to ensure time would be different
	time.Sleep(10 * time.Millisecond)

	// Second error should not change CompletedAt
	downloader.RecordError(errors.New("second error"))
	secondCompletedAt := downloader.Status().CompletedAt

	if !firstCompletedAt.Equal(secondCompletedAt) {
		t.Errorf("Expected CompletedAt to remain %v, got %v",
			firstCompletedAt, secondCompletedAt)
	}
}

// TestImageDownloader_FailedStateBlocks tests that GetPaths returns false when failed.
func TestImageDownloader_FailedStateBlocks(t *testing.T) {
	tempDir := t.TempDir()

	downloader := NewImageDownloader(DownloadConfig{
		ImageRef: "ghcr.io/test/image:latest",
		DataDir:  tempDir,
	})

	// Set paths manually (simulating successful download)
	downloader.kernelPath = "/test/kernel"
	downloader.baseDiskPath = "/test/disk"

	// But mark as failed
	downloader.RecordError(errors.New("test error"))

	// GetPaths should still return false because state is failed
	_, _, ok := downloader.GetPaths()
	if ok {
		t.Error("Expected GetPaths to return false when state is Failed")
	}
}
