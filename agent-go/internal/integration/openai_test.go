package integration

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	_ "github.com/obot-platform/discobot/agent-go/providers/openai"
)

const testModel = "gpt-4.1-nano"

func openaiProvider(t *testing.T) providers.Provider {
	t.Helper()
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	p, err := providers.New("openai", providers.Config{"api_key": apiKey})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestOpenAI_SimpleTextCompletion(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "Reply with only the word 'pong'. Nothing else."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "ping"},
			}},
		},
	}

	var gotText strings.Builder
	var gotStreamStart, gotFinish bool
	var usage message.Usage

	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		switch c := chunk.(type) {
		case message.StreamStartChunk:
			gotStreamStart = true
		case message.TextDeltaChunk:
			gotText.WriteString(c.Delta)
		case message.FinishChunk:
			gotFinish = true
			usage = c.Usage
		}
	}

	if !gotStreamStart {
		t.Error("expected StreamStartChunk")
	}
	if !gotFinish {
		t.Error("expected FinishChunk")
	}
	text := strings.TrimSpace(strings.ToLower(gotText.String()))
	if !strings.Contains(text, "pong") {
		t.Errorf("expected response to contain 'pong', got %q", gotText.String())
	}
	if usage.InputTokens.Total == 0 {
		t.Error("expected non-zero input tokens")
	}
	if usage.OutputTokens.Total == 0 {
		t.Error("expected non-zero output tokens")
	}
}

func TestOpenAI_ToolCall(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You must use the get_weather tool to answer weather questions. Do not answer without calling the tool."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is the weather in Paris?"},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "get_weather",
				Description: "Get the current weather for a location",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"location":{"type":"string","description":"City name"}},"required":["location"],"additionalProperties":false}`),
			},
		},
	}

	var toolCalls []toolCallCapture
	var gotFinish bool
	var finishReason string
	var currentCallID string
	var currentName string
	var currentArgs strings.Builder

	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		switch c := chunk.(type) {
		case message.ToolInputStartChunk:
			currentCallID = c.ToolCallID
			currentName = c.ToolName
			currentArgs.Reset()
		case message.ToolInputDeltaChunk:
			currentArgs.WriteString(c.InputTextDelta)
		case message.ToolInputEndChunk:
			toolCalls = append(toolCalls, toolCallCapture{
				callID:    currentCallID,
				toolName:  currentName,
				arguments: currentArgs.String(),
			})
		case message.FinishChunk:
			gotFinish = true
			finishReason = c.FinishReason.Unified
		}
	}

	if !gotFinish {
		t.Fatal("expected FinishChunk")
	}
	if finishReason != "tool-calls" {
		t.Errorf("expected finish reason 'tool-calls', got %q", finishReason)
	}
	if len(toolCalls) == 0 {
		t.Fatal("expected at least one tool call")
	}

	tc := toolCalls[0]
	if tc.toolName != "get_weather" {
		t.Errorf("expected tool name 'get_weather', got %q", tc.toolName)
	}
	if tc.callID == "" {
		t.Error("expected non-empty call ID")
	}

	var args map[string]any
	if err := json.Unmarshal([]byte(tc.arguments), &args); err != nil {
		t.Fatalf("expected valid JSON arguments, got %q: %v", tc.arguments, err)
	}
	loc, _ := args["location"].(string)
	if !strings.Contains(strings.ToLower(loc), "paris") {
		t.Errorf("expected location to contain 'paris', got %q", loc)
	}
}

func TestOpenAI_ToolCallRoundTrip(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	// Turn 1: model calls the tool.
	turn1Req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You must use the get_temperature tool. After receiving the result, state the temperature."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is the temperature in London?"},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "get_temperature",
				Description: "Get current temperature for a city",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
			},
		},
	}

	var callID string
	for chunk, err := range p.Complete(context.Background(), turn1Req) {
		if err != nil {
			t.Fatal(err)
		}
		if c, ok := chunk.(message.ToolInputStartChunk); ok {
			callID = c.ToolCallID
		}
	}
	if callID == "" {
		t.Fatal("expected a tool call in turn 1")
	}

	// Turn 2: provide tool result, expect text response.
	turn2Req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You must use the get_temperature tool. After receiving the result, state the temperature."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is the temperature in London?"},
			}},
			{Role: "assistant", Parts: []message.Part{
				message.ToolCallPart{
					ToolCallID: callID,
					ToolName:   "get_temperature",
					Input:      json.RawMessage(`{"city":"London"}`),
				},
			}},
			{Role: "tool", Parts: []message.Part{
				message.ToolResultPart{
					ToolCallID: callID,
					ToolName:   "get_temperature",
					Output:     message.TextOutput{Value: "18 degrees Celsius"},
				},
			}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "get_temperature",
				Description: "Get current temperature for a city",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"],"additionalProperties":false}`),
			},
		},
	}

	var text strings.Builder
	var finishReason string
	for chunk, err := range p.Complete(context.Background(), turn2Req) {
		if err != nil {
			t.Fatal(err)
		}
		switch c := chunk.(type) {
		case message.TextDeltaChunk:
			text.WriteString(c.Delta)
		case message.FinishChunk:
			finishReason = c.FinishReason.Unified
		}
	}

	if finishReason != "stop" {
		t.Errorf("expected finish reason 'stop', got %q", finishReason)
	}
	response := strings.ToLower(text.String())
	if !strings.Contains(response, "18") {
		t.Errorf("expected response to mention '18', got %q", text.String())
	}
}

func TestOpenAI_MultiTurnConversation(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "system", Parts: []message.Part{
				message.TextPart{Text: "You are a helpful assistant. Keep responses very brief."},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "My name is Alice."},
			}},
			{Role: "assistant", Parts: []message.Part{
				message.TextPart{Text: "Hello Alice! How can I help you?"},
			}},
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is my name?"},
			}},
		},
	}

	var text strings.Builder
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		if c, ok := chunk.(message.TextDeltaChunk); ok {
			text.WriteString(c.Delta)
		}
	}

	if !strings.Contains(strings.ToLower(text.String()), "alice") {
		t.Errorf("expected response to contain 'alice', got %q", text.String())
	}
}

func TestOpenAI_CountTokens(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	resp, err := p.CountTokens(context.Background(), providers.CountTokensRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Hello, world!"},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.TotalTokens == 0 {
		t.Error("expected non-zero token count")
	}
	if resp.TotalTokens > 100 {
		t.Errorf("token count seems too high: %d", resp.TotalTokens)
	}
}

func TestOpenAI_CountTokensWithTools(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	withoutTools, err := p.CountTokens(context.Background(), providers.CountTokensRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Hello"}}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	withTools, err := p.CountTokens(context.Background(), providers.CountTokensRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{message.TextPart{Text: "Hello"}}},
		},
		Tools: []providers.ToolDefinition{
			{
				Name:        "search",
				Description: "Search the web for information",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"query":{"type":"string","description":"The search query"}},"required":["query"],"additionalProperties":false}`),
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if withTools.TotalTokens <= withoutTools.TotalTokens {
		t.Errorf("expected tool definitions to add tokens: without=%d, with=%d",
			withoutTools.TotalTokens, withTools.TotalTokens)
	}
}

func TestOpenAI_StreamLifecycle(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Say 'hello' and nothing else."},
			}},
		},
	}

	var events []string
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		switch chunk.(type) {
		case message.StreamStartChunk:
			events = append(events, "stream-start")
		case message.ResponseMetadataChunk:
			events = append(events, "response-metadata")
		case message.TextStartChunk:
			events = append(events, "text-start")
		case message.TextDeltaChunk:
			if len(events) == 0 || events[len(events)-1] != "text-delta" {
				events = append(events, "text-delta")
			}
		case message.TextEndChunk:
			events = append(events, "text-end")
		case message.FinishChunk:
			events = append(events, "finish")
		}
	}

	expected := []string{
		"stream-start",
		"response-metadata",
		"text-start",
		"text-delta",
		"text-end",
		"finish",
	}
	if len(events) != len(expected) {
		t.Fatalf("expected event sequence %v, got %v", expected, events)
	}
	for i, e := range expected {
		if events[i] != e {
			t.Errorf("event[%d]: expected %q, got %q", i, e, events[i])
		}
	}
}

func TestOpenAI_ContextCancellation(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := providers.CompleteRequest{
		Model: testModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Write a very long essay about the history of computing."},
			}},
		},
	}

	chunkCount := 0
	for chunk, err := range p.Complete(ctx, req) {
		if err != nil {
			// Context cancellation may surface as an error — that's expected.
			return
		}
		_ = chunk
		chunkCount++
		if chunkCount >= 3 {
			cancel()
		}
	}
}

const reasoningModel = "o4-mini"

func TestOpenAI_ReasoningCompletion(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: reasoningModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "What is 2+2? Reply with just the number."},
			}},
		},
		Reasoning: "enabled",
	}

	var gotReasoningStart, gotReasoningEnd bool
	var reasoningText strings.Builder
	var answerText strings.Builder
	var usage message.Usage

	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		switch c := chunk.(type) {
		case message.ReasoningStartChunk:
			gotReasoningStart = true
		case message.ReasoningDeltaChunk:
			reasoningText.WriteString(c.Delta)
		case message.ReasoningEndChunk:
			gotReasoningEnd = true
			// Verify encrypted_content is captured in ProviderMetadata.
			if len(c.ProviderMetadata) == 0 {
				t.Error("expected ProviderMetadata with encrypted_content on ReasoningEndChunk")
			}
		case message.TextDeltaChunk:
			answerText.WriteString(c.Delta)
		case message.FinishChunk:
			usage = c.Usage
		}
	}

	if !gotReasoningStart {
		t.Error("expected ReasoningStartChunk")
	}
	if !gotReasoningEnd {
		t.Error("expected ReasoningEndChunk")
	}
	if !strings.Contains(answerText.String(), "4") {
		t.Errorf("expected answer to contain '4', got %q", answerText.String())
	}
	// Note: usage.OutputTokens.Reasoning may be 0 for trivial prompts
	// even with reasoning enabled — this is an API behavior, not a bug.
	if usage.OutputTokens.Total == 0 {
		t.Error("expected non-zero output tokens in usage")
	}
}

func TestOpenAI_ReasoningMultiTurn(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	// Turn 1: ask a question with reasoning enabled.
	turn1Req := providers.CompleteRequest{
		Model: reasoningModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Remember the number 73. What is 73 * 2? Reply with just the number."},
			}},
		},
		Reasoning: "enabled",
	}

	// Accumulate turn 1 using ChunkAccumulator.
	acc := message.NewChunkAccumulator()
	for chunk, err := range p.Complete(context.Background(), turn1Req) {
		if err != nil {
			t.Fatal(err)
		}
		acc.Push(chunk)
	}
	acc.Close()
	turn1Msg := acc.Message()

	// Verify turn 1 has a reasoning part with ProviderMetadata.
	var hasReasoning bool
	var hasProviderMetadata bool
	for _, part := range turn1Msg.Parts {
		if rp, ok := part.(message.ReasoningPart); ok {
			hasReasoning = true
			hasProviderMetadata = len(rp.ProviderMetadata) > 0
		}
	}
	if !hasReasoning {
		t.Fatal("expected reasoning part in turn 1 response")
	}
	if !hasProviderMetadata {
		t.Error("expected ProviderMetadata on reasoning part (encrypted_content)")
	}

	// Turn 2: send reasoning + answer from turn 1 back, ask a follow-up.
	// The reasoning context should help the model maintain continuity.
	turn2Req := providers.CompleteRequest{
		Model: reasoningModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Remember the number 73. What is 73 * 2? Reply with just the number."},
			}},
			turn1Msg, // assistant message with reasoning + text
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Now add 1 to that result. Reply with just the number."},
			}},
		},
		Reasoning: "enabled",
	}

	var turn2Text strings.Builder
	for chunk, err := range p.Complete(context.Background(), turn2Req) {
		if err != nil {
			t.Fatal(err)
		}
		if c, ok := chunk.(message.TextDeltaChunk); ok {
			turn2Text.WriteString(c.Delta)
		}
	}

	// 73 * 2 = 146, 146 + 1 = 147
	response := strings.TrimSpace(turn2Text.String())
	if !strings.Contains(response, "147") {
		t.Errorf("expected response to contain '147', got %q", response)
	}
}

func TestOpenAI_ReasoningStreamLifecycle(t *testing.T) {
	t.Parallel()
	p := openaiProvider(t)

	req := providers.CompleteRequest{
		Model: reasoningModel,
		Messages: []message.Message{
			{Role: "user", Parts: []message.Part{
				message.TextPart{Text: "Say 'yes'. One word."},
			}},
		},
		Reasoning: "enabled",
	}

	var events []string
	for chunk, err := range p.Complete(context.Background(), req) {
		if err != nil {
			t.Fatal(err)
		}
		switch chunk.(type) {
		case message.StreamStartChunk:
			events = append(events, "stream-start")
		case message.ResponseMetadataChunk:
			events = append(events, "response-metadata")
		case message.ReasoningStartChunk:
			events = append(events, "reasoning-start")
		case message.ReasoningDeltaChunk:
			if len(events) == 0 || events[len(events)-1] != "reasoning-delta" {
				events = append(events, "reasoning-delta")
			}
		case message.ReasoningEndChunk:
			events = append(events, "reasoning-end")
		case message.TextStartChunk:
			events = append(events, "text-start")
		case message.TextDeltaChunk:
			if len(events) == 0 || events[len(events)-1] != "text-delta" {
				events = append(events, "text-delta")
			}
		case message.TextEndChunk:
			events = append(events, "text-end")
		case message.FinishChunk:
			events = append(events, "finish")
		}
	}

	// Verify required events are present in correct order.
	// reasoning-delta is optional (summary can be empty for trivial prompts).
	required := []string{
		"stream-start",
		"response-metadata",
		"reasoning-start",
		"reasoning-end",
		"text-start",
		"text-delta",
		"text-end",
		"finish",
	}
	ri := 0
	for _, e := range events {
		if ri < len(required) && e == required[ri] {
			ri++
		}
	}
	if ri != len(required) {
		t.Errorf("missing required events; want %v in order, got %v", required, events)
	}
}

type toolCallCapture struct {
	callID    string
	toolName  string
	arguments string
}
