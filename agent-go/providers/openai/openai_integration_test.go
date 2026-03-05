//go:build integration

package openai

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
)

// readAPIKey reads the OpenAI API key from OPENAI_API_KEY env var or key.txt.
func readAPIKey(t *testing.T) string {
	t.Helper()
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	// Try reading from key.txt at repo root.
	for _, path := range []string{"../../../key.txt", "../../../../key.txt"} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "export OPENAI_API_KEY=") {
				key := strings.TrimPrefix(line, "export OPENAI_API_KEY=")
				key = strings.Trim(key, `"'`)
				if key != "" {
					return key
				}
			}
		}
	}
	t.Skip("OPENAI_API_KEY not set and key.txt not found")
	return ""
}

// TestCompleteToolCallDeltasHaveCallID tests that all ToolInputDeltaChunk and
// ToolInputEndChunk values produced by a real OpenAI Responses API call carry a
// non-empty ToolCallID.
//
// Before the fix, the parser was stateless: it tried to parse call_id directly
// from response.function_call_arguments.delta events, but the real API always
// returns an empty string there. Only item_id is populated in those events, and
// the actual call_id is available earlier in response.output_item.added.
//
// Run with: go test -tags integration -run TestCompleteToolCallDeltasHaveCallID ./providers/openai/
func TestCompleteToolCallDeltasHaveCallID(t *testing.T) {
	apiKey := readAPIKey(t)

	p, err := New(providers.Config{"api_key": apiKey})
	if err != nil {
		t.Fatal(err)
	}

	req := providers.CompleteRequest{
		Model: providers.ModelRef{ProviderID: "openai", ModelID: "gpt-4o"},
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Use the echo tool to echo back the text: hello world"},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "echo",
				Description: "Echo back the provided text",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"text":{"type":"string","description":"The text to echo"}},"required":["text"]}`),
			},
		},
	}

	var (
		sawToolInputStart bool
		deltaIDs          []string
		endIDs            []string
	)
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatalf("unexpected error from Complete: %v", err)
		}
		switch c := chunk.(type) {
		case message.ToolInputStartChunk:
			sawToolInputStart = true
			if c.ToolCallID == "" {
				t.Error("ToolInputStartChunk has empty ToolCallID")
			}
		case message.ToolInputDeltaChunk:
			deltaIDs = append(deltaIDs, c.ToolCallID)
		case message.ToolInputEndChunk:
			endIDs = append(endIDs, c.ToolCallID)
		}
	}

	if !sawToolInputStart {
		t.Fatal("expected at least one tool call, got none (model may have responded without tool use)")
	}

	for i, id := range deltaIDs {
		if id == "" {
			t.Errorf("ToolInputDeltaChunk[%d] has empty ToolCallID (bug: item_id→call_id lookup missing)", i)
		}
	}
	for i, id := range endIDs {
		if id == "" {
			t.Errorf("ToolInputEndChunk[%d] has empty ToolCallID (bug: item_id→call_id lookup missing)", i)
		}
	}

	if len(deltaIDs) == 0 {
		t.Error("expected at least one ToolInputDeltaChunk")
	}
}
