//go:build mcp_go_client_oauth

package mcp

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	sdkauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
	"golang.org/x/oauth2"

	"github.com/obot-platform/discobot/agent-go/sessionconfig"
)

// channelCodeFetcher satisfies auth.AuthorizationCodeHandlerConfig.AuthorizationCodeFetcher.
// FetchCode blocks until SubmitCode is called (from the HTTP handler).
type channelCodeFetcher struct {
	redirectURI string

	mu      sync.Mutex
	authURL string // populated when FetchCode is called; cleared after code received
	codeCh  chan *sdkauth.AuthorizationResult
}

func newChannelCodeFetcher(redirectURI string) *channelCodeFetcher {
	return &channelCodeFetcher{
		redirectURI: redirectURI,
		codeCh:      make(chan *sdkauth.AuthorizationResult, 1),
	}
}

// FetchCode is called by auth.AuthorizationCodeHandler to initiate the authorization flow.
// It stores the auth URL (visible via CurrentAuthURL) and blocks until SubmitCode is called.
func (f *channelCodeFetcher) FetchCode(ctx context.Context, args *sdkauth.AuthorizationArgs) (*sdkauth.AuthorizationResult, error) {
	f.mu.Lock()
	f.authURL = args.URL
	f.mu.Unlock()

	select {
	case result := <-f.codeCh:
		f.mu.Lock()
		f.authURL = ""
		f.mu.Unlock()
		return result, nil
	case <-ctx.Done():
		f.mu.Lock()
		f.authURL = ""
		f.mu.Unlock()
		return nil, ctx.Err()
	}
}

// CurrentAuthURL returns the current authorization URL, or "" if no OAuth flow is in progress.
func (f *channelCodeFetcher) CurrentAuthURL() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.authURL
}

// SubmitCode delivers the authorization code and state to the waiting FetchCode call.
func (f *channelCodeFetcher) SubmitCode(code, state string) error {
	select {
	case f.codeCh <- &sdkauth.AuthorizationResult{Code: code, State: state}:
		return nil
	default:
		return fmt.Errorf("no OAuth flow is currently in progress for this server")
	}
}

// newOAuthHandler returns the OAuth setup for the given MCP server.
//
// When a valid existing token is found in existingTokens for serverURL, it returns
// (nil, preseededClient) — a custom HTTP client that injects the Bearer token so
// that the OAuth flow is skipped entirely.
//
// When no existing token is found, it returns (handler, nil) — an
// auth.AuthorizationCodeHandler that drives the full OAuth authorization-code flow.
//
// Note: onToken is accepted for API compatibility but token persistence after a new
// exchange is not yet supported by the SDK (the token is held inside the handler and
// not exposed after Authorize completes).
func newOAuthHandler(
	serverName string,
	serverURL string,
	cfg *sessionconfig.MCPOAuthConfig,
	fetcher *channelCodeFetcher,
	existingTokens []MCPOAuthToken,
	_ func(*oauth2.Token), // onToken — TODO: wire up when SDK exposes post-Authorize token
) (sdkauth.OAuthHandler, *http.Client) {
	// If we have a valid existing token, serve it via a custom HTTP client.
	// This avoids triggering the OAuth flow for known-good tokens.
	if entry := findToken(existingTokens, serverURL); entry != nil {
		tok := tokenFromEntry(entry)
		if tok.Valid() || tok.RefreshToken != "" {
			client := &http.Client{
				Transport: &bearerRoundTripper{
					token: tok.AccessToken,
					inner: http.DefaultTransport,
				},
			}
			return nil, client
		}
	}

	// No existing token — build a full authorization-code flow handler.
	handler := buildAuthorizationCodeHandler(serverName, cfg, fetcher)
	return handler, nil
}

// bearerRoundTripper injects a static Bearer token into every outbound request.
// Used for pre-seeded tokens to skip the OAuth flow.
type bearerRoundTripper struct {
	token string
	inner http.RoundTripper
}

func (rt *bearerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+rt.token)
	return rt.inner.RoundTrip(req2)
}

// buildAuthorizationCodeHandler creates an auth.AuthorizationCodeHandler.
// Client registration is configured in priority order:
// ClientMetadataURI → pre-registered (ClientID/Secret) → dynamic registration.
func buildAuthorizationCodeHandler(
	serverName string,
	cfg *sessionconfig.MCPOAuthConfig,
	fetcher *channelCodeFetcher,
) *sdkauth.AuthorizationCodeHandler {
	handlerCfg := &sdkauth.AuthorizationCodeHandlerConfig{
		RedirectURL:              fetcher.redirectURI,
		AuthorizationCodeFetcher: fetcher.FetchCode,
	}

	if cfg != nil && cfg.ClientMetadataURI != "" {
		handlerCfg.ClientIDMetadataDocumentConfig = &sdkauth.ClientIDMetadataDocumentConfig{
			URL: cfg.ClientMetadataURI,
		}
	} else if cfg != nil && cfg.ClientID != "" {
		handlerCfg.PreregisteredClientConfig = &sdkauth.PreregisteredClientConfig{
			ClientSecretAuthConfig: &sdkauth.ClientSecretAuthConfig{
				ClientID:     cfg.ClientID,
				ClientSecret: cfg.ClientSecret,
			},
		}
	} else {
		// Default: dynamic client registration.
		handlerCfg.DynamicClientRegistrationConfig = &sdkauth.DynamicClientRegistrationConfig{
			Metadata: &oauthex.ClientRegistrationMetadata{
				ClientName:   "discobot-" + serverName,
				RedirectURIs: []string{fetcher.redirectURI},
				GrantTypes:   []string{"authorization_code"},
			},
		}
	}

	handler, err := sdkauth.NewAuthorizationCodeHandler(handlerCfg)
	if err != nil {
		// Configuration error — return nil; the server will connect without auth.
		return nil
	}
	return handler
}
