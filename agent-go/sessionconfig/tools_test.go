package sessionconfig

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestBuiltinTools_AllDefined(t *testing.T) {
	tools := builtinTools()

	expectedNames := []string{
		// Execution
		"Bash",
		// File operations
		"Read", "Write", "Edit", "NotebookEdit",
		// Search
		"Glob", "Grep",
		// Web
		"WebFetch", "WebSearch",
		// Agent orchestration
		"Agent",
		// Task management
		"TaskCreate", "TaskUpdate", "TaskGet", "TaskList",
		// Background tasks
		"TaskOutput", "TaskStop",
		// User interaction
		"AskUserQuestion",
		// Plan mode
		"EnterPlanMode", "ExitPlanMode",
		// Worktree
		"EnterWorktree",
		// Skills
		"Skill",
	}

	if len(tools) != len(expectedNames) {
		t.Fatalf("expected %d tools, got %d", len(expectedNames), len(tools))
	}

	nameSet := make(map[string]bool)
	for _, tool := range tools {
		nameSet[tool.Name] = true
	}

	for _, name := range expectedNames {
		if !nameSet[name] {
			t.Errorf("missing tool: %s", name)
		}
	}
}

func TestBuiltinTools_ValidSchemas(t *testing.T) {
	tools := builtinTools()

	for _, tool := range tools {
		if tool.Name == "" {
			t.Error("tool has empty name")
		}
		if tool.Description == "" {
			t.Errorf("tool %s has empty description", tool.Name)
		}

		// Verify input schema is valid JSON.
		var schema map[string]any
		if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
			t.Errorf("tool %s has invalid JSON schema: %v", tool.Name, err)
			continue
		}

		// Should be an object type with properties.
		if schema["type"] != "object" {
			t.Errorf("tool %s schema type = %v, want object", tool.Name, schema["type"])
		}
		if _, ok := schema["properties"]; !ok {
			t.Errorf("tool %s schema missing properties", tool.Name)
		}
	}
}

func TestBuiltinTools_BashSchema(t *testing.T) {
	schema := findToolSchema(t, "Bash")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"command", "description", "timeout", "run_in_background"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Bash schema missing '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "command" {
		t.Errorf("Bash required = %v, want [command]", required)
	}
}

func TestBuiltinTools_ReadSchema(t *testing.T) {
	schema := findToolSchema(t, "Read")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"file_path", "offset", "limit", "pages"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Read schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_EditSchema(t *testing.T) {
	schema := findToolSchema(t, "Edit")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"file_path", "old_string", "new_string", "replace_all"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Edit schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_GrepSchema(t *testing.T) {
	schema := findToolSchema(t, "Grep")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{
		"pattern", "path", "glob", "type", "output_mode",
		"-i", "-n", "-A", "-B", "-C", "context",
		"multiline", "head_limit", "offset",
	} {
		if _, ok := props[field]; !ok {
			t.Errorf("Grep schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_NotebookEditSchema(t *testing.T) {
	schema := findToolSchema(t, "NotebookEdit")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"notebook_path", "new_source", "cell_id", "cell_type", "edit_mode"} {
		if _, ok := props[field]; !ok {
			t.Errorf("NotebookEdit schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_AgentSchema(t *testing.T) {
	schema := findToolSchema(t, "Agent")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"description", "prompt", "subagent_type", "model", "resume", "run_in_background", "max_turns", "isolation"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Agent schema missing '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	reqSet := make(map[string]bool)
	for _, r := range required {
		reqSet[r.(string)] = true
	}
	for _, r := range []string{"description", "prompt", "subagent_type"} {
		if !reqSet[r] {
			t.Errorf("Agent missing required field: %s", r)
		}
	}
}

func TestBuiltinTools_TaskCreateSchema(t *testing.T) {
	schema := findToolSchema(t, "TaskCreate")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"subject", "description", "activeForm"} {
		if _, ok := props[field]; !ok {
			t.Errorf("TaskCreate schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_AskUserQuestionSchema(t *testing.T) {
	schema := findToolSchema(t, "AskUserQuestion")
	props := schema["properties"].(map[string]any)

	if _, ok := props["questions"]; !ok {
		t.Error("AskUserQuestion schema missing 'questions' property")
	}
}

func TestBuiltinTools_WebSearchSchema(t *testing.T) {
	schema := findToolSchema(t, "WebSearch")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"query", "allowed_domains", "blocked_domains"} {
		if _, ok := props[field]; !ok {
			t.Errorf("WebSearch schema missing '%s' property", field)
		}
	}
}

func TestBuiltinTools_SkillSchema(t *testing.T) {
	schema := findToolSchema(t, "Skill")
	props := schema["properties"].(map[string]any)

	for _, field := range []string{"skill", "args"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Skill schema missing '%s' property", field)
		}
	}

	required := schema["required"].([]any)
	if len(required) != 1 || required[0] != "skill" {
		t.Errorf("Skill required = %v, want [skill]", required)
	}
}

func TestBuiltinTools_WebSearchDescriptionUsesCurrentMonthYear(t *testing.T) {
	tools := builtinTools()

	var description string
	for _, tool := range tools {
		if tool.Name == "WebSearch" {
			description = tool.Description
			break
		}
	}
	if description == "" {
		t.Fatal("WebSearch tool not found")
	}

	monthYear := time.Now().Format("January 2006")
	if !strings.Contains(description, "The current month is "+monthYear+".") {
		t.Errorf("WebSearch description should include current month/year %q", monthYear)
	}
}

// findToolSchema returns the parsed input schema for a tool by name.
func findToolSchema(t *testing.T, name string) map[string]any {
	t.Helper()
	tools := builtinTools()
	for _, tool := range tools {
		if tool.Name == name {
			var schema map[string]any
			if err := json.Unmarshal(tool.InputSchema, &schema); err != nil {
				t.Fatalf("%s schema: %v", name, err)
			}
			return schema
		}
	}
	t.Fatalf("%s tool not found", name)
	return nil
}
