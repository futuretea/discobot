// Package agentimpl provides the default Agent implementation.
// The Agent interface and PromptRequest type live in the agent package;
// this package contains the concrete DefaultAgent that implements them.
package agentimpl

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"strings"
	"sync"

	"github.com/obot-platform/discobot/agent-go/agent"
	"github.com/obot-platform/discobot/agent-go/mcp"
	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/providers/modelsdev"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// mcpConfig holds MCP OAuth and connectivity settings.
type mcpConfig struct {
	redirectBase      string // base URL for OAuth callbacks (MCPOAuthRedirectBase)
	sessionID         string // session ID used in OAuth redirect URL
	discobotServerURL string // Discobot server URL for token persistence
	projectID         string // project ID for token persistence
}

// DefaultAgent is the built-in Agent implementation that uses the thread
// package for file-based persistence and the crash-resilient step loop.
type DefaultAgent struct {
	store    *thread.Store
	registry *providers.ProviderRegistry
	executor thread.ToolExecutor
	cwd      string // working directory for session config discovery
	mcpCfg   mcpConfig

	mu      sync.Mutex
	cancels map[string]context.CancelFunc

	mcpMu      sync.Mutex
	mcpMgr     *mcp.Manager                   // nil until first Prompt with MCP servers
	mcpServers []sessionconfig.MCPServerConfig // config the manager was initialized with
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
	mcpCfg mcpConfig,
) *DefaultAgent {
	return &DefaultAgent{
		store:    store,
		registry: registry,
		executor: executor,
		cwd:      cwd,
		mcpCfg:   mcpCfg,
		cancels:  make(map[string]context.CancelFunc),
	}
}

// NewMCPConfig creates an mcpConfig from individual configuration values.
func NewMCPConfig(redirectBase, sessionID, discobotServerURL, projectID string) mcpConfig {
	return mcpConfig{
		redirectBase:      redirectBase,
		sessionID:         sessionID,
		discobotServerURL: discobotServerURL,
		projectID:         projectID,
	}
}

// MCPManager returns the lazily-initialized MCP manager, or nil if MCP has
// not been started yet (no Prompt with MCP servers has been called).
func (a *DefaultAgent) MCPManager() *mcp.Manager {
	a.mcpMu.Lock()
	defer a.mcpMu.Unlock()
	return a.mcpMgr
}

// Close shuts down the MCP manager (closes all server connections).
func (a *DefaultAgent) Close() {
	a.mcpMu.Lock()
	mgr := a.mcpMgr
	a.mcpMu.Unlock()
	if mgr != nil {
		mgr.Close()
	}
}

// Prompt sends a user message and streams the response as an iterator.
// If the thread has an interrupted turn, it resumes that instead.
//
// If req.SubagentType is set, the named SubAgentConfig from session config is
// used to restrict tools, override the model, and set the system prompt.
//
// The req.Model field should be in "providerId/modelId" format for new turns.
// For resume (empty req), the provider is resolved from the persisted turn state.
func (a *DefaultAgent) Prompt(ctx context.Context, threadID string, req agent.PromptRequest) iter.Seq2[message.MessageChunk, error] {
	// Load session config from the working directory.
	sessionCfg, err := sessionconfig.Load(a.cwd)
	if err != nil {
		log.Printf("agent: warning: session config: %v", err)
		sessionCfg = &sessionconfig.SessionConfig{}
	}

	// Look up sub-agent config if a subagent type is specified.
	var subAgentCfg *sessionconfig.SubAgentConfig
	if req.SubagentType != "" {
		for i := range sessionCfg.SubAgents {
			if sessionCfg.SubAgents[i].Name == req.SubagentType {
				subAgentCfg = &sessionCfg.SubAgents[i]
				break
			}
		}
		if subAgentCfg == nil {
			log.Printf("agent: sub-agent type %q not found in session config", req.SubagentType)
		}
	}

	// Determine tool set: sub-agent restrictions take priority over request override.
	tools := req.Tools
	if tools == nil {
		tools = sessionCfg.Tools
	}
	if subAgentCfg != nil {
		tools = filterTools(tools, subAgentCfg.AllowedTools, subAgentCfg.DisallowedTools)
	}

	// Determine model: sub-agent model overrides request model.
	model := req.Model
	if subAgentCfg != nil && subAgentCfg.Model != "" {
		model = subAgentCfg.Model
	}

	// Determine system prompt: sub-agent prompt overrides session default.
	systemPrompt := sessionCfg.SystemPrompt
	if subAgentCfg != nil && subAgentCfg.Prompt != "" {
		systemPrompt = subAgentCfg.Prompt
	}

	// Init or reload MCP manager whenever the server list changes.
	var mcpMgr *mcp.Manager
	a.mcpMu.Lock()
	if !mcpServersEqual(a.mcpServers, sessionCfg.MCPServers) {
		if a.mcpMgr != nil {
			log.Printf("agent: .mcp.json changed, reloading MCP manager")
			a.mcpMgr.Close()
			a.mcpMgr = nil
		}
		if len(sessionCfg.MCPServers) > 0 {
			callback := mcp.MakeTokenCallback(a.mcpCfg.discobotServerURL, a.mcpCfg.projectID)
			a.mcpMgr = mcp.NewManager(callback)
			a.mcpMgr.Connect(ctx, sessionCfg.MCPServers,
				a.mcpCfg.redirectBase, a.mcpCfg.sessionID)
		}
		a.mcpServers = sessionCfg.MCPServers
	}
	mcpMgr = a.mcpMgr
	a.mcpMu.Unlock()

	if mcpMgr != nil {
		// Augment tool list with all currently-connected MCP tools.
		tools = append(tools, mcpMgr.Tools()...)
	}

	// Parse "providerId/modelId" from request (for new turns).
	var providerID, modelID string
	if model != "" {
		ref, err := providers.ParseModelRef(model)
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

	// MaxSteps: take the stricter of the request value and the sub-agent config value.
	maxSteps := req.MaxTurns
	if subAgentCfg != nil && subAgentCfg.MaxTurns > 0 {
		if maxSteps == 0 || subAgentCfg.MaxTurns < maxSteps {
			maxSteps = subAgentCfg.MaxTurns
		}
	}

	cfg := thread.TurnConfig{
		ProviderID:      providerID,
		Model:           modelID,
		Reasoning:       req.Reasoning,
		UserParts:       req.UserParts,
		Tools:           tools,
		ContextWindow:   contextWindow,
		MaxOutputTokens: maxOutputTokens,
		MaxSteps:        maxSteps,
	}

	return func(yield func(message.MessageChunk, error) bool) {
		// Inject system prompt and user instructions as root messages on new threads.
		if req.LeafID == "" && systemPrompt != "" {
			leaf, _ := a.store.FindLeaf(threadID)
			if leaf == "" {
				// 1. System prompt as role: "system".
				sysID := "system-" + agent.GenerateID()
				if err := a.store.SaveMessage(threadID, thread.StoredMessage{
					ID: sysID,
					Message: message.Message{
						Role:  "system",
						Parts: []message.Part{message.TextPart{Text: systemPrompt}},
					},
				}); err != nil {
					yield(nil, fmt.Errorf("save system prompt: %w", err))
					return
				}
				req.LeafID = sysID

				// 2. User instructions as role: "user" with <system-reminder> tags.
				// Only inject when using the default session config (not a sub-agent prompt).
				if subAgentCfg == nil {
					userInstr := sessionconfig.FormatUserInstructions(sessionCfg.UserInstructions)
					if userInstr != "" {
						instrID := "instructions-" + agent.GenerateID()
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

		// Wrap the executor with MCP routing if the MCP manager is active.
		executor := thread.ToolExecutor(a.executor)
		if mcpMgr != nil {
			executor = mcp.NewExecutor(a.executor, mcpMgr)
		}

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

			for chunk, chunkErr := range thread.ResumeTurn(promptCtx, provider, executor, a.store, state) {
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
		for chunk, chunkErr := range thread.RunTurn(promptCtx, provider, executor, a.store, threadID, req.LeafID, cfg) {
			if !yield(chunk, chunkErr) {
				return
			}
		}
	}
}

// mcpServersEqual reports whether two MCP server config slices are identical.
// Uses JSON marshaling so that any field change (URL, auth, args, etc.) triggers a reload.
func mcpServersEqual(a, b []sessionconfig.MCPServerConfig) bool {
	if len(a) != len(b) {
		return false
	}
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
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

// FinalResponse returns the last assistant text from a completed thread turn.
// Returns empty string if the thread has no content or if a turn is in progress.
func (a *DefaultAgent) FinalResponse(threadID string) (string, error) {
	// If a turn is interrupted, the thread isn't done yet.
	state, err := a.store.LoadTurnState(threadID)
	if err != nil {
		return "", fmt.Errorf("load turn state: %w", err)
	}
	if state != nil {
		return "", nil
	}

	leafID, err := a.store.FindLeaf(threadID)
	if err != nil {
		return "", fmt.Errorf("find leaf: %w", err)
	}
	if leafID == "" {
		return "", nil
	}

	history, err := a.store.BuildHistory(threadID, leafID)
	if err != nil {
		return "", fmt.Errorf("build history: %w", err)
	}

	// Return the last assistant message's text content.
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role == "assistant" {
			var sb strings.Builder
			for _, p := range history[i].Parts {
				if tp, ok := p.(message.TextPart); ok {
					sb.WriteString(tp.Text)
				}
			}
			return sb.String(), nil
		}
	}
	return "", nil
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

// filterTools applies allowed/disallowed lists to a tool set.
// If allowedTools is non-empty, only listed tools are kept.
// disallowedTools always removes the named tools.
func filterTools(tools []providers.ToolDefinition, allowedTools, disallowedTools []string) []providers.ToolDefinition {
	if len(allowedTools) == 0 && len(disallowedTools) == 0 {
		return tools
	}

	allowed := make(map[string]bool, len(allowedTools))
	for _, t := range allowedTools {
		allowed[t] = true
	}

	disallowed := make(map[string]bool, len(disallowedTools))
	for _, t := range disallowedTools {
		disallowed[t] = true
	}

	filtered := make([]providers.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		if len(allowedTools) > 0 && !allowed[t.Name] {
			continue
		}
		if disallowed[t.Name] {
			continue
		}
		filtered = append(filtered, t)
	}
	return filtered
}
