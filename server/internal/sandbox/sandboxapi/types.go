// Package sandboxapi defines the request/response types for the sandbox HTTP API.
//
// These types must be kept in sync with the TypeScript agent's API types
// located at: agent/src/api/types.ts
//
// API Endpoints:
//
//	GET  /        - Health check
//	GET  /health  - Detailed health status
//	GET  /chat    - Get all messages
//	POST /chat    - Send messages and stream response (SSE)
//	DELETE /chat  - Clear session and messages
package sandboxapi

import "encoding/json"

// ============================================================================
// Request Types
// ============================================================================

// ChatRequest is the POST /chat request body.
type ChatRequest struct {
	// Messages is the array of UIMessages to send.
	// Kept as raw JSON to pass through without requiring Go to understand
	// the full UIMessage structure from the AI SDK.
	Messages json.RawMessage `json:"messages"`
}

// ============================================================================
// Response Types
// ============================================================================

// RootResponse is the GET / response.
type RootResponse struct {
	Status  string `json:"status"`  // Always "ok"
	Service string `json:"service"` // Always "agent"
}

// HealthResponse is the GET /health response.
type HealthResponse struct {
	Healthy   bool `json:"healthy"`
	Connected bool `json:"connected"`
}

// GetMessagesResponse is the GET /chat response.
type GetMessagesResponse struct {
	Messages []UIMessage `json:"messages"`
}

// ClearSessionResponse is the DELETE /chat response.
type ClearSessionResponse struct {
	Success bool `json:"success"`
}

// ErrorResponse is returned for 4xx/5xx errors.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ============================================================================
// Shared Types
// ============================================================================

// UIMessage represents a message in AI SDK UIMessage format.
// This is a minimal representation - the full structure is passed through
// as raw JSON where possible to avoid tight coupling with AI SDK internals.
type UIMessage struct {
	ID        string          `json:"id"`
	Role      string          `json:"role"` // "user", "assistant", "system"
	Parts     json.RawMessage `json:"parts"`
	CreatedAt string          `json:"createdAt,omitempty"`
}

// ============================================================================
// File System Types
// ============================================================================

// FileEntry represents a single file or directory entry.
type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "directory"
	Size int64  `json:"size,omitempty"`
}

// ListFilesResponse is the GET /files response.
type ListFilesResponse struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// ReadFileResponse is the GET /files/read response.
type ReadFileResponse struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"` // "utf8" or "base64"
	Size     int64  `json:"size"`
}

// WriteFileRequest is the POST /files/write request body.
type WriteFileRequest struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding,omitempty"` // defaults to "utf8"
}

// WriteFileResponse is the POST /files/write response.
type WriteFileResponse struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// FileDiffEntry represents a single changed file in the diff.
type FileDiffEntry struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // "added", "modified", "deleted", "renamed"
	OldPath   string `json:"oldPath,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
	Patch     string `json:"patch,omitempty"`
}

// DiffStats contains summary statistics for a diff.
type DiffStats struct {
	FilesChanged int `json:"filesChanged"`
	Additions    int `json:"additions"`
	Deletions    int `json:"deletions"`
}

// DiffResponse is the GET /diff response (full diff with patches).
type DiffResponse struct {
	Files []FileDiffEntry `json:"files"`
	Stats DiffStats       `json:"stats"`
}

// DiffFilesResponse is the GET /diff?format=files response (just file paths).
type DiffFilesResponse struct {
	Files []string  `json:"files"`
	Stats DiffStats `json:"stats"`
}

// SingleFileDiffResponse is the GET /diff?path=... response.
type SingleFileDiffResponse struct {
	Path      string `json:"path"`
	Status    string `json:"status"` // "added", "modified", "deleted", "renamed", "unchanged"
	OldPath   string `json:"oldPath,omitempty"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Binary    bool   `json:"binary"`
	Patch     string `json:"patch"`
}
