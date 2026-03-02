package sessionconfig

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverInstructions_SingleCLAUDEMD(t *testing.T) {
	dir := t.TempDir()

	// Create a CLAUDE.md at the project root.
	writeFile(t, filepath.Join(dir, "CLAUDE.md"), "Project instructions here.")
	// Create .git to mark project root.
	mkdirAll(t, filepath.Join(dir, ".git"))

	entries, err := discoverInstructions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Content != "Project instructions here." {
		t.Errorf("content = %q, want %q", entries[0].Content, "Project instructions here.")
	}
	if entries[0].Description != "project instructions, checked into the codebase" {
		t.Errorf("description = %q", entries[0].Description)
	}
}

func TestDiscoverInstructions_MultipleFiles(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))

	// Create multiple instruction files.
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "Root CLAUDE.md")
	writeFile(t, filepath.Join(root, "AGENTS.md"), "Root AGENTS.md")

	mkdirAll(t, filepath.Join(root, ".claude"))
	writeFile(t, filepath.Join(root, ".claude", "CLAUDE.md"), "Dot-claude CLAUDE.md")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	// Verify all three files are present by content.
	contents := make(map[string]bool)
	for _, e := range entries {
		contents[e.Content] = true
	}
	for _, want := range []string{"Root CLAUDE.md", "Dot-claude CLAUDE.md", "Root AGENTS.md"} {
		if !contents[want] {
			t.Errorf("missing entry with content %q", want)
		}
	}

	// Check AGENTS.md has correct description.
	for _, e := range entries {
		if e.Content == "Root AGENTS.md" {
			if e.Description != "agent instructions, checked into the codebase" {
				t.Errorf("AGENTS.md description = %q", e.Description)
			}
		}
	}
}

func TestDiscoverInstructions_WalkUpDirectory(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, "CLAUDE.md"), "Root instructions")

	subdir := filepath.Join(root, "src", "deep")
	mkdirAll(t, subdir)
	writeFile(t, filepath.Join(subdir, "CLAUDE.md"), "Deep instructions")

	entries, err := discoverInstructions(subdir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}

	// Deep instructions should come first (closer to cwd).
	if entries[0].Content != "Deep instructions" {
		t.Errorf("first entry = %q, want %q", entries[0].Content, "Deep instructions")
	}
	if entries[1].Content != "Root instructions" {
		t.Errorf("second entry = %q, want %q", entries[1].Content, "Root instructions")
	}
}

func TestDiscoverInstructions_Rules(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))

	rulesDir := filepath.Join(root, ".claude", "rules")
	mkdirAll(t, rulesDir)
	writeFile(t, filepath.Join(rulesDir, "formatting.md"), "Use tabs.")
	writeFile(t, filepath.Join(rulesDir, "testing.md"), "Write tests.")

	entries, err := discoverInstructions(root)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 rule entries, got %d", len(entries))
	}

	// Rules should be sorted alphabetically.
	if entries[0].Content != "Use tabs." {
		t.Errorf("first rule content = %q", entries[0].Content)
	}
	if entries[0].Path != ".claude/rules/formatting.md" {
		t.Errorf("first rule path = %q", entries[0].Path)
	}
	if entries[0].Description != "project rule" {
		t.Errorf("first rule description = %q", entries[0].Description)
	}
	if entries[1].Content != "Write tests." {
		t.Errorf("second rule content = %q", entries[1].Content)
	}
}

func TestDiscoverInstructions_NoFiles(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, ".git"))

	entries, err := discoverInstructions(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(entries))
	}
}

func TestFindProjectRoot_WithGit(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	subdir := filepath.Join(root, "a", "b", "c")
	mkdirAll(t, subdir)

	got := findProjectRoot(subdir)
	if got != root {
		t.Errorf("findProjectRoot(%s) = %s, want %s", subdir, got, root)
	}
}

func TestFindProjectRoot_NoGit(t *testing.T) {
	dir := t.TempDir()
	got := findProjectRoot(dir)
	if got != dir {
		t.Errorf("findProjectRoot(%s) = %s, want %s (self)", dir, got, dir)
	}
}

// helpers

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
}
