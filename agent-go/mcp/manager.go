package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	gosdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/providers"
	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

// ServerStatus is the connection state of an MCP server.
type ServerStatus string

const (
	ServerStatusConnecting ServerStatus = "connecting"
	ServerStatusConnected  ServerStatus = "connected"
	ServerStatusOAuth      ServerStatus = "oauth_required"
	ServerStatusError      ServerStatus = "error"
)

// ServerInfo describes the current state of an MCP server connection.
type ServerInfo struct {
	Name      string       `json:"name"`
	Status    ServerStatus `json:"status"`
	Error     string       `json:"error,omitempty"`
	ToolCount int          `json:"toolCount"`
	// OAuthURL is set when Status == ServerStatusOAuth.
	OAuthURL string `json:"oauthUrl,omitempty"`
}

// TokenCallback is invoked after a successful OAuth token exchange.
// resourceURL is the canonical URL of the MCP server. The implementation
// should persist the token to the Discobot server so it can be re-delivered
// on subsequent sessions via the X-Discobot-Credentials header.
type TokenCallback func(resourceURL string, token *oauth2.Token)

// OAuthToken is the structure of a single entry in MCP_OAUTH_TOKENS.
type OAuthToken struct {
	URL          string `json:"url"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // unix timestamp
}

// serverConn holds the state for one connected MCP server.
type serverConn struct {
	cfg     sessionconfig.MCPServerConfig
	fetcher *channelCodeFetcher // nil for stdio servers

	mu      sync.RWMutex
	status  ServerStatus
	errMsg  string
	session *gosdkmcp.ClientSession
	tools   []providers.ToolDefinition
}

// Manager manages connections to all MCP servers discovered from session config.
// Connections are established in background goroutines; the manager is safe
// for concurrent use after Connect returns.
type Manager struct {
	tokenCallback TokenCallback

	mu      sync.RWMutex
	servers map[string]*serverConn
}

// mcpTokenPayload matches the MCPTokenData struct expected by the server.
type mcpTokenPayload struct {
	URL          string `json:"url"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken,omitempty"`
	ExpiresAt    int64  `json:"expiresAt,omitempty"` // unix timestamp
}

// MakeTokenCallback creates a TokenCallback that persists MCP OAuth tokens to the
// Discobot server so they can be re-delivered on subsequent sessions.
// Returns nil if serverURL or projectID is empty (token persistence disabled).
func MakeTokenCallback(serverURL, projectID string) TokenCallback {
	if serverURL == "" || projectID == "" {
		return nil
	}
	endpoint := fmt.Sprintf("%s/api/projects/%s/credentials/mcp", serverURL, projectID)
	return func(resourceURL string, token *oauth2.Token) {
		payload := mcpTokenPayload{
			URL:          resourceURL,
			AccessToken:  token.AccessToken,
			RefreshToken: token.RefreshToken,
		}
		if !token.Expiry.IsZero() {
			payload.ExpiresAt = token.Expiry.Unix()
		}

		body, err := json.Marshal(payload)
		if err != nil {
			log.Printf("mcp: failed to marshal token for %s: %v", resourceURL, err)
			return
		}

		resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body)) //nolint:noctx
		if err != nil {
			log.Printf("mcp: failed to persist token for %s: %v", resourceURL, err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("mcp: server returned %d when persisting token for %s", resp.StatusCode, resourceURL)
			return
		}
		log.Printf("mcp: token for %s persisted to server", resourceURL)
	}
}

// NewManager creates a new Manager.
// tokenCallback is called (possibly nil-safe to skip persistence) after each
// successful OAuth token exchange.
func NewManager(tokenCallback TokenCallback) *Manager {
	return &Manager{
		tokenCallback: tokenCallback,
		servers:       make(map[string]*serverConn),
	}
}

// Connect starts background goroutines to connect to each configured MCP server.
// redirectURLBase and sessionID are used to build the OAuth callback URL:
//
//	{redirectURLBase}/sessions/{sessionID}/mcp/{serverName}/callback
//
// Connect returns immediately; use Status() to poll connection state.
func (m *Manager) Connect(
	ctx context.Context,
	servers []sessionconfig.MCPServerConfig,
	redirectURLBase string,
	sessionID string,
) {
	existingTokens := loadOAuthTokens()

	for _, cfg := range servers {
		if cfg.Name == "" {
			continue
		}

		sc := &serverConn{
			cfg:    cfg,
			status: ServerStatusConnecting,
		}

		m.mu.Lock()
		m.servers[cfg.Name] = sc
		m.mu.Unlock()

		redirectURL := ""
		if redirectURLBase != "" && sessionID != "" {
			redirectURL = fmt.Sprintf("%s/sessions/%s/mcp/%s/callback",
				strings.TrimRight(redirectURLBase, "/"), sessionID, cfg.Name)
		}

		go m.connectServer(ctx, sc, existingTokens, redirectURL)
	}
}

// connectServer establishes the connection to a single MCP server.
func (m *Manager) connectServer(
	ctx context.Context,
	sc *serverConn,
	existingTokens []OAuthToken,
	redirectURL string,
) {
	cfg := sc.cfg

	client := gosdkmcp.NewClient(&gosdkmcp.Implementation{
		Name:    "discobot-agent",
		Version: "1.0.0",
	}, nil)

	var transport gosdkmcp.Transport
	var connectErr error

	switch cfg.Transport {
	case "stdio":
		transport, connectErr = buildStdioTransport(cfg)
	case "http", "sse", "":
		if cfg.URL == "" {
			sc.setError("URL is required for HTTP/SSE transport")
			return
		}
		var fetcher *channelCodeFetcher
		var oauthHandler sdkauth.OAuthHandler
		var preseededClient *http.Client
		if cfg.OAuth != nil || redirectURL != "" {
			fetcher = newChannelCodeFetcher(redirectURL)
			sc.mu.Lock()
			sc.fetcher = fetcher
			sc.mu.Unlock()
			oauthHandler, preseededClient = newOAuthHandler(cfg.Name, cfg.URL, cfg.OAuth, fetcher, existingTokens,
				func(token *oauth2.Token) {
					if m.tokenCallback != nil {
						m.tokenCallback(cfg.URL, token)
					}
				})
		}
		transport = buildHTTPTransport(cfg, oauthHandler, preseededClient)
	default:
		sc.setError(fmt.Sprintf("unsupported transport: %q", cfg.Transport))
		return
	}

	if connectErr != nil {
		sc.setError(fmt.Sprintf("build transport: %v", connectErr))
		return
	}

	// Apply server-specific env vars before connecting.
	applyEnv(cfg.Env)

	// Connect with the MCP client.
	connectCtx := ctx
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		sc.setError(fmt.Sprintf("connect: %v", err))
		return
	}

	// Discover tools from the connected server.
	tools, err := discoverTools(ctx, sc.cfg.Name, session)
	if err != nil {
		log.Printf("mcp: %s: discover tools: %v", cfg.Name, err)
		// Non-fatal — server is connected but no tools.
	}

	sc.mu.Lock()
	sc.session = session
	sc.tools = tools
	sc.status = ServerStatusConnected
	sc.mu.Unlock()

	log.Printf("mcp: %s: connected (%d tools)", cfg.Name, len(tools))
}

// setError marks the server as errored.
func (sc *serverConn) setError(msg string) {
	sc.mu.Lock()
	sc.status = ServerStatusError
	sc.errMsg = msg
	sc.mu.Unlock()
	log.Printf("mcp: %s: error: %s", sc.cfg.Name, msg)
}

// buildStdioTransport creates a CommandTransport for a stdio MCP server.
func buildStdioTransport(cfg sessionconfig.MCPServerConfig) (gosdkmcp.Transport, error) {
	if cfg.Command == "" {
		return nil, fmt.Errorf("command is required for stdio transport")
	}
	//nolint:gosec // Command is from user-provided .mcp.json config.
	cmd := exec.Command(cfg.Command, cfg.Args...)
	// Pass through current environment plus server-specific overrides.
	cmd.Env = os.Environ()
	for k, v := range cfg.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	return &gosdkmcp.CommandTransport{Command: cmd}, nil
}

// buildHTTPTransport creates a StreamableClientTransport for an HTTP/SSE MCP server.
// If httpClient is non-nil it is used instead of the default client (for pre-seeded token injection).
func buildHTTPTransport(cfg sessionconfig.MCPServerConfig, oauthHandler sdkauth.OAuthHandler, httpClient *http.Client) gosdkmcp.Transport {
	t := &gosdkmcp.StreamableClientTransport{
		Endpoint:     cfg.URL,
		OAuthHandler: oauthHandler,
	}
	if httpClient != nil {
		t.HTTPClient = httpClient
	}
	return t
}

// applyEnv sets environment variables from the server's env map.
// For stdio servers this is done per-command via cmd.Env; for HTTP servers
// it's a process-level side effect (acceptable for token env vars).
func applyEnv(env map[string]string) {
	for k, v := range env {
		os.Setenv(k, v) //nolint:errcheck
	}
}

// discoverTools retrieves the tool list from a connected session.
// Tool names are prefixed as "serverName__toolName".
func discoverTools(ctx context.Context, serverName string, session *gosdkmcp.ClientSession) ([]providers.ToolDefinition, error) {
	result, err := session.ListTools(ctx, nil)
	if err != nil {
		return nil, err
	}

	tools := make([]providers.ToolDefinition, 0, len(result.Tools))
	for _, t := range result.Tools {
		schema, err := json.Marshal(t.InputSchema)
		if err != nil {
			log.Printf("mcp: %s: marshal input schema for %s: %v", serverName, t.Name, err)
			continue
		}
		tools = append(tools, providers.ToolDefinition{
			Name:        serverName + "__" + t.Name,
			Description: t.Description,
			InputSchema: json.RawMessage(schema),
		})
	}
	return tools, nil
}

// loadOAuthTokens parses the MCP_OAUTH_TOKENS environment variable.
// Returns nil if the variable is not set or cannot be parsed.
func loadOAuthTokens() []OAuthToken {
	raw := os.Getenv("MCP_OAUTH_TOKENS")
	if raw == "" {
		return nil
	}
	var tokens []OAuthToken
	if err := json.Unmarshal([]byte(raw), &tokens); err != nil {
		log.Printf("mcp: failed to parse MCP_OAUTH_TOKENS: %v", err)
		return nil
	}
	return tokens
}

// findToken returns the stored OAuth token whose URL matches the given server URL.
func findToken(tokens []OAuthToken, serverURL string) *OAuthToken {
	for i := range tokens {
		if tokens[i].URL == serverURL {
			return &tokens[i]
		}
	}
	return nil
}

// tokenFromEntry converts an OAuthToken to an oauth2.Token.
func tokenFromEntry(entry *OAuthToken) *oauth2.Token {
	t := &oauth2.Token{
		AccessToken:  entry.AccessToken,
		RefreshToken: entry.RefreshToken,
		TokenType:    "Bearer",
	}
	if entry.ExpiresAt > 0 {
		t.Expiry = time.Unix(entry.ExpiresAt, 0)
	}
	return t
}

// Tools returns the tool definitions from all currently-connected MCP servers.
// Tools are named "serverName__toolName".
func (m *Manager) Tools() []providers.ToolDefinition {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []providers.ToolDefinition
	for _, sc := range m.servers {
		sc.mu.RLock()
		if sc.status == ServerStatusConnected {
			all = append(all, sc.tools...)
		}
		sc.mu.RUnlock()
	}
	return all
}

// CallTool calls a tool on the appropriate MCP server.
// toolFullName must be in "serverName__toolName" format.
func (m *Manager) CallTool(
	ctx context.Context,
	toolFullName string,
	input json.RawMessage,
	toolCallID string,
) (message.ToolResultPart, error) {
	serverName, toolName, ok := splitToolName(toolFullName)
	if !ok {
		return message.ToolResultPart{}, fmt.Errorf("invalid MCP tool name %q (expected servername__toolname)", toolFullName)
	}

	m.mu.RLock()
	sc, exists := m.servers[serverName]
	m.mu.RUnlock()
	if !exists {
		return message.ToolResultPart{}, fmt.Errorf("unknown MCP server %q", serverName)
	}

	sc.mu.RLock()
	status := sc.status
	session := sc.session
	sc.mu.RUnlock()

	if status != ServerStatusConnected || session == nil {
		return message.ToolResultPart{
			ToolCallID: toolCallID,
			ToolName:   toolFullName,
			Output:     message.ErrorTextOutput{Value: fmt.Sprintf("MCP server %q is not connected (status: %s)", serverName, status)},
		}, nil
	}

	// Unmarshal input to map for the MCP call.
	var args map[string]any
	if len(input) > 0 && string(input) != "null" {
		if err := json.Unmarshal(input, &args); err != nil {
			return message.ToolResultPart{}, fmt.Errorf("unmarshal tool input: %w", err)
		}
	}

	result, err := session.CallTool(ctx, &gosdkmcp.CallToolParams{
		Name:      toolName,
		Arguments: args,
	})
	if err != nil {
		return message.ToolResultPart{
			ToolCallID: toolCallID,
			ToolName:   toolFullName,
			Output:     message.ErrorTextOutput{Value: fmt.Sprintf("MCP tool call failed: %v", err)},
		}, nil
	}

	output := mcpResultToOutput(result)
	return message.ToolResultPart{
		ToolCallID: toolCallID,
		ToolName:   toolFullName,
		Output:     output,
	}, nil
}

// mcpResultToOutput converts an MCP CallToolResult to a message output.
func mcpResultToOutput(result *gosdkmcp.CallToolResult) message.ToolResultOutput {
	if result.IsError {
		var sb strings.Builder
		for _, c := range result.Content {
			if tc, ok := c.(*gosdkmcp.TextContent); ok {
				sb.WriteString(tc.Text)
			}
		}
		return message.ErrorTextOutput{Value: sb.String()}
	}

	var sb strings.Builder
	for i, c := range result.Content {
		if i > 0 {
			sb.WriteString("\n")
		}
		switch v := c.(type) {
		case *gosdkmcp.TextContent:
			sb.WriteString(v.Text)
		case *gosdkmcp.ImageContent:
			sb.WriteString(fmt.Sprintf("[image: %s, %d bytes]", v.MIMEType, len(v.Data)))
		default:
			data, _ := json.Marshal(c)
			sb.Write(data)
		}
	}
	return message.TextOutput{Value: sb.String()}
}

// Status returns the current connection status of all MCP servers.
func (m *Manager) Status() []ServerInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ServerInfo, 0, len(m.servers))
	for name, sc := range m.servers {
		sc.mu.RLock()
		info := ServerInfo{
			Name:      name,
			Status:    sc.status,
			Error:     sc.errMsg,
			ToolCount: len(sc.tools),
		}
		if sc.fetcher != nil {
			if url := sc.fetcher.CurrentAuthURL(); url != "" {
				info.OAuthURL = url
				// Promote status to oauth_required while the user must authorize.
				if info.Status == ServerStatusConnecting {
					info.Status = ServerStatusOAuth
				}
			}
		}
		sc.mu.RUnlock()
		infos = append(infos, info)
	}
	return infos
}

// PendingOAuthURL returns the authorization URL for the named server,
// if it is currently waiting for user authorization.
func (m *Manager) PendingOAuthURL(serverName string) (string, bool) {
	m.mu.RLock()
	sc, ok := m.servers[serverName]
	m.mu.RUnlock()
	if !ok {
		return "", false
	}

	sc.mu.RLock()
	fetcher := sc.fetcher
	sc.mu.RUnlock()
	if fetcher == nil {
		return "", false
	}
	url := fetcher.CurrentAuthURL()
	if url == "" {
		return "", false
	}
	return url, true
}

// SubmitOAuthCode delivers the authorization code (and state) to the server
// that is waiting for it. Returns an error if the server is not found or
// is not currently waiting for OAuth.
func (m *Manager) SubmitOAuthCode(serverName, code, state string) error {
	m.mu.RLock()
	sc, ok := m.servers[serverName]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown MCP server %q", serverName)
	}

	sc.mu.RLock()
	fetcher := sc.fetcher
	sc.mu.RUnlock()
	if fetcher == nil {
		return fmt.Errorf("server %q does not support OAuth", serverName)
	}
	return fetcher.SubmitCode(code, state)
}

// Close closes all open MCP server connections.
func (m *Manager) Close() {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, sc := range m.servers {
		sc.mu.Lock()
		session := sc.session
		sc.mu.Unlock()
		if session != nil {
			if err := session.Close(); err != nil {
				log.Printf("mcp: %s: close: %v", name, err)
			}
		}
	}
}

// splitToolName splits "serverName__toolName" into its two components.
// Returns ok=false if the format is invalid.
func splitToolName(fullName string) (serverName, toolName string, ok bool) {
	idx := strings.Index(fullName, "__")
	if idx < 0 {
		return "", "", false
	}
	return fullName[:idx], fullName[idx+2:], true
}

// IsMCPTool returns true if the tool name follows the "server__tool" format.
func IsMCPTool(toolName string) bool {
	return strings.Contains(toolName, "__")
}
