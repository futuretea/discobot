package sessionconfig

import (
	"log"

	"github.com/obot-platform/discobot/agent-go/providers"
)

// InstructionEntry represents a discovered instruction file with metadata.
type InstructionEntry struct {
	// Path is the display path (e.g., "CLAUDE.md", "~/.claude/CLAUDE.md").
	Path string

	// Description describes the source (e.g., "project instructions, checked into the codebase").
	Description string

	// Content is the file content.
	Content string
}

// SessionConfig holds the discovered session configuration.
type SessionConfig struct {
	// SystemPrompt is the default base system prompt (behavioral instructions).
	SystemPrompt string

	// UserInstructions are discovered CLAUDE.md, AGENTS.md, and rules files.
	// These are delivered separately from the system prompt in <system-reminder> tags.
	UserInstructions []InstructionEntry

	// Tools are the built-in tool definitions sent to the LLM provider.
	Tools []providers.ToolDefinition

	// MCPServers are parsed MCP server definitions from .mcp.json files.
	// These are not connected yet — just parsed for future use.
	MCPServers []MCPServerConfig

	// SubAgents are sub-agent configurations from .claude/agents/*.md.
	SubAgents []SubAgentConfig
}

// Load discovers and loads session configuration from the given working directory.
// Non-critical errors (missing optional files) are logged as warnings.
// Returns an error only for I/O failures on files that do exist.
func Load(cwd string) (*SessionConfig, error) {
	cfg := &SessionConfig{}

	// 1. Set the default base system prompt.
	cfg.SystemPrompt = defaultSystemPrompt()

	// 2. Discover user instruction files (CLAUDE.md, AGENTS.md, rules).
	entries, err := discoverInstructions(cwd)
	if err != nil {
		return nil, err
	}
	cfg.UserInstructions = entries

	// 3. Load built-in tool definitions.
	cfg.Tools = builtinTools()

	// 4. Discover MCP server configs.
	projectRoot := findProjectRoot(cwd)
	mcpServers, err := discoverMCPServers(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: MCP discovery: %v", err)
		// Non-fatal — continue without MCP servers.
	} else {
		cfg.MCPServers = mcpServers
	}

	// 5. Discover sub-agent configs.
	subAgents, err := discoverSubAgents(projectRoot)
	if err != nil {
		log.Printf("sessionconfig: warning: sub-agent discovery: %v", err)
		// Non-fatal — continue without sub-agents.
	} else {
		cfg.SubAgents = subAgents
	}

	return cfg, nil
}
