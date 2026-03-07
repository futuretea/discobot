package cli

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolOutputDetail_DefaultTool(t *testing.T) {
	detail := toolOutputDetail("Bash", json.RawMessage(`{"command":"pwd"}`))
	if detail != "" {
		t.Fatalf("expected no detail for non-special tool, got %q", detail)
	}
}

func TestToolOutputDetail_Write(t *testing.T) {
	detail := toolOutputDetail("Write", json.RawMessage(`{"file_path":"/tmp/file.txt","content":"line1\nline2\n"}`))
	if !strings.HasPrefix(detail, "written content:\n") {
		t.Fatalf("expected written content header, got %q", detail)
	}
	if !strings.Contains(detail, "line1") || !strings.Contains(detail, "line2") {
		t.Fatalf("expected written content lines in detail, got %q", detail)
	}
}

func TestToolOutputDetail_Edit(t *testing.T) {
	detail := toolOutputDetail("Edit", json.RawMessage(`{"file_path":"/tmp/file.txt","old_string":"line one\nline two","new_string":"line one\nline 2\nline three"}`))
	if !strings.HasPrefix(detail, "applied diff:\n") {
		t.Fatalf("expected applied diff header, got %q", detail)
	}
	for _, want := range []string{"--- old", "+++ new", " line one", "-line two", "+line 2", "+line three"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("expected detail to contain %q, got %q", want, detail)
		}
	}
}
