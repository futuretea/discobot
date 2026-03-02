package modelsdev

import "testing"

func TestLookup(t *testing.T) {
	t.Run("known model", func(t *testing.T) {
		m := Lookup("openai", "gpt-4o")
		if m == nil {
			t.Fatal("expected gpt-4o to be found")
		}
		if m.Name == "" {
			t.Error("expected non-empty name")
		}
		if m.ContextWindow == 0 {
			t.Error("expected non-zero context window")
		}
		if m.MaxOutputTokens == 0 {
			t.Error("expected non-zero max output tokens")
		}
	})

	t.Run("reasoning model", func(t *testing.T) {
		m := Lookup("openai", "o3")
		if m == nil {
			t.Fatal("expected o3 to be found")
		}
		if !m.Reasoning {
			t.Error("expected o3 to have Reasoning=true")
		}
	})

	t.Run("unknown model", func(t *testing.T) {
		m := Lookup("openai", "nonexistent-model")
		if m != nil {
			t.Error("expected nil for unknown model")
		}
	})

	t.Run("unknown provider", func(t *testing.T) {
		m := Lookup("nonexistent-provider", "gpt-4o")
		if m != nil {
			t.Error("expected nil for unknown provider")
		}
	})
}

func TestAllForProvider(t *testing.T) {
	t.Run("returns models for known provider", func(t *testing.T) {
		models := AllForProvider("openai")
		if len(models) == 0 {
			t.Fatal("expected models for openai")
		}
		// Verify at least one model has full metadata.
		found := false
		for _, m := range models {
			if m.ID == "gpt-4o" && m.ContextWindow > 0 {
				found = true
			}
		}
		if !found {
			t.Error("expected gpt-4o with context window in results")
		}
	})

	t.Run("returns nil for unknown provider", func(t *testing.T) {
		models := AllForProvider("nonexistent")
		if models != nil {
			t.Errorf("expected nil, got %d models", len(models))
		}
	})
}
