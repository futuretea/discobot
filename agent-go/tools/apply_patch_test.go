package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runApplyPatch(t *testing.T, e *Executor, patch string) (string, bool) {
	t.Helper()
	return runTool(t, e, "apply_patch", map[string]any{"input": patch})
}

func TestApplyPatch_MissingInput(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runTool(t, e, "apply_patch", map[string]any{})
	if ok {
		t.Fatal("expected error for missing input")
	}
	if !strings.Contains(out, "input is required") {
		t.Fatalf("expected missing input error, got: %q", out)
	}
}

func TestApplyPatch_InvalidPatchBoundaries(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runApplyPatch(t, e, "*** Add File: a.txt\n+hello\n")
	if ok {
		t.Fatal("expected parse error for invalid boundaries")
	}
	if !strings.Contains(out, "the first line of the patch must be '*** Begin Patch'") {
		t.Fatalf("unexpected error output: %q", out)
	}
}

func TestApplyPatch_RejectsAbsolutePaths(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	patch := `*** Begin Patch
*** Add File: /tmp/abs.txt
+hello
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if ok {
		t.Fatal("expected absolute path to be rejected")
	}
	if !strings.Contains(out, "relative, NEVER ABSOLUTE") {
		t.Fatalf("unexpected error output: %q", out)
	}
}

func TestApplyPatch_EmptyPatchHasNoChanges(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runApplyPatch(t, e, "*** Begin Patch\n*** End Patch")
	if ok {
		t.Fatal("expected no-change patch to fail")
	}
	if !strings.Contains(out, "No files were modified.") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestApplyPatch_AddFile(t *testing.T) {
	cwd := t.TempDir()
	e := New(cwd, t.TempDir(), t.Name())

	patch := `*** Begin Patch
*** Add File: added.txt
+hello
+world
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "A added.txt") {
		t.Fatalf("expected add summary, got: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(cwd, "added.txt"))
	if err != nil {
		t.Fatalf("failed reading added file: %v", err)
	}
	if string(data) != "hello\nworld\n" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestApplyPatch_DeleteFile(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(cwd, "remove.txt")
	if err := os.WriteFile(path, []byte("to be removed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "remove.txt")

	patch := `*** Begin Patch
*** Delete File: remove.txt
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "D remove.txt") {
		t.Fatalf("expected delete summary, got: %q", out)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected file to be deleted, stat err=%v", err)
	}
}

func TestApplyPatch_UpdateFileWithChunks(t *testing.T) {
	cwd := t.TempDir()
	path := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(path, []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")

	patch := `*** Begin Patch
*** Update File: file.txt
@@
 alpha
-beta
+BETA
@@
 gamma
+delta
*** End of File
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "M file.txt") {
		t.Fatalf("expected modify summary, got: %q", out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed reading updated file: %v", err)
	}
	if string(data) != "alpha\nBETA\ngamma\ndelta\n" {
		t.Fatalf("unexpected file contents: %q", string(data))
	}
}

func TestApplyPatch_MoveFile(t *testing.T) {
	cwd := t.TempDir()
	src := filepath.Join(cwd, "old.txt")
	if err := os.WriteFile(src, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "old.txt")

	patch := `*** Begin Patch
*** Update File: old.txt
*** Move to: new/renamed.txt
@@
-before
+after
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "M new/renamed.txt") {
		t.Fatalf("expected moved-file summary, got: %q", out)
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("expected source file removed, stat err=%v", err)
	}

	dst := filepath.Join(cwd, "new", "renamed.txt")
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("failed reading moved file: %v", err)
	}
	if string(data) != "after\n" {
		t.Fatalf("unexpected moved file contents: %q", string(data))
	}
}

func TestApplyPatch_ContextNotFound(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "file.txt"), []byte("hello\nworld\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")

	patch := `*** Begin Patch
*** Update File: file.txt
@@
-not-there
+replacement
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if ok {
		t.Fatal("expected failure when old lines are missing")
	}
	if !strings.Contains(out, "failed to find expected lines") {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestApplyPatch_UnicodeNormalizedMatching(t *testing.T) {
	cwd := t.TempDir()
	line := "import asyncio  # local import – avoids top‑level dep\n"
	if err := os.WriteFile(filepath.Join(cwd, "unicode.py"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "unicode.py")

	patch := `*** Begin Patch
*** Update File: unicode.py
@@
-import asyncio  # local import - avoids top-level dep
+import asyncio  # HELLO
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "M unicode.py") {
		t.Fatalf("expected modify summary, got: %q", out)
	}

	data, err := os.ReadFile(filepath.Join(cwd, "unicode.py"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "import asyncio  # HELLO\n" {
		t.Fatalf("unexpected content: %q", string(data))
	}
}

func TestApplyPatch_ReadBeforeWriteEnforced(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "file.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := New(cwd, t.TempDir(), t.Name())
	patch := `*** Begin Patch
*** Update File: file.txt
@@
-hello
+HELLO
*** End Patch`

	out, ok := runApplyPatch(t, e, patch)
	if ok {
		t.Fatal("expected read-before-write error")
	}
	if !strings.Contains(out, "read") {
		t.Fatalf("expected read hint in output, got: %q", out)
	}
}
