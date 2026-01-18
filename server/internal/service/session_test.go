package service

import (
	"strings"
	"testing"
)

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid alphanumeric",
			sessionID: "abc123",
			wantErr:   false,
		},
		{
			name:      "valid with hyphens",
			sessionID: "session-123-abc",
			wantErr:   false,
		},
		{
			name:      "valid UUID format",
			sessionID: "550e8400-e29b-41d4-a716-446655440000",
			wantErr:   false,
		},
		{
			name:      "valid at max length (65 chars)",
			sessionID: strings.Repeat("a", 65),
			wantErr:   false,
		},
		{
			name:      "empty string",
			sessionID: "",
			wantErr:   true,
			errMsg:    "session ID is required",
		},
		{
			name:      "exceeds max length (66 chars)",
			sessionID: strings.Repeat("a", 66),
			wantErr:   true,
			errMsg:    "exceeds maximum length",
		},
		{
			name:      "contains underscore",
			sessionID: "session_123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains space",
			sessionID: "session 123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains special characters",
			sessionID: "session@123!",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains dot",
			sessionID: "session.123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "contains slash",
			sessionID: "session/123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
		{
			name:      "only hyphens",
			sessionID: "---",
			wantErr:   false,
		},
		{
			name:      "single character",
			sessionID: "a",
			wantErr:   false,
		},
		{
			name:      "contains newline",
			sessionID: "session\n123",
			wantErr:   true,
			errMsg:    "must contain only alphanumeric characters and hyphens",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSessionID(tt.sessionID)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateSessionID(%q) expected error, got nil", tt.sessionID)
					return
				}
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateSessionID(%q) error = %v, expected to contain %q", tt.sessionID, err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateSessionID(%q) unexpected error: %v", tt.sessionID, err)
				}
			}
		})
	}
}

func TestSessionIDMaxLength(t *testing.T) {
	// Verify the constant is set to 65
	if SessionIDMaxLength != 65 {
		t.Errorf("SessionIDMaxLength = %d, want 65", SessionIDMaxLength)
	}
}
