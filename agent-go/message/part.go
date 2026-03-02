package message

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Part represents a content part within a Message.
// The concrete type determines the "type" discriminator in JSON.
type Part interface {
	partType() string
}

// TextPart is a text content part.
type TextPart struct {
	ID               string          `json:"id,omitempty"`
	Text             string          `json:"text"`
	State            string          `json:"state,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
	ProviderOptions  json.RawMessage `json:"providerOptions,omitempty"`
}

func (TextPart) partType() string { return "text" }

// ReasoningPart is a reasoning/thinking content part.
type ReasoningPart struct {
	ID               string          `json:"id,omitempty"`
	Text             string          `json:"text"`
	State            string          `json:"state,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
	ProviderOptions  json.RawMessage `json:"providerOptions,omitempty"`
}

func (ReasoningPart) partType() string { return "reasoning" }

// ImagePart is an image content part (user messages).
// Image holds the image data as a base64-encoded string or a URL string.
type ImagePart struct {
	Image           string          `json:"image"`
	MediaType       string          `json:"mediaType,omitempty"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ImagePart) partType() string { return "image" }

// FilePart is a file content part.
// Data holds a URL, data-URI, or base64-encoded string.
type FilePart struct {
	Data             string          `json:"data"`
	MediaType        string          `json:"mediaType"`
	Filename         string          `json:"filename,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
	ProviderOptions  json.RawMessage `json:"providerOptions,omitempty"`
}

func (FilePart) partType() string { return "file" }

// ToolCallPart is a tool call invocation (assistant messages).
type ToolCallPart struct {
	ToolCallID       string          `json:"toolCallId"`
	ToolName         string          `json:"toolName"`
	Input            json.RawMessage `json:"input"`
	ProviderExecuted *bool           `json:"providerExecuted,omitempty"`
	ProviderOptions  json.RawMessage `json:"providerOptions,omitempty"`
}

func (ToolCallPart) partType() string { return "tool-call" }

// ToolResultPart is a tool execution result (tool messages, or
// inline in assistant messages for provider-executed tools).
// Output is marshaled/unmarshaled as a nested discriminated union.
type ToolResultPart struct {
	ToolCallID      string           `json:"toolCallId"`
	ToolName        string           `json:"toolName"`
	Output          ToolResultOutput `json:"-"`
	ProviderOptions json.RawMessage  `json:"providerOptions,omitempty"`
}

func (ToolResultPart) partType() string { return "tool-result" }

// ToolApprovalRequest requests user approval for a tool call.
type ToolApprovalRequest struct {
	ApprovalID string `json:"approvalId"`
	ToolCallID string `json:"toolCallId"`
}

func (ToolApprovalRequest) partType() string { return "tool-approval-request" }

// ToolApprovalResponse responds to a tool approval request.
type ToolApprovalResponse struct {
	ApprovalID       string `json:"approvalId"`
	Approved         bool   `json:"approved"`
	Reason           string `json:"reason,omitempty"`
	ProviderExecuted *bool  `json:"providerExecuted,omitempty"`
}

func (ToolApprovalResponse) partType() string { return "tool-approval-response" }

// SourceURLPart is a URL source reference.
type SourceURLPart struct {
	SourceID         string          `json:"sourceId"`
	URL              string          `json:"url"`
	Title            string          `json:"title,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (SourceURLPart) partType() string { return "source-url" }

// SourceDocumentPart is a document source reference.
type SourceDocumentPart struct {
	SourceID         string          `json:"sourceId"`
	MediaType        string          `json:"mediaType"`
	Title            string          `json:"title"`
	Filename         string          `json:"filename,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (SourceDocumentPart) partType() string { return "source-document" }

// StepStartPart marks a step boundary in the message.
type StepStartPart struct{}

func (StepStartPart) partType() string { return "step-start" }

// DataPart is a custom data part with a type prefix of "data-".
type DataPart struct {
	// DataType is the suffix after "data-" (e.g. "mode-change" for type "data-mode-change").
	DataType string          `json:"-"`
	ID       string          `json:"id,omitempty"`
	Data     json.RawMessage `json:"data"`
}

func (p DataPart) partType() string { return "data-" + p.DataType }

// MarshalPart serializes a Part to JSON, injecting the "type" discriminator.
func MarshalPart(p Part) ([]byte, error) {
	switch v := p.(type) {
	case TextPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			TextPart
		}{"text", v})
	case ReasoningPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			ReasoningPart
		}{"reasoning", v})
	case ImagePart:
		return json.Marshal(struct {
			Type string `json:"type"`
			ImagePart
		}{"image", v})
	case FilePart:
		return json.Marshal(struct {
			Type string `json:"type"`
			FilePart
		}{"file", v})
	case ToolCallPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			ToolCallPart
		}{"tool-call", v})
	case ToolResultPart:
		outputData, err := MarshalToolResultOutput(v.Output)
		if err != nil {
			return nil, fmt.Errorf("marshal ToolResultPart.Output: %w", err)
		}
		return json.Marshal(struct {
			Type            string          `json:"type"`
			ToolCallID      string          `json:"toolCallId"`
			ToolName        string          `json:"toolName"`
			Output          json.RawMessage `json:"output"`
			ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
		}{
			Type:            "tool-result",
			ToolCallID:      v.ToolCallID,
			ToolName:        v.ToolName,
			Output:          outputData,
			ProviderOptions: v.ProviderOptions,
		})
	case ToolApprovalRequest:
		return json.Marshal(struct {
			Type string `json:"type"`
			ToolApprovalRequest
		}{"tool-approval-request", v})
	case ToolApprovalResponse:
		return json.Marshal(struct {
			Type string `json:"type"`
			ToolApprovalResponse
		}{"tool-approval-response", v})
	case SourceURLPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			SourceURLPart
		}{"source-url", v})
	case SourceDocumentPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			SourceDocumentPart
		}{"source-document", v})
	case StepStartPart:
		return json.Marshal(struct {
			Type string `json:"type"`
		}{"step-start"})
	case DataPart:
		return json.Marshal(struct {
			Type string `json:"type"`
			DataPart
		}{"data-" + v.DataType, v})
	default:
		return nil, fmt.Errorf("unknown Part type: %T", p)
	}
}

// UnmarshalPart deserializes JSON into the appropriate Part variant
// based on the "type" discriminator field.
func UnmarshalPart(data []byte) (Part, error) {
	var disc struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return nil, fmt.Errorf("unmarshal Part type discriminator: %w", err)
	}

	switch {
	case disc.Type == "text":
		var p TextPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "reasoning":
		var p ReasoningPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "image":
		var p ImagePart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "file":
		var p FilePart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "tool-call":
		var p ToolCallPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "tool-result":
		var raw struct {
			ToolCallID      string          `json:"toolCallId"`
			ToolName        string          `json:"toolName"`
			Output          json.RawMessage `json:"output"`
			ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		output, err := UnmarshalToolResultOutput(raw.Output)
		if err != nil {
			return nil, fmt.Errorf("unmarshal ToolResultPart.Output: %w", err)
		}
		return ToolResultPart{
			ToolCallID:      raw.ToolCallID,
			ToolName:        raw.ToolName,
			Output:          output,
			ProviderOptions: raw.ProviderOptions,
		}, nil
	case disc.Type == "tool-approval-request":
		var p ToolApprovalRequest
		return p, json.Unmarshal(data, &p)
	case disc.Type == "tool-approval-response":
		var p ToolApprovalResponse
		return p, json.Unmarshal(data, &p)
	case disc.Type == "source-url":
		var p SourceURLPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "source-document":
		var p SourceDocumentPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "step-start":
		return StepStartPart{}, nil
	case strings.HasPrefix(disc.Type, "data-"):
		var p DataPart
		if err := json.Unmarshal(data, &p); err != nil {
			return nil, err
		}
		p.DataType = strings.TrimPrefix(disc.Type, "data-")
		return p, nil
	default:
		return nil, fmt.Errorf("unknown Part type: %q", disc.Type)
	}
}
