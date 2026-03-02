package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
)

func TestVerifySecret(t *testing.T) {
	// Create a known salt:hash pair.
	salt := []byte("testsalt12345678")
	saltHex := hex.EncodeToString(salt)
	plaintext := "my-secret-token"

	h := sha256.New()
	h.Write(salt)
	h.Write([]byte(plaintext))
	hashHex := hex.EncodeToString(h.Sum(nil))

	secretHash := saltHex + ":" + hashHex

	tests := []struct {
		name   string
		token  string
		hash   string
		expect bool
	}{
		{"valid token", plaintext, secretHash, true},
		{"wrong token", "wrong-token", secretHash, false},
		{"empty token", "", secretHash, false},
		{"empty hash", plaintext, "", false},
		{"no colon", plaintext, "nocolon", false},
		{"bad salt hex", plaintext, "zzzz:" + hashHex, false},
		{"wrong hash", plaintext, saltHex + ":0000000000000000000000000000000000000000000000000000000000000000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := verifySecret(tt.token, tt.hash)
			if got != tt.expect {
				t.Errorf("verifySecret(%q, %q) = %v, want %v", tt.token, tt.hash, got, tt.expect)
			}
		})
	}
}
