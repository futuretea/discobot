package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"sync"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/providers/modelsdev"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// DefaultAgent is the built-in Agent implementation that uses the thread
// package for file-based persistence and the crash-resilient step loop.
type DefaultAgent struct {
	store    *thread.Store
	registry *providers.ProviderRegistry
	executor thread.ToolExecutor
	cwd      string // working directory for session config discovery

	mu      sync.Mutex
	cancels map[string]context.CancelFunc
}

// Store returns the underlying thread store.
func (a *DefaultAgent) Store() *thread.Store {
	return a.store
}

// NewDefaultAgent creates a DefaultAgent. Session configuration (system prompt,
// tools, instructions) is loaded fresh from the cwd when a new thread is created.
func NewDefaultAgent(
	store *thread.Store,
	registry *providers.ProviderRegistry,
	executor thread.ToolExecutor,
	cwd string,
) *DefaultAgent {
	return &DefaultAgent{
		store:    store,
		registry: registry,
		executor: executor,
		cwd:      cwd,
		cancels:  make(map[string]context.CancelFunc),
	}
}

// Prompt sends a user message and streams the response as an iterator.
// If the thread has an interrupted turn, it resumes that instead.
//
// The req.Model field should be in "providerId/modelId" format for new turns.
// For resume (empty req), the provider is resolved from the persisted turn state.
func (a *DefaultAgent) Prompt(ctx context.Context, threadID string, req PromptRequest) iter.Seq2[message.MessageChunk, error] {
	// Load session config from the working directory.
	sessionCfg, err := sessionconfig.Load(a.cwd)
	if err != nil {
		log.Printf("agent: warning: session config: %v", err)
		sessionCfg = &sessionconfig.SessionConfig{}
	}

	tools := req.Tools
	if tools == nil {
		tools = sessionCfg.Tools
	}

	// Parse "providerId/modelId" from request (for new turns).
	var providerID, modelID string
	if req.Model != "" {
		ref, err := providers.ParseModelRef(req.Model)
		if err != nil {
			return errorIter(fmt.Errorf("invalid model: %w", err))
		}
		providerID = ref.ProviderID
		modelID = ref.ModelID
	}

	// Look up context window from models.dev data.
	var contextWindow, maxOutputTokens int
	if md := modelsdev.Lookup(providerID, modelID); md != nil {
		contextWindow = md.ContextWindow
		maxOutputTokens = md.MaxOutputTokens
	}

	cfg := thread.TurnConfig{
		ProviderID:      providerID,
		Model:           modelID,
		Reasoning:       req.Reasoning,
		UserParts:       req.UserParts,
		Tools:           tools,
		ContextWindow:   contextWindow,
		MaxOutputTokens: maxOutputTokens,
	}

	return func(yield func(message.MessageChunk, error) bool) {
		// Inject system prompt and user instructions as root messages on new threads.
		if req.LeafID == "" && sessionCfg.SystemPrompt != "" {
			leaf, _ := a.store.FindLeaf(threadID)
			if leaf == "" {
				// 1. System prompt as role: "system".
				sysID := "system-" + generateID()
				if err := a.store.SaveMessage(threadID, thread.StoredMessage{
					ID: sysID,
					Message: message.Message{
						Role:  "system",
						Parts: []message.Part{message.TextPart{Text: sessionCfg.SystemPrompt}},
					},
				}); err != nil {
					yield(nil, fmt.Errorf("save system prompt: %w", err))
					return
				}
				req.LeafID = sysID

				// 2. User instructions as role: "user" with <system-reminder> tags.
				userInstr := sessionconfig.FormatUserInstructions(sessionCfg.UserInstructions)
				if userInstr != "" {
					instrID := "instructions-" + generateID()
					if err := a.store.SaveMessage(threadID, thread.StoredMessage{
						ID:       instrID,
						ParentID: sysID,
						Message: message.Message{
							Role:  "user",
							Parts: []message.Part{message.TextPart{Text: userInstr}},
						},
					}); err != nil {
						yield(nil, fmt.Errorf("save user instructions: %w", err))
						return
					}
					req.LeafID = instrID
				}
			}
		}

		// Create a child context so Cancel(threadID) can stop this prompt.
		promptCtx, cancel := context.WithCancel(ctx)
		defer func() {
			a.mu.Lock()
			delete(a.cancels, threadID)
			a.mu.Unlock()
			cancel()
		}()

		a.mu.Lock()
		a.cancels[threadID] = cancel
		a.mu.Unlock()

		// Check for interrupted turn first.
		state, err := a.store.LoadTurnState(threadID)
		if err != nil {
			yield(nil, err)
			return
		}

		if state != nil {
			log.Printf("agent: resuming interrupted turn %s for thread %s (step %d, phase %s)",
				state.ID, threadID, state.CurrentStep, state.Phase)

			// Resolve provider from persisted turn state.
			provider, resolveErr := a.registry.Get(state.Config.ProviderID)
			if resolveErr != nil {
				yield(nil, fmt.Errorf("resolve provider for resume: %w", resolveErr))
				return
			}

			for chunk, chunkErr := range thread.ResumeTurn(promptCtx, provider, a.executor, a.store, state) {
				if !yield(chunk, chunkErr) {
					return
				}
			}
			return
		}

		// Resolve provider for new turn.
		provider, resolveErr := a.registry.Get(cfg.ProviderID)
		if resolveErr != nil {
			yield(nil, fmt.Errorf("resolve provider: %w", resolveErr))
			return
		}

		// Start new turn.
		for chunk, chunkErr := range thread.RunTurn(promptCtx, provider, a.executor, a.store, threadID, req.LeafID, cfg) {
			if !yield(chunk, chunkErr) {
				return
			}
		}
	}
}

// errorIter returns an iterator that yields a single error.
func errorIter(err error) iter.Seq2[message.MessageChunk, error] {
	return func(yield func(message.MessageChunk, error) bool) {
		yield(nil, err)
	}
}

// Cancel cancels the active prompt for a thread.
func (a *DefaultAgent) Cancel(threadID string) bool {
	a.mu.Lock()
	cancel, ok := a.cancels[threadID]
	a.mu.Unlock()
	if ok {
		cancel()
		return true
	}
	return false
}

// Messages returns the conversation history as UI-projected JSON.
func (a *DefaultAgent) Messages(threadID, leafID string) ([]json.RawMessage, error) {
	if leafID == "" {
		return nil, nil
	}
	history, err := a.store.BuildHistory(threadID, leafID)
	if err != nil {
		return nil, err
	}
	return message.ProjectUIMessages(history)
}

// ListModels returns available models from all registered providers.
// Model IDs are prefixed with "providerId/".
func (a *DefaultAgent) ListModels(ctx context.Context) ([]providers.ModelInfo, error) {
	return a.registry.ListModels(ctx)
}

// ListThreads returns all thread IDs.
func (a *DefaultAgent) ListThreads() ([]string, error) {
	return a.store.ListThreads()
}

// InterruptedThreads returns thread IDs that have unfinished turns.
// Threads paused for AskUserQuestion (waiting_for_answer) are excluded.
func (a *DefaultAgent) InterruptedThreads() ([]string, error) {
	threads, err := a.store.ListThreads()
	if err != nil {
		return nil, err
	}

	var interrupted []string
	for _, threadID := range threads {
		state, err := a.store.LoadTurnState(threadID)
		if err != nil {
			log.Printf("agent: check interrupted turn for %s: %v", threadID, err)
			continue
		}
		if state != nil && state.Phase != thread.PhaseWaitingForAnswer {
			interrupted = append(interrupted, threadID)
		}
	}
	return interrupted, nil
}

// PendingQuestion returns the pending AskUserQuestion for a thread, or nil.
func (a *DefaultAgent) PendingQuestion(threadID string) (*thread.PendingQuestionState, error) {
	state, err := a.store.LoadTurnState(threadID)
	if err != nil {
		return nil, err
	}
	if state == nil || state.Phase != thread.PhaseWaitingForAnswer {
		return nil, nil
	}
	return a.store.LoadQuestion(threadID, state.ID)
}

// SubmitAnswer persists the user's answer for a pending AskUserQuestion.
func (a *DefaultAgent) SubmitAnswer(threadID, toolCallID string, answers map[string]string) error {
	state, err := a.store.LoadTurnState(threadID)
	if err != nil {
		return fmt.Errorf("load turn state: %w", err)
	}
	if state == nil || state.Phase != thread.PhaseWaitingForAnswer {
		return fmt.Errorf("no pending question for thread %s", threadID)
	}

	// Verify the toolCallID matches.
	q, err := a.store.LoadQuestion(threadID, state.ID)
	if err != nil {
		return fmt.Errorf("load question: %w", err)
	}
	if q == nil || q.ToolCallID != toolCallID {
		return fmt.Errorf("question %s not found for thread %s", toolCallID, threadID)
	}

	return a.store.SaveAnswer(threadID, state.ID, thread.QuestionAnswer{
		ToolCallID: toolCallID,
		Answers:    answers,
	})
}
