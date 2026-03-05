// Command agent-api runs the agent API.
//
// By default it runs as an interactive terminal agent (stdin/stdout).
// Pass --server to start the HTTP API server instead.
//
// Configuration is entirely via environment variables (see internal/config).
package main

import (
	"flag"

	"github.com/obot-platform/discobot/agent-go/cmd/agent-api/cli"
	"github.com/obot-platform/discobot/agent-go/cmd/agent-api/server"
	"github.com/obot-platform/discobot/agent-go/internal/config"

	// Side-effect imports register provider factories so the registry can
	// build them on demand when credentials arrive via X-Discobot-Credentials.
	_ "github.com/obot-platform/discobot/agent-go/providers/anthropic"
	_ "github.com/obot-platform/discobot/agent-go/providers/openai"
)

func main() {
	serverMode := flag.Bool("server", false, "Run as HTTP API server (default: interactive terminal mode)")
	flags := cli.AddFlags()
	flag.Parse()

	cfg := config.Load()

	if *serverMode {
		server.Run(cfg)
	} else {
		cli.Run(cfg, flags)
	}
}
