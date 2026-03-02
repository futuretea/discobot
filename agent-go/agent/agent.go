package agent

import (
	"context"
	"encoding/json"
	"iter"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/thread"
)

// Agent abstracts the underlying agent implementation.
// It mirrors the TypeScript Agent interface from agent-api,
// adapted to Go patterns using iter.Seq2 for streaming.
//
// All methods are thread-safe. The threadID parameter maps to
// the TS sessionId concept (a conversation thread).
type Agent interface {
	// Prompt sends a user message and streams the response as an iterator.
	// The iterator yields MessageChunks until the turn is complete.
	//
	// If the thread has an interrupted turn from a previous crash,
	// the interrupted turn is resumed instead of starting a new one.
	//
	// Only one Prompt may be active per threadID. Calling Prompt while
	// another is active for the same thread returns an error.
	Prompt(ctx context.Context, threadID string, req PromptRequest) iter.Seq2[message.MessageChunk, error]

	// Cancel cancels the active prompt for a thread.
	// Returns true if a prompt was active and cancelled, false otherwise.
	Cancel(threadID string) bool

	// Messages returns the conversation history for a thread as
	// UI-projected JSON messages.
	Messages(threadID, leafID string) ([]json.RawMessage, error)

	// ListModels returns available models from the underlying provider.
	ListModels(ctx context.Context) ([]providers.ModelInfo, error)

	// ListThreads returns all thread IDs.
	ListThreads() ([]string, error)

	// InterruptedThreads returns thread IDs that have unfinished turns
	// from a previous crash. Used for startup recovery.
	// Threads paused for AskUserQuestion are NOT included.
	InterruptedThreads() ([]string, error)

	// PendingQuestion returns the pending AskUserQuestion for a thread,
	// or nil if no question is pending. Used by GET /chat/question.
	PendingQuestion(threadID string) (*thread.PendingQuestionState, error)

	// SubmitAnswer persists the user's answer for a pending question.
	// The turn is resumed by calling Prompt again (which detects the
	// waiting_for_answer state and resumes with the answer).
	SubmitAnswer(threadID, toolCallID string, answers map[string]string) error
}

// PromptRequest holds the parameters for a Prompt call.
type PromptRequest struct {
	// LeafID is the parent message to branch from. Empty for new conversations.
	LeafID string

	// UserParts is the user message content.
	UserParts []message.Part

	// Model overrides the default model for this request.
	Model string

	// Reasoning controls extended thinking ("enabled" or "").
	Reasoning string

	// Mode is the permission mode ("plan" or "").
	Mode string

	// Tools overrides the default tool set. Nil means use agent defaults.
	Tools []providers.ToolDefinition
}
