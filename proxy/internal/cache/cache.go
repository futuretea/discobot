// Package cache provides HTTP response caching with LRU eviction.
package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

var (
	// ErrCacheMiss indicates the requested item is not in cache.
	ErrCacheMiss = errors.New("cache miss")
	// ErrCacheDisabled indicates the cache is not enabled.
	ErrCacheDisabled = errors.New("cache disabled")
)

// Cache provides content caching with LRU eviction.
type Cache struct {
	dir     string
	maxSize int64
	enabled bool
	logger  *zap.Logger

	mu    sync.RWMutex
	index *lruIndex // LRU index for eviction
	stats Stats
}

// Stats tracks cache statistics.
type Stats struct {
	Hits        int64
	Misses      int64
	Stores      int64
	Evictions   int64
	Errors      int64
	CurrentSize int64
}

// Entry represents a cached HTTP response.
type Entry struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
	CachedAt   time.Time
	Size       int64
}

// New creates a new cache instance.
func New(dir string, maxSize int64, enabled bool, logger *zap.Logger) (*Cache, error) {
	if !enabled {
		return &Cache{enabled: false, logger: logger}, nil
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create cache directory: %w", err)
	}

	c := &Cache{
		dir:     dir,
		maxSize: maxSize,
		enabled: enabled,
		logger:  logger,
		index:   newLRUIndex(),
	}

	// Initialize index from existing files
	if err := c.loadIndex(); err != nil {
		logger.Warn("failed to load cache index", zap.Error(err))
	}

	return c, nil
}

// Get retrieves a cached response.
func (c *Cache) Get(key string) (*Entry, error) {
	if !c.enabled {
		return nil, ErrCacheDisabled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key exists in index
	if !c.index.exists(key) {
		c.stats.Misses++
		return nil, ErrCacheMiss
	}

	// Read from disk
	entry, err := c.readEntry(key)
	if err != nil {
		c.stats.Errors++
		c.logger.Debug("cache read error", zap.String("key", key), zap.Error(err))
		return nil, err
	}

	// Update LRU
	c.index.access(key)
	c.stats.Hits++

	return entry, nil
}

// Put stores a response in the cache.
func (c *Cache) Put(key string, entry *Entry) error {
	if !c.enabled {
		return ErrCacheDisabled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// Write to disk
	if err := c.writeEntry(key, entry); err != nil {
		c.stats.Errors++
		return err
	}

	// Add to index
	c.index.add(key, entry.Size)
	c.stats.CurrentSize += entry.Size
	c.stats.Stores++

	// Evict if over size limit
	for c.stats.CurrentSize > c.maxSize {
		if err := c.evictLRU(); err != nil {
			c.logger.Warn("eviction failed", zap.Error(err))
			break
		}
	}

	return nil
}

// GetStats returns current cache statistics.
func (c *Cache) GetStats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.stats
}

// Clear removes all cached entries.
func (c *Cache) Clear() error {
	if !c.enabled {
		return ErrCacheDisabled
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := os.RemoveAll(c.dir); err != nil {
		return err
	}
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return err
	}

	c.index = newLRUIndex()
	c.stats = Stats{}

	return nil
}

// cacheKey generates a filesystem-safe cache key from a URL path.
func cacheKey(path string) string {
	hash := sha256.Sum256([]byte(path))
	return hex.EncodeToString(hash[:])
}

// readEntry reads a cache entry from disk.
func (c *Cache) readEntry(key string) (*Entry, error) {
	path := filepath.Join(c.dir, cacheKey(key))

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("read cache file: %w", err)
	}

	entry, err := deserializeEntry(data)
	if err != nil {
		// Corrupt cache file, remove it
		_ = os.Remove(path)
		c.index.remove(key)
		return nil, ErrCacheMiss
	}

	return entry, nil
}

// writeEntry writes a cache entry to disk.
func (c *Cache) writeEntry(key string, entry *Entry) error {
	hash := cacheKey(key)
	path := filepath.Join(c.dir, hash)

	data, err := serializeEntry(entry)
	if err != nil {
		return fmt.Errorf("serialize entry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write cache file: %w", err)
	}

	// Write metadata file with original key
	metaPath := filepath.Join(c.dir, hash+".meta")
	if err := os.WriteFile(metaPath, []byte(key), 0644); err != nil {
		return fmt.Errorf("write meta file: %w", err)
	}

	return nil
}

// evictLRU evicts the least recently used entry.
func (c *Cache) evictLRU() error {
	key, size := c.index.evict()
	if key == "" {
		return errors.New("no entries to evict")
	}

	hash := cacheKey(key)
	path := filepath.Join(c.dir, hash)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove cache file: %w", err)
	}

	// Remove metadata file
	metaPath := filepath.Join(c.dir, hash+".meta")
	_ = os.Remove(metaPath) // Ignore error if meta file doesn't exist

	c.stats.CurrentSize -= size
	c.stats.Evictions++

	return nil
}

// loadIndex rebuilds the LRU index from existing cache files.
func (c *Cache) loadIndex() error {
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Skip metadata files
		if filepath.Ext(entry.Name()) == ".meta" {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Read the original key from metadata file
		metaPath := filepath.Join(c.dir, entry.Name()+".meta")
		keyData, err := os.ReadFile(metaPath)
		if err != nil {
			// If no metadata file, skip this entry (orphaned cache file)
			continue
		}

		key := string(keyData)
		c.index.add(key, info.Size())
		c.stats.CurrentSize += info.Size()
	}

	return nil
}

// serializeEntry converts an Entry to bytes.
func serializeEntry(entry *Entry) ([]byte, error) {
	var buf bytes.Buffer

	// Write status code (4 bytes)
	statusBytes := []byte{
		byte(entry.StatusCode >> 24),
		byte(entry.StatusCode >> 16),
		byte(entry.StatusCode >> 8),
		byte(entry.StatusCode),
	}
	if _, err := buf.Write(statusBytes); err != nil {
		return nil, err
	}

	// Write timestamp (8 bytes)
	timestamp := entry.CachedAt.Unix()
	timeBytes := []byte{
		byte(timestamp >> 56),
		byte(timestamp >> 48),
		byte(timestamp >> 40),
		byte(timestamp >> 32),
		byte(timestamp >> 24),
		byte(timestamp >> 16),
		byte(timestamp >> 8),
		byte(timestamp),
	}
	if _, err := buf.Write(timeBytes); err != nil {
		return nil, err
	}

	// Write headers (length-prefixed)
	headersData := serializeHeaders(entry.Headers)
	headerLen := len(headersData)
	headerLenBytes := []byte{
		byte(headerLen >> 24),
		byte(headerLen >> 16),
		byte(headerLen >> 8),
		byte(headerLen),
	}
	if _, err := buf.Write(headerLenBytes); err != nil {
		return nil, err
	}
	if _, err := buf.Write(headersData); err != nil {
		return nil, err
	}

	// Write body
	if _, err := buf.Write(entry.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// deserializeEntry converts bytes to an Entry.
func deserializeEntry(data []byte) (*Entry, error) {
	if len(data) < 16 {
		return nil, errors.New("invalid entry data")
	}

	entry := &Entry{}

	// Read status code
	entry.StatusCode = int(data[0])<<24 | int(data[1])<<16 | int(data[2])<<8 | int(data[3])

	// Read timestamp
	timestamp := int64(data[4])<<56 | int64(data[5])<<48 | int64(data[6])<<40 | int64(data[7])<<32 |
		int64(data[8])<<24 | int64(data[9])<<16 | int64(data[10])<<8 | int64(data[11])
	entry.CachedAt = time.Unix(timestamp, 0)

	// Read headers length
	headerLen := int(data[12])<<24 | int(data[13])<<16 | int(data[14])<<8 | int(data[15])
	if len(data) < 16+headerLen {
		return nil, errors.New("invalid header length")
	}

	// Read headers
	headersData := data[16 : 16+headerLen]
	entry.Headers = deserializeHeaders(headersData)

	// Read body
	entry.Body = data[16+headerLen:]
	entry.Size = int64(len(data))

	return entry, nil
}

// serializeHeaders converts http.Header to bytes.
func serializeHeaders(headers http.Header) []byte {
	var buf bytes.Buffer
	for key, values := range headers {
		for _, value := range values {
			buf.WriteString(key)
			buf.WriteByte(':')
			buf.WriteString(value)
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

// deserializeHeaders converts bytes to http.Header.
func deserializeHeaders(data []byte) http.Header {
	headers := make(http.Header)
	lines := bytes.Split(data, []byte{'\n'})
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		parts := bytes.SplitN(line, []byte{':'}, 2)
		if len(parts) == 2 {
			key := string(parts[0])
			value := string(parts[1])
			headers.Add(key, value)
		}
	}
	return headers
}

// CaptureResponse captures an HTTP response for caching.
func CaptureResponse(resp *http.Response) (*Entry, error) {
	if resp == nil {
		return nil, errors.New("nil response")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	resp.Body.Close()

	// Restore body for downstream use
	resp.Body = io.NopCloser(bytes.NewReader(body))

	entry := &Entry{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		Body:       body,
		CachedAt:   time.Now(),
		Size:       int64(len(body)),
	}

	return entry, nil
}

// RestoreResponse creates an HTTP response from a cache entry.
func RestoreResponse(entry *Entry, req *http.Request) *http.Response {
	resp := &http.Response{
		StatusCode: entry.StatusCode,
		Header:     entry.Headers.Clone(),
		Body:       io.NopCloser(bytes.NewReader(entry.Body)),
		Request:    req,
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
	}

	// Add cache header
	resp.Header.Set("X-Cache", "HIT")
	resp.Header.Set("X-Cache-Date", entry.CachedAt.Format(time.RFC3339))

	return resp
}
