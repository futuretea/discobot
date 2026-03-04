package sessionconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServerConfig represents a single MCP server definition from .mcp.json.
type MCPServerConfig struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"` // "stdio", "sse", "http"
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	OAuth     *MCPOAuthConfig   `json:"oauth,omitempty"`
}

// MCPOAuthConfig holds OAuth settings for an HTTP/SSE MCP server.
type MCPOAuthConfig struct {
	// DynamicRegistration enables RFC 7591 dynamic client registration.
	// When true, the agent registers itself with the authorization server at runtime.
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`

	// ClientID and ClientSecret are used for pre-registered OAuth clients.
	ClientID     string `json:"clientId,omitempty"`
	ClientSecret string `json:"clientSecret,omitempty"`

	// ClientMetadataURI enables Client ID Metadata Document-based registration
	// per the MCP 2025-11-25 specification.
	ClientMetadataURI string `json:"clientMetadataUri,omitempty"`

	// Scopes is an optional list of OAuth scopes to request.
	// If empty, scopes are discovered from the authorization server metadata.
	Scopes []string `json:"scopes,omitempty"`
}

// mcpFileSchema matches the structure of a .mcp.json file.
// The top-level key "mcpServers" maps server names to their config.
type mcpFileSchema struct {
	MCPServers map[string]mcpServerEntry `json:"mcpServers"`
}

// mcpServerEntry is a single server entry within .mcp.json.
type mcpServerEntry struct {
	// Stdio transport fields.
	Command string   `json:"command,omitempty"`
	Args    []string `json:"args,omitempty"`

	// HTTP/SSE transport fields.
	URL string `json:"url,omitempty"`

	// Shared fields.
	Type  string            `json:"type,omitempty"` // "stdio" (default), "sse", "http"
	Env   map[string]string `json:"env,omitempty"`
	OAuth *MCPOAuthConfig   `json:"oauth,omitempty"`
}

// discoverMCPServers loads MCP server definitions from .mcp.json files.
// It checks the project root and ~/.claude/.mcp.json.
func discoverMCPServers(projectRoot string) ([]MCPServerConfig, error) {
	var servers []MCPServerConfig

	// 1. Project-level .mcp.json
	projectMCP := filepath.Join(projectRoot, ".mcp.json")
	s, err := parseMCPFile(projectMCP)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", projectMCP, err)
	}
	servers = append(servers, s...)

	// 2. User-level ~/.claude/.mcp.json
	if home, err := os.UserHomeDir(); err == nil {
		userMCP := filepath.Join(home, ".claude", ".mcp.json")
		s, err := parseMCPFile(userMCP)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", userMCP, err)
		}
		servers = append(servers, s...)
	}

	return servers, nil
}

// parseMCPFile reads and parses a single .mcp.json file.
// Returns nil if the file does not exist.
func parseMCPFile(path string) ([]MCPServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var schema mcpFileSchema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	var servers []MCPServerConfig
	for name, entry := range schema.MCPServers {
		transport := entry.Type
		if transport == "" {
			if entry.Command != "" {
				transport = "stdio"
			} else if entry.URL != "" {
				transport = "sse"
			}
		}

		servers = append(servers, MCPServerConfig{
			Name:      name,
			Transport: transport,
			Command:   entry.Command,
			Args:      entry.Args,
			URL:       entry.URL,
			Env:       expandEnvVars(entry.Env),
			OAuth:     entry.OAuth,
		})
	}

	return servers, nil
}

// expandEnvVars replaces ${VAR} patterns in env values with os.Getenv values.
func expandEnvVars(env map[string]string) map[string]string {
	if len(env) == 0 {
		return env
	}
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = os.Expand(v, func(key string) string {
			// os.Expand handles ${VAR} and $VAR patterns.
			// Only expand from environment, don't use the input map.
			return os.Getenv(key)
		})
	}
	return result
}
