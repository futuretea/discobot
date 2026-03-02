package message

import "time"

// Warning is a non-fatal issue reported by the provider.
type Warning struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// FinishReason describes why generation stopped.
type FinishReason struct {
	// Unified is the normalized reason: "stop", "length", "content-filter",
	// "tool-calls", "error", or "other".
	Unified string `json:"unified"`
	// Raw is the provider-specific reason string, if available.
	Raw string `json:"raw,omitempty"`
}

// Usage contains token usage statistics from a completion.
type Usage struct {
	InputTokens  InputTokens  `json:"inputTokens"`
	OutputTokens OutputTokens `json:"outputTokens"`
}

// InputTokens breaks down input token usage.
type InputTokens struct {
	Total      int `json:"total"`
	NoCache    int `json:"noCache,omitempty"`
	CacheRead  int `json:"cacheRead,omitempty"`
	CacheWrite int `json:"cacheWrite,omitempty"`
}

// OutputTokens breaks down output token usage.
type OutputTokens struct {
	Total     int `json:"total"`
	Text      int `json:"text,omitempty"`
	Reasoning int `json:"reasoning,omitempty"`
}

// ResponseMetadata provides metadata about a provider response.
type ResponseMetadata struct {
	ID        string     `json:"id,omitempty"`
	Timestamp *time.Time `json:"timestamp,omitempty"`
	ModelID   string     `json:"modelId,omitempty"`
}

// ModeChangeData is the payload for a data-mode-change chunk.
type ModeChangeData struct {
	Mode string `json:"mode"`
}
