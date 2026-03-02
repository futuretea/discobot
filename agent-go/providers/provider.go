package providers

import (
	"context"
	"encoding/json"
	"iter"

	"github.com/obot-platform/discobot/agent-go/message"
)

// Provider is the interface that LLM provider implementations must satisfy.
// Each provider is identified by an ID matching its models.dev ID
// (e.g., "anthropic", "openai").
type Provider interface {
	// ID returns the provider identifier, matching the models.dev provider ID.
	ID() string

	// Complete sends messages to the LLM and returns a streaming iterator
	// of response chunks. The caller iterates with:
	//
	//   for chunk, err := range provider.Complete(ctx, req) {
	//       if err != nil { ... }
	//       // process chunk
	//   }
	//
	// The iterator stops when the response is complete, the context is
	// cancelled, or an error occurs. After an error is yielded, no
	// further chunks are produced.
	Complete(ctx context.Context, req CompleteRequest) iter.Seq2[message.ProviderMessageChunk, error]

	// CountTokens counts the number of tokens in the given messages
	// for the specified model. This is used for context window management.
	CountTokens(ctx context.Context, req CountTokensRequest) (CountTokensResponse, error)

	// ListModels returns the models available from this provider
	// with the current configuration/credentials.
	ListModels(ctx context.Context) ([]ModelInfo, error)
}

// Config holds provider configuration, typically API keys and endpoint URLs.
// Keys are provider-specific (e.g., "api_key", "base_url").
type Config map[string]string

// APIKey returns the "api_key" config value.
func (c Config) APIKey() string {
	return c["api_key"]
}

// BaseURL returns the "base_url" config value.
func (c Config) BaseURL() string {
	return c["base_url"]
}

// ToolDefinition describes a tool available to the model.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"` // JSON Schema
}

// CompleteRequest is the input to Provider.Complete.
type CompleteRequest struct {
	Model    string                      `json:"model"`
	Messages []message.Message `json:"messages"`
	Tools    []ToolDefinition  `json:"tools,omitempty"`

	// Optional parameters. Nil means use provider default.
	MaxTokens   *int     `json:"maxTokens,omitempty"`
	Temperature *float64 `json:"temperature,omitempty"`
	TopP        *float64 `json:"topP,omitempty"`

	// Reasoning controls extended thinking. Empty means disabled.
	// "enabled" means the model should use extended thinking if supported.
	Reasoning string `json:"reasoning,omitempty"`

	// ProviderOptions is an opaque JSON blob for provider-specific parameters
	// that don't fit the common fields.
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

// CountTokensRequest is the input to Provider.CountTokens.
type CountTokensRequest struct {
	Model    string                      `json:"model"`
	Messages []message.Message `json:"messages"`
	Tools    []ToolDefinition  `json:"tools,omitempty"`
}

// CountTokensResponse is the output of Provider.CountTokens.
type CountTokensResponse struct {
	TotalTokens int `json:"totalTokens"`
}
