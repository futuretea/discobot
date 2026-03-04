package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// Task/Agent tool — launches a sub-agent for a complex task.
// Each task runs as an async operation with its own mini turn loop.

type taskInput struct {
	Description string `json:"description"`
	Prompt      string `json:"prompt"`

	// Agent-specific fields (from the Agent/Task tool schema).
	SubagentType string `json:"subagent_type"`
	MaxTurns     int    `json:"max_turns"`
}

// taskRecord tracks an in-progress or completed Task.
type taskRecord struct {
	id      string
	status  string // "pending", "in_progress", "completed", "failed"
	output  string
	created time.Time

	mu     sync.Mutex
	done   chan struct{}
	cancel context.CancelFunc // non-nil for Task/Agent sub-agent tasks; called by TaskStop
}

// taskStore holds all tasks for this executor instance.
type taskStore struct {
	mu    sync.Mutex
	tasks map[string]*taskRecord
}

var globalTasks = &taskStore{tasks: make(map[string]*taskRecord)}

func newTaskID() string {
	return fmt.Sprintf("task-%d", time.Now().UnixNano())
}

func (e *Executor) executeTask(_ context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	prompt := input.Prompt
	if prompt == "" {
		prompt = input.Description
	}
	if prompt == "" {
		return errResult(call, "prompt or description is required for Task/Agent tool"), nil
	}

	if e.subAgent == nil {
		return errResult(call, "Task tool is not available: no sub-agent configured"), nil
	}

	taskID := newTaskID()
	subThreadID := fmt.Sprintf("%s.sub.%s", e.threadID, taskID)

	rec := &taskRecord{
		id:      taskID,
		status:  "in_progress",
		created: time.Now(),
		done:    make(chan struct{}),
	}

	globalTasks.mu.Lock()
	globalTasks.tasks[taskID] = rec
	globalTasks.mu.Unlock()

	// Create the context before starting the goroutine so rec.cancel is always
	// set by the time taskHandle.Wait can observe it — no race on cancellation.
	subCtx, cancel := context.WithCancel(context.Background())
	rec.cancel = cancel

	subAgent := e.subAgent
	go runSubAgentGoroutine(subCtx, rec, subAgent, subThreadID, prompt, input.SubagentType, input.MaxTurns)

	return thread.ToolExecuteResult{Async: taskHandle(call, rec, taskID)}, nil
}

// resumeTask re-attaches to an in-flight or completed task after a crash.
func (e *Executor) resumeTask(_ context.Context, call message.ToolCallPart, taskID string) (thread.ToolExecuteResult, error) {
	// Fast path: task is still alive in memory.
	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[taskID]
	globalTasks.mu.Unlock()
	if ok {
		return thread.ToolExecuteResult{Async: taskHandle(call, rec, taskID)}, nil
	}

	if e.subAgent == nil {
		return thread.ToolExecuteResult{
			Result: errorResult(call, fmt.Sprintf("task %s lost after crash (sub-agent not configured)", taskID)),
		}, nil
	}

	subThreadID := fmt.Sprintf("%s.sub.%s", e.threadID, taskID)

	// Check whether the sub-agent already completed before the crash.
	if output, err := e.subAgent.FinalResponse(subThreadID); err == nil && output != "" {
		return thread.ToolExecuteResult{
			Result: message.ToolResultPart{
				ToolCallID: call.ToolCallID,
				ToolName:   call.ToolName,
				Output:     message.TextOutput{Value: output},
			},
		}, nil
	}

	// Sub-agent was mid-turn when the process crashed. Re-parse the original
	// input (persisted in the AsyncTaskInfo) and restart the goroutine.
	// DefaultAgent.Prompt detects the interrupted turn state and resumes it.
	var input taskInput
	if err := unmarshalInput(call, &input); err != nil {
		return thread.ToolExecuteResult{
			Result: errorResult(call, fmt.Sprintf("task %s: cannot recover input after crash: %v", taskID, err)),
		}, nil
	}
	prompt := input.Prompt
	if prompt == "" {
		prompt = input.Description
	}

	rec = &taskRecord{
		id:      taskID,
		status:  "in_progress",
		created: time.Now(),
		done:    make(chan struct{}),
	}
	globalTasks.mu.Lock()
	globalTasks.tasks[taskID] = rec
	globalTasks.mu.Unlock()

	subCtx, cancel := context.WithCancel(context.Background())
	rec.cancel = cancel

	subAgent := e.subAgent
	go runSubAgentGoroutine(subCtx, rec, subAgent, subThreadID, prompt, input.SubagentType, input.MaxTurns)

	return thread.ToolExecuteResult{Async: taskHandle(call, rec, taskID)}, nil
}

// runSubAgentGoroutine runs a full agent turn loop for a sub-agent task.
// ctx is created by the caller before the goroutine starts so rec.cancel is
// always populated by the time taskHandle.Wait can fire.
func runSubAgentGoroutine(ctx context.Context, rec *taskRecord, subAgent agent.Agent, subThreadID, prompt, subagentType string, maxTurns int) {
	defer close(rec.done)
	defer rec.cancel()

	req := agent.PromptRequest{
		UserParts:    []message.Part{message.TextPart{Text: prompt}},
		SubagentType: subagentType,
		MaxTurns:     maxTurns,
	}

	// Drain the Prompt iterator to run the full multi-step turn loop.
	var runErr error
	for _, err := range subAgent.Prompt(ctx, subThreadID, req) {
		if err != nil {
			runErr = err
			break
		}
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if runErr != nil {
		rec.status = "failed"
		rec.output = fmt.Sprintf("sub-agent failed: %v", runErr)
		return
	}

	// Read the final assistant text from the completed thread.
	output, err := subAgent.FinalResponse(subThreadID)
	if err != nil {
		rec.status = "failed"
		rec.output = fmt.Sprintf("sub-agent completed but result unavailable: %v", err)
		return
	}

	rec.status = "completed"
	rec.output = output
}

// taskHandle builds the AsyncTaskHandle that the turn loop waits on.
func taskHandle(call message.ToolCallPart, rec *taskRecord, taskID string) *thread.AsyncTaskHandle {
	return &thread.AsyncTaskHandle{
		TaskID: taskID,
		Wait: func(ctx context.Context) (message.ToolResultPart, error) {
			select {
			case <-rec.done:
			case <-ctx.Done():
				// Cancel the sub-agent goroutine when the parent is cancelled.
				rec.mu.Lock()
				if rec.cancel != nil {
					rec.cancel()
				}
				rec.mu.Unlock()
				return errorResult(call, "task cancelled"), nil
			}
			rec.mu.Lock()
			defer rec.mu.Unlock()
			if rec.status == "failed" {
				return errorResult(call, rec.output), nil
			}
			return message.ToolResultPart{
				ToolCallID: call.ToolCallID,
				ToolName:   call.ToolName,
				Output:     message.TextOutput{Value: rec.output},
			}, nil
		},
	}
}

// --- TaskCreate ---

type taskCreateInput struct {
	Content    string `json:"content"`
	ActiveForm string `json:"activeForm"`
	Status     string `json:"status"`
}

func (e *Executor) executeTaskCreate(_ context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskCreateInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	taskID := newTaskID()
	rec := &taskRecord{
		id:      taskID,
		status:  coalesce(input.Status, "pending"),
		output:  input.Content,
		created: time.Now(),
		done:    make(chan struct{}),
	}
	if rec.status != "pending" && rec.status != "in_progress" && rec.status != "completed" {
		rec.status = "pending"
	}
	if rec.status == "completed" {
		close(rec.done)
	}

	globalTasks.mu.Lock()
	globalTasks.tasks[taskID] = rec
	globalTasks.mu.Unlock()

	out, _ := json.Marshal(map[string]string{
		"id":     taskID,
		"status": rec.status,
	})
	return textResult(call, string(out)), nil
}

// --- TaskUpdate ---

type taskUpdateInput struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Output string `json:"output"`
}

func (e *Executor) executeTaskUpdate(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskUpdateInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.ID]
	globalTasks.mu.Unlock()

	if !ok {
		return errResult(call, fmt.Sprintf("task %s not found", input.ID)), nil
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if input.Status != "" {
		prevStatus := rec.status
		rec.status = input.Status
		if input.Status == "completed" && prevStatus != "completed" {
			select {
			case <-rec.done:
			default:
				close(rec.done)
			}
		}
	}
	if input.Output != "" {
		rec.output = input.Output
	}

	return textResult(call, fmt.Sprintf("Task %s updated: status=%s", input.ID, rec.status)), nil
}

// --- TaskGet ---

type taskGetInput struct {
	ID string `json:"id"`
}

func (e *Executor) executeTaskGet(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskGetInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.ID]
	globalTasks.mu.Unlock()

	if !ok {
		return errResult(call, fmt.Sprintf("task %s not found", input.ID)), nil
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	out, _ := json.Marshal(map[string]string{
		"id":     rec.id,
		"status": rec.status,
		"output": rec.output,
	})
	return textResult(call, string(out)), nil
}

// --- TaskList ---

func (e *Executor) executeTaskList(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	globalTasks.mu.Lock()
	defer globalTasks.mu.Unlock()

	type taskSummary struct {
		ID      string `json:"id"`
		Status  string `json:"status"`
		Created string `json:"created"`
	}

	summaries := make([]taskSummary, 0, len(globalTasks.tasks))
	for _, rec := range globalTasks.tasks {
		rec.mu.Lock()
		summaries = append(summaries, taskSummary{
			ID:      rec.id,
			Status:  rec.status,
			Created: rec.created.Format(time.RFC3339),
		})
		rec.mu.Unlock()
	}

	out, _ := json.MarshalIndent(summaries, "", "  ")
	return textResult(call, string(out)), nil
}

// --- TaskOutput ---

type taskOutputInput struct {
	ID string `json:"id"`
}

func (e *Executor) executeTaskOutput(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskOutputInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.ID]
	globalTasks.mu.Unlock()

	if !ok {
		return errResult(call, fmt.Sprintf("task %s not found", input.ID)), nil
	}

	rec.mu.Lock()
	output := rec.output
	rec.mu.Unlock()

	if output == "" {
		return textResult(call, "(no output yet)"), nil
	}
	return textResult(call, output), nil
}

// --- TaskStop ---

type taskStopInput struct {
	ID string `json:"id"`
}

func (e *Executor) executeTaskStop(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input taskStopInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}

	globalTasks.mu.Lock()
	rec, ok := globalTasks.tasks[input.ID]
	globalTasks.mu.Unlock()

	if !ok {
		return errResult(call, fmt.Sprintf("task %s not found", input.ID)), nil
	}

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if rec.status != "completed" && rec.status != "failed" {
		rec.status = "failed"
		rec.output += "\n[Task stopped by agent]"
		if rec.cancel != nil {
			rec.cancel()
		}
		select {
		case <-rec.done:
		default:
			close(rec.done)
		}
	}

	return textResult(call, fmt.Sprintf("Task %s stopped", input.ID)), nil
}

// coalesce returns the first non-empty string.
func coalesce(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
