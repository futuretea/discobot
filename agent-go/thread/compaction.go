package thread

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

// CompactionRecord is the on-disk record of a message history compaction.
// Stored at {threadID}/compaction.json. Non-destructive: the original
// message chain remains intact on disk.
type CompactionRecord struct {
	SummaryText   string    `json:"summaryText"`
	LeafMessageID string    `json:"leafMessageId"` // everything up to and including this message is summarized
	SummaryTokens int       `json:"summaryTokens"`
	Model         string    `json:"model"`
	CreatedAt     time.Time `json:"createdAt"`
}

// tokenBudget holds the calculated token budgets for compaction.
type tokenBudget struct {
	InputLimit        int // max tokens for input (history + tools)
	CompactionTrigger int // 80% of InputLimit — threshold to fire compaction
	SummaryMaxTokens  int // 20% of InputLimit — cap on generated summary
}

// computeBudget calculates token budgets from the model's context window.
func computeBudget(cfg *TurnConfig) tokenBudget {
	cw := cfg.ContextWindow

	// Reserve for output: use MaxOutputTokens if available, else 25%.
	outputReserve := cfg.MaxOutputTokens
	if outputReserve == 0 {
		outputReserve = cw / 4
	}

	// Input budget is everything minus the output reserve.
	inputLimit := cw - outputReserve

	// Compaction fires at 80% of the input budget.
	compactionTrigger := inputLimit * 80 / 100

	// Summary generation gets at most 20% of the input budget.
	summaryMaxTokens := inputLimit * 20 / 100

	return tokenBudget{
		InputLimit:        inputLimit,
		CompactionTrigger: compactionTrigger,
		SummaryMaxTokens:  summaryMaxTokens,
	}
}

// maybeCompact checks if the conversation history approaches the context
// window limit and, if so, summarizes the entire conversation into a compact form.
// Returns the (possibly compacted) history ready for the LLM call.
//
// Non-destructive: never modifies messages on disk. Persists a
// CompactionRecord to {threadDir}/compaction.json for reuse.
func maybeCompact(
	ctx context.Context,
	provider providers.Provider,
	store *Store,
	threadID string,
	_ *TurnState,
	cfg *TurnConfig,
	historyEntries []HistoryEntry,
) ([]message.Message, error) {
	if cfg.ContextWindow == 0 {
		return entriesToMessages(historyEntries), nil
	}

	// Too few messages to compact.
	nonSystemCount := 0
	for _, e := range historyEntries {
		if e.Message.Role != "system" {
			nonSystemCount++
		}
	}
	if nonSystemCount <= 4 {
		return entriesToMessages(historyEntries), nil
	}

	budget := computeBudget(cfg)
	fullHistory := entriesToMessages(historyEntries)

	// Check if existing compaction applies.
	existing, _ := store.LoadCompaction(threadID)
	if existing != nil {
		compacted := applyCompaction(existing, historyEntries)

		tokenCount, err := provider.CountTokens(ctx, providers.CountTokensRequest{
			Model:    cfg.Model,
			Messages: compacted,
			Tools:    cfg.Tools,
		})
		if err != nil {
			return fullHistory, fmt.Errorf("count tokens: %w", err)
		}

		if tokenCount.TotalTokens <= budget.InputLimit {
			return compacted, nil
		}
		// Existing compaction is stale — fall through to re-compact.
	}

	// Count tokens on the full history.
	tokenCount, err := provider.CountTokens(ctx, providers.CountTokensRequest{
		Model:    cfg.Model,
		Messages: fullHistory,
		Tools:    cfg.Tools,
	})
	if err != nil {
		return fullHistory, fmt.Errorf("count tokens: %w", err)
	}

	// Compact at 80% of input budget (CompactionTrigger).
	if tokenCount.TotalTokens <= budget.CompactionTrigger {
		return fullHistory, nil
	}

	// Perform full-conversation compaction.
	return performCompaction(ctx, provider, store, threadID, cfg, historyEntries, budget)
}

// performCompaction summarizes the entire conversation and returns compacted history.
// Unlike a cut-point approach, this summarizes ALL non-system messages.
func performCompaction(
	ctx context.Context,
	provider providers.Provider,
	store *Store,
	threadID string,
	cfg *TurnConfig,
	entries []HistoryEntry,
	budget tokenBudget,
) ([]message.Message, error) {
	fullHistory := entriesToMessages(entries)

	// Find the system message boundary.
	systemEnd := 0
	for systemEnd < len(entries) && entries[systemEnd].Message.Role == "system" {
		systemEnd++
	}

	messagesToSummarize := fullHistory[systemEnd:]
	if len(messagesToSummarize) == 0 {
		return fullHistory, nil
	}

	// Generate the summary of the entire conversation.
	summaryText, err := generateSummary(ctx, provider, cfg, messagesToSummarize, budget)
	if err != nil {
		return fullHistory, fmt.Errorf("generate summary: %w", err)
	}

	summaryMsg := makeSummaryMessage(summaryText)

	// Measure summary token count.
	summaryTokens := 0
	if tc, err := provider.CountTokens(ctx, providers.CountTokensRequest{
		Model:    cfg.Model,
		Messages: []message.Message{summaryMsg},
	}); err == nil {
		summaryTokens = tc.TotalTokens
	}

	// Persist compaction record.
	record := CompactionRecord{
		SummaryText:   summaryText,
		LeafMessageID: entries[len(entries)-1].ID,
		SummaryTokens: summaryTokens,
		Model:         cfg.Model,
		CreatedAt:     time.Now(),
	}
	if err := store.SaveCompaction(threadID, record); err != nil {
		log.Printf("compaction: failed to save record: %v", err)
	}

	// Build compacted history: [system messages] + [summary].
	compacted := make([]message.Message, 0, systemEnd+1)
	compacted = append(compacted, fullHistory[:systemEnd]...)
	compacted = append(compacted, summaryMsg)
	return compacted, nil
}

const summarySystemPrompt = `Your task is to create a detailed summary of the conversation so far, paying close attention to the user's explicit requests and your previous actions. This summary will replace the conversation history, so it must be thorough enough to continue development work without losing context.

Before providing your final summary, wrap your analysis in <analysis> tags to organize your thoughts and ensure completeness. In your analysis:

1. Chronologically walk through each message in the conversation
2. Identify all key decisions and their rationale
3. Note important technical details, configurations, and code snippets
4. Track all file changes, creations, and modifications
5. List any unresolved issues or pending tasks
6. Capture the current state of the work

Then provide your summary with the following sections:

## 1. Primary Request and Intent
Describe the user's original request and core intent. What are they trying to accomplish?

## 2. Key Technical Concepts
List important technical concepts, technologies, frameworks, and patterns discussed or used.

## 3. Files and Code Sections
List ALL files that were read, created, modified, or discussed. For each file include:
- The file path
- What was done to it (read, created, modified)
- Key content or changes (include relevant code snippets for context)

## 4. Errors and Fixes
Document any errors encountered:
- What the error was
- What caused it
- How it was fixed (or if it is still unresolved)

## 5. Problem Solving
Describe the problem-solving approach:
- What approaches were considered
- What was tried
- What worked and what did not

## 6. All User Messages
Reproduce ALL user messages (not tool results) to preserve their exact requests, preferences, and feedback. This is critical for maintaining context about changing user intent.

## 7. Pending Tasks
List any tasks that are:
- Currently in progress
- Planned but not yet started
- Blocked by something

## 8. Current Work
Describe the most recent work in detail, including:
- What files are being worked on
- What specific changes are being made (include code snippets)
- What the immediate next steps are

## 9. Optional Next Step
If there is a clear next step, describe it with enough detail to continue without losing context. Include direct quotes from the user where relevant to prevent task drift.

Do NOT start on tangential requests or old requests that were already completed without confirming with the user first.`

// generateSummary calls the LLM to summarize the conversation.
func generateSummary(
	ctx context.Context,
	provider providers.Provider,
	cfg *TurnConfig,
	messagesToSummarize []message.Message,
	budget tokenBudget,
) (string, error) {
	transcript := formatTranscript(messagesToSummarize)

	summaryMessages := []message.Message{
		{Role: "system", Parts: []message.Part{message.TextPart{Text: summarySystemPrompt}}},
		{Role: "user", Parts: []message.Part{message.TextPart{Text: "Here is the conversation to summarize:\n\n" + transcript}}},
	}

	maxTokens := budget.SummaryMaxTokens
	req := providers.CompleteRequest{
		Model:     cfg.Model,
		Messages:  summaryMessages,
		MaxTokens: &maxTokens,
	}

	acc := message.NewChunkAccumulator()
	for chunk, chunkErr := range provider.Complete(ctx, req) {
		if chunkErr != nil {
			acc.Close()
			return "", fmt.Errorf("summary completion: %w", chunkErr)
		}
		acc.Push(chunk)
	}
	acc.Close()

	result := acc.Message()
	var sb strings.Builder
	for _, part := range result.Parts {
		if tp, ok := part.(message.TextPart); ok {
			sb.WriteString(tp.Text)
		}
	}
	text := sb.String()
	if text == "" {
		return "", fmt.Errorf("empty summary generated")
	}
	return text, nil
}

// formatTranscript converts messages into a human-readable transcript.
func formatTranscript(messages []message.Message) string {
	var sb strings.Builder
	for _, msg := range messages {
		sb.WriteString(fmt.Sprintf("[%s]: ", msg.Role))
		for _, part := range msg.Parts {
			switch p := part.(type) {
			case message.TextPart:
				sb.WriteString(p.Text)
			case message.ToolCallPart:
				sb.WriteString(fmt.Sprintf("<tool_call name=%q id=%q>", p.ToolName, p.ToolCallID))
			case message.ToolResultPart:
				output := toolResultOutputToString(p.Output)
				if len(output) > 500 {
					output = output[:500] + "... [truncated]"
				}
				sb.WriteString(fmt.Sprintf("<tool_result name=%q>%s</tool_result>", p.ToolName, output))
			case message.ReasoningPart:
				// Skip reasoning in transcript.
			default:
				sb.WriteString(fmt.Sprintf("<%T>", p))
			}
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// toolResultOutputToString converts a ToolResultOutput to a string representation.
func toolResultOutputToString(output message.ToolResultOutput) string {
	if output == nil {
		return ""
	}
	switch o := output.(type) {
	case message.TextOutput:
		return o.Value
	case message.ErrorTextOutput:
		return "ERROR: " + o.Value
	case message.ExecutionDeniedOutput:
		return "DENIED: " + o.Reason
	case message.JSONOutput:
		return string(o.Value)
	case message.ErrorJSONOutput:
		return "ERROR: " + string(o.Value)
	case message.ContentOutput:
		var parts []string
		for _, item := range o.Value {
			if t, ok := item.(message.ContentTextItem); ok {
				parts = append(parts, t.Text)
			}
		}
		return strings.Join(parts, "\n")
	default:
		return fmt.Sprintf("<%T>", o)
	}
}

// applyCompaction builds a compacted history from an existing record.
// Returns [system messages] + [summary] + [any messages added after the compaction leaf].
func applyCompaction(record *CompactionRecord, entries []HistoryEntry) []message.Message {
	systemEnd := 0
	for systemEnd < len(entries) && entries[systemEnd].Message.Role == "system" {
		systemEnd++
	}

	// Find the compaction leaf in the entry list.
	leafIndex := -1
	for i := systemEnd; i < len(entries); i++ {
		if entries[i].ID == record.LeafMessageID {
			leafIndex = i
			break
		}
	}

	if leafIndex < 0 {
		// Compaction leaf not found — compaction is stale, return full history.
		return entriesToMessages(entries)
	}

	summaryMsg := makeSummaryMessage(record.SummaryText)

	// Build: [system messages] + [summary] + [messages after the compaction leaf].
	afterLeafStart := leafIndex + 1
	compacted := make([]message.Message, 0, systemEnd+1+(len(entries)-afterLeafStart))
	for i := 0; i < systemEnd; i++ {
		compacted = append(compacted, entries[i].Message)
	}
	compacted = append(compacted, summaryMsg)
	for i := afterLeafStart; i < len(entries); i++ {
		compacted = append(compacted, entries[i].Message)
	}
	return compacted
}

// makeSummaryMessage creates the summary message to insert into history.
func makeSummaryMessage(summaryText string) message.Message {
	return message.Message{
		Role: "user",
		Parts: []message.Part{
			message.TextPart{
				Text: "<conversation_summary>\n" + summaryText + "\n</conversation_summary>\n\nThe above is a summary of our earlier conversation. Continue from where we left off.",
			},
		},
		Metadata: json.RawMessage(`{"compaction":true}`),
	}
}

// entriesToMessages extracts just the messages from history entries.
func entriesToMessages(entries []HistoryEntry) []message.Message {
	msgs := make([]message.Message, len(entries))
	for i, e := range entries {
		msgs[i] = e.Message
	}
	return msgs
}
