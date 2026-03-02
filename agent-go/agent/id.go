package agent

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateID returns a random 16-character hex ID.
func GenerateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// generateID is an internal alias for GenerateID.
func generateID() string {
	return GenerateID()
}
