package cache

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func TestCache_GetPut(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	c, err := New(tmpDir, 10*1024*1024, true, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Test cache miss
	_, err = c.Get("test-key")
	if err != ErrCacheMiss {
		t.Errorf("expected cache miss, got %v", err)
	}

	// Put entry
	entry := &Entry{
		StatusCode: 200,
		Headers:    http.Header{"Content-Type": []string{"text/plain"}},
		Body:       []byte("test body"),
		Size:       9,
	}

	if err := c.Put("test-key", entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Test cache hit
	retrieved, err := c.Get("test-key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if retrieved.StatusCode != entry.StatusCode {
		t.Errorf("status code mismatch: got %d, want %d", retrieved.StatusCode, entry.StatusCode)
	}

	if string(retrieved.Body) != string(entry.Body) {
		t.Errorf("body mismatch: got %s, want %s", retrieved.Body, entry.Body)
	}

	// Check stats
	stats := c.GetStats()
	if stats.Hits != 1 {
		t.Errorf("expected 1 hit, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("expected 1 miss, got %d", stats.Misses)
	}
	if stats.Stores != 1 {
		t.Errorf("expected 1 store, got %d", stats.Stores)
	}
}

func TestCache_Eviction(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	// Create cache with small max size (100 bytes)
	c, err := New(tmpDir, 100, true, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Add entries that exceed max size
	for i := 0; i < 5; i++ {
		entry := &Entry{
			StatusCode: 200,
			Headers:    http.Header{},
			Body:       make([]byte, 50), // 50 bytes each
			Size:       50,
		}
		key := string(rune('a' + i))
		if err := c.Put(key, entry); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Check that evictions occurred
	stats := c.GetStats()
	if stats.Evictions == 0 {
		t.Error("expected evictions to occur")
	}

	// Cache size should be under max
	if stats.CurrentSize > 100 {
		t.Errorf("cache size %d exceeds max 100", stats.CurrentSize)
	}
}

func TestCache_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	c, err := New(tmpDir, 10*1024*1024, true, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Add some entries
	for i := 0; i < 3; i++ {
		entry := &Entry{
			StatusCode: 200,
			Body:       []byte("test"),
			Size:       4,
		}
		if err := c.Put(string(rune('a'+i)), entry); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
	}

	// Clear cache
	if err := c.Clear(); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Check stats reset
	stats := c.GetStats()
	if stats.CurrentSize != 0 {
		t.Errorf("expected size 0 after clear, got %d", stats.CurrentSize)
	}

	// Verify cache directory is empty
	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty cache directory, got %d entries", len(entries))
	}
}

func TestCache_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	c, err := New(tmpDir, 10*1024*1024, false, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Operations should return ErrCacheDisabled
	entry := &Entry{
		StatusCode: 200,
		Body:       []byte("test"),
		Size:       4,
	}

	if err := c.Put("key", entry); err != ErrCacheDisabled {
		t.Errorf("expected ErrCacheDisabled, got %v", err)
	}

	if _, err := c.Get("key"); err != ErrCacheDisabled {
		t.Errorf("expected ErrCacheDisabled, got %v", err)
	}
}

func TestCache_Persistence(t *testing.T) {
	tmpDir := t.TempDir()
	logger := zap.NewNop()

	// Create cache and add entry
	c1, err := New(tmpDir, 10*1024*1024, true, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	entry := &Entry{
		StatusCode: 200,
		Headers:    http.Header{"X-Test": []string{"value"}},
		Body:       []byte("persistent data"),
		Size:       15,
	}

	if err := c1.Put("persist-key", entry); err != nil {
		t.Fatalf("Put failed: %v", err)
	}

	// Create new cache instance with same directory
	c2, err := New(tmpDir, 10*1024*1024, true, logger)
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// Verify entry still exists
	retrieved, err := c2.Get("persist-key")
	if err != nil {
		t.Fatalf("Get failed after reload: %v", err)
	}

	if string(retrieved.Body) != string(entry.Body) {
		t.Errorf("body mismatch after reload: got %s, want %s", retrieved.Body, entry.Body)
	}
}

func TestCacheKey(t *testing.T) {
	key1 := cacheKey("/v2/library/ubuntu/blobs/sha256:abc123")
	key2 := cacheKey("/v2/library/ubuntu/blobs/sha256:abc123")
	key3 := cacheKey("/v2/library/ubuntu/blobs/sha256:def456")

	// Same path should generate same key
	if key1 != key2 {
		t.Error("same path generated different keys")
	}

	// Different path should generate different key
	if key1 == key3 {
		t.Error("different paths generated same key")
	}

	// Key should be filesystem-safe (hex)
	if len(key1) != 64 {
		t.Errorf("expected 64-char hex key, got %d chars", len(key1))
	}
}

func TestSerializeDeserialize(t *testing.T) {
	entry := &Entry{
		StatusCode: 404,
		Headers: http.Header{
			"Content-Type":   []string{"application/json"},
			"X-Custom":       []string{"value1", "value2"},
			"Content-Length": []string{"123"},
		},
		Body: []byte("test body content"),
		Size: 17,
	}

	// Serialize
	data, err := serializeEntry(entry)
	if err != nil {
		t.Fatalf("serializeEntry failed: %v", err)
	}

	// Deserialize
	retrieved, err := deserializeEntry(data)
	if err != nil {
		t.Fatalf("deserializeEntry failed: %v", err)
	}

	// Verify fields
	if retrieved.StatusCode != entry.StatusCode {
		t.Errorf("status code mismatch: got %d, want %d", retrieved.StatusCode, entry.StatusCode)
	}

	if string(retrieved.Body) != string(entry.Body) {
		t.Errorf("body mismatch: got %s, want %s", retrieved.Body, entry.Body)
	}

	// Verify all headers
	for key, values := range entry.Headers {
		retrievedValues := retrieved.Headers[key]
		if len(retrievedValues) != len(values) {
			t.Errorf("header %s: value count mismatch", key)
			continue
		}
		for i, v := range values {
			if retrievedValues[i] != v {
				t.Errorf("header %s[%d]: got %s, want %s", key, i, retrievedValues[i], v)
			}
		}
	}
}

func TestCaptureAndRestoreResponse(t *testing.T) {
	// Create a test directory for cache files
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Create a file response
	file, err := os.Open(testFile)
	if err != nil {
		t.Fatalf("failed to open test file: %v", err)
	}
	defer file.Close()

	resp := &http.Response{
		StatusCode: 200,
		Header: http.Header{
			"Content-Type": []string{"text/plain"},
		},
		Body: file,
	}

	// Capture response
	entry, err := CaptureResponse(resp)
	if err != nil {
		t.Fatalf("CaptureResponse failed: %v", err)
	}

	if entry.StatusCode != 200 {
		t.Errorf("status code mismatch: got %d, want 200", entry.StatusCode)
	}

	if string(entry.Body) != "test content" {
		t.Errorf("body mismatch: got %s, want 'test content'", entry.Body)
	}

	// Restore response
	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	restored := RestoreResponse(entry, req)

	if restored.StatusCode != entry.StatusCode {
		t.Errorf("restored status code mismatch: got %d, want %d", restored.StatusCode, entry.StatusCode)
	}

	if restored.Header.Get("X-Cache") != "HIT" {
		t.Error("expected X-Cache: HIT header")
	}
}
