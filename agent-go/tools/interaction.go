package tools

import (
	"encoding/json"
	"fmt"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// AskUserQuestion — pauses the turn and presents questions to the user.
// The LLM sends a questions array; we route this through the ApprovalRequest
// mechanism so the handler can surface it to the client.

type askUserQuestionInput struct {
	Questions json.RawMessage `json:"questions"`
}

func (e *Executor) executeAskUserQuestion(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input askUserQuestionInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if len(input.Questions) == 0 {
		return errResult(call, "questions is required"), nil
	}

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Questions: input.Questions,
		},
	}, nil
}

// resolveAskUserQuestion converts the user's answers back into a tool result.
// answers is a map of question → answer strings.
func (e *Executor) resolveAskUserQuestion(call message.ToolCallPart, answers map[string]string) (message.ToolResultPart, error) {
	// Re-parse the original questions so we can format a nice result.
	var input askUserQuestionInput
	if err := json.Unmarshal(call.Input, &input); err != nil {
		return message.ToolResultPart{}, fmt.Errorf("re-parse AskUserQuestion input: %w", err)
	}

	// Merge questions with answers as JSON so the LLM can read them.
	type qaItem struct {
		Question string `json:"question"`
		Answer   string `json:"answer"`
	}

	// Parse the questions array to extract question texts.
	var questions []struct {
		Question string `json:"question"`
		Header   string `json:"header"`
	}
	_ = json.Unmarshal(input.Questions, &questions)

	// Build the merged Q&A output.
	merged := make([]qaItem, 0, len(answers))
	for _, q := range questions {
		answer, ok := answers[q.Question]
		if !ok {
			answer = answers[q.Header]
		}
		merged = append(merged, qaItem{Question: q.Question, Answer: answer})
	}

	// Fall back: if no questions parsed, just include raw answers.
	if len(merged) == 0 {
		for q, a := range answers {
			merged = append(merged, qaItem{Question: q, Answer: a})
		}
	}

	out, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return message.ToolResultPart{}, fmt.Errorf("marshal Q&A: %w", err)
	}

	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.TextOutput{Value: string(out)},
	}, nil
}

// EnterPlanMode — signals that the agent wants to enter plan mode.
// We surface this as an ApprovalRequest; the frontend decides whether to allow it.

func (e *Executor) executeEnterPlanMode(_ message.ToolCallPart) (thread.ToolExecuteResult, error) {
	// EnterPlanMode has no required input parameters.
	prompt := json.RawMessage(`[{"question":"Enter plan mode?","header":"Plan mode","options":[{"label":"Yes","description":"Allow the agent to switch to plan mode"},{"label":"No","description":"Stay in the current mode"}]}]`)

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Questions: prompt,
		},
	}, nil
}

func (e *Executor) resolveEnterPlanMode(call message.ToolCallPart, answers map[string]string) (message.ToolResultPart, error) {
	approved := false
	for _, v := range answers {
		if v == "Yes" || v == "yes" || v == "true" {
			approved = true
			break
		}
	}

	var result string
	if approved {
		result = "Plan mode activated. You are now in plan mode."
	} else {
		result = "Plan mode request denied. Continuing in current mode."
	}

	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.TextOutput{Value: result},
	}, nil
}

// ExitPlanMode — signals the agent is done with the plan and ready to implement.

type exitPlanModeInput struct {
	AllowedPrompts []json.RawMessage `json:"allowedPrompts"`
}

func (e *Executor) executeExitPlanMode(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input exitPlanModeInput
	_ = json.Unmarshal(call.Input, &input) // optional fields

	// Surface as an approval so the user can review the plan.
	prompt := json.RawMessage(`[{"question":"Approve the plan and proceed with implementation?","header":"Plan approval","options":[{"label":"Approve","description":"Approve the plan and let the agent implement it"},{"label":"Reject","description":"Reject the plan and ask the agent to revise it"}]}]`)

	return thread.ToolExecuteResult{
		Approval: &thread.ApprovalRequest{
			Questions: prompt,
		},
	}, nil
}

func (e *Executor) resolveExitPlanMode(call message.ToolCallPart, answers map[string]string) (message.ToolResultPart, error) {
	approved := false
	for _, v := range answers {
		if v == "Approve" || v == "approve" || v == "yes" || v == "true" {
			approved = true
			break
		}
	}

	var result string
	if approved {
		result = "Plan approved. You may now proceed with implementation."
	} else {
		result = "Plan rejected. Please revise and present a new plan."
	}

	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.TextOutput{Value: result},
	}, nil
}
