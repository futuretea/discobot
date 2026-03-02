package message

import "encoding/json"

// ProjectProviderMessages converts Messages for provider consumption.
// It filters out UI-only parts and returns the messages ready for
// serialization with MarshalProviderJSON.
//
// Since Message already stores data in the provider-native format
// (with "tool" role, ToolCallPart, ToolResultPart), this is mostly
// a filtering operation.
func ProjectProviderMessages(messages []Message) []Message {
	result := make([]Message, 0, len(messages))
	for _, msg := range messages {
		projected := projectProviderMessage(msg)
		if projected != nil {
			result = append(result, *projected)
		}
	}
	return result
}

func projectProviderMessage(msg Message) *Message {
	var parts []Part
	for _, p := range msg.Parts {
		if !isUIOnlyPart(p) {
			parts = append(parts, p)
		}
	}
	if len(parts) == 0 && msg.Role != "system" {
		return nil
	}
	return &Message{
		Role:            msg.Role,
		Parts:           parts,
		ProviderOptions: msg.ProviderOptions,
	}
}

// MarshalProviderMessages serializes a slice of Messages in the provider
// wire format as a JSON array. UI-only parts are excluded.
func MarshalProviderMessages(messages []Message) ([]byte, error) {
	projected := ProjectProviderMessages(messages)
	items := make([]json.RawMessage, len(projected))
	for i, msg := range projected {
		data, err := msg.MarshalProviderJSON()
		if err != nil {
			return nil, err
		}
		items[i] = data
	}
	return json.Marshal(items)
}
