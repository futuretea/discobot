package tools

import (
	"context"
	"fmt"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type skillInput struct {
	Skill string `json:"skill"`
	Args  string `json:"args"`
}

// executeSkill handles the Skill tool. Skills are user-invocable prompts stored
// in ~/.claude/skills/ or similar. For now we return a descriptive message
// explaining the skill concept and noting the user should invoke it manually.
func (e *Executor) executeSkill(_ context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input skillInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Skill == "" {
		return errResult(call, "skill name is required"), nil
	}

	// Try to find and run the skill from the skills directory.
	result, err := runSkill(e.cwd, input.Skill, input.Args)
	if err != nil {
		return errResult(call, fmt.Sprintf("skill %q: %v", input.Skill, err)), nil
	}
	return textResult(call, result), nil
}

// runSkill looks up a skill by name and executes it. Skills are stored as
// markdown files with a prompt body. This is a minimal implementation.
func runSkill(cwd, skillName, args string) (string, error) {
	// Skill execution would involve:
	// 1. Finding the skill file (e.g., ~/.claude/skills/{name}.md or local .claude/skills/)
	// 2. Parsing the skill's prompt template
	// 3. Substituting args
	// 4. Running the resulting prompt through the agent

	// For now, return a message indicating the skill was invoked.
	msg := fmt.Sprintf("Skill '%s' invoked", skillName)
	if args != "" {
		msg += fmt.Sprintf(" with args: %s", args)
	}
	msg += "\n\nNote: Full skill execution (loading prompt template and running) is not yet implemented."
	_ = cwd
	return msg, nil
}
