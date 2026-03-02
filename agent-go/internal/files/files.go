// Package files provides file system operations scoped to a workspace root.
// All paths are validated to prevent directory traversal.
package files

import (
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// MaxFileSize is the maximum file size for read operations (10MB).
const MaxFileSize = 10 * 1024 * 1024

// Error represents a file operation error with an HTTP status code.
type Error struct {
	Message string
	Status  int
}

func (e *Error) Error() string {
	return e.Message
}

func newError(status int, message string) *Error {
	return &Error{Message: message, Status: status}
}

// FileEntry represents a single file or directory entry.
type FileEntry struct {
	Name string `json:"name"`
	Type string `json:"type"` // "file" or "directory"
	Size int64  `json:"size,omitempty"`
}

// ListResult is the result of a directory listing.
type ListResult struct {
	Path    string      `json:"path"`
	Entries []FileEntry `json:"entries"`
}

// ReadResult is the result of reading a file.
type ReadResult struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	Encoding string `json:"encoding"` // "utf8" or "base64"
	Size     int64  `json:"size"`
}

// WriteResult is the result of writing a file.
type WriteResult struct {
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// DeleteResult is the result of deleting a file.
type DeleteResult struct {
	Path string `json:"path"`
	Type string `json:"type"` // "file" or "directory"
}

// RenameResult is the result of renaming a file.
type RenameResult struct {
	OldPath string `json:"oldPath"`
	NewPath string `json:"newPath"`
}

// SearchResultEntry is a single result from file search.
type SearchResultEntry struct {
	Path  string  `json:"path"`
	Type  string  `json:"type"` // "file" or "directory"
	Score float64 `json:"score"`
}

// SearchResult is the result of a file search.
type SearchResult struct {
	Query   string              `json:"query"`
	Results []SearchResultEntry `json:"results"`
}

// Known text file extensions.
var textExtensions = map[string]bool{
	// Code
	".ts": true, ".tsx": true, ".js": true, ".jsx": true, ".mjs": true, ".cjs": true,
	".py": true, ".rb": true, ".go": true, ".rs": true, ".java": true, ".kt": true,
	".scala": true, ".c": true, ".cpp": true, ".h": true, ".hpp": true, ".cs": true,
	".swift": true, ".php": true, ".lua": true, ".pl": true, ".sh": true, ".bash": true,
	".zsh": true,
	// Config
	".json": true, ".yaml": true, ".yml": true, ".toml": true, ".xml": true, ".ini": true,
	".env": true, ".gitignore": true, ".editorconfig": true, ".prettierrc": true,
	".eslintrc": true, ".dockerignore": true, ".npmrc": true, ".nvmrc": true,
	// Markup
	".md": true, ".mdx": true, ".html": true, ".htm": true, ".css": true, ".scss": true,
	".less": true, ".svg": true, ".vue": true, ".svelte": true, ".astro": true,
	// Data
	".txt": true, ".csv": true, ".log": true, ".sql": true,
	// Special
	".lock": true, ".sum": true, ".mod": true,
}

// Known binary file extensions.
var binaryExtensions = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".ico": true,
	".bmp": true, ".tiff": true, ".woff": true, ".woff2": true, ".ttf": true, ".otf": true,
	".eot": true, ".pdf": true, ".zip": true, ".tar": true, ".gz": true, ".bz2": true,
	".xz": true, ".7z": true, ".rar": true, ".exe": true, ".dll": true, ".so": true,
	".dylib": true, ".a": true, ".wasm": true, ".node": true, ".mp3": true, ".mp4": true,
	".wav": true, ".ogg": true, ".webm": true, ".avi": true, ".mov": true, ".db": true,
	".sqlite": true, ".sqlite3": true,
}

// Known text file basenames (no extension).
var textBasenames = map[string]bool{
	"Makefile": true, "Dockerfile": true, "Vagrantfile": true,
	"Gemfile": true, "Rakefile": true, "LICENSE": true,
	"README": true, "CHANGELOG": true,
}

// ValidatePath validates and resolves a path relative to the workspace root.
// Returns the resolved absolute path or an error if the path is invalid.
func ValidatePath(inputPath, workspaceRoot string) (string, error) {
	if inputPath == "" || inputPath == "." {
		return workspaceRoot, nil
	}

	// Reject absolute paths
	if filepath.IsAbs(inputPath) {
		return "", fmt.Errorf("absolute paths are not allowed")
	}

	resolved := filepath.Join(workspaceRoot, inputPath)
	resolved = filepath.Clean(resolved)

	// Check for traversal
	rel, err := filepath.Rel(workspaceRoot, resolved)
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("path traversal is not allowed")
	}

	return resolved, nil
}

// IsTextFile determines if a file should be treated as text or binary.
func IsTextFile(path string, content []byte) bool {
	baseName := filepath.Base(path)

	// Check extension-less files by name
	if textBasenames[baseName] {
		return true
	}

	ext := strings.ToLower(filepath.Ext(path))
	if textExtensions[ext] {
		return true
	}
	if binaryExtensions[ext] {
		return false
	}

	// Unknown extension — check content for null bytes
	if len(content) > 0 {
		checkLen := len(content)
		if checkLen > 8192 {
			checkLen = 8192
		}
		for i := 0; i < checkLen; i++ {
			if content[i] == 0 {
				return false
			}
		}
		return true
	}

	// Default to text for unknown without content check
	return true
}

// ListDirectory lists the contents of a directory.
func ListDirectory(inputPath, workspaceRoot string, includeHidden bool) (*ListResult, *Error) {
	resolved, err := ValidatePath(inputPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid path")
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newError(404, "Directory not found")
		}
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}
	if !info.IsDir() {
		return nil, newError(400, "Not a directory")
	}

	dirEntries, err := os.ReadDir(resolved)
	if err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	entries := make([]FileEntry, 0, len(dirEntries))
	for _, de := range dirEntries {
		if !includeHidden && strings.HasPrefix(de.Name(), ".") {
			continue
		}

		entry := FileEntry{
			Name: de.Name(),
			Type: "file",
		}
		if de.IsDir() {
			entry.Type = "directory"
		} else {
			if fi, err := de.Info(); err == nil {
				entry.Size = fi.Size()
			}
		}
		entries = append(entries, entry)
	}

	// Sort: directories first, then alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Type != entries[j].Type {
			return entries[i].Type == "directory"
		}
		return entries[i].Name < entries[j].Name
	})

	relPath, _ := filepath.Rel(workspaceRoot, resolved)
	if relPath == "" || relPath == "." {
		relPath = "."
	}

	return &ListResult{Path: relPath, Entries: entries}, nil
}

// ReadFile reads the content of a file.
func ReadFile(inputPath, workspaceRoot string) (*ReadResult, *Error) {
	resolved, err := ValidatePath(inputPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid path")
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newError(404, "File not found")
		}
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}
	if info.IsDir() {
		return nil, newError(400, "Is a directory")
	}
	if info.Size() > MaxFileSize {
		return nil, newError(413, "File too large")
	}

	content, err := os.ReadFile(resolved)
	if err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	relPath, _ := filepath.Rel(workspaceRoot, resolved)
	isText := IsTextFile(inputPath, content)

	result := &ReadResult{
		Path: relPath,
		Size: info.Size(),
	}
	if isText {
		result.Content = string(content)
		result.Encoding = "utf8"
	} else {
		result.Content = base64.StdEncoding.EncodeToString(content)
		result.Encoding = "base64"
	}

	return result, nil
}

// WriteFile writes content to a file.
func WriteFile(inputPath, content, encoding, workspaceRoot string) (*WriteResult, *Error) {
	resolved, err := ValidatePath(inputPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid path")
	}

	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	var data []byte
	if encoding == "base64" {
		data, err = base64.StdEncoding.DecodeString(content)
		if err != nil {
			return nil, newError(400, "Invalid base64 content")
		}
	} else {
		data = []byte(content)
	}

	if err := os.WriteFile(resolved, data, 0o644); err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	relPath, _ := filepath.Rel(workspaceRoot, resolved)
	return &WriteResult{Path: relPath, Size: int64(len(data))}, nil
}

// DeleteFile deletes a file or directory.
func DeleteFile(inputPath, workspaceRoot string) (*DeleteResult, *Error) {
	resolved, err := ValidatePath(inputPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid path")
	}

	// Prevent deleting the workspace root
	if resolved == workspaceRoot {
		return nil, newError(400, "Cannot delete workspace root")
	}

	info, err := os.Stat(resolved)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, newError(404, "File not found")
		}
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	entryType := "file"
	if info.IsDir() {
		entryType = "directory"
	}

	if err := os.RemoveAll(resolved); err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	relPath, _ := filepath.Rel(workspaceRoot, resolved)
	return &DeleteResult{Path: relPath, Type: entryType}, nil
}

// RenameFile renames (moves) a file or directory.
func RenameFile(oldPath, newPath, workspaceRoot string) (*RenameResult, *Error) {
	resolvedOld, err := ValidatePath(oldPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid source path")
	}

	resolvedNew, err := ValidatePath(newPath, workspaceRoot)
	if err != nil {
		return nil, newError(400, "Invalid destination path")
	}

	// Verify source exists
	if _, err := os.Stat(resolvedOld); err != nil {
		if os.IsNotExist(err) {
			return nil, newError(404, "File not found")
		}
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	// Ensure parent directory of destination exists
	if err := os.MkdirAll(filepath.Dir(resolvedNew), 0o755); err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	if err := os.Rename(resolvedOld, resolvedNew); err != nil {
		if os.IsPermission(err) {
			return nil, newError(403, "Permission denied")
		}
		return nil, newError(500, err.Error())
	}

	relOld, _ := filepath.Rel(workspaceRoot, resolvedOld)
	relNew, _ := filepath.Rel(workspaceRoot, resolvedNew)
	return &RenameResult{OldPath: relOld, NewPath: relNew}, nil
}

// Directories to skip during manual file walk.
var skipDirs = map[string]bool{
	"node_modules": true, ".git": true, ".next": true, ".nuxt": true,
	"dist": true, "build": true, "out": true, "__pycache__": true,
	".cache": true, ".pytest_cache": true, "target": true, ".cargo": true,
	"vendor": true, ".venv": true, "venv": true, "env": true,
	"coverage": true, ".nyc_output": true,
}

// enumerateWithRg uses ripgrep to list files respecting .gitignore.
func enumerateWithRg(cwd string) ([]string, error) {
	cmd := exec.Command("rg", "--files", "--hidden", "--glob", "!.git", "--sort", "path")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			result = append(result, line)
		}
	}
	return result, nil
}

// enumerateWithWalk is a fallback recursive walk when rg is not available.
func enumerateWithWalk(root string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors
		}
		if d.IsDir() {
			name := d.Name()
			if name != "." && strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			if skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		paths = append(paths, rel)
		return nil
	})
	return paths, err
}

// deriveDirs extracts unique directory paths from a list of file paths.
func deriveDirs(filePaths []string) []string {
	dirs := make(map[string]bool)
	for _, p := range filePaths {
		parts := strings.Split(filepath.ToSlash(p), "/")
		for i := 1; i < len(parts); i++ {
			dirs[strings.Join(parts[:i], "/")] = true
		}
	}

	result := make([]string, 0, len(dirs))
	for d := range dirs {
		result = append(result, d)
	}
	sort.Strings(result)
	return result
}

// SearchFiles fuzzy-searches files and directories in the workspace.
func SearchFiles(query, workspaceRoot string, limit int) (*SearchResult, *Error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}

	info, err := os.Stat(workspaceRoot)
	if err != nil || !info.IsDir() {
		return nil, newError(500, "Workspace root not accessible")
	}

	// Enumerate files — try rg first, fall back to manual walk
	filePaths, err := enumerateWithRg(workspaceRoot)
	if err != nil || len(filePaths) == 0 {
		filePaths, _ = enumerateWithWalk(workspaceRoot)
	}

	// Build combined entry list: files + directories derived from file paths
	dirPaths := deriveDirs(filePaths)

	type entry struct {
		path     string
		entType  string
		score    float64
		baseName string
	}

	allEntries := make([]entry, 0, len(filePaths)+len(dirPaths))
	for _, p := range filePaths {
		allEntries = append(allEntries, entry{path: p, entType: "file", baseName: filepath.Base(p)})
	}
	for _, p := range dirPaths {
		allEntries = append(allEntries, entry{path: p, entType: "directory", baseName: filepath.Base(p)})
	}

	if query == "" {
		// No query — return sorted alphabetically up to limit
		sort.Slice(allEntries, func(i, j int) bool {
			return allEntries[i].path < allEntries[j].path
		})
		if len(allEntries) > limit {
			allEntries = allEntries[:limit]
		}
		results := make([]SearchResultEntry, len(allEntries))
		for i, e := range allEntries {
			results[i] = SearchResultEntry{Path: e.path, Type: e.entType, Score: 0}
		}
		return &SearchResult{Query: query, Results: results}, nil
	}

	// Fuzzy matching with fzf-style scoring
	scored := make([]entry, 0, len(allEntries))
	for _, e := range allEntries {
		score, matched := fuzzyScore(query, e.path)
		if !matched {
			continue
		}
		e.score = float64(score)
		scored = append(scored, e)
	}

	// Sort by score descending
	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].path < scored[j].path
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	results := make([]SearchResultEntry, len(scored))
	for i, e := range scored {
		results[i] = SearchResultEntry{Path: e.path, Type: e.entType, Score: e.score}
	}

	return &SearchResult{Query: query, Results: results}, nil
}
