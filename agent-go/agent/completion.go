package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// TurnCompleteFunc is called when a completion finishes.
// threadID is the thread that completed, err is non-nil on failure.
type TurnCompleteFunc func(threadID string, err error)

// CompletionManager wraps an Agent with goroutine management, chunk caching,
// and SSE polling. It bridges the synchronous streaming Agent interface to
// the async HTTP handler world.
//
// All critical state is persisted to disk by the underlying Agent; the
// in-memory event cache is non-authoritative and can be rebuilt after a crash.
type CompletionManager struct {
	agent Agent

	mu             sync.Mutex
	active         map[string]*activeCompletion // keyed by threadID
	onTurnComplete TurnCompleteFunc
}

type activeCompletion struct {
	id       string
	threadID string
	cancel   context.CancelFunc

	mu      sync.Mutex
	events  []message.MessageChunk // cache for SSE replay (non-authoritative)
	done    bool
	err     error
	leafMsg string // tracks the leaf message ID as messages are saved
}

// NewCompletionManager creates a CompletionManager wrapping the given Agent.
func NewCompletionManager(agent Agent) *CompletionManager {
	return &CompletionManager{
		agent:  agent,
		active: make(map[string]*activeCompletion),
	}
}

// Recover checks all threads for interrupted turns and resumes them.
// Call this on startup before handling any requests.
func (cm *CompletionManager) Recover() {
	threads, err := cm.agent.InterruptedThreads()
	if err != nil {
		log.Printf("completion: recover: %v", err)
		return
	}

	for _, threadID := range threads {
		log.Printf("completion: recovering interrupted thread %s", threadID)
		cm.startPrompt(threadID, PromptRequest{})
	}
}

// Chat starts a new turn for the given thread. It returns the completion ID
// or an error if a completion is already running for this thread.
// The turn runs in a background goroutine; chunks are cached for SSE replay.
func (cm *CompletionManager) Chat(threadID string, req PromptRequest) (string, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if existing, ok := cm.active[threadID]; ok {
		existing.mu.Lock()
		done := existing.done
		existing.mu.Unlock()
		if !done {
			return "", fmt.Errorf("completion_in_progress:%s", existing.id)
		}
	}

	completionID := generateID()

	ctx, cancel := context.WithCancel(context.Background())
	comp := &activeCompletion{
		id:       completionID,
		threadID: threadID,
		cancel:   cancel,
		leafMsg:  req.LeafID,
	}
	cm.active[threadID] = comp

	go cm.runCompletion(ctx, comp, threadID, req)

	return completionID, nil
}

// startPrompt starts a background prompt for recovery or hook re-prompting.
// It does NOT check for existing active completions (caller must handle that).
func (cm *CompletionManager) startPrompt(threadID string, req PromptRequest) {
	completionID := generateID()

	ctx, cancel := context.WithCancel(context.Background())
	comp := &activeCompletion{
		id:       completionID,
		threadID: threadID,
		cancel:   cancel,
		leafMsg:  req.LeafID,
	}

	cm.mu.Lock()
	cm.active[threadID] = comp
	cm.mu.Unlock()

	go cm.runCompletion(ctx, comp, threadID, req)
}

// runCompletion drives the Agent.Prompt iterator in a goroutine, caching chunks.
func (cm *CompletionManager) runCompletion(ctx context.Context, comp *activeCompletion, threadID string, req PromptRequest) {
	var turnErr error
	for chunk, err := range cm.agent.Prompt(ctx, threadID, req) {
		comp.mu.Lock()
		if err != nil {
			turnErr = err
			comp.err = err
			comp.events = append(comp.events, message.ErrorChunk{ErrorText: err.Error()})
			comp.done = true
			comp.mu.Unlock()
			cm.notifyComplete(threadID, turnErr)
			return
		}
		if chunk != nil {
			comp.events = append(comp.events, chunk)
		}
		comp.mu.Unlock()
	}
	comp.mu.Lock()
	comp.done = true
	comp.mu.Unlock()
	cm.notifyComplete(threadID, nil)
}

// PollResult holds the chunks returned by PollChunks.
type PollResult struct {
	Chunks []message.MessageChunk
	Done   bool
}

// PollChunks returns cached events from the given offset for a thread's
// active completion. Returns nil if no active completion exists.
// The event cache is non-authoritative; on crash recovery, events are
// replayed from the step JSONL files on disk.
func (cm *CompletionManager) PollChunks(threadID string, offset int) *PollResult {
	cm.mu.Lock()
	comp, ok := cm.active[threadID]
	cm.mu.Unlock()
	if !ok {
		return nil
	}

	comp.mu.Lock()
	defer comp.mu.Unlock()

	if offset >= len(comp.events) {
		return &PollResult{Done: comp.done}
	}

	chunks := make([]message.MessageChunk, len(comp.events)-offset)
	copy(chunks, comp.events[offset:])
	return &PollResult{Chunks: chunks, Done: comp.done}
}

// Cancel cancels the active completion for a thread.
// Returns the completion ID and true if there was an active completion,
// or empty string and false if no active completion exists.
func (cm *CompletionManager) Cancel(threadID string) (string, bool) {
	cm.mu.Lock()
	comp, ok := cm.active[threadID]
	cm.mu.Unlock()
	if !ok {
		return "", false
	}

	comp.mu.Lock()
	done := comp.done
	comp.mu.Unlock()

	if done {
		return "", false
	}

	comp.cancel()
	return comp.id, true
}

// ActiveCompletionID returns the active completion ID for a thread,
// or empty string if none is active.
func (cm *CompletionManager) ActiveCompletionID(threadID string) string {
	cm.mu.Lock()
	comp, ok := cm.active[threadID]
	cm.mu.Unlock()
	if !ok {
		return ""
	}
	comp.mu.Lock()
	defer comp.mu.Unlock()
	if comp.done {
		return ""
	}
	return comp.id
}

// Messages returns the conversation history for a thread as UI-projected JSON.
func (cm *CompletionManager) Messages(threadID, leafID string) ([]json.RawMessage, error) {
	return cm.agent.Messages(threadID, leafID)
}

// MessagesJSON returns the conversation history as marshaled JSON.
func (cm *CompletionManager) MessagesJSON(threadID, leafID string) (json.RawMessage, error) {
	msgs, err := cm.Messages(threadID, leafID)
	if err != nil {
		return nil, err
	}
	if msgs == nil {
		return json.RawMessage("[]"), nil
	}
	data, err := json.Marshal(msgs)
	if err != nil {
		return nil, fmt.Errorf("marshal messages: %w", err)
	}
	return data, nil
}

// ListModels returns available models from the provider.
func (cm *CompletionManager) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return cm.agent.ListModels(ctx)
}

// ListThreads returns all thread IDs.
func (cm *CompletionManager) ListThreads() ([]string, error) {
	return cm.agent.ListThreads()
}

// PendingQuestion returns the pending AskUserQuestion for a thread, or nil.
func (cm *CompletionManager) PendingQuestion(threadID string) (*thread.PendingQuestionState, error) {
	return cm.agent.PendingQuestion(threadID)
}

// SubmitAnswer persists the user's answer for a pending AskUserQuestion.
func (cm *CompletionManager) SubmitAnswer(threadID, toolCallID string, answers map[string]string) error {
	return cm.agent.SubmitAnswer(threadID, toolCallID, answers)
}

// SetOnTurnComplete sets a callback that fires when any completion finishes.
func (cm *CompletionManager) SetOnTurnComplete(fn TurnCompleteFunc) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.onTurnComplete = fn
}

// notifyComplete calls the onTurnComplete callback if set.
func (cm *CompletionManager) notifyComplete(threadID string, err error) {
	cm.mu.Lock()
	fn := cm.onTurnComplete
	cm.mu.Unlock()
	if fn != nil {
		fn(threadID, err)
	}
}
