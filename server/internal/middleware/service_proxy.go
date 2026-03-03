package middleware

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

// serviceSubdomainPattern matches a single {session-id}-svc-{service-id} subdomain component.
// Session IDs are 10-26 alphanumeric chars (case-insensitive in URLs).
// Service IDs are normalized lowercase (a-z0-9_- only).
var serviceSubdomainPattern = regexp.MustCompile(`^([0-9A-Za-z]{10,26})-svc-([a-z0-9_-]+)$`)

// ConnectionTracker tracks active connections per session.
// Implementations must be safe for concurrent use.
type ConnectionTracker interface {
	// Track registers an active connection for sessionID and returns a release
	// function that must be called when the connection ends.
	Track(sessionID string) func()
}

// findSessionID finds the actual session ID with correct casing.
// DNS/URLs are case-insensitive, so we need to do a case-insensitive lookup.
func findSessionID(ctx context.Context, provider sandbox.Provider, urlSessionID string) (string, error) {
	// First try exact match (fast path)
	sb, err := provider.Get(ctx, urlSessionID)
	if err == nil && sb != nil {
		return sb.SessionID, nil
	}

	// Fall back to case-insensitive search via List
	sandboxes, err := provider.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list sandboxes: %w", err)
	}

	lowerURLSessionID := strings.ToLower(urlSessionID)
	for _, sb := range sandboxes {
		if strings.ToLower(sb.SessionID) == lowerURLSessionID {
			return sb.SessionID, nil
		}
	}

	return "", fmt.Errorf("session not found: %s", urlSessionID)
}

// ServiceProxy creates middleware that intercepts requests to service subdomains
// and proxies them to the agent-api's HTTP proxy endpoint using httputil.ReverseProxy.
//
// Subdomain format: {session-id}-svc-{service-id}.{base-domain}
// Example: 01HXYZ123456789ABCDEFGHIJ-svc-myservice.localhost:3000
//
// The proxy does NOT pass credentials to the agent-api, as service HTTP
// endpoints are considered public within the sandbox.
//
// This properly handles:
// - HTTP/1.1 and HTTP/2
// - WebSocket upgrades
// - Server-Sent Events (SSE)
// - Chunked transfer encoding
// - Request/response streaming
//
// tracker, if non-nil, is notified for every proxied request (including long-lived
// connections such as SSE and WebSocket) so that the idle monitor can avoid
// shutting down sandboxes with live service-proxy connections.
func ServiceProxy(provider sandbox.Provider, tracker ConnectionTracker) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check both Host and X-Forwarded-Host for service subdomains.
			// In nested discobot, the outer proxy sets X-Forwarded-Host to
			// the original host before rewriting, so the inner instance's
			// service subdomain may only appear there.
			hosts := []string{r.Host}
			if fwdHost := r.Header.Get("X-Forwarded-Host"); fwdHost != "" && fwdHost != r.Host {
				hosts = append(hosts, fwdHost)
			}

			// Split each host into subdomain components and find the first one
			// with a valid session ID. This handles nested discobot where
			// multiple {id}-svc-{name} components may be chained, e.g.:
			//   inner-svc-ui.outer-svc-api.localhost:3001
			// We need to find the component whose session ID exists on THIS instance.
			ctx := r.Context()
			var sessionID, serviceID string
			for _, host := range hosts {
				parts := strings.Split(host, ".")
				for _, part := range parts {
					matches := serviceSubdomainPattern.FindStringSubmatch(part)
					if matches == nil {
						continue
					}
					sid, err := findSessionID(ctx, provider, matches[1])
					if err != nil {
						continue
					}
					sessionID = sid
					serviceID = matches[2]
					break
				}
				if sessionID != "" {
					break
				}
			}

			if sessionID == "" {
				// No valid service subdomain found, continue to next handler
				next.ServeHTTP(w, r)
				return
			}

			// Get HTTP client for the sandbox (handles transport-level routing)
			client, err := provider.HTTPClient(ctx, sessionID)
			if err != nil {
				writeJSONError(w, http.StatusBadGateway, "Failed to connect to sandbox", map[string]string{
					"sessionId": sessionID,
					"serviceId": serviceID,
					"message":   err.Error(),
				})
				return
			}

			// Target URL for the agent-api
			// The agent-api expects: /services/:id/http/*
			target, _ := url.Parse("http://sandbox")

			// Create reverse proxy
			proxy := &httputil.ReverseProxy{
				Director: func(req *http.Request) {
					req.URL.Scheme = target.Scheme
					req.URL.Host = target.Host
					req.URL.Path = "/services/" + serviceID + "/http" + r.URL.Path
					req.URL.RawQuery = r.URL.RawQuery

					// Set the Host header to the target
					req.Host = target.Host

					// Set x-forwarded-* headers.
					req.Header.Set("X-Forwarded-Path", r.URL.Path)
					req.Header.Set("X-Forwarded-Proto", getScheme(r))

					// Preserve existing X-Forwarded-Host so the full subdomain
					// chain survives through nested discobot levels. Only set it
					// on the first proxy layer (when no forwarded host exists yet).
					if r.Header.Get("X-Forwarded-Host") == "" {
						req.Header.Set("X-Forwarded-Host", r.Host)
					}

					// Preserve or append X-Forwarded-For
					clientIP := r.RemoteAddr
					if idx := strings.LastIndex(clientIP, ":"); idx != -1 {
						clientIP = clientIP[:idx]
					}
					if prior := r.Header.Get("X-Forwarded-For"); prior != "" {
						req.Header.Set("X-Forwarded-For", prior+", "+clientIP)
					} else {
						req.Header.Set("X-Forwarded-For", clientIP)
					}
				},
				Transport: client.Transport,
				ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
					log.Printf("[ServiceProxy] Error proxying request to %s: %v", r.URL.String(), err)
					writeJSONError(w, http.StatusBadGateway, "Service unavailable", map[string]string{
						"sessionId": sessionID,
						"serviceId": serviceID,
						"message":   err.Error(),
					})
				},
				// Streaming support - don't buffer responses
				FlushInterval: -1, // Flush immediately
			}

			// Track this connection so the idle monitor won't stop the sandbox
			// while the request (including long-lived SSE/WebSocket) is in flight.
			if tracker != nil {
				release := tracker.Track(sessionID)
				defer release()
			}

			proxy.ServeHTTP(w, r)
		})
	}
}

// writeJSONError writes a JSON error response.
func writeJSONError(w http.ResponseWriter, status int, errorType string, fields map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	// Build JSON manually to avoid import cycles
	parts := []string{fmt.Sprintf(`"error":%q`, errorType)}
	for k, v := range fields {
		parts = append(parts, fmt.Sprintf(`%q:%q`, k, v))
	}
	fmt.Fprintf(w, "{%s}", strings.Join(parts, ","))
}

// getScheme returns the request scheme (http or https).
func getScheme(r *http.Request) string {
	if r.TLS != nil {
		return "https"
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	return "http"
}
