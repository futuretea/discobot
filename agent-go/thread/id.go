package thread

import (
	"crypto/rand"
	"encoding/hex"
)

// generateID returns a random 16-character hex ID.
func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
