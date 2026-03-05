package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
)

// runEdit is a test helper that executes an Edit tool call and returns the output text.
func runEdit(t *testing.T, e *Executor, input map[string]any) (string, bool) {
	t.Helper()
	raw, _ := json.Marshal(input)
	result, err := e.Execute(context.Background(), message.ToolCallPart{
		ToolCallID: t.Name() + "-edit",
		ToolName:   "Edit",
		Input:      raw,
	})
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	switch v := result.Result.Output.(type) {
	case message.TextOutput:
		return v.Value, true
	case message.ErrorTextOutput:
		return v.Value, false
	}
	return "", false
}

// primeRead performs a Read tool call so that Write/Edit guards are satisfied.
func primeRead(t *testing.T, e *Executor, filePath string) {
	t.Helper()
	out, ok := runTool(t, e, "Read", map[string]any{"file_path": filePath})
	if !ok {
		t.Fatalf("primeRead failed for %s: %s", filePath, out)
	}
}

func TestEdit_MissingFilePath(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runEdit(t, e, map[string]any{"old_string": "foo"})
	if ok {
		t.Error("expected error for missing file_path, got success")
	}
	if !strings.Contains(out, "file_path is required") {
		t.Errorf("expected 'file_path is required' in output, got: %q", out)
	}
}

func TestEdit_MissingOldString(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runEdit(t, e, map[string]any{"file_path": "foo.txt"})
	if ok {
		t.Error("expected error for missing old_string, got success")
	}
	if !strings.Contains(out, "old_string is required") {
		t.Errorf("expected 'old_string is required' in output, got: %q", out)
	}
}

func TestEdit_FileNotFound(t *testing.T) {
	e := New(t.TempDir(), t.TempDir(), t.Name())
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "nonexistent.txt",
		"old_string": "foo",
	})
	if ok {
		t.Error("expected error for nonexistent file, got success")
	}
	if !strings.Contains(out, "file not found") {
		t.Errorf("expected 'file not found' in output, got: %q", out)
	}
}

func TestEdit_OldStringNotFound(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "file.txt"), []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "not present",
	})
	if ok {
		t.Error("expected error when old_string not in file, got success")
	}
	if !strings.Contains(out, "old_string not found") {
		t.Errorf("expected 'old_string not found' in output, got: %q", out)
	}
}

func TestEdit_SimpleReplacement(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "world",
		"new_string": "Go",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "Successfully edited") {
		t.Errorf("expected success message, got: %q", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "hello Go\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestEdit_MultipleOccurrencesWithoutReplaceAll verifies that editing a file
// with multiple matches fails unless replace_all is set.
func TestEdit_MultipleOccurrencesWithoutReplaceAll(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "file.txt"), []byte("foo foo foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "foo",
	})
	if ok {
		t.Error("expected error for multiple occurrences without replace_all, got success")
	}
	if !strings.Contains(out, "3 times") {
		t.Errorf("expected occurrence count in error message, got: %q", out)
	}
	if !strings.Contains(out, "replace_all") {
		t.Errorf("expected 'replace_all' hint in error message, got: %q", out)
	}
}

// TestEdit_ReplaceAllMultipleOccurrences verifies that replace_all replaces every match
// and reports the count.
func TestEdit_ReplaceAllMultipleOccurrences(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(filePath, []byte("foo foo foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":   "file.txt",
		"old_string":  "foo",
		"new_string":  "bar",
		"replace_all": true,
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "3 occurrences") {
		t.Errorf("expected '3 occurrences' in success message, got: %q", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "bar bar bar\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestEdit_ReplaceAllSingleOccurrence verifies that replace_all with only one match
// still succeeds and uses the standard success message (not the count variant).
func TestEdit_ReplaceAllSingleOccurrence(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":   "file.txt",
		"old_string":  "world",
		"new_string":  "Go",
		"replace_all": true,
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	if !strings.Contains(out, "Successfully edited") {
		t.Errorf("expected 'Successfully edited' message, got: %q", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "hello Go\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestEdit_DeleteContent verifies that setting new_string to empty deletes the matched text.
func TestEdit_DeleteContent(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": " world",
		"new_string": "",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "hello\n" {
		t.Errorf("unexpected file content after deletion: %q", string(data))
	}
}

// TestEdit_MultilineReplacement verifies replacements work across newlines.
func TestEdit_MultilineReplacement(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	original := "line one\nline two\nline three\n"
	if err := os.WriteFile(filePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "line one\nline two",
		"new_string": "replaced",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "replaced\nline three\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestEdit_WhitespaceSensitivity verifies that old_string must match exactly
// including indentation.
func TestEdit_WhitespaceSensitivity(t *testing.T) {
	cwd := t.TempDir()
	if err := os.WriteFile(filepath.Join(cwd, "file.txt"), []byte("\tfoo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	// Searching with spaces instead of a tab should fail.
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "    foo", // spaces, not a tab
	})
	if ok {
		t.Error("expected error for whitespace mismatch, got success")
	}
	if !strings.Contains(out, "old_string not found") {
		t.Errorf("expected 'old_string not found' for whitespace mismatch, got: %q", out)
	}
}

// TestEdit_AbsolutePath verifies that an absolute path is resolved correctly.
func TestEdit_AbsolutePath(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "abs.txt")
	if err := os.WriteFile(filePath, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, filePath) // absolute path
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  filePath, // absolute
		"old_string": "before",
		"new_string": "after",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "after\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}

// TestEdit_OnlyFirstOccurrenceReplaced verifies that without replace_all, only the
// first occurrence is changed when there would be exactly one match.
func TestEdit_OnlyFirstOccurrenceReplaced(t *testing.T) {
	cwd := t.TempDir()
	filePath := filepath.Join(cwd, "file.txt")
	if err := os.WriteFile(filePath, []byte("alpha beta gamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := New(cwd, t.TempDir(), t.Name())
	primeRead(t, e, "file.txt")
	out, ok := runEdit(t, e, map[string]any{
		"file_path":  "file.txt",
		"old_string": "beta",
		"new_string": "BETA",
	})
	if !ok {
		t.Fatalf("unexpected error: %s", out)
	}
	data, _ := os.ReadFile(filePath)
	if string(data) != "alpha BETA gamma\n" {
		t.Errorf("unexpected file content: %q", string(data))
	}
}
