// Package tools provides a concrete implementation of thread.ToolExecutor
// that executes all built-in tools natively in Go.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// maxOutputLen is the maximum number of characters returned inline to the LLM.
// Outputs longer than this are written to a file and truncated.
const maxOutputLen = 30_000

// previewLen is the number of characters shown inline when output is spilled.
const previewLen = 5_000

// fileRecord stores the mtime and size of a file at the time it was last read
// via the Read tool. It is used to enforce the read-before-write invariant.
type fileRecord struct {
	modTime time.Time
	size    int64
}

// planModeBlockedTools lists tools that are rejected when plan mode is active.
// Plan mode is read-only: the agent may explore but must not write code or execute commands.
var planModeBlockedTools = map[string]bool{
	"Bash":          true,
	"Write":         true,
	"Edit":          true,
	"NotebookEdit":  true,
	"EnterPlanMode": true, // already in plan mode
}

// Executor implements thread.ToolExecutor with native Go tool implementations.
type Executor struct {
	cwd      string // workspace root for file and shell operations
	threadID string // thread this executor is scoped to (used for log paths)

	// cwdMu guards currentCwd, which tracks the shell working directory
	// across Bash calls (cwd persists between commands, shell state does not).
	cwdMu      sync.Mutex
	currentCwd string

	// fileReadsMu guards fileReads, which records the mtime+size of every file
	// read via the Read tool. Write and Edit consult this to enforce
	// read-before-write: an existing file may not be overwritten unless the
	// executor has a matching record for it.
	fileReadsMu sync.RWMutex
	fileReads   map[string]fileRecord // keyed by absolute path

	// subAgent enables the Task tool to launch sub-agents.
	// Nil means the Task tool falls back to the stub behaviour.
	subAgent agent.Agent

	// planModeMu guards planMode.
	planModeMu sync.RWMutex
	planMode   bool // true while the thread is in plan mode
}

// New creates an Executor rooted at cwd for the given thread.
// All bash output is logged to {cwd}/.discobot/bash/{threadID}/.
func New(cwd, threadID string) *Executor {
	return &Executor{
		cwd:        cwd,
		threadID:   threadID,
		currentCwd: cwd,
		fileReads:  make(map[string]fileRecord),
	}
}

// recordFileRead saves the mtime and size of a file after a successful Read.
func (e *Executor) recordFileRead(absPath string, info os.FileInfo) {
	e.fileReadsMu.Lock()
	defer e.fileReadsMu.Unlock()
	e.fileReads[absPath] = fileRecord{modTime: info.ModTime(), size: info.Size()}
}

// recordFileWritten updates the stored record for a file after a successful
// Write or Edit, so subsequent writes don't require a re-read.
func (e *Executor) recordFileWritten(absPath string) {
	info, err := os.Stat(absPath)
	if err != nil {
		return
	}
	e.recordFileRead(absPath, info)
}

// checkWriteAllowed returns nil when it is safe to write to absPath:
//   - the file does not exist yet (new file creation is always permitted), or
//   - the file was previously read via the Read tool AND its mtime+size still
//     match the recorded snapshot (the file has not changed underneath us).
//
// displayPath is the user-facing path used in error messages.
func (e *Executor) checkWriteAllowed(absPath, displayPath string) error {
	info, err := os.Stat(absPath)
	if os.IsNotExist(err) {
		return nil // new file — no prior read required
	}
	if err != nil {
		return err
	}

	e.fileReadsMu.RLock()
	rec, ok := e.fileReads[absPath]
	e.fileReadsMu.RUnlock()

	if !ok {
		return fmt.Errorf("you must read %q before writing it", displayPath)
	}
	if rec.modTime != info.ModTime() || rec.size != info.Size() {
		return fmt.Errorf("%q has changed since it was last read — re-read it before writing", displayPath)
	}
	return nil
}

// SetSubAgent wires an Agent into the executor so that the Task tool can
// launch real sub-agent turns. Call this after constructing both the executor
// and the agent to break the construction cycle:
//
//	exec := tools.New(cwd, threadID)
//	a    := agentimpl.NewDefaultAgent(store, registry, exec, cwd)
//	exec.SetSubAgent(a)
func (e *Executor) SetSubAgent(a agent.Agent) {
	e.subAgent = a
}

// SetPlanMode switches the executor into or out of plan mode.
// Write and execute tools are rejected while plan mode is active.
func (e *Executor) SetPlanMode(enabled bool) {
	e.planModeMu.Lock()
	defer e.planModeMu.Unlock()
	e.planMode = enabled
}

func (e *Executor) isPlanMode() bool {
	e.planModeMu.RLock()
	defer e.planModeMu.RUnlock()
	return e.planMode
}

// Execute dispatches to the appropriate tool handler and enforces output size limits.
func (e *Executor) Execute(ctx context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	result, err := e.dispatch(ctx, call)
	if err != nil {
		return result, err
	}
	return e.limitOutput(call, result), nil
}

// dispatch routes a tool call to its handler.
func (e *Executor) dispatch(ctx context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	if e.isPlanMode() && planModeBlockedTools[call.ToolName] {
		return errResult(call, fmt.Sprintf("%s is not available in plan mode — finish your plan and call ExitPlanMode first", call.ToolName)), nil
	}

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

// limitOutput checks whether a successful TextOutput exceeds maxOutputLen.
// If it does, the full content is written to a file and the inline value is
// replaced with a short preview plus a path to the full output.
func (e *Executor) limitOutput(call message.ToolCallPart, result thread.ToolExecuteResult) thread.ToolExecuteResult {
	to, ok := result.Result.Output.(message.TextOutput)
	if !ok || len(to.Value) <= maxOutputLen {
		return result
	}

	outPath, writeErr := e.spillToFile(call, to.Value)

	preview := to.Value[:previewLen]
	var truncated string
	if writeErr != nil {
		truncated = fmt.Sprintf(
			"[Output too long (%d chars). Could not write to file: %v]\n\n%s\n\n[...truncated]",
			len(to.Value), writeErr, preview,
		)
	} else {
		truncated = fmt.Sprintf(
			"[Output too long (%d chars). Full output written to: %s]\n\n%s\n\n[...truncated — read %s for the full output]",
			len(to.Value), outPath, preview, outPath,
		)
	}

	result.Result.Output = message.TextOutput{Value: truncated}
	return result
}

// spillToFile writes text to {cwd}/.discobot/output/{threadID}/{toolCallID}.txt
// and returns the absolute path.
func (e *Executor) spillToFile(call message.ToolCallPart, text string) (string, error) {
	dir := filepath.Join(e.cwd, ".discobot", "output", e.threadID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, call.ToolCallID+".txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		return "", err
	}
	return path, nil
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
	if filepath.IsAbs(path) {
		return path
	}
	if cwd == "" || path == "" {
		return path
	}
	return filepath.Join(cwd, path)
}
