package message

import (
	"encoding/json"
	"fmt"
	"time"
)

// DynamicToolPart represents a tool invocation with its full lifecycle state,
// used only in the UI projection format. It is not a Part variant — it exists
// only in the JSON output of ProjectUIMessages.
type DynamicToolPart struct {
	ToolName             string          `json:"toolName"`
	ToolCallID           string          `json:"toolCallId"`
	State                string          `json:"state"`
	Title                string          `json:"title,omitempty"`
	ProviderExecuted     *bool           `json:"providerExecuted,omitempty"`
	Input                json.RawMessage `json:"input,omitempty"`
	Output               json.RawMessage `json:"output,omitempty"`
	ErrorText            string          `json:"errorText,omitempty"`
	Approval             *ToolApproval   `json:"approval,omitempty"`
	Preliminary          *bool           `json:"preliminary,omitempty"`
	CallProviderMetadata json.RawMessage `json:"callProviderMetadata,omitempty"`
}

// ToolApproval represents a tool approval request/response within a DynamicToolPart.
type ToolApproval struct {
	ID       string `json:"id"`
	Approved *bool  `json:"approved,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// uiMessage is the JSON wire format for a UIMessage in the AI SDK v6 protocol.
type uiMessage struct {
	ID        string            `json:"id"`
	Role      string            `json:"role"`
	Parts     []json.RawMessage `json:"parts"`
	Metadata  json.RawMessage   `json:"metadata,omitempty"`
	CreatedAt *time.Time        `json:"createdAt,omitempty"`
}

// ProjectUIMessages converts a slice of Messages (which may include "tool" role
// messages) into the AI SDK v6 UIMessage JSON format. Consecutive assistant+tool
// pairs are merged into single assistant messages with DynamicToolParts.
func ProjectUIMessages(messages []Message) ([]json.RawMessage, error) {
	var result []json.RawMessage
	i := 0
	for i < len(messages) {
		msg := messages[i]
		switch msg.Role {
		case "system":
			data, err := marshalUISystemMessage(msg)
			if err != nil {
				return nil, err
			}
			result = append(result, data)
			i++
		case "user":
			data, err := marshalUIUserMessage(msg)
			if err != nil {
				return nil, err
			}
			result = append(result, data)
			i++
		case "assistant":
			// Consume consecutive (assistant, optional tool) pairs into one UIMessage.
			ui := uiMessage{
				ID:        msg.ID,
				Role:      "assistant",
				Metadata:  msg.Metadata,
				CreatedAt: msg.CreatedAt,
			}
			for i < len(messages) && messages[i].Role == "assistant" {
				ass := messages[i]
				i++
				var toolMsg *Message
				if i < len(messages) && messages[i].Role == "tool" {
					t := messages[i]
					toolMsg = &t
					i++
				}
				// Add step-start marker.
				stepStart, _ := json.Marshal(struct {
					Type string `json:"type"`
				}{"step-start"})
				ui.Parts = append(ui.Parts, stepStart)

				// Convert assistant+tool pair to UI parts.
				parts, err := convertAssistantToolPairToUI(ass, toolMsg)
				if err != nil {
					return nil, err
				}
				ui.Parts = append(ui.Parts, parts...)
			}
			data, err := json.Marshal(ui)
			if err != nil {
				return nil, err
			}
			result = append(result, data)
		default:
			// Skip unknown roles (including orphan "tool" messages).
			i++
		}
	}
	return result, nil
}

func marshalUISystemMessage(msg Message) (json.RawMessage, error) {
	text := ""
	for _, p := range msg.Parts {
		if tp, ok := p.(TextPart); ok {
			text += tp.Text
		}
	}
	ui := uiMessage{
		ID:        msg.ID,
		Role:      "system",
		Metadata:  msg.Metadata,
		CreatedAt: msg.CreatedAt,
	}
	partData, err := json.Marshal(struct {
		Type  string `json:"type"`
		Text  string `json:"text"`
		State string `json:"state"`
	}{"text", text, "done"})
	if err != nil {
		return nil, err
	}
	ui.Parts = []json.RawMessage{partData}
	return json.Marshal(ui)
}

func marshalUIUserMessage(msg Message) (json.RawMessage, error) {
	ui := uiMessage{
		ID:        msg.ID,
		Role:      "user",
		Metadata:  msg.Metadata,
		CreatedAt: msg.CreatedAt,
	}
	for _, p := range msg.Parts {
		switch v := p.(type) {
		case TextPart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				Text             string          `json:"text"`
				State            string          `json:"state"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"text", v.Text, "done", v.ProviderMetadata})
			ui.Parts = append(ui.Parts, data)
		case FilePart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				URL              string          `json:"url"`
				MediaType        string          `json:"mediaType"`
				Filename         string          `json:"filename,omitempty"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"file", v.Data, v.MediaType, v.Filename, v.ProviderMetadata})
			ui.Parts = append(ui.Parts, data)
		case ImagePart:
			data, _ := json.Marshal(struct {
				Type      string `json:"type"`
				URL       string `json:"url"`
				MediaType string `json:"mediaType,omitempty"`
			}{"file", v.Image, v.MediaType})
			ui.Parts = append(ui.Parts, data)
		}
	}
	return json.Marshal(ui)
}

func convertAssistantToolPairToUI(ass Message, toolMsg *Message) ([]json.RawMessage, error) {
	// Index tool results and approval responses from the tool message.
	toolResults := make(map[string]ToolResultPart)
	approvalResponses := make(map[string]ToolApprovalResponse)

	if toolMsg != nil {
		for _, p := range toolMsg.Parts {
			switch v := p.(type) {
			case ToolResultPart:
				toolResults[v.ToolCallID] = v
			case ToolApprovalResponse:
				approvalResponses[v.ApprovalID] = v
			}
		}
	}

	// Index approval requests from the assistant message.
	approvalRequests := make(map[string]ToolApprovalRequest)
	for _, p := range ass.Parts {
		if ar, ok := p.(ToolApprovalRequest); ok {
			approvalRequests[ar.ToolCallID] = ar
		}
	}

	var parts []json.RawMessage
	// Track DynamicToolParts by index for back-patching provider-executed results.
	type dynEntry struct {
		idx int
		dp  DynamicToolPart
	}
	toolCallDyns := make(map[string]*dynEntry)

	for _, p := range ass.Parts {
		switch v := p.(type) {
		case TextPart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				Text             string          `json:"text"`
				State            string          `json:"state"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"text", v.Text, "done", v.ProviderMetadata})
			parts = append(parts, data)

		case ReasoningPart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				Text             string          `json:"text"`
				State            string          `json:"state"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"reasoning", v.Text, "done", v.ProviderMetadata})
			parts = append(parts, data)

		case FilePart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				URL              string          `json:"url"`
				MediaType        string          `json:"mediaType"`
				Filename         string          `json:"filename,omitempty"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"file", v.Data, v.MediaType, v.Filename, v.ProviderMetadata})
			parts = append(parts, data)

		case ToolCallPart:
			dp := DynamicToolPart{
				ToolName:         v.ToolName,
				ToolCallID:       v.ToolCallID,
				Input:            json.RawMessage(v.Input),
				ProviderExecuted: v.ProviderExecuted,
				State:            "input-available",
			}

			// Attach approval if present and derive state.
			if ar, ok := approvalRequests[v.ToolCallID]; ok {
				dp.Approval = &ToolApproval{ID: ar.ApprovalID}
				if resp, ok := approvalResponses[ar.ApprovalID]; ok {
					dp.Approval.Approved = &resp.Approved
					dp.Approval.Reason = resp.Reason
				} else {
					// Approval requested but no response yet.
					dp.State = "approval-requested"
				}
			}

			// Apply tool result from the tool message (non-provider-executed).
			if result, ok := toolResults[v.ToolCallID]; ok {
				applyToolResultToDynamicPart(&dp, result)
			}

			data, err := marshalDynamicToolPart(dp)
			if err != nil {
				return nil, err
			}
			idx := len(parts)
			parts = append(parts, data)
			toolCallDyns[v.ToolCallID] = &dynEntry{idx: idx, dp: dp}

		case ToolResultPart:
			// Provider-executed tool results appear in the assistant message.
			if entry, ok := toolCallDyns[v.ToolCallID]; ok {
				applyToolResultToDynamicPart(&entry.dp, v)
				data, err := marshalDynamicToolPart(entry.dp)
				if err != nil {
					return nil, err
				}
				parts[entry.idx] = data
			}

		case ToolApprovalRequest:
			// Already handled when processing ToolCallPart.

		case SourceURLPart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				SourceID         string          `json:"sourceId"`
				URL              string          `json:"url"`
				Title            string          `json:"title,omitempty"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"source-url", v.SourceID, v.URL, v.Title, v.ProviderMetadata})
			parts = append(parts, data)

		case SourceDocumentPart:
			data, _ := json.Marshal(struct {
				Type             string          `json:"type"`
				SourceID         string          `json:"sourceId"`
				MediaType        string          `json:"mediaType"`
				Title            string          `json:"title"`
				Filename         string          `json:"filename,omitempty"`
				ProviderMetadata json.RawMessage `json:"providerMetadata,omitempty"`
			}{"source-document", v.SourceID, v.MediaType, v.Title, v.Filename, v.ProviderMetadata})
			parts = append(parts, data)

		case DataPart:
			data, _ := json.Marshal(struct {
				Type string          `json:"type"`
				ID   string          `json:"id,omitempty"`
				Data json.RawMessage `json:"data"`
			}{"data-" + v.DataType, v.ID, v.Data})
			parts = append(parts, data)
		}
	}

	return parts, nil
}

func applyToolResultToDynamicPart(dp *DynamicToolPart, result ToolResultPart) {
	if result.Output == nil {
		return
	}
	switch o := result.Output.(type) {
	case JSONOutput:
		dp.State = "output-available"
		dp.Output = o.Value
	case TextOutput:
		dp.State = "output-available"
		data, _ := json.Marshal(o.Value)
		dp.Output = data
	case ErrorTextOutput:
		dp.State = "output-error"
		dp.ErrorText = o.Value
	case ErrorJSONOutput:
		dp.State = "output-error"
		dp.ErrorText = string(o.Value)
	case ExecutionDeniedOutput:
		dp.State = "output-denied"
	default:
		dp.State = "output-available"
		data, _ := MarshalToolResultOutput(result.Output)
		dp.Output = data
	}
}

func marshalDynamicToolPart(dp DynamicToolPart) (json.RawMessage, error) {
	data, err := json.Marshal(struct {
		Type string `json:"type"`
		DynamicToolPart
	}{"dynamic-tool", dp})
	if err != nil {
		return nil, fmt.Errorf("marshal DynamicToolPart: %w", err)
	}
	return data, nil
}
