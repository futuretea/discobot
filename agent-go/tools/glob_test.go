package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGlob_DoubleStarMatchesRecursively(t *testing.T) {
	cwd := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cwd, "level1", "level2"), 0o755); err != nil {
		t.Fatal(err)
	}

	topLevelMatch := filepath.Join(cwd, "sandbox-root.txt")
	if err := os.WriteFile(topLevelMatch, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}

	nestedMatch := filepath.Join(cwd, "level1", "level2", "sandbox-nested.txt")
	if err := os.WriteFile(nestedMatch, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}

	nonMatch := filepath.Join(cwd, "level1", "level2", "regular.txt")
	if err := os.WriteFile(nonMatch, []byte("c"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	out, ok := runTool(t, e, "Glob", map[string]any{
		"path":    cwd,
		"pattern": "**/*sandbox*",
	})
	if !ok {
		t.Fatalf("expected successful glob result, got error: %s", out)
	}

	if !strings.Contains(out, "sandbox-root.txt") {
		t.Fatalf("expected top-level sandbox file to match, got: %q", out)
	}
	if !strings.Contains(out, filepath.Join("level1", "level2", "sandbox-nested.txt")) {
		t.Fatalf("expected nested sandbox file to match recursively, got: %q", out)
	}
	if strings.Contains(out, "regular.txt") {
		t.Fatalf("expected non-matching file to be excluded, got: %q", out)
	}
}
