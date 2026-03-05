// Package gitignore implements .gitignore pattern matching for directory walking.
// It handles nested .gitignore files, .git/info/exclude, and global gitignore.
package gitignore

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// pattern represents a single .gitignore pattern rule.
type pattern struct {
	re      *regexp.Regexp
	negated bool
	dirOnly bool
	baseDir string // directory containing the .gitignore that defined this pattern
}

// Matcher accumulates gitignore patterns and checks paths against them.
// It is safe for concurrent use after initial construction.
type Matcher struct {
	mu       sync.RWMutex
	patterns []pattern
	rootDir  string
}

// New creates a Matcher for the given root directory.
// It automatically loads .gitignore, .git/info/exclude, and global gitignore
// if the root is inside a git repository.
func New(rootDir string) *Matcher {
	m := &Matcher{rootDir: filepath.Clean(rootDir)}

	// Find the git repo root (walk up looking for .git)
	gitRoot := findGitRoot(rootDir)
	if gitRoot == "" {
		return m
	}

	// Load global gitignore
	if globalFile := globalGitignorePath(); globalFile != "" {
		m.loadFile(globalFile, gitRoot)
	}

	// Load .git/info/exclude
	excludeFile := filepath.Join(gitRoot, ".git", "info", "exclude")
	m.loadFile(excludeFile, gitRoot)

	// Load .gitignore from git root up to (and including) rootDir
	// We need patterns from ancestor directories too
	rel, err := filepath.Rel(gitRoot, rootDir)
	if err == nil && !strings.HasPrefix(rel, "..") {
		// rootDir is inside gitRoot
		parts := strings.Split(rel, string(filepath.Separator))
		dir := gitRoot
		m.loadFile(filepath.Join(dir, ".gitignore"), dir)
		if rel != "." {
			for _, part := range parts {
				dir = filepath.Join(dir, part)
				m.loadFile(filepath.Join(dir, ".gitignore"), dir)
			}
		}
	}

	return m
}

// LoadDir loads a .gitignore file from the given directory if one exists.
// Call this during directory traversal for nested .gitignore support.
// Safe for concurrent use.
func (m *Matcher) LoadDir(dirPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.loadFile(filepath.Join(dirPath, ".gitignore"), dirPath)
}

// IsIgnored returns true if the given path should be ignored.
// path must be an absolute path. isDir indicates whether it's a directory.
// Safe for concurrent use.
func (m *Matcher) IsIgnored(path string, isDir bool) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rel, err := filepath.Rel(m.rootDir, path)
	if err != nil {
		return false
	}
	// Normalize to forward slashes for matching
	rel = filepath.ToSlash(rel)

	ignored := false
	for _, p := range m.patterns {
		if p.dirOnly && !isDir {
			continue
		}

		// Compute relative path from the pattern's base directory
		matchPath := rel
		if p.baseDir != m.rootDir {
			r, err := filepath.Rel(p.baseDir, path)
			if err != nil || strings.HasPrefix(r, "..") {
				continue
			}
			matchPath = filepath.ToSlash(r)
		}

		if p.re.MatchString(matchPath) {
			ignored = !p.negated
		}
	}
	return ignored
}

func (m *Matcher) loadFile(path, baseDir string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m.addPattern(scanner.Text(), baseDir)
	}
}

func (m *Matcher) addPattern(line, baseDir string) {
	// Trim trailing whitespace (simplified: not handling escaped spaces)
	line = strings.TrimRight(line, " \t\r")

	// Skip blank lines and comments
	if line == "" || line[0] == '#' {
		return
	}

	p := pattern{baseDir: baseDir}

	// Escaped leading characters: \! and \# match literal ! and #
	escaped := false
	if len(line) >= 2 && line[0] == '\\' && (line[1] == '!' || line[1] == '#') {
		line = line[1:]
		escaped = true
	}

	// Negation (skip if the leading char was escaped)
	if !escaped && line[0] == '!' {
		p.negated = true
		line = line[1:]
	}

	// Directory-only pattern (trailing /)
	if strings.HasSuffix(line, "/") {
		p.dirOnly = true
		line = strings.TrimSuffix(line, "/")
	}

	// Determine if pattern is anchored (contains / other than trailing)
	anchored := false
	if strings.HasPrefix(line, "/") {
		anchored = true
		line = line[1:]
	} else if strings.Contains(line, "/") {
		anchored = true
	}

	// Convert gitignore glob to regex
	reStr := gitignoreToRegex(line, anchored)
	re, err := regexp.Compile(reStr)
	if err != nil {
		return
	}
	p.re = re
	m.patterns = append(m.patterns, p)
}

// gitignoreToRegex converts a gitignore glob pattern to a Go regex.
func gitignoreToRegex(pattern string, anchored bool) string {
	var b strings.Builder
	b.WriteString("^")

	if !anchored {
		// Non-anchored: can match at any directory level
		b.WriteString("(?:.*/)?")
	}

	i := 0
	for i < len(pattern) {
		ch := pattern[i]
		switch ch {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				// **
				if i+2 < len(pattern) && pattern[i+2] == '/' {
					// **/ matches zero or more directories
					b.WriteString("(?:.*/)?")
					i += 3
				} else {
					// ** at end or before non-/ matches everything
					b.WriteString(".*")
					i += 2
				}
			} else {
				// * matches anything except /
				b.WriteString("[^/]*")
				i++
			}
		case '?':
			b.WriteString("[^/]")
			i++
		case '[':
			// Character class - pass through to regex (simplified)
			j := i + 1
			if j < len(pattern) && pattern[j] == '!' {
				b.WriteByte('[')
				b.WriteByte('^')
				j++
			} else {
				b.WriteByte('[')
			}
			for j < len(pattern) && pattern[j] != ']' {
				b.WriteByte(pattern[j])
				j++
			}
			if j < len(pattern) {
				b.WriteByte(']')
				j++
			}
			i = j
		case '\\':
			// Backslash escapes the next character (gitignore spec)
			if i+1 < len(pattern) {
				i++
				next := pattern[i]
				switch next {
				case '.', '+', '(', ')', '{', '}', '^', '$', '|', '\\', '*', '?', '[', ']':
					b.WriteByte('\\')
				}
				b.WriteByte(next)
				i++
			} else {
				b.WriteByte('\\')
				b.WriteByte('\\')
				i++
			}
		case '.', '+', '(', ')', '{', '}', '^', '$', '|':
			b.WriteByte('\\')
			b.WriteByte(ch)
			i++
		default:
			b.WriteByte(ch)
			i++
		}
	}

	b.WriteString("$")
	return b.String()
}

// findGitRoot walks up from dir looking for a .git directory.
func findGitRoot(dir string) string {
	dir = filepath.Clean(dir)
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// globalGitignorePath returns the path to the global gitignore file.
func globalGitignorePath() string {
	// Check GIT_CONFIG_GLOBAL and core.excludesFile (simplified)
	// Just check the common default locations
	if home, err := os.UserHomeDir(); err == nil {
		candidates := []string{
			filepath.Join(home, ".config", "git", "ignore"),
			filepath.Join(home, ".gitignore_global"),
			filepath.Join(home, ".gitignore"),
		}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				return c
			}
		}
	}
	return ""
}
