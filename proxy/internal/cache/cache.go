// Package cache provides HTTP response caching with LRU eviction.
package cache

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
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
	bodyReader io.ReadCloser
	storedSize int64
}

// StreamingPut incrementally persists a response body while it is being
// streamed to the client.
type StreamingPut struct {
	cache      *Cache
	key        string
	hash       string
	path       string
	metaPath   string
	tempPath   string
	file       *os.File
	digest     hash.Hash
	bodySize   int64
	storedSize int64
	finalized  bool
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

	// Write to disk
	storedSize, err := c.writeEntry(key, entry)
	if err != nil {
		c.mu.Lock()
		c.stats.Errors++
		c.mu.Unlock()
		return err
	}

	c.recordStore(key, storedSize)

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

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrCacheMiss
		}
		return nil, fmt.Errorf("read cache file: %w", err)
	}

	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat cache file: %w", err)
	}

	entry, err := deserializeEntryFromFile(file, info.Size())
	if err != nil {
		_ = file.Close()
		// Corrupt cache file, remove it
		_ = os.Remove(path)
		c.index.remove(key)
		return nil, ErrCacheMiss
	}

	return entry, nil
}

// writeEntry writes a cache entry to disk.
func (c *Cache) writeEntry(key string, entry *Entry) (int64, error) {
	hash := cacheKey(key)
	path := filepath.Join(c.dir, hash)

	data, err := serializeEntry(entry)
	if err != nil {
		return 0, fmt.Errorf("serialize entry: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return 0, fmt.Errorf("write cache file: %w", err)
	}

	// Write metadata file with original key
	metaPath := filepath.Join(c.dir, hash+".meta")
	if err := os.WriteFile(metaPath, []byte(key), 0644); err != nil {
		return 0, fmt.Errorf("write meta file: %w", err)
	}

	return int64(len(data)), nil
}

// BeginStreamingPut creates a cache writer that can persist a response body as
// it streams through the proxy.
func (c *Cache) BeginStreamingPut(key string, resp *http.Response) (*StreamingPut, error) {
	if !c.enabled {
		return nil, ErrCacheDisabled
	}

	cacheHash := cacheKey(key)
	headerData, err := serializeEntryHeader(&Entry{
		StatusCode: resp.StatusCode,
		Headers:    resp.Header.Clone(),
		CachedAt:   time.Now(),
	})
	if err != nil {
		return nil, fmt.Errorf("serialize entry header: %w", err)
	}

	file, err := os.CreateTemp(c.dir, cacheHash+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temp cache file: %w", err)
	}

	if _, err := file.Write(headerData); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, fmt.Errorf("write cache header: %w", err)
	}

	return &StreamingPut{
		cache:      c,
		key:        key,
		hash:       cacheHash,
		path:       filepath.Join(c.dir, cacheHash),
		metaPath:   filepath.Join(c.dir, cacheHash+".meta"),
		tempPath:   file.Name(),
		file:       file,
		digest:     sha256.New(),
		storedSize: int64(len(headerData)),
	}, nil
}

// Write appends body bytes to the cache file while updating the digest.
func (s *StreamingPut) Write(p []byte) (int, error) {
	if s == nil || s.finalized || s.file == nil {
		return 0, os.ErrClosed
	}

	n, err := s.file.Write(p)
	if n > 0 {
		_, _ = s.digest.Write(p[:n])
		s.bodySize += int64(n)
		s.storedSize += int64(n)
	}

	if err != nil {
		return n, fmt.Errorf("write cache body: %w", err)
	}

	return n, nil
}

// Size returns the streamed response body size.
func (s *StreamingPut) Size() int64 {
	if s == nil {
		return 0
	}
	return s.bodySize
}

// DigestHex returns the sha256 digest of the streamed body.
func (s *StreamingPut) DigestHex() string {
	if s == nil {
		return ""
	}
	return hex.EncodeToString(s.digest.Sum(nil))
}

// Commit atomically moves the streamed response into the cache and updates the
// in-memory index.
func (s *StreamingPut) Commit() error {
	if s == nil || s.finalized {
		return nil
	}
	s.finalized = true

	if s.file != nil {
		if err := s.file.Close(); err != nil {
			_ = os.Remove(s.tempPath)
			return fmt.Errorf("close temp cache file: %w", err)
		}
		s.file = nil
	}

	if err := os.Rename(s.tempPath, s.path); err != nil {
		_ = os.Remove(s.tempPath)
		return fmt.Errorf("rename cache file: %w", err)
	}

	metaFile, err := os.CreateTemp(s.cache.dir, s.hash+".meta.tmp-*")
	if err != nil {
		_ = os.Remove(s.path)
		return fmt.Errorf("create temp meta file: %w", err)
	}

	if _, err := metaFile.WriteString(s.key); err != nil {
		_ = metaFile.Close()
		_ = os.Remove(metaFile.Name())
		_ = os.Remove(s.path)
		return fmt.Errorf("write temp meta file: %w", err)
	}
	if err := metaFile.Close(); err != nil {
		_ = os.Remove(metaFile.Name())
		_ = os.Remove(s.path)
		return fmt.Errorf("close temp meta file: %w", err)
	}
	if err := os.Rename(metaFile.Name(), s.metaPath); err != nil {
		_ = os.Remove(metaFile.Name())
		_ = os.Remove(s.path)
		return fmt.Errorf("rename meta file: %w", err)
	}

	s.cache.recordStore(s.key, s.storedSize)
	return nil
}

// Abort discards any partially written cache file.
func (s *StreamingPut) Abort() error {
	if s == nil || s.finalized {
		return nil
	}
	s.finalized = true

	if s.file != nil {
		_ = s.file.Close()
		s.file = nil
	}
	if err := os.Remove(s.tempPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove temp cache file: %w", err)
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

func (c *Cache) recordStore(key string, storedSize int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	previousSize := int64(0)
	if item, exists := c.index.items[key]; exists {
		previousSize = item.size
	}

	c.index.add(key, storedSize)
	c.stats.CurrentSize += storedSize - previousSize
	c.stats.Stores++

	for c.stats.CurrentSize > c.maxSize {
		if err := c.evictLRU(); err != nil {
			c.logger.Warn("eviction failed", zap.Error(err))
			break
		}
	}
}

// serializeEntry converts an Entry to bytes.
func serializeEntry(entry *Entry) ([]byte, error) {
	headerData, err := serializeEntryHeader(entry)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := buf.Write(headerData); err != nil {
		return nil, err
	}

	// Write body
	if _, err := buf.Write(entry.Body); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func serializeEntryHeader(entry *Entry) ([]byte, error) {
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

	return buf.Bytes(), nil
}

// deserializeEntry converts bytes to an Entry.
func deserializeEntry(data []byte) (*Entry, error) {
	statusCode, cachedAt, headers, bodyOffset, err := deserializeEntryHeader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	entry := &Entry{
		StatusCode: statusCode,
		Headers:    headers,
		CachedAt:   cachedAt,
		Body:       data[bodyOffset:],
		Size:       int64(len(data)) - int64(bodyOffset),
		storedSize: int64(len(data)),
	}

	return entry, nil
}

func deserializeEntryFromFile(file *os.File, totalSize int64) (*Entry, error) {
	statusCode, cachedAt, headers, bodyOffset, err := deserializeEntryHeader(file, totalSize)
	if err != nil {
		return nil, err
	}

	return &Entry{
		StatusCode: statusCode,
		Headers:    headers,
		CachedAt:   cachedAt,
		Size:       totalSize - int64(bodyOffset),
		bodyReader: file,
		storedSize: totalSize,
	}, nil
}

func deserializeEntryHeader(r io.Reader, totalSize int64) (statusCode int, cachedAt time.Time, headers http.Header, bodyOffset int, err error) {
	if totalSize < 16 {
		return 0, time.Time{}, nil, 0, errors.New("invalid entry data")
	}

	fixed := make([]byte, 16)
	if _, err := io.ReadFull(r, fixed); err != nil {
		return 0, time.Time{}, nil, 0, err
	}

	statusCode = int(fixed[0])<<24 | int(fixed[1])<<16 | int(fixed[2])<<8 | int(fixed[3])
	timestamp := int64(fixed[4])<<56 | int64(fixed[5])<<48 | int64(fixed[6])<<40 | int64(fixed[7])<<32 |
		int64(fixed[8])<<24 | int64(fixed[9])<<16 | int64(fixed[10])<<8 | int64(fixed[11])
	cachedAt = time.Unix(timestamp, 0)

	headerLen := int(fixed[12])<<24 | int(fixed[13])<<16 | int(fixed[14])<<8 | int(fixed[15])
	if headerLen < 0 || totalSize < int64(16+headerLen) {
		return 0, time.Time{}, nil, 0, errors.New("invalid header length")
	}

	headersData := make([]byte, headerLen)
	if _, err := io.ReadFull(r, headersData); err != nil {
		return 0, time.Time{}, nil, 0, err
	}

	return statusCode, cachedAt, deserializeHeaders(headersData), 16 + headerLen, nil
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
	var body io.ReadCloser = http.NoBody
	if entry.bodyReader != nil {
		body = entry.bodyReader
	} else if len(entry.Body) > 0 {
		body = io.NopCloser(bytes.NewReader(entry.Body))
	}

	resp := &http.Response{
		StatusCode:    entry.StatusCode,
		Header:        entry.Headers.Clone(),
		Body:          body,
		ContentLength: entry.Size,
		Request:       req,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
	}

	// Add cache header
	resp.Header.Set("X-Cache", "HIT")
	resp.Header.Set("X-Cache-Date", entry.CachedAt.Format(time.RFC3339))

	return resp
}
