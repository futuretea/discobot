// Package recorder records full HTTP request/response information to disk.
// Each entry is written as a JSON line to a daily-rotated file in the
// configured directory, enabling later inspection of agent network behavior.
package recorder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Config holds recording configuration.
type Config struct {
	Enabled bool   `yaml:"enabled"       json:"enabled"`
	Dir     string `yaml:"dir"           json:"dir"`
	// MaxBodySize is the maximum number of bytes to capture per request or
	// response body. 0 disables body capture. -1 captures without a size limit.
	MaxBodySize int64 `yaml:"max_body_size" json:"max_body_size"`
}

// Entry is a single recorded HTTP exchange.
type Entry struct {
	ID        string        `json:"id"`
	Timestamp time.Time     `json:"timestamp"`
	Request   RequestInfo   `json:"request"`
	Response  *ResponseInfo `json:"response,omitempty"`
	CacheHit  bool          `json:"cache_hit,omitempty"`
	Blocked   bool          `json:"blocked,omitempty"`
}

// RequestInfo holds request metadata.
type RequestInfo struct {
	Method        string      `json:"method"`
	URL           string      `json:"url"`
	Proto         string      `json:"proto"`
	Headers       http.Header `json:"headers"`
	BodySize      int64       `json:"body_size"`
	Body          []byte      `json:"body,omitempty"`
	BodyTruncated bool        `json:"body_truncated,omitempty"`
	RemoteAddr    string      `json:"remote_addr,omitempty"`
}

// ResponseInfo holds response metadata.
type ResponseInfo struct {
	Status        int         `json:"status"`
	StatusText    string      `json:"status_text"`
	Headers       http.Header `json:"headers"`
	BodySize      int64       `json:"body_size"`
	Body          []byte      `json:"body,omitempty"`
	BodyTruncated bool        `json:"body_truncated,omitempty"`
	DurationMs    int64       `json:"duration_ms"`
}

var idCounter atomic.Uint64

func generateID() string {
	return fmt.Sprintf("%d-%d", time.Now().UnixNano(), idCounter.Add(1))
}

// Recorder writes HTTP exchange entries to daily-rotated JSONL files.
type Recorder struct {
	cfg     Config
	mu      sync.Mutex
	file    *os.File
	fileDay string // "YYYY-MM-DD" of the currently open file
}

// ResponseCapture incrementally captures a streamed response body for later
// recording without blocking delivery to the client.
type ResponseCapture struct {
	entry     *Entry
	maxSize   int64
	buf       bytes.Buffer
	truncated bool
	discard   bool
	finalized bool
}

// New creates a new Recorder. If cfg.Enabled is false the recorder is a no-op.
func New(cfg Config) (*Recorder, error) {
	if !cfg.Enabled {
		return &Recorder{cfg: cfg}, nil
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("create recording dir: %w", err)
	}
	return &Recorder{cfg: cfg}, nil
}

// NewEntry builds a new Entry from an incoming request.
// The caller must later call SetResponse (and Record) to flush it.
func NewEntry(req *http.Request) *Entry {
	return &Entry{
		ID:        generateID(),
		Timestamp: time.Now().UTC(),
		Request: RequestInfo{
			Method:     req.Method,
			URL:        buildURL(req),
			Proto:      req.Proto,
			Headers:    sanitizeHeaders(req.Header),
			BodySize:   req.ContentLength,
			RemoteAddr: req.RemoteAddr,
		},
	}
}

// SetResponse populates the response fields of an entry.
func SetResponse(entry *Entry, resp *http.Response, duration time.Duration) {
	entry.Response = &ResponseInfo{
		Status:     resp.StatusCode,
		StatusText: http.StatusText(resp.StatusCode),
		Headers:    sanitizeHeaders(resp.Header),
		BodySize:   resp.ContentLength,
		DurationMs: duration.Milliseconds(),
	}
}

// CaptureRequestBody reads up to MaxBodySize bytes from req.Body, stores them
// in entry.Request.Body, and replaces req.Body with a reader that replays
// those bytes followed by the remainder of the original stream.
// This must be called before the request is forwarded.
func (r *Recorder) CaptureRequestBody(entry *Entry, req *http.Request) {
	if !r.cfg.Enabled || r.cfg.MaxBodySize == 0 || req.Body == nil {
		return
	}
	if isBinaryContentType(req.Header.Get("Content-Type")) {
		return
	}
	captured, restored, truncated := captureStream(req.Body, r.cfg.MaxBodySize)
	req.Body = restored
	if bytes.IndexByte(captured, 0) >= 0 {
		// Null bytes detected — treat as binary and discard the capture.
		return
	}
	entry.Request.Body = captured
	entry.Request.BodyTruncated = truncated
}

// CaptureResponseBody reads up to MaxBodySize bytes from resp.Body, stores
// them in entry.Response.Body, and replaces resp.Body with a reader that
// replays those bytes followed by the remainder of the original stream.
// entry.Response must be populated (via SetResponse) before calling this.
// This must be called before the response is forwarded to the client.
func (r *Recorder) CaptureResponseBody(entry *Entry, resp *http.Response) {
	if !r.cfg.Enabled || r.cfg.MaxBodySize == 0 || entry.Response == nil || resp.Body == nil {
		return
	}
	if isBinaryContentType(resp.Header.Get("Content-Type")) {
		return
	}
	captured, restored, truncated := captureStream(resp.Body, r.cfg.MaxBodySize)
	resp.Body = restored
	if bytes.IndexByte(captured, 0) >= 0 {
		// Null bytes detected — treat as binary and discard the capture.
		return
	}
	entry.Response.Body = captured
	entry.Response.BodyTruncated = truncated
}

// BeginResponseCapture prepares a streaming response capture for resp.
func (r *Recorder) BeginResponseCapture(entry *Entry, resp *http.Response) *ResponseCapture {
	if !r.cfg.Enabled || r.cfg.MaxBodySize == 0 || entry == nil || entry.Response == nil || resp == nil || resp.Body == nil {
		return nil
	}
	if isBinaryContentType(resp.Header.Get("Content-Type")) {
		return nil
	}
	return &ResponseCapture{entry: entry, maxSize: r.cfg.MaxBodySize}
}

// Write adds streamed response bytes to the capture buffer.
func (c *ResponseCapture) Write(p []byte) {
	if c == nil || c.discard || c.finalized || len(p) == 0 {
		return
	}
	if bytes.IndexByte(p, 0) >= 0 {
		c.discard = true
		c.buf.Reset()
		return
	}
	if c.maxSize < 0 {
		_, _ = c.buf.Write(p)
		return
	}

	remaining := int(c.maxSize - int64(c.buf.Len()))
	if remaining <= 0 {
		c.truncated = true
		return
	}
	if len(p) > remaining {
		_, _ = c.buf.Write(p[:remaining])
		c.truncated = true
		return
	}
	_, _ = c.buf.Write(p)
}

// Finish copies the captured body onto the recorder entry.
func (c *ResponseCapture) Finish() {
	if c == nil || c.finalized {
		return
	}
	c.finalized = true
	if c.discard || c.entry == nil || c.entry.Response == nil {
		return
	}
	c.entry.Response.Body = bytes.Clone(c.buf.Bytes())
	c.entry.Response.BodyTruncated = c.truncated
}

// captureStream reads up to maxSize bytes from rc.
// It returns the captured bytes, a restored ReadCloser that replays those
// bytes followed by the rest of rc, and whether the capture was truncated.
// If maxSize < 0 the entire stream is captured with no limit.
func captureStream(rc io.ReadCloser, maxSize int64) (captured []byte, restored io.ReadCloser, truncated bool) {
	if maxSize < 0 {
		// Unlimited — read everything and replace with a bytes.Reader.
		data, _ := io.ReadAll(rc)
		return data, io.NopCloser(bytes.NewReader(data)), false
	}

	// Read one extra byte so we can detect truncation without consuming
	// more than necessary from the underlying stream.
	data, err := io.ReadAll(io.LimitReader(rc, maxSize+1))
	if err != nil || int64(len(data)) <= maxSize {
		// Fully captured (or read error) — restore the stream transparently.
		return data, io.NopCloser(io.MultiReader(bytes.NewReader(data), rc)), false
	}

	// data has maxSize+1 bytes, so the body is larger than maxSize.
	// Capture only maxSize bytes for the log, but restore all of data into
	// the stream so the full body still flows through to the client/cache.
	captured = data[:maxSize]
	restored = io.NopCloser(io.MultiReader(bytes.NewReader(data), rc))
	return captured, restored, true
}

// Record writes entry to the current day's JSONL file.
// Errors are silently swallowed — recording must never break the proxy.
func (r *Recorder) Record(entry *Entry) {
	if !r.cfg.Enabled {
		return
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	f, err := r.currentFile(time.Now())
	if err != nil {
		return
	}
	_, _ = f.Write(append(data, '\n'))
}

// Close flushes and closes the current log file.
func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		err := r.file.Close()
		r.file = nil
		return err
	}
	return nil
}

// currentFile returns the open file for today, rotating if the day has changed.
// Must be called with r.mu held.
func (r *Recorder) currentFile(now time.Time) (*os.File, error) {
	day := now.UTC().Format("2006-01-02")
	if r.file != nil && r.fileDay == day {
		return r.file, nil
	}
	if r.file != nil {
		_ = r.file.Close()
		r.file = nil
	}
	path := filepath.Join(r.cfg.Dir, fmt.Sprintf("requests-%s.jsonl", day))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	r.file = f
	r.fileDay = day
	return f, nil
}

// binaryMIMEPrefixes are Content-Type prefixes that indicate binary content
// that should not be captured in logs.
var binaryMIMEPrefixes = []string{
	"image/",
	"video/",
	"audio/",
	"font/",
	"application/octet-stream",
	"application/zip",
	"application/gzip",
	"application/x-gzip",
	"application/x-tar",
	"application/x-bz2",
	"application/x-xz",
	"application/zstd",
	"application/x-zstd",
	"application/pdf",
	"application/wasm",
	"application/vnd.docker.",
	"application/vnd.oci.",
}

// isBinaryContentType reports whether a Content-Type value indicates binary
// content that should not be logged to disk.
func isBinaryContentType(ct string) bool {
	// Strip parameters (e.g. "; charset=utf-8").
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))
	for _, prefix := range binaryMIMEPrefixes {
		if strings.HasPrefix(ct, prefix) {
			return true
		}
	}
	return false
}

// sensitiveHeaderSuffixes are lowercased name suffixes that indicate a header
// likely contains a credential.  We match on both hyphen and underscore
// separators to handle non-standard naming (e.g. X_API_KEY).
var sensitiveHeaderSuffixes = []string{
	"-key", "_key",
	"-token", "_token",
	"-secret", "_secret",
	"-password", "_password",
}

// sensitiveHeaderSubstrings are fragments whose presence anywhere in a
// lowercased header name indicates a credential.
var sensitiveHeaderSubstrings = []string{
	"authorization", // catches x-authorization, custom-authorization, etc.
	"api-key", "api_key", "apikey",
}

// sanitizeHeaders returns a copy of h with the values of any headers that
// look like they might carry credentials replaced with "[REDACTED]".
// The original header map is never modified.
func sanitizeHeaders(h http.Header) http.Header {
	out := h.Clone()
	for name := range out {
		if isSensitiveHeader(name) {
			out[name] = []string{"[REDACTED]"}
		}
	}
	return out
}

// isSensitiveHeader reports whether a header name looks like it may carry a
// credential.  The check is intentionally broad: it is better to redact a
// benign header than to accidentally log a secret.
func isSensitiveHeader(name string) bool {
	lower := strings.ToLower(name)

	// Exact well-known sensitive headers.
	switch lower {
	case "authorization", "proxy-authorization", "cookie", "set-cookie":
		return true
	}

	// Suffix-based check: X-API-Key, X-Auth-Token, X-Client-Secret, …
	for _, suffix := range sensitiveHeaderSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}

	// Substring-based check: anything containing "authorization", "api-key", …
	for _, sub := range sensitiveHeaderSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}

	return false
}

// buildURL reconstructs a full URL from a goproxy request.
// For plain HTTP requests, req.URL already contains the full URL.
// For HTTPS MITM requests, req.URL only contains the path and the host
// is in req.Host; scheme is inferred from whether TLS was active.
func buildURL(req *http.Request) string {
	scheme := req.URL.Scheme
	if scheme == "" {
		if req.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := req.URL.Host
	if host == "" {
		host = req.Host
	}
	return scheme + "://" + host + req.URL.RequestURI()
}
