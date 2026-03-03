package sessionconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// discoverInstructions walks from cwd upward to find CLAUDE.md, AGENTS.md,
// and related instruction files. Returns structured entries preserving each
// file's path, description, and content individually.
func discoverInstructions(cwd string) ([]InstructionEntry, error) {
	projectRoot := findProjectRoot(cwd)

	var entries []InstructionEntry

	// 1. Walk from cwd upward to filesystem root, collecting per-directory files.
	seen := make(map[string]bool)
	dir := cwd
	for {
		dir = filepath.Clean(dir)
		if seen[dir] {
			break
		}
		seen[dir] = true

		for _, name := range []string{"CLAUDE.md", ".claude/CLAUDE.md", "AGENTS.md"} {
			p := filepath.Join(dir, name)
			content, err := readFileIfExists(p)
			if err != nil {
				return nil, fmt.Errorf("read %s: %w", p, err)
			}
			if content != "" {
				rel, _ := filepath.Rel(projectRoot, p)
				if rel == "" {
					rel = p
				}
				entries = append(entries, InstructionEntry{
					Path:        rel,
					Description: descriptionForFile(name),
					Content:     strings.TrimSpace(content),
				})
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}

	// 2. User-level instructions (~/.claude/CLAUDE.md).
	if home, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(home, ".claude", "CLAUDE.md")
		content, err := readFileIfExists(p)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", p, err)
		}
		if content != "" {
			entries = append(entries, InstructionEntry{
				Path:        "~/.claude/CLAUDE.md",
				Description: "user-level instructions",
				Content:     strings.TrimSpace(content),
			})
		}
	}

	// 3. Modular rules from project root (.claude/rules/*.md).
	rulesDir := filepath.Join(projectRoot, ".claude", "rules")
	ruleEntries, err := discoverRules(rulesDir)
	if err != nil {
		return nil, err
	}
	entries = append(entries, ruleEntries...)

	return entries, nil
}

// descriptionForFile returns a human-readable description for an instruction file.
func descriptionForFile(name string) string {
	switch name {
	case "AGENTS.md":
		return "agent instructions, checked into the codebase"
	default:
		return "project instructions, checked into the codebase"
	}
}

// discoverRules loads .claude/rules/*.md files sorted by name.
func discoverRules(rulesDir string) ([]InstructionEntry, error) {
	dirEntries, err := os.ReadDir(rulesDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read rules dir: %w", err)
	}

	// Sort by name for deterministic ordering.
	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	var entries []InstructionEntry
	for _, e := range dirEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p := filepath.Join(rulesDir, e.Name())
		content, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("read rule %s: %w", p, err)
		}
		if len(content) > 0 {
			entries = append(entries, InstructionEntry{
				Path:        ".claude/rules/" + e.Name(),
				Description: "project rule",
				Content:     strings.TrimSpace(string(content)),
			})
		}
	}
	return entries, nil
}

// findProjectRoot walks up from dir looking for a .git directory.
// Returns the directory containing .git, or dir itself if not found.
func findProjectRoot(dir string) string {
	dir = filepath.Clean(dir)
	cur := dir
	for {
		gitDir := filepath.Join(cur, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return cur
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			break
		}
		cur = parent
	}
	return dir
}

// readFileIfExists reads a file and returns its content as a string.
// Returns empty string if the file does not exist.
func readFileIfExists(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}
