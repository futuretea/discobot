package message

import (
	"encoding/json"
	"testing"
)

func TestMessage_InternalRoundTrip(t *testing.T) {
	msg := Message{
		ID:   "msg1",
		Role: "assistant",
		Parts: []Part{
			TextPart{Text: "hello"},
			ToolCallPart{ToolCallID: "tc1", ToolName: "read", Input: json.RawMessage(`{}`)},
		},
		Metadata: json.RawMessage(`{"usage":100}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}

	var got Message
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}

	if got.ID != msg.ID {
		t.Errorf("ID: got %q, want %q", got.ID, msg.ID)
	}
	if got.Role != msg.Role {
		t.Errorf("Role: got %q, want %q", got.Role, msg.Role)
	}
	if len(got.Parts) != 2 {
		t.Fatalf("Parts: got %d, want 2", len(got.Parts))
	}
	if tp, ok := got.Parts[0].(TextPart); !ok || tp.Text != "hello" {
		t.Errorf("Parts[0]: got %v", got.Parts[0])
	}
	if tc, ok := got.Parts[1].(ToolCallPart); !ok || tc.ToolCallID != "tc1" {
		t.Errorf("Parts[1]: got %v", got.Parts[1])
	}
}

func TestMessage_SystemStringContent(t *testing.T) {
	// Provider format: system message with string content.
	data := []byte(`{"role":"system","content":"You are a helpful assistant."}`)

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatal(err)
	}

	if msg.Role != "system" {
		t.Errorf("Role: got %q", msg.Role)
	}
	if len(msg.Parts) != 1 {
		t.Fatalf("Parts: got %d, want 1", len(msg.Parts))
	}
	tp, ok := msg.Parts[0].(TextPart)
	if !ok {
		t.Fatalf("Parts[0] type: got %T", msg.Parts[0])
	}
	if tp.Text != "You are a helpful assistant." {
		t.Errorf("Text: got %q", tp.Text)
	}
}

func TestMessage_ProviderJSON_System(t *testing.T) {
	msg := Message{
		Role:  "system",
		Parts: []Part{TextPart{Text: "Be concise."}},
	}

	data, err := msg.MarshalProviderJSON()
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	json.Unmarshal(data, &raw)

	// Content should be a string, not an array.
	var content string
	if err := json.Unmarshal(raw["content"], &content); err != nil {
		t.Fatalf("content is not a string: %s", raw["content"])
	}
	if content != "Be concise." {
		t.Errorf("content: got %q", content)
	}

	// ID, Metadata, CreatedAt should not be present.
	if _, ok := raw["id"]; ok {
		t.Error("id should not be in provider JSON")
	}
}

func TestMessage_ProviderJSON_FiltersUIOnlyParts(t *testing.T) {
	msg := Message{
		Role: "assistant",
		Parts: []Part{
			TextPart{Text: "found it"},
			SourceURLPart{SourceID: "s1", URL: "https://example.com"},
			StepStartPart{},
			DataPart{DataType: "foo", Data: json.RawMessage(`{}`)},
		},
	}

	data, err := msg.MarshalProviderJSON()
	if err != nil {
		t.Fatal(err)
	}

	var raw struct {
		Content []json.RawMessage `json:"content"`
	}
	json.Unmarshal(data, &raw)

	// Only the TextPart should survive.
	if len(raw.Content) != 1 {
		t.Fatalf("content length: got %d, want 1", len(raw.Content))
	}

	var disc struct {
		Type string `json:"type"`
	}
	json.Unmarshal(raw.Content[0], &disc)
	if disc.Type != "text" {
		t.Errorf("content[0] type: got %q, want text", disc.Type)
	}
}

func TestMessage_ProviderJSON_RoundTrip(t *testing.T) {
	msg := Message{
		Role: "user",
		Parts: []Part{
			TextPart{Text: "hello"},
			FilePart{Data: "base64data", MediaType: "image/png"},
		},
	}

	data, err := msg.MarshalProviderJSON()
	if err != nil {
		t.Fatal(err)
	}

	var got Message
	if err := got.UnmarshalProviderJSON(data); err != nil {
		t.Fatal(err)
	}

	if got.Role != "user" {
		t.Errorf("Role: got %q", got.Role)
	}
	if len(got.Parts) != 2 {
		t.Fatalf("Parts: got %d, want 2", len(got.Parts))
	}
}
