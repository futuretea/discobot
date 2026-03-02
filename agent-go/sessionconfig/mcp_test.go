package sessionconfig

import (
	"path/filepath"
	"testing"
)

func TestParseMCPFile_Stdio(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"filesystem": {
				"command": "npx",
				"args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]
			}
		}
	}`)

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.Name != "filesystem" {
		t.Errorf("name = %s, want filesystem", s.Name)
	}
	if s.Transport != "stdio" {
		t.Errorf("transport = %s, want stdio", s.Transport)
	}
	if s.Command != "npx" {
		t.Errorf("command = %s, want npx", s.Command)
	}
	if len(s.Args) != 3 || s.Args[0] != "-y" {
		t.Errorf("args = %v, want [-y @modelcontextprotocol/server-filesystem /tmp]", s.Args)
	}
}

func TestParseMCPFile_SSE(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"remote-tools": {
				"type": "sse",
				"url": "https://mcp.example.com/sse"
			}
		}
	}`)

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.Name != "remote-tools" {
		t.Errorf("name = %s, want remote-tools", s.Name)
	}
	if s.Transport != "sse" {
		t.Errorf("transport = %s, want sse", s.Transport)
	}
	if s.URL != "https://mcp.example.com/sse" {
		t.Errorf("url = %s, want https://mcp.example.com/sse", s.URL)
	}
}

func TestParseMCPFile_HTTP(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"api": {
				"type": "http",
				"url": "https://api.example.com/mcp"
			}
		}
	}`)

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}
	if servers[0].Transport != "http" {
		t.Errorf("transport = %s, want http", servers[0].Transport)
	}
}

func TestParseMCPFile_EnvVarExpansion(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"db": {
				"command": "mcp-db",
				"env": {
					"DB_HOST": "${TEST_MCP_DB_HOST}",
					"DB_PORT": "5432"
				}
			}
		}
	}`)

	t.Setenv("TEST_MCP_DB_HOST", "localhost")

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("expected 1 server, got %d", len(servers))
	}

	s := servers[0]
	if s.Env["DB_HOST"] != "localhost" {
		t.Errorf("DB_HOST = %s, want localhost", s.Env["DB_HOST"])
	}
	if s.Env["DB_PORT"] != "5432" {
		t.Errorf("DB_PORT = %s, want 5432", s.Env["DB_PORT"])
	}
}

func TestParseMCPFile_MissingFile(t *testing.T) {
	servers, err := parseMCPFile("/nonexistent/.mcp.json")
	if err != nil {
		t.Fatal(err)
	}
	if servers != nil {
		t.Errorf("expected nil for missing file, got %v", servers)
	}
}

func TestParseMCPFile_MultipleServers(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"server-a": {"command": "a"},
			"server-b": {"url": "https://b.example.com"}
		}
	}`)

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("expected 2 servers, got %d", len(servers))
	}

	// Check both are present (order may vary due to map iteration).
	names := map[string]bool{}
	for _, s := range servers {
		names[s.Name] = true
	}
	if !names["server-a"] || !names["server-b"] {
		t.Errorf("expected server-a and server-b, got %v", names)
	}
}

func TestDiscoverMCPServers_ProjectLevel(t *testing.T) {
	root := t.TempDir()
	mkdirAll(t, filepath.Join(root, ".git"))
	writeFile(t, filepath.Join(root, ".mcp.json"), `{
		"mcpServers": {
			"local": {"command": "local-mcp"}
		}
	}`)

	servers, err := discoverMCPServers(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) < 1 {
		t.Fatal("expected at least 1 server")
	}

	found := false
	for _, s := range servers {
		if s.Name == "local" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find 'local' server")
	}
}

func TestParseMCPFile_TransportAutoDetect(t *testing.T) {
	dir := t.TempDir()
	mcpFile := filepath.Join(dir, ".mcp.json")

	// No explicit "type" field — should auto-detect from presence of command vs url.
	writeFile(t, mcpFile, `{
		"mcpServers": {
			"cmd-server": {"command": "some-cmd"},
			"url-server": {"url": "https://example.com/sse"}
		}
	}`)

	servers, err := parseMCPFile(mcpFile)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range servers {
		switch s.Name {
		case "cmd-server":
			if s.Transport != "stdio" {
				t.Errorf("cmd-server transport = %s, want stdio", s.Transport)
			}
		case "url-server":
			if s.Transport != "sse" {
				t.Errorf("url-server transport = %s, want sse", s.Transport)
			}
		}
	}
}
