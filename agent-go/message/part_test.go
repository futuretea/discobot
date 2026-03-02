package message

import (
	"encoding/json"
	"testing"
)

func partRoundTrip(t *testing.T, name string, p Part) {
	t.Helper()
	data, err := MarshalPart(p)
	if err != nil {
		t.Fatalf("%s: marshal error: %v", name, err)
	}
	got, err := UnmarshalPart(data)
	if err != nil {
		t.Fatalf("%s: unmarshal error: %v", name, err)
	}
	data2, err := MarshalPart(got)
	if err != nil {
		t.Fatalf("%s: re-marshal error: %v", name, err)
	}
	if string(data) != string(data2) {
		t.Errorf("%s: round-trip mismatch\n  got:  %s\n  want: %s", name, data2, data)
	}
}

func TestPartRoundTrip_Text(t *testing.T) {
	partRoundTrip(t, "TextPart", TextPart{
		Text:             "hello",
		State:            "done",
		ProviderMetadata: json.RawMessage(`{"k":"v"}`),
		ProviderOptions:  json.RawMessage(`{"o":"p"}`),
	})
}

func TestPartRoundTrip_Reasoning(t *testing.T) {
	partRoundTrip(t, "ReasoningPart", ReasoningPart{
		Text:  "thinking...",
		State: "streaming",
	})
}

func TestPartRoundTrip_Image(t *testing.T) {
	partRoundTrip(t, "ImagePart", ImagePart{
		Image:     "base64data",
		MediaType: "image/png",
	})
}

func TestPartRoundTrip_File(t *testing.T) {
	partRoundTrip(t, "FilePart", FilePart{
		Data:      "data:image/png;base64,abc",
		MediaType: "image/png",
		Filename:  "test.png",
	})
}

func TestPartRoundTrip_ToolCall(t *testing.T) {
	boolTrue := true
	partRoundTrip(t, "ToolCallPart", ToolCallPart{
		ToolCallID:       "tc1",
		ToolName:         "read",
		Input:            json.RawMessage(`{"path":"foo"}`),
		ProviderExecuted: &boolTrue,
	})
}

func TestPartRoundTrip_ToolResult(t *testing.T) {
	partRoundTrip(t, "ToolResultPart/text", ToolResultPart{
		ToolCallID: "tc1",
		ToolName:   "read",
		Output:     TextOutput{Value: "file contents"},
	})
	partRoundTrip(t, "ToolResultPart/json", ToolResultPart{
		ToolCallID: "tc2",
		ToolName:   "search",
		Output:     JSONOutput{Value: json.RawMessage(`[1,2,3]`)},
	})
	partRoundTrip(t, "ToolResultPart/error-text", ToolResultPart{
		ToolCallID: "tc3",
		ToolName:   "exec",
		Output:     ErrorTextOutput{Value: "command failed"},
	})
	partRoundTrip(t, "ToolResultPart/denied", ToolResultPart{
		ToolCallID: "tc4",
		ToolName:   "rm",
		Output:     ExecutionDeniedOutput{Reason: "too dangerous"},
	})
}

func TestPartRoundTrip_ToolApproval(t *testing.T) {
	partRoundTrip(t, "ToolApprovalRequest", ToolApprovalRequest{
		ApprovalID: "a1",
		ToolCallID: "tc1",
	})
	partRoundTrip(t, "ToolApprovalResponse", ToolApprovalResponse{
		ApprovalID: "a1",
		Approved:   true,
		Reason:     "looks safe",
	})
}

func TestPartRoundTrip_Source(t *testing.T) {
	partRoundTrip(t, "SourceURLPart", SourceURLPart{
		SourceID: "s1",
		URL:      "https://example.com",
		Title:    "Example",
	})
	partRoundTrip(t, "SourceDocumentPart", SourceDocumentPart{
		SourceID:  "s2",
		MediaType: "application/pdf",
		Title:     "Doc",
		Filename:  "test.pdf",
	})
}

func TestPartRoundTrip_StepStart(t *testing.T) {
	partRoundTrip(t, "StepStartPart", StepStartPart{})
}

func TestPartRoundTrip_Data(t *testing.T) {
	partRoundTrip(t, "DataPart", DataPart{
		DataType: "custom",
		ID:       "d1",
		Data:     json.RawMessage(`{"value":42}`),
	})
}

func TestPartTypeField(t *testing.T) {
	data, err := MarshalPart(TextPart{Text: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	if raw["type"] != "text" {
		t.Errorf("expected type=text, got %v", raw["type"])
	}
}

func TestUnmarshalPartUnknownType(t *testing.T) {
	_, err := UnmarshalPart([]byte(`{"type":"unknown-type"}`))
	if err == nil {
		t.Error("expected error for unknown type")
	}
}
