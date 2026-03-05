package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/obot-platform/discobot/agent-go/internal/config"
)

func TestAgentSlashCommands_LoadsSkillsAndCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "deploy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("---\nname: deploy\ndescription: Deploy app\n---\nDeploy now."), 0o644); err != nil {
		t.Fatal(err)
	}

	commandsDir := filepath.Join(root, ".claude", "commands")
	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(commandsDir, "release.md"), []byte("Release command body."), 0o644); err != nil {
		t.Fatal(err)
	}

	commands, err := agentSlashCommands(root)
	if err != nil {
		t.Fatalf("agentSlashCommands() error = %v", err)
	}
	if _, ok := commands["/deploy"]; !ok {
		t.Fatalf("expected /deploy in discovered commands")
	}
	if _, ok := commands["/release"]; !ok {
		t.Fatalf("expected /release in discovered commands")
	}
}

func TestHandleSlashCommand_ForwardsKnownAgentCommand(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "deploy")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Deploy skill body."), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{AgentCwd: root}
	threadID, handled := handleSlashCommand(context.Background(), "/deploy", nil, nil, cfg, "thread-1", nil, nil, nil)
	if handled {
		t.Fatalf("expected /deploy to be forwarded to agent, handled=%v", handled)
	}
	if threadID != "thread-1" {
		t.Fatalf("threadID changed unexpectedly: %q", threadID)
	}
}

func TestHandleSlashCommand_UnknownStillHandledLocally(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	cfg := &config.Config{AgentCwd: root}
	threadID, handled := handleSlashCommand(context.Background(), "/does-not-exist", nil, nil, cfg, "thread-1", nil, nil, nil)
	if !handled {
		t.Fatalf("expected unknown slash command to be handled locally")
	}
	if threadID != "thread-1" {
		t.Fatalf("threadID changed unexpectedly: %q", threadID)
	}
}

func TestHandleSlashCommand_LocalCommandsTakePriority(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", filepath.Join(root, "home"))

	skillDir := filepath.Join(root, ".claude", "skills", "clear")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("Clear skill body."), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{AgentCwd: root}
	newThreadID, handled := handleSlashCommand(context.Background(), "/clear", nil, nil, cfg, "thread-1", nil, nil, nil)
	if !handled {
		t.Fatalf("expected /clear to be handled locally")
	}
	if newThreadID == "thread-1" {
		t.Fatalf("expected /clear to start a new thread")
	}
}
