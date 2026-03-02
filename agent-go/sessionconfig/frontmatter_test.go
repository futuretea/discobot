package sessionconfig

import (
	"testing"
)

func TestParseFrontmatter_Valid(t *testing.T) {
	content := `---
name: test-agent
description: A test agent
model: gpt-4
allowedTools:
  - Bash
  - Read
maxTurns: 10
---
This is the agent prompt.

It has multiple paragraphs.`

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if fm == nil {
		t.Fatal("expected frontmatter, got nil")
	}
	if fm["name"] != "test-agent" {
		t.Errorf("name = %v, want test-agent", fm["name"])
	}
	if fm["description"] != "A test agent" {
		t.Errorf("description = %v, want A test agent", fm["description"])
	}
	if fm["model"] != "gpt-4" {
		t.Errorf("model = %v, want gpt-4", fm["model"])
	}
	tools, ok := fm["allowedTools"].([]any)
	if !ok || len(tools) != 2 {
		t.Fatalf("allowedTools = %v, want [Bash Read]", fm["allowedTools"])
	}
	if tools[0] != "Bash" || tools[1] != "Read" {
		t.Errorf("allowedTools = %v, want [Bash Read]", tools)
	}
	if fm["maxTurns"] != 10 {
		t.Errorf("maxTurns = %v, want 10", fm["maxTurns"])
	}

	want := "This is the agent prompt.\n\nIt has multiple paragraphs."
	if body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestParseFrontmatter_NoFrontmatter(t *testing.T) {
	content := "Just a regular markdown file.\n\nNo frontmatter here."

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if fm != nil {
		t.Errorf("expected nil frontmatter, got %v", fm)
	}
	if body != content {
		t.Errorf("body = %q, want %q", body, content)
	}
}

func TestParseFrontmatter_EmptyBody(t *testing.T) {
	content := `---
name: empty-body
---`

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if fm == nil {
		t.Fatal("expected frontmatter, got nil")
	}
	if fm["name"] != "empty-body" {
		t.Errorf("name = %v, want empty-body", fm["name"])
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}

func TestParseFrontmatter_MalformedYAML(t *testing.T) {
	content := `---
name: [invalid yaml
  missing bracket
---
Body content.`

	_, _, err := parseFrontmatter(content)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestParseFrontmatter_NoClosingDelimiter(t *testing.T) {
	content := `---
name: unclosed
This never ends`

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if fm != nil {
		t.Errorf("expected nil frontmatter for unclosed delimiter, got %v", fm)
	}
	if body != content {
		t.Errorf("body should be original content")
	}
}

func TestParseFrontmatter_LeadingNewlines(t *testing.T) {
	content := "\n\n---\nname: with-newlines\n---\nBody."

	fm, body, err := parseFrontmatter(content)
	if err != nil {
		t.Fatal(err)
	}
	if fm == nil {
		t.Fatal("expected frontmatter, got nil")
	}
	if fm["name"] != "with-newlines" {
		t.Errorf("name = %v, want with-newlines", fm["name"])
	}
	if body != "Body." {
		t.Errorf("body = %q, want %q", body, "Body.")
	}
}
