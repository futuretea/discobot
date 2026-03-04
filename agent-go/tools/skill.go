package tools

import (
	"context"
	"fmt"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type skillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// executeSkill handles the Skill tool. It looks up the named skill file,
// expands $ARGUMENTS substitutions, and returns the body wrapped in a
// <skill-name> tag so Claude knows to follow the embedded instructions.
func (e *Executor) executeSkill(_ context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input skillInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Skill == "" {
		return errResult(call, "skill name is required"), nil
	}

	result, err := runSkill(e.cwd, input.Skill, input.Args)
	if err != nil {
		return errResult(call, fmt.Sprintf("skill %q: %v", input.Skill, err)), nil
	}
	return textResult(call, result), nil
}

// runSkill looks up a skill by name, substitutes arguments, and returns the
// skill body wrapped in a tag so Claude follows the embedded instructions.
// It searches skills/ first, then falls back to commands/ so the LLM can
// invoke legacy commands via the Skill tool too.
func runSkill(cwd, skillName, args string) (string, error) {
	projectRoot := sessionconfig.FindProjectRoot(cwd)

	cfg, found, err := sessionconfig.LookupSkill(projectRoot, skillName)
	if err != nil {
		return "", err
	}
	if !found {
		cfg, found, err = sessionconfig.LookupCommand(projectRoot, skillName)
		if err != nil {
			return "", err
		}
	}
	if !found {
		return "", fmt.Errorf("skill %q not found in .claude/skills or .claude/commands", skillName)
	}

	body := cfg.Expand(args)

	// Wrap in a tag matching the skill name. The Skill tool description tells
	// Claude: "If you see a <command-name> tag in the current conversation
	// turn, the skill has ALREADY been loaded — follow the instructions
	// directly instead of calling this tool again."
	return fmt.Sprintf("<%s>\n%s\n</%s>", skillName, body, skillName), nil
}
