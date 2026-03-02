package credentials

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"sort"
	"sync"
)

// EnvVar represents a credential mapped to an environment variable.
type EnvVar struct {
	EnvVar    string `json:"envVar"`
	Value     string `json:"value"`
	Provider  string `json:"provider"`
	AuthType  string `json:"authType"`            // "api_key" or "oauth"
	ExpiresAt *int64 `json:"expiresAt,omitempty"` // OAuth only (unix timestamp)
}

// Manager tracks credentials and detects changes.
type Manager struct {
	mu   sync.Mutex
	hash string
}

// NewManager creates a new credential Manager.
func NewManager() *Manager {
	return &Manager{}
}

// Apply parses the credentials header, detects changes, and applies them.
// It sets environment variables for credential values and configures git user if provided.
func (m *Manager) Apply(credentialsHeader, gitUserName, gitUserEmail string) {
	if credentialsHeader != "" {
		creds := parseHeader(credentialsHeader)
		if m.update(creds) {
			applyEnv(creds)
		}
	}

	configureGitUser(gitUserName, gitUserEmail)
}

// parseHeader parses the X-Discobot-Credentials JSON array header.
func parseHeader(headerValue string) []EnvVar {
	if headerValue == "" {
		return nil
	}

	var creds []EnvVar
	if err := json.Unmarshal([]byte(headerValue), &creds); err != nil {
		log.Printf("credentials: failed to parse header: %v", err)
		return nil
	}
	return creds
}

// update checks if credentials changed and updates the stored hash.
// Returns true if credentials changed.
func (m *Manager) update(creds []EnvVar) bool {
	newHash := computeHash(creds)

	m.mu.Lock()
	defer m.mu.Unlock()

	if newHash == m.hash {
		return false
	}

	m.hash = newHash
	return true
}

// computeHash computes a SHA-256 hash of credentials for change detection.
func computeHash(creds []EnvVar) string {
	// Sort by envVar for consistent ordering.
	sorted := make([]EnvVar, len(creds))
	copy(sorted, creds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].EnvVar < sorted[j].EnvVar
	})

	data, err := json.Marshal(sorted)
	if err != nil {
		return ""
	}

	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// applyEnv sets environment variables from credentials.
func applyEnv(creds []EnvVar) {
	for _, c := range creds {
		if c.EnvVar != "" && c.Value != "" {
			os.Setenv(c.EnvVar, c.Value)
		}
	}
}

// configureGitUser sets git global user.name and user.email if provided.
func configureGitUser(name, email string) {
	if name != "" {
		if err := exec.Command("git", "config", "--global", "user.name", name).Run(); err != nil {
			log.Printf("credentials: failed to set git user.name: %v", err)
		}
	}
	if email != "" {
		if err := exec.Command("git", "config", "--global", "user.email", email).Run(); err != nil {
			log.Printf("credentials: failed to set git user.email: %v", err)
		}
	}
}
