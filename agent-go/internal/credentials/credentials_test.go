package credentials

import (
	"os"
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

	// First update should detect change.
	if !mgr.update(creds) {
		t.Error("first update should return true (changed)")
	}

	// Same credentials again should not detect change.
	if mgr.update(creds) {
		t.Error("second update with same creds should return false")
	}

	// Different credentials should detect change.
	creds2 := []EnvVar{{EnvVar: "KEY", Value: "new-val", Provider: "p", AuthType: "api_key"}}
	if !mgr.update(creds2) {
		t.Error("update with different creds should return true")
	}
}

func TestApplyEnv(t *testing.T) {
	key := "DISCOBOT_TEST_CRED_" + t.Name()
	defer os.Unsetenv(key)

	creds := []EnvVar{{EnvVar: key, Value: "test-value", Provider: "test", AuthType: "api_key"}}
	applyEnv(creds)

	if got := os.Getenv(key); got != "test-value" {
		t.Errorf("os.Getenv(%q) = %q, want %q", key, got, "test-value")
	}
}

func TestApplyEnvSkipsEmpty(t *testing.T) {
	creds := []EnvVar{
		{EnvVar: "", Value: "val", Provider: "p", AuthType: "api_key"},
		{EnvVar: "KEY", Value: "", Provider: "p", AuthType: "api_key"},
	}
	// Should not panic or set anything.
	applyEnv(creds)
}
