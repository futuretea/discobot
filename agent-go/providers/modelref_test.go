package providers

import "testing"

func TestParseModelRef_Valid(t *testing.T) {
	ref, err := ParseModelRef("anthropic/claude-sonnet-4-20250514")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ProviderID != "anthropic" {
		t.Errorf("expected provider 'anthropic', got %q", ref.ProviderID)
	}
	if ref.ModelID != "claude-sonnet-4-20250514" {
		t.Errorf("expected model 'claude-sonnet-4-20250514', got %q", ref.ModelID)
	}
}

func TestParseModelRef_MultipleSlashes(t *testing.T) {
	// Only split on first "/".
	ref, err := ParseModelRef("openai/gpt-4/turbo")
	if err != nil {
		t.Fatal(err)
	}
	if ref.ProviderID != "openai" {
		t.Errorf("expected provider 'openai', got %q", ref.ProviderID)
	}
	if ref.ModelID != "gpt-4/turbo" {
		t.Errorf("expected model 'gpt-4/turbo', got %q", ref.ModelID)
	}
}

func TestParseModelRef_NoSlash(t *testing.T) {
	_, err := ParseModelRef("just-a-model")
	if err == nil {
		t.Error("expected error for missing slash")
	}
}

func TestParseModelRef_EmptyProvider(t *testing.T) {
	_, err := ParseModelRef("/model-id")
	if err == nil {
		t.Error("expected error for empty provider")
	}
}

func TestParseModelRef_EmptyModel(t *testing.T) {
	_, err := ParseModelRef("provider/")
	if err == nil {
		t.Error("expected error for empty model")
	}
}

func TestParseModelRef_Empty(t *testing.T) {
	_, err := ParseModelRef("")
	if err == nil {
		t.Error("expected error for empty string")
	}
}

func TestModelRef_String(t *testing.T) {
	ref := ModelRef{ProviderID: "anthropic", ModelID: "claude-sonnet-4-20250514"}
	if s := ref.String(); s != "anthropic/claude-sonnet-4-20250514" {
		t.Errorf("expected 'anthropic/claude-sonnet-4-20250514', got %q", s)
	}
}
