package message

import (
	"encoding/json"
	"fmt"
)

// ToolResultOutput represents the output of a tool execution.
// It is a discriminated union on the "type" field.
type ToolResultOutput interface {
	toolResultOutputType() string
}

// TextOutput is a plain text tool result.
type TextOutput struct {
	Value           string          `json:"value"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (TextOutput) toolResultOutputType() string { return "text" }

// JSONOutput is a JSON tool result.
type JSONOutput struct {
	Value           json.RawMessage `json:"value"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (JSONOutput) toolResultOutputType() string { return "json" }

// ExecutionDeniedOutput indicates the tool execution was denied.
type ExecutionDeniedOutput struct {
	Reason          string          `json:"reason,omitempty"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ExecutionDeniedOutput) toolResultOutputType() string { return "execution-denied" }

// ErrorTextOutput is a plain text error from tool execution.
type ErrorTextOutput struct {
	Value           string          `json:"value"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ErrorTextOutput) toolResultOutputType() string { return "error-text" }

// ErrorJSONOutput is a JSON error from tool execution.
type ErrorJSONOutput struct {
	Value           json.RawMessage `json:"value"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ErrorJSONOutput) toolResultOutputType() string { return "error-json" }

// ContentOutput is a rich content tool result with multiple items.
type ContentOutput struct {
	Value []ToolResultContentItem `json:"-"`
}

func (ContentOutput) toolResultOutputType() string { return "content" }

// --- ToolResultContentItem ---

// ToolResultContentItem represents an item within a ContentOutput.
// It is a discriminated union on the "type" field.
type ToolResultContentItem interface {
	toolResultContentItemType() string
}

// ContentTextItem is a text item in tool result content.
type ContentTextItem struct {
	Text            string          `json:"text"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentTextItem) toolResultContentItemType() string { return "text" }

// ContentMediaItem is a media item in tool result content.
// Deprecated: use ContentFileDataItem or ContentImageDataItem instead.
type ContentMediaItem struct {
	Data      string `json:"data"`
	MediaType string `json:"mediaType"`
}

func (ContentMediaItem) toolResultContentItemType() string { return "media" }

// ContentFileDataItem is a file data item in tool result content.
type ContentFileDataItem struct {
	Data            string          `json:"data"`
	MediaType       string          `json:"mediaType"`
	Filename        string          `json:"filename,omitempty"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentFileDataItem) toolResultContentItemType() string { return "file-data" }

// ContentFileURLItem is a file URL item in tool result content.
type ContentFileURLItem struct {
	URL             string          `json:"url"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentFileURLItem) toolResultContentItemType() string { return "file-url" }

// ContentFileIDItem is a file ID reference in tool result content.
// FileID can be a string or an object (Record<string, string>).
type ContentFileIDItem struct {
	FileID          json.RawMessage `json:"fileId"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentFileIDItem) toolResultContentItemType() string { return "file-id" }

// ContentImageDataItem is an image data item in tool result content.
type ContentImageDataItem struct {
	Data            string          `json:"data"`
	MediaType       string          `json:"mediaType"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentImageDataItem) toolResultContentItemType() string { return "image-data" }

// ContentImageURLItem is an image URL item in tool result content.
type ContentImageURLItem struct {
	URL             string          `json:"url"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentImageURLItem) toolResultContentItemType() string { return "image-url" }

// ContentImageFileIDItem is an image file ID reference in tool result content.
// FileID can be a string or an object (Record<string, string>).
type ContentImageFileIDItem struct {
	FileID          json.RawMessage `json:"fileId"`
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentImageFileIDItem) toolResultContentItemType() string { return "image-file-id" }

// ContentCustomItem is a custom item in tool result content.
type ContentCustomItem struct {
	ProviderOptions json.RawMessage `json:"providerOptions,omitempty"`
}

func (ContentCustomItem) toolResultContentItemType() string { return "custom" }

// --- Marshal/Unmarshal for ToolResultOutput ---

// MarshalToolResultOutput serializes a ToolResultOutput to JSON with a "type" discriminator.
func MarshalToolResultOutput(o ToolResultOutput) ([]byte, error) {
	switch v := o.(type) {
	case TextOutput:
		return json.Marshal(struct {
			Type string `json:"type"`
			TextOutput
		}{"text", v})
	case JSONOutput:
		return json.Marshal(struct {
			Type string `json:"type"`
			JSONOutput
		}{"json", v})
	case ExecutionDeniedOutput:
		return json.Marshal(struct {
			Type string `json:"type"`
			ExecutionDeniedOutput
		}{"execution-denied", v})
	case ErrorTextOutput:
		return json.Marshal(struct {
			Type string `json:"type"`
			ErrorTextOutput
		}{"error-text", v})
	case ErrorJSONOutput:
		return json.Marshal(struct {
			Type string `json:"type"`
			ErrorJSONOutput
		}{"error-json", v})
	case ContentOutput:
		items := make([]json.RawMessage, len(v.Value))
		for i, item := range v.Value {
			data, err := marshalToolResultContentItem(item)
			if err != nil {
				return nil, fmt.Errorf("marshal ContentOutput.Value[%d]: %w", i, err)
			}
			items[i] = data
		}
		return json.Marshal(struct {
			Type  string            `json:"type"`
			Value []json.RawMessage `json:"value"`
		}{"content", items})
	default:
		return nil, fmt.Errorf("unknown ToolResultOutput type: %T", o)
	}
}

// UnmarshalToolResultOutput deserializes a ToolResultOutput from JSON with a "type" discriminator.
func UnmarshalToolResultOutput(data []byte) (ToolResultOutput, error) {
	var disc struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return nil, fmt.Errorf("unmarshal ToolResultOutput type discriminator: %w", err)
	}

	switch disc.Type {
	case "text":
		var o TextOutput
		return o, json.Unmarshal(data, &o)
	case "json":
		var o JSONOutput
		return o, json.Unmarshal(data, &o)
	case "execution-denied":
		var o ExecutionDeniedOutput
		return o, json.Unmarshal(data, &o)
	case "error-text":
		var o ErrorTextOutput
		return o, json.Unmarshal(data, &o)
	case "error-json":
		var o ErrorJSONOutput
		return o, json.Unmarshal(data, &o)
	case "content":
		var raw struct {
			Value []json.RawMessage `json:"value"`
		}
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, err
		}
		items := make([]ToolResultContentItem, len(raw.Value))
		for i, itemData := range raw.Value {
			item, err := unmarshalToolResultContentItem(itemData)
			if err != nil {
				return nil, fmt.Errorf("unmarshal ContentOutput.Value[%d]: %w", i, err)
			}
			items[i] = item
		}
		return ContentOutput{Value: items}, nil
	default:
		return nil, fmt.Errorf("unknown ToolResultOutput type: %q", disc.Type)
	}
}

// --- Marshal/Unmarshal for ToolResultContentItem ---

func marshalToolResultContentItem(item ToolResultContentItem) ([]byte, error) {
	switch v := item.(type) {
	case ContentTextItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentTextItem
		}{"text", v})
	case ContentMediaItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentMediaItem
		}{"media", v})
	case ContentFileDataItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentFileDataItem
		}{"file-data", v})
	case ContentFileURLItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentFileURLItem
		}{"file-url", v})
	case ContentFileIDItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentFileIDItem
		}{"file-id", v})
	case ContentImageDataItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentImageDataItem
		}{"image-data", v})
	case ContentImageURLItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentImageURLItem
		}{"image-url", v})
	case ContentImageFileIDItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentImageFileIDItem
		}{"image-file-id", v})
	case ContentCustomItem:
		return json.Marshal(struct {
			Type string `json:"type"`
			ContentCustomItem
		}{"custom", v})
	default:
		return nil, fmt.Errorf("unknown ToolResultContentItem type: %T", item)
	}
}

func unmarshalToolResultContentItem(data []byte) (ToolResultContentItem, error) {
	var disc struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &disc); err != nil {
		return nil, fmt.Errorf("unmarshal ToolResultContentItem type discriminator: %w", err)
	}

	switch disc.Type {
	case "text":
		var item ContentTextItem
		return item, json.Unmarshal(data, &item)
	case "media":
		var item ContentMediaItem
		return item, json.Unmarshal(data, &item)
	case "file-data":
		var item ContentFileDataItem
		return item, json.Unmarshal(data, &item)
	case "file-url":
		var item ContentFileURLItem
		return item, json.Unmarshal(data, &item)
	case "file-id":
		var item ContentFileIDItem
		return item, json.Unmarshal(data, &item)
	case "image-data":
		var item ContentImageDataItem
		return item, json.Unmarshal(data, &item)
	case "image-url":
		var item ContentImageURLItem
		return item, json.Unmarshal(data, &item)
	case "image-file-id":
		var item ContentImageFileIDItem
		return item, json.Unmarshal(data, &item)
	case "custom":
		var item ContentCustomItem
		return item, json.Unmarshal(data, &item)
	default:
		return nil, fmt.Errorf("unknown ToolResultContentItem type: %q", disc.Type)
	}
}
