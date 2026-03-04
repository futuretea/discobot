package credentials

import (
	"testing"
)

func TestParseHeader(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"empty", "", 0},
		{"valid array", `[{"envVar":"API_KEY","value":"sk-123","provider":"anthropic","authType":"api_key"}]`, 1},
		{"multiple", `[{"envVar":"A","value":"1","provider":"p","authType":"api_key"},{"envVar":"B","value":"2","provider":"q","authType":"oauth"}]`, 2},
		{"invalid json", `not-json`, 0},
		{"not array", `{"envVar":"A"}`, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseHeader(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseHeader(%q) returned %d creds, want %d", tt.input, len(got), tt.want)
			}
		})
	}
}

func TestManagerChangeDetection(t *testing.T) {
	mgr := NewManager()

	creds := []EnvVar{{EnvVar: "KEY", Value: "val", Provider: "p", AuthType: "api_key"}}

	// First update should store the creds.
	mgr.update(creds)
	if mgr.Get("KEY") == nil {
		t.Error("expected KEY to be stored after first update")
	}

	// Same credentials again: value unchanged.
	mgr.update(creds)
	if mgr.Get("KEY").Value != "val" {
		t.Error("value should remain 'val'")
	}

	// Different credentials should replace the stored set.
	creds2 := []EnvVar{{EnvVar: "KEY", Value: "new-val", Provider: "p", AuthType: "api_key"}}
	mgr.update(creds2)
	if mgr.Get("KEY").Value != "new-val" {
		t.Errorf("expected 'new-val', got %q", mgr.Get("KEY").Value)
	}
}

func TestManagerGet(t *testing.T) {
	mgr := NewManager()
	mgr.update([]EnvVar{
		{EnvVar: "ANTHROPIC_API_KEY", Value: "sk-123", Provider: "anthropic", AuthType: "api_key"},
		{EnvVar: "OPENAI_API_KEY", Value: "sk-456", Provider: "openai", AuthType: "api_key"},
	})

	c := mgr.Get("ANTHROPIC_API_KEY")
	if c == nil {
		t.Fatal("expected credential for ANTHROPIC_API_KEY")
	}
	if c.Value != "sk-123" {
		t.Errorf("expected 'sk-123', got %q", c.Value)
	}

	if mgr.Get("MISSING") != nil {
		t.Error("expected nil for unknown key")
	}
}

func TestManagerForProvider(t *testing.T) {
	mgr := NewManager()
	mgr.update([]EnvVar{
		{EnvVar: "ANTHROPIC_API_KEY", Value: "sk-123", Provider: "anthropic", AuthType: "api_key"},
		{EnvVar: "CLAUDE_OAUTH_TOKEN", Value: "tok-abc", Provider: "anthropic", AuthType: "oauth"},
		{EnvVar: "OPENAI_API_KEY", Value: "sk-456", Provider: "openai", AuthType: "api_key"},
	})

	anthropicCreds := mgr.ForProvider("anthropic")
	if len(anthropicCreds) != 2 {
		t.Fatalf("expected 2 anthropic credentials, got %d", len(anthropicCreds))
	}

	openaiCreds := mgr.ForProvider("openai")
	if len(openaiCreds) != 1 {
		t.Fatalf("expected 1 openai credential, got %d", len(openaiCreds))
	}

	if mgr.ForProvider("unknown") != nil {
		t.Error("expected nil for unknown provider")
	}
}
