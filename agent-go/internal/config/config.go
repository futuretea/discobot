package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds all configuration for the agent-go process.
type Config struct {
	// Server settings (HTTP API mode)
	Port       int    // HTTP server port (default: 3002)
	SecretHash string // Auth secret for API access (DISCOBOT_SECRET)

	// Agent settings
	AgentCwd string // Working directory for the agent (default: cwd)
	Model    string // Default model in "providerId/modelId" format

	// Storage
	DataDir    string // Root data directory (default: ~/.discobot)
	ThreadsDir string // Thread persistence directory

	// Hooks
	HooksEnabled bool   // Enable file hooks (DISCOBOT_HOOKS_ENABLED)
	SessionID    string // Session ID for hooks (default: "default")

	// Idle timeout
	IdleTimeout time.Duration // Exit after idle period with no active completions (0 = disabled)

	// MCP OAuth settings
	MCPOAuthRedirectBase string // Base URL for OAuth callbacks (MCP_OAUTH_REDIRECT_BASE)
	DiscobotServerURL    string // Discobot server URL for posting tokens (DISCOBOT_SERVER_URL)
	DiscobotProjectID    string // Project ID for the token POST path (DISCOBOT_PROJECT_ID)
}

// Load reads configuration from environment variables.
// Call godotenv.Load() before this to support .env files.
func Load() *Config {
	cwd, _ := os.Getwd()

	cfg := &Config{}

	// Server
	cfg.Port = getEnvInt("PORT", 3002)
	cfg.SecretHash = getEnv("DISCOBOT_SECRET", "")

	// Agent
	cfg.AgentCwd = getEnv("AGENT_CWD", cwd)
	cfg.Model = getEnv("MODEL", "")

	// Storage — default to ~/.discobot
	home, _ := os.UserHomeDir()
	if home == "" {
		home = cwd
	}
	cfg.DataDir = getEnv("DATA_DIR", filepath.Join(home, ".discobot"))
	cfg.ThreadsDir = getEnv("THREADS_DIR", filepath.Join(cfg.DataDir, "threads"))

	// Hooks
	cfg.HooksEnabled = getEnvBool("DISCOBOT_HOOKS_ENABLED", false)
	cfg.SessionID = getEnv("SESSION_ID", "default")

	// Idle timeout
	cfg.IdleTimeout = getEnvDuration("IDLE_TIMEOUT", 0)

	// MCP OAuth
	cfg.MCPOAuthRedirectBase = getEnv("MCP_OAUTH_REDIRECT_BASE", "")
	cfg.DiscobotServerURL = getEnv("DISCOBOT_SERVER_URL", "")
	cfg.DiscobotProjectID = getEnv("DISCOBOT_PROJECT_ID", "")

	return cfg
}

// ProviderConfig holds configuration for a single LLM provider.
type ProviderConfig struct {
	APIKey  string
	BaseURL string
}

// ProviderConfigs reads provider configuration from environment variables.
// For each provider ID, it checks {UPPER_ID}_API_KEY and optionally {UPPER_ID}_BASE_URL.
// Returns a map of provider ID → ProviderConfig for providers that have an API key set.
func ProviderConfigs(providerIDs []string) map[string]ProviderConfig {
	configs := make(map[string]ProviderConfig)
	for _, id := range providerIDs {
		prefix := strings.ToUpper(id) + "_"
		apiKey := os.Getenv(prefix + "API_KEY")
		if apiKey == "" {
			continue
		}
		pc := ProviderConfig{APIKey: apiKey}
		if baseURL := os.Getenv(prefix + "BASE_URL"); baseURL != "" {
			pc.BaseURL = baseURL
		}
		configs[id] = pc
	}
	return configs
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if i, err := strconv.Atoi(value); err == nil {
			return i
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if d, err := time.ParseDuration(value); err == nil {
			return d
		}
	}
	return defaultValue
}
