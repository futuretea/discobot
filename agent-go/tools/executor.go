// Package tools provides a concrete implementation of thread.ToolExecutor
// that executes all built-in tools natively in Go.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// Executor implements thread.ToolExecutor with native Go tool implementations.
type Executor struct {
	cwd      string // workspace root for file and shell operations
	threadID string // thread this executor is scoped to (used for log paths)

	// cwdMu guards currentCwd, which tracks the shell working directory
	// across Bash calls (cwd persists between commands, shell state does not).
	cwdMu      sync.Mutex
	currentCwd string
}

// New creates an Executor rooted at cwd for the given thread.
// All bash output is logged to {cwd}/.discobot/bash/{threadID}/.
func New(cwd, threadID string) *Executor {
	return &Executor{
		cwd:        cwd,
		threadID:   threadID,
		currentCwd: cwd,
	}
}

// Execute dispatches to the appropriate tool handler.
func (e *Executor) Execute(ctx context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	switch call.ToolName {
	case "Bash":
		return e.executeBash(ctx, call)
	case "Read":
		return e.executeRead(call)
	case "Write":
		return e.executeWrite(call)
	case "Edit":
		return e.executeEdit(call)
	case "Glob":
		return e.executeGlob(call)
	case "Grep":
		return e.executeGrep(call)
	case "WebFetch":
		return e.executeWebFetch(ctx, call)
	case "WebSearch":
		return e.executeWebSearch(ctx, call)
	case "NotebookEdit":
		return e.executeNotebookEdit(call)
	case "AskUserQuestion":
		return e.executeAskUserQuestion(call)
	case "EnterPlanMode":
		return e.executeEnterPlanMode(call)
	case "ExitPlanMode":
		return e.executeExitPlanMode(call)
	case "Task", "Agent":
		return e.executeTask(ctx, call)
	case "TaskCreate":
		return e.executeTaskCreate(ctx, call)
	case "TaskUpdate":
		return e.executeTaskUpdate(call)
	case "TaskGet":
		return e.executeTaskGet(call)
	case "TaskList":
		return e.executeTaskList(call)
	case "TaskOutput":
		return e.executeTaskOutput(call)
	case "TaskStop":
		return e.executeTaskStop(call)
	case "Skill":
		return e.executeSkill(ctx, call)
	default:
		return textResult(call, fmt.Sprintf("unknown tool: %s", call.ToolName)), nil
	}
}

// ResolveApproval converts a user's answers into a tool result after an ApprovalRequest.
func (e *Executor) ResolveApproval(call message.ToolCallPart, answers map[string]string) (message.ToolResultPart, error) {
	switch call.ToolName {
	case "AskUserQuestion":
		return e.resolveAskUserQuestion(call, answers)
	case "EnterPlanMode":
		return e.resolveEnterPlanMode(call, answers)
	case "ExitPlanMode":
		return e.resolveExitPlanMode(call, answers)
	default:
		return message.ToolResultPart{}, fmt.Errorf("ResolveApproval not supported for tool %s", call.ToolName)
	}
}

// ResumeAsync re-attaches to a previously launched async background task.
func (e *Executor) ResumeAsync(ctx context.Context, call message.ToolCallPart, taskID string) (thread.ToolExecuteResult, error) {
	switch call.ToolName {
	case "Task", "Agent":
		return e.resumeTask(ctx, call, taskID)
	default:
		return thread.ToolExecuteResult{
			Result: errorResult(call, fmt.Sprintf("async task for %s lost after crash (taskID: %s)", call.ToolName, taskID)),
		}, nil
	}
}

// --- helpers ---

// textResult builds a successful text tool result.
func textResult(call message.ToolCallPart, text string) thread.ToolExecuteResult {
	return thread.ToolExecuteResult{
		Result: message.ToolResultPart{
			ToolCallID: call.ToolCallID,
			ToolName:   call.ToolName,
			Output:     message.TextOutput{Value: text},
		},
	}
}

// errorResult builds an error text tool result.
func errorResult(call message.ToolCallPart, msg string) message.ToolResultPart {
	return message.ToolResultPart{
		ToolCallID: call.ToolCallID,
		ToolName:   call.ToolName,
		Output:     message.ErrorTextOutput{Value: msg},
	}
}

// errResult wraps errorResult in a ToolExecuteResult.
func errResult(call message.ToolCallPart, msg string) thread.ToolExecuteResult {
	return thread.ToolExecuteResult{Result: errorResult(call, msg)}
}

// unmarshalInput decodes the tool call input JSON into dst.
func unmarshalInput(call message.ToolCallPart, dst any) error {
	if err := json.Unmarshal(call.Input, dst); err != nil {
		return fmt.Errorf("invalid input for %s: %w", call.ToolName, err)
	}
	return nil
}

// resolvePath resolves a file path relative to cwd.
// Absolute paths are returned as-is; relative paths are joined with cwd.
func resolvePath(cwd, path string) string {
	if len(path) > 0 && path[0] == '/' {
		return path
	}
	if cwd == "" || path == "" {
		return path
	}
	return cwd + "/" + path
}
