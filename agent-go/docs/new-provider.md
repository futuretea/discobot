# Implementing a New LLM Provider

## File Structure

Create `providers/<id>/<id>.go` and `providers/<id>/<id>_test.go`. The `<id>` must match the provider's [models.dev](https://models.dev) identifier (e.g., `anthropic`, `google`, `mistral`).

## Step 1: Implement the Provider Interface

```go
// providers/<id>/<id>.go
package <id>

import (
    "context"
    "iter"
    "github.com/obot-platform/discobot/agent-go/message"
    "github.com/obot-platform/discobot/agent-go/providers"
)

const providerID = "<id>"

func init() {
    providers.Register(providerID, New)
}

type Provider struct { /* apiKey, baseURL, client */ }

func New(cfg providers.Config) (providers.Provider, error) { ... }
func (p *Provider) ID() string { return providerID }
func (p *Provider) Complete(ctx context.Context, req providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] { ... }
func (p *Provider) CountTokens(ctx context.Context, req providers.CountTokensRequest) (providers.CountTokensResponse, error) { ... }
func (p *Provider) ListModels(ctx context.Context) ([]providers.ModelInfo, error) { ... }
```

## Step 2: Factory Registration

The `init()` function calls `providers.Register(providerID, New)` to register a factory. The factory signature is `func(cfg providers.Config) (providers.Provider, error)`.

Config provides `cfg.APIKey()` and `cfg.BaseURL()`. Require `api_key`; use a sensible default for `base_url`. Strip trailing slashes from base URL.

## Step 3: Blank Import

Add to `cmd/agent-api/main.go`:

```go
import _ "github.com/obot-platform/discobot/agent-go/providers/<id>"
```

This ensures `init()` runs. The provider is auto-configured at startup via env var `{UPPER_ID}_API_KEY` (e.g., `ANTHROPIC_API_KEY`). Optional: `{UPPER_ID}_BASE_URL`.

## Step 4: Message Conversion

Convert `[]message.Message` to the provider's wire format. Each message has:
- `Role`: `"system"`, `"user"`, `"assistant"`, or `"tool"`
- `Parts`: slice of `message.Part` (type-switch on concrete types)

Handle these part types per role:

| Role | Part Types to Handle |
|------|---------------------|
| `system` | `TextPart` |
| `user` | `TextPart`, `ImagePart`, `FilePart` |
| `assistant` | `ReasoningPart`, `TextPart`, `ToolCallPart` |
| `tool` | `ToolResultPart` |

**`ReasoningPart`** fields: `ID`, `Text` (summary), `ProviderMetadata` (opaque JSON from the provider — pass back as-is in multi-turn to preserve reasoning context). See "Reasoning Multi-Turn" section below.

**`ToolCallPart`** fields: `ToolCallID`, `ToolName`, `Input` (json.RawMessage).

**`ToolResultPart`** fields: `ToolCallID`, `ToolName`, `Output` (interface — type-switch to extract string):
- `TextOutput` → `.Value` string
- `JSONOutput` → `string(.Value)` raw JSON
- `ErrorTextOutput` → `.Value` string
- `ErrorJSONOutput` → `string(.Value)` raw JSON
- `ExecutionDeniedOutput` → `.Reason` string
- `ContentOutput` → iterate `.Value` items, extract `ContentTextItem.Text`

**`ImagePart`** fields: `Image` (URL or base64 string), `MediaType`.

## Step 5: Tool Conversion

Convert `[]providers.ToolDefinition` to the provider's tool/function format:
```go
type ToolDefinition struct {
    Name        string          // function name
    Description string          // may be empty
    InputSchema json.RawMessage // JSON Schema for parameters
}
```

## Step 6: Streaming via `Complete()`

`Complete()` returns `iter.Seq2[message.ProviderMessageChunk, error]` — a Go range iterator. Implementation pattern:

```go
func (p *Provider) Complete(ctx context.Context, req providers.CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error] {
    return func(yield func(message.ProviderMessageChunk, error) bool) {
        // 1. Convert messages & tools to provider wire format
        // 2. Build HTTP request (with streaming enabled)
        // 3. Send request, check status
        // 4. Parse streaming response, yield chunks
        // 5. On error: yield(nil, err) then return
    }
}
```

**`yield` returns `false` when the consumer stops iterating (e.g., context cancelled). Always check the return value and stop if false.**

### Required Chunk Sequence

Yield chunks in this order:

1. `message.StreamStartChunk{}` — first chunk
2. `message.ResponseMetadataChunk{ID, Timestamp, ModelID}` — response metadata
3. Content chunks (text, tool calls, reasoning) — see below
4. `message.FinishChunk{FinishReason, Usage}` — last chunk

### Content Chunks

**Text streaming:**
```
TextStartChunk{ID}  →  TextDeltaChunk{ID, Delta} × N  →  TextEndChunk{ID}
```

**Tool call streaming:**
```
ToolInputStartChunk{ToolCallID, ToolName}  →  ToolInputDeltaChunk{ToolCallID, InputTextDelta} × N  →  ToolInputEndChunk{ToolCallID}
```

**Reasoning/thinking streaming:**
```
ReasoningStartChunk{ID}  →  ReasoningDeltaChunk{ID, Delta} × N  →  ReasoningEndChunk{ID}
```

Multiple content blocks can be interleaved (e.g., reasoning then text, or text then tool calls).

### Reasoning Multi-Turn

To maintain reasoning context across turns, the provider must:

1. **Request opaque reasoning data** from the API (e.g., OpenAI's `encrypted_content` via `include: ["reasoning.encrypted_content"]`).
2. **Store it on `ReasoningEndChunk.ProviderMetadata`** — the accumulator propagates this to `ReasoningPart.ProviderMetadata`, which persists in the thread store.
3. **Pass it back in `convertAssistantMessage`** — use `p.MetadataType()` to check whether the metadata is your provider's own format before using it. Fall back gracefully for foreign metadata (see below).
4. **Include required item fields** — some APIs require the message item following a reasoning item to include `id` and `status` fields. Capture item IDs during streaming (e.g., from `content_part.added` events) and include them when reconstructing items.

### Cross-Provider Reasoning (Required)

Thread history may contain `ReasoningPart` values produced by a different provider (e.g., the user switched from OpenAI to Anthropic mid-thread). Each provider's `ProviderMetadata` is opaque to other providers and must not be forwarded blindly.

**Always use `p.MetadataType()` to check the metadata type before using it:**

```go
case message.ReasoningPart:
    if p.MetadataType() == "<your-type>" {
        // Native format — pass through (e.g. Anthropic "thinking", OpenAI "reasoning").
        var block any
        json.Unmarshal(p.ProviderMetadata, &block)
        content = append(content, block)
    } else if p.Text != "" {
        // Foreign provider's reasoning — degrade gracefully using the summary text.
        // Wrap in a provider-appropriate construct (text block, summary item, etc.).
        content = append(content, map[string]any{"type": "text", "text": p.Text})
    }
    // Skip if no text and no compatible metadata.
```

`p.MetadataType()` reads the `"type"` field from `ProviderMetadata` — each provider embeds its own type tag (e.g. `"thinking"` for Anthropic, `"reasoning"` for OpenAI). If the type doesn't match, the metadata belongs to a different provider and must not be forwarded — use `p.Text` (the reasoning summary) as a plain-text fallback instead.

### FinishChunk

```go
message.FinishChunk{
    FinishReason: message.FinishReason{
        Unified: "stop"|"tool-calls"|"length"|"content-filter"|"error"|"other",
        Raw:     "<provider-specific string>",
    },
    Usage: message.Usage{
        InputTokens:  message.InputTokens{Total, NoCache, CacheRead, CacheWrite},
        OutputTokens: message.OutputTokens{Total, Text, Reasoning},
    },
}
```

Set `Unified` to `"tool-calls"` when the response contains tool call output items.

## Step 7: Data Storage

Disable provider-side data retention. Most providers store request/response data by default for monitoring. Set `"store": false` (or equivalent) in the request body for both `Complete()` and `CountTokens()` to prevent this. This ensures user data is not persisted on the provider's servers.

## Step 8: Handle `CompleteRequest` Parameters

| Field | Action |
|-------|--------|
| `req.Model` | Pass as model ID |
| `req.Messages` | Convert (Step 4) |
| `req.Tools` | Convert (Step 5), omit if empty |
| `req.MaxTokens` | Set if non-nil |
| `req.Temperature` | Set if non-nil |
| `req.TopP` | Set if non-nil |
| `req.Reasoning` | If `"enabled"`, activate provider's extended thinking/reasoning mode |
| `req.ProviderOptions` | Opaque JSON — merge into request body if non-nil |

## Step 9: `CountTokens()`

If the provider has a token counting API, use it. Convert messages and tools the same way as `Complete()`, send to the counting endpoint, return `CountTokensResponse{TotalTokens: n}`.

If no native API exists, estimate at ~4 chars/token from the serialized provider JSON.

## Step 10: `ListModels()`

Return `[]providers.ModelInfo` for known models. Preferred approach: query the provider's models endpoint for live IDs, then enrich with metadata from the `modelsdev` package.

```go
import "github.com/obot-platform/discobot/agent-go/providers/modelsdev"

// 1. Fetch live model IDs from provider API (e.g., GET /v1/models)
// 2. Enrich each model with metadata from models.dev:
for _, m := range apiModels {
    info := providers.ModelInfo{ID: m.ID, DisplayName: m.ID}
    if md := modelsdev.Lookup(providerID, m.ID); md != nil {
        info.DisplayName     = md.Name
        info.Reasoning       = md.Reasoning
        info.ContextWindow   = md.ContextWindow
        info.MaxOutputTokens = md.MaxOutputTokens
    }
    models = append(models, info)
}
```

The `modelsdev` package embeds `models-dev-api.json` and provides:
- `Lookup(providerID, modelID) *ModelInfo` — single model metadata
- `AllForProvider(providerID) []ModelInfo` — all models for a provider (fallback if no live API)

Do NOT set `ProviderID` — `ProviderRegistry` sets it automatically.

## Step 11: Tests

### Unit Tests (`providers/<id>/<id>_test.go`)

Test in the same package (access to internals). Use `httptest.NewServer` for HTTP mocking. Cover:
- Factory: missing API key errors, default/custom base URL
- Message conversion: all role/part combinations
- Tool conversion
- SSE/stream parsing: text, tool calls, reasoning, errors, unknown events
- `Complete()` end-to-end with mock server
- `CountTokens()` with mock server
- `ListModels()` returns expected models
- Factory registration via `providers.Has("<id>")`

### Integration Tests (`internal/integration/<id>_test.go`)

Package `integration`. Use the real API with a cheap model. Skip if API key unset:

```go
func provider(t *testing.T) providers.Provider {
    t.Helper()
    apiKey := os.Getenv("<UPPER_ID>_API_KEY")
    if apiKey == "" { t.Skip("<UPPER_ID>_API_KEY not set") }
    p, err := providers.New("<id>", providers.Config{"api_key": apiKey})
    if err != nil { t.Fatal(err) }
    return p
}
```

Cover: simple text completion, tool call, tool call round-trip (2 turns), multi-turn conversation, token counting, stream lifecycle ordering, context cancellation, reasoning completion, reasoning multi-turn (verify reasoning context preserved across turns), reasoning stream lifecycle.

Run with: `go test -v ./internal/integration/...`

## SSE Parsing Pattern

Most LLM providers stream via Server-Sent Events. Reusable pattern:

```go
func parseSSEStream(body io.Reader, yield func(message.ProviderMessageChunk, error) bool) {
    scanner := bufio.NewScanner(body)
    scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
    var eventType string
    for scanner.Scan() {
        line := scanner.Text()
        if strings.HasPrefix(line, "event: ") {
            eventType = strings.TrimPrefix(line, "event: ")
        } else if strings.HasPrefix(line, "data: ") {
            data := strings.TrimPrefix(line, "data: ")
            if eventType != "" {
                if !handleEvent(eventType, []byte(data), yield) { return }
                eventType = ""
            }
        } else if line == "" {
            eventType = ""
        }
    }
    if err := scanner.Err(); err != nil {
        yield(nil, fmt.Errorf("<id>: read SSE stream: %w", err))
    }
}
```

## Error Handling

- Non-200 HTTP status: read body, yield `(nil, fmt.Errorf("<id>: API error %d: %s", status, body))`
- Stream errors: yield `(nil, err)` then return (no further chunks after error)
- JSON parse errors in events: yield `(nil, fmt.Errorf("<id>: parse <event>: %w", err))`
- Prefix all errors with `"<id>: "` for debuggability

## Reference Implementation

See `providers/openai/openai.go` for a complete working example.
