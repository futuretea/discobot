//go:build !mcp_go_client_oauth

package mcp

import (
	"fmt"
	"net/http"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"golang.org/x/oauth2"

	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

// channelCodeFetcher is a stub when OAuth support is not compiled in.
type channelCodeFetcher struct{}

func newChannelCodeFetcher(_ string) *channelCodeFetcher {
	return &channelCodeFetcher{}
}

// CurrentAuthURL always returns "" in the stub (no OAuth flow possible).
func (f *channelCodeFetcher) CurrentAuthURL() string { return "" }

// SubmitCode always returns an error in the stub.
func (f *channelCodeFetcher) SubmitCode(_, _ string) error {
	return fmt.Errorf("OAuth support is not compiled in (build with -tags mcp_go_client_oauth)")
}

// newOAuthHandler returns nils in the stub (no OAuth support).
func newOAuthHandler(
	_ string,
	_ string,
	_ *sessionconfig.MCPOAuthConfig,
	_ *channelCodeFetcher,
	_ []OAuthToken,
	_ func(*oauth2.Token),
) (sdkauth.OAuthHandler, *http.Client) {
	return nil, nil
}
