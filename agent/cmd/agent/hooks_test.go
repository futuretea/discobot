package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestParseHookFrontMatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected hookConfig
	}{
		{
			name: "session hook with run_as root",
			content: `#!/bin/bash
#---
# name: Install deps
# type: session
# run_as: root
#---
apt-get install -y curl`,
			expected: hookConfig{Name: "Install deps", Type: "session", RunAs: "root"},
		},
		{
			name: "session hook default run_as",
			content: `#!/bin/bash
#---
# name: Setup env
# type: session
#---
pnpm install`,
			expected: hookConfig{Name: "Setup env", Type: "session"},
		},
		{
			name: "file hook",
			content: `#!/bin/bash
#---
# name: Go format
# type: file
#---
gofmt -l`,
			expected: hookConfig{Name: "Go format", Type: "file"},
		},
		{
			name: "pre-commit hook",
			content: `#!/bin/bash
#---
# name: Typecheck
# type: pre-commit
#---
pnpm typecheck`,
			expected: hookConfig{Name: "Typecheck", Type: "pre-commit"},
		},
		{
			name: "plain delimiter",
			content: `#!/bin/bash
---
name: Plain hook
type: session
---
echo hello`,
			expected: hookConfig{Name: "Plain hook", Type: "session"},
		},
		{
			name: "slash delimiter",
			content: `#!/usr/bin/env node
//---
// name: Node hook
// type: session
//---
console.log("hello")`,
			expected: hookConfig{Name: "Node hook", Type: "session"},
		},
		{
			name: "no front matter",
			content: `#!/bin/bash
echo hello`,
			expected: hookConfig{},
		},
		{
			name: "no shebang",
			content: `#---
# name: No shebang
# type: session
#---
echo hello`,
			expected: hookConfig{Name: "No shebang", Type: "session"},
		},
		{
			name: "no closing delimiter",
			content: `#!/bin/bash
#---
# name: Unclosed
# type: session
echo hello`,
			expected: hookConfig{},
		},
		{
			name: "quoted values",
			content: `#!/bin/bash
#---
# name: "Quoted name"
# type: session
#---
echo hello`,
			expected: hookConfig{Name: "Quoted name", Type: "session"},
		},
		{
			name: "single quoted values",
			content: `#!/bin/bash
#---
# name: 'Single quoted'
# type: session
#---
echo hello`,
			expected: hookConfig{Name: "Single quoted", Type: "session"},
		},
		{
			name: "empty content",
			content:  "",
			expected: hookConfig{},
		},
		{
			name: "run_as user explicit",
			content: `#!/bin/bash
#---
# name: User hook
# type: session
# run_as: user
#---
echo hello`,
			expected: hookConfig{Name: "User hook", Type: "session", RunAs: "user"},
		},
		{
			name: "blocking true",
			content: `#!/bin/bash
#---
# name: Install deps
# type: session
# blocking: true
#---
apt-get install -y curl`,
			expected: hookConfig{Name: "Install deps", Type: "session", Blocking: true},
		},
		{
			name: "blocking false explicit",
			content: `#!/bin/bash
#---
# name: Background task
# type: session
# blocking: false
#---
echo hello`,
			expected: hookConfig{Name: "Background task", Type: "session", Blocking: false},
		},
		{
			name: "blocking default is false",
			content: `#!/bin/bash
#---
# name: Default hook
# type: session
#---
echo hello`,
			expected: hookConfig{Name: "Default hook", Type: "session", Blocking: false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := parseHookFrontMatter(tt.content)

			if config.Name != tt.expected.Name {
				t.Errorf("Name: got %q, want %q", config.Name, tt.expected.Name)
			}
			if config.Type != tt.expected.Type {
				t.Errorf("Type: got %q, want %q", config.Type, tt.expected.Type)
			}
			if config.RunAs != tt.expected.RunAs {
				t.Errorf("RunAs: got %q, want %q", config.RunAs, tt.expected.RunAs)
			}
			if config.Blocking != tt.expected.Blocking {
				t.Errorf("Blocking: got %v, want %v", config.Blocking, tt.expected.Blocking)
			}
		})
	}
}

func TestNormalizeHookID(t *testing.T) {
	tests := []struct {
		filename string
		expected string
	}{
		{"dev.sh", "dev"},
		{"server.py", "server"},
		{"app.js", "app"},
		{"run.bash", "run"},
		{"start.zsh", "start"},
		{"MyService.sh", "myservice"},
		{"DEV", "dev"},
		{"foo.bar.sh", "foo-bar"},
		{"my.config.service", "my-config-service"},
		{"my_service.sh", "my_service"},
		{"dev_server", "dev_server"},
		{"my-service.sh", "my-service"},
		{"my@service!.sh", "myservice"},
		{"test (1).sh", "test1"},
		{"webapp", "webapp"},
		{".hidden.sh", "hidden"},
		{"service..sh", "service"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := normalizeHookID(tt.filename)
			if got != tt.expected {
				t.Errorf("normalizeHookID(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestHookStatusPersistence(t *testing.T) {
	tempDir := t.TempDir()

	t.Run("loadHookStatus returns empty status for missing file", func(t *testing.T) {
		status := loadHookStatus(tempDir)
		if len(status.Hooks) != 0 {
			t.Errorf("expected empty hooks, got %d", len(status.Hooks))
		}
		if len(status.PendingHooks) != 0 {
			t.Errorf("expected empty pendingHooks, got %d", len(status.PendingHooks))
		}
		if status.LastEvaluatedAt != "" {
			t.Errorf("expected empty lastEvaluatedAt, got %q", status.LastEvaluatedAt)
		}
	})

	t.Run("saveHookStatus and loadHookStatus roundtrip", func(t *testing.T) {
		status := hookStatusFile{
			Hooks: map[string]hookRunStatus{
				"test-hook": {
					HookID:              "test-hook",
					HookName:            "Test Hook",
					Type:                "session",
					LastRunAt:           "2024-01-01T00:00:00.000Z",
					LastResult:          "success",
					LastExitCode:        0,
					OutputPath:          "/tmp/test.log",
					RunCount:            5,
					FailCount:           1,
					ConsecutiveFailures: 0,
				},
			},
			PendingHooks:    []string{"hook-a", "hook-b"},
			LastEvaluatedAt: "2024-01-01T00:00:00.000Z",
		}

		err := saveHookStatus(tempDir, status)
		if err != nil {
			t.Fatalf("saveHookStatus failed: %v", err)
		}

		loaded := loadHookStatus(tempDir)
		if len(loaded.Hooks) != 1 {
			t.Fatalf("expected 1 hook, got %d", len(loaded.Hooks))
		}
		h := loaded.Hooks["test-hook"]
		if h.RunCount != 5 {
			t.Errorf("RunCount = %d, want 5", h.RunCount)
		}
		if h.FailCount != 1 {
			t.Errorf("FailCount = %d, want 1", h.FailCount)
		}
		if len(loaded.PendingHooks) != 2 {
			t.Errorf("PendingHooks length = %d, want 2", len(loaded.PendingHooks))
		}
		if loaded.LastEvaluatedAt != "2024-01-01T00:00:00.000Z" {
			t.Errorf("LastEvaluatedAt = %q, want %q", loaded.LastEvaluatedAt, "2024-01-01T00:00:00.000Z")
		}
	})

	t.Run("updateSessionHookStatus creates new hook entry", func(t *testing.T) {
		dir := t.TempDir()
		updateSessionHookStatus(dir, "my-hook", "My Hook", true, 0, "/tmp/out.log")

		status := loadHookStatus(dir)
		h, ok := status.Hooks["my-hook"]
		if !ok {
			t.Fatal("expected hook entry to exist")
		}
		if h.LastResult != "success" {
			t.Errorf("LastResult = %q, want %q", h.LastResult, "success")
		}
		if h.RunCount != 1 {
			t.Errorf("RunCount = %d, want 1", h.RunCount)
		}
		if h.FailCount != 0 {
			t.Errorf("FailCount = %d, want 0", h.FailCount)
		}
		if h.ConsecutiveFailures != 0 {
			t.Errorf("ConsecutiveFailures = %d, want 0", h.ConsecutiveFailures)
		}
	})

	t.Run("updateSessionHookStatus increments failure counts", func(t *testing.T) {
		dir := t.TempDir()
		updateSessionHookStatus(dir, "fail-hook", "Fail Hook", false, 1, "/tmp/out.log")
		updateSessionHookStatus(dir, "fail-hook", "Fail Hook", false, 1, "/tmp/out.log")

		status := loadHookStatus(dir)
		h := status.Hooks["fail-hook"]
		if h.RunCount != 2 {
			t.Errorf("RunCount = %d, want 2", h.RunCount)
		}
		if h.FailCount != 2 {
			t.Errorf("FailCount = %d, want 2", h.FailCount)
		}
		if h.ConsecutiveFailures != 2 {
			t.Errorf("ConsecutiveFailures = %d, want 2", h.ConsecutiveFailures)
		}
	})

	t.Run("updateSessionHookStatus resets consecutive failures on success", func(t *testing.T) {
		dir := t.TempDir()
		updateSessionHookStatus(dir, "reset-hook", "Reset Hook", false, 1, "/tmp/out.log")
		updateSessionHookStatus(dir, "reset-hook", "Reset Hook", false, 1, "/tmp/out.log")
		updateSessionHookStatus(dir, "reset-hook", "Reset Hook", true, 0, "/tmp/out.log")

		status := loadHookStatus(dir)
		h := status.Hooks["reset-hook"]
		if h.RunCount != 3 {
			t.Errorf("RunCount = %d, want 3", h.RunCount)
		}
		if h.FailCount != 2 {
			t.Errorf("FailCount = %d, want 2", h.FailCount)
		}
		if h.ConsecutiveFailures != 0 {
			t.Errorf("ConsecutiveFailures = %d, want 0", h.ConsecutiveFailures)
		}
	})

	t.Run("status.json schema matches TypeScript", func(t *testing.T) {
		dir := t.TempDir()
		updateSessionHookStatus(dir, "schema-hook", "Schema Hook", true, 0, "/tmp/out.log")

		data, err := os.ReadFile(filepath.Join(dir, "status.json"))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}

		// Unmarshal as generic map to check field names
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}

		// Check top-level fields
		if _, ok := raw["hooks"]; !ok {
			t.Error("missing 'hooks' field")
		}
		if _, ok := raw["pendingHooks"]; !ok {
			t.Error("missing 'pendingHooks' field")
		}
		if _, ok := raw["lastEvaluatedAt"]; !ok {
			t.Error("missing 'lastEvaluatedAt' field")
		}

		// Check hook entry fields
		var hooks map[string]map[string]json.RawMessage
		if err := json.Unmarshal(raw["hooks"], &hooks); err != nil {
			t.Fatalf("Unmarshal hooks: %v", err)
		}

		hook, ok := hooks["schema-hook"]
		if !ok {
			t.Fatal("missing 'schema-hook' entry")
		}

		requiredFields := []string{
			"hookId", "hookName", "type", "lastRunAt", "lastResult",
			"lastExitCode", "outputPath", "runCount", "failCount", "consecutiveFailures",
		}
		for _, field := range requiredFields {
			if _, ok := hook[field]; !ok {
				t.Errorf("missing field %q in hook status", field)
			}
		}
	})
}
