package message

import (
	"encoding/json"
	"fmt"
	"strings"
)

// UIPart represents a content part within a UIMessage.
// The concrete type determines the "type" discriminator in JSON.
type UIPart interface {
	uiPartType() string
}

// UITextPart is a text content part in the AI SDK v6 UIMessage format.
type UITextPart struct {
	Type             string          `json:"type"`
	Text             string          `json:"text"`
	State            string          `json:"state"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (UITextPart) uiPartType() string { return "text" }

// UIReasoningPart is a reasoning/thinking content part in the AI SDK v6 UIMessage format.
type UIReasoningPart struct {
	Type             string          `json:"type"`
	Text             string          `json:"text"`
	State            string          `json:"state"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (UIReasoningPart) uiPartType() string { return "reasoning" }

// UIFilePart is a file content part in the AI SDK v6 UIMessage format.
type UIFilePart struct {
	Type             string          `json:"type"`
	URL              string          `json:"url"`
	MediaType        string          `json:"mediaType"`
	Filename         string          `json:"filename,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (UIFilePart) uiPartType() string { return "file" }

// UIStepStartPart marks the start of an agent step in the AI SDK v6 UIMessage format.
type UIStepStartPart struct {
	Type string `json:"type"`
}

func (UIStepStartPart) uiPartType() string { return "step-start" }

// UISourceURLPart is a URL source reference in the AI SDK v6 UIMessage format.
type UISourceURLPart struct {
	Type             string          `json:"type"`
	SourceID         string          `json:"sourceId"`
	URL              string          `json:"url"`
	Title            string          `json:"title,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (UISourceURLPart) uiPartType() string { return "source-url" }

// UISourceDocumentPart is a document source reference in the AI SDK v6 UIMessage format.
type UISourceDocumentPart struct {
	Type             string          `json:"type"`
	SourceID         string          `json:"sourceId"`
	MediaType        string          `json:"mediaType"`
	Title            string          `json:"title"`
	Filename         string          `json:"filename,omitempty"`
	ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
}

func (UISourceDocumentPart) uiPartType() string { return "source-document" }

// UIDataPart is a custom data part in the AI SDK v6 UIMessage format.
// Type contains the full "data-{dataType}" discriminator string.
type UIDataPart struct {
	Type string          `json:"type"`
	ID   string          `json:"id,omitempty"`
	Data json.RawMessage `json:"data"`
}

func (p UIDataPart) uiPartType() string { return p.Type }

// MarshalUIPart serializes a UIPart to JSON. Each concrete UIPart type carries
// its own "type" discriminator field so plain json.Marshal produces the correct
// wire format.
func MarshalUIPart(p UIPart) (json.RawMessage, error) {
	data, err := json.Marshal(p)
	if err != nil {
		return nil, fmt.Errorf("marshal UIPart (%T): %w", p, err)
	}
	return data, nil
}

// UnmarshalUIPart deserializes JSON into the appropriate UIPart variant
// based on the "type" discriminator field.
func UnmarshalUIPart(data []byte) (UIPart, error) {
	var disc struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return nil, fmt.Errorf("unmarshal UIPart type: %w", err)
	}
	switch {
	case disc.Type == "text":
		var p UITextPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "reasoning":
		var p UIReasoningPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "file":
		var p UIFilePart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "step-start":
		var p UIStepStartPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "source-url":
		var p UISourceURLPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "source-document":
		var p UISourceDocumentPart
		return p, json.Unmarshal(data, &p)
	case disc.Type == "dynamic-tool":
		var p DynamicToolPart
		return p, json.Unmarshal(data, &p)
	case strings.HasPrefix(disc.Type, "data-"):
		var p UIDataPart
		return p, json.Unmarshal(data, &p)
	default:
		return nil, fmt.Errorf("unknown UIPart type: %q", disc.Type)
	}
}
