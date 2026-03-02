package services

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

// hopByHopHeaders are headers that should not be forwarded through a proxy.
var hopByHopHeaders = map[string]bool{
	"Connection":          true,
	"Keep-Alive":          true,
	"Proxy-Authenticate":  true,
	"Proxy-Authorization": true,
	"Te":                  true,
	"Trailers":            true,
	"Transfer-Encoding":   true,
	"Upgrade":             true,
}

// ProxyHTTP creates an HTTP reverse proxy handler for a service port.
func ProxyHTTP(port int) http.Handler {
	target := &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", port),
	}

	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Use x-forwarded-path if set, otherwise keep original path
			if fwdPath := req.Header.Get("X-Forwarded-Path"); fwdPath != "" {
				req.URL.Path = fwdPath
			}

			// Set forwarding headers
			if xff := req.Header.Get("X-Forwarded-For"); xff == "" {
				if xri := req.Header.Get("X-Real-Ip"); xri != "" {
					req.Header.Set("X-Forwarded-For", xri)
				} else {
					req.Header.Set("X-Forwarded-For", "127.0.0.1")
				}
			}
			if req.Header.Get("X-Forwarded-Host") == "" {
				req.Header.Set("X-Forwarded-Host", "localhost")
			}
			if req.Header.Get("X-Forwarded-Proto") == "" {
				req.Header.Set("X-Forwarded-Proto", "http")
			}

			// Remove hop-by-hop headers
			for h := range hopByHopHeaders {
				req.Header.Del(h)
			}
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			if isConnectionRefused(err) {
				if acceptsHTML(r) {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.WriteHeader(http.StatusServiceUnavailable)
					_, _ = w.Write([]byte(connectionRefusedHTML(port)))
					return
				}
				writeJSONError(w, http.StatusServiceUnavailable, "connection_refused",
					fmt.Sprintf("Connection refused to localhost:%d", port), port)
				return
			}

			if acceptsHTML(r) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(proxyErrorHTML(port, err)))
				return
			}
			writeJSONError(w, http.StatusBadGateway, "proxy_error", err.Error(), port)
		},
	}

	return proxy
}

// isConnectionRefused checks if the error is a connection refused error.
func isConnectionRefused(err error) bool {
	if err == nil {
		return false
	}
	if opErr, ok := err.(*net.OpError); ok {
		if sysErr, ok := opErr.Err.(*os.SyscallError); ok {
			return sysErr.Err.Error() == "connection refused"
		}
	}
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "Connection refused") ||
		strings.Contains(errStr, "ECONNREFUSED")
}

// acceptsHTML checks if the request accepts HTML responses.
func acceptsHTML(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") ||
		strings.Contains(accept, "application/xhtml+xml")
}

// writeJSONError writes a JSON error response for proxy errors.
func writeJSONError(w http.ResponseWriter, status int, errCode, msg string, port int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error":   errCode,
		"message": msg,
		"port":    port,
	})
}

// connectionRefusedHTML returns an HTML page that auto-retries when the service becomes available.
func connectionRefusedHTML(port int) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Waiting for service...</title>
<style>
  body { font-family: system-ui, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; color: #333; }
  @media (prefers-color-scheme: dark) { body { background: #1a1a1a; color: #e0e0e0; } }
  .container { text-align: center; }
  .spinner { width: 40px; height: 40px; border: 3px solid #ccc; border-top-color: #666; border-radius: 50%%; animation: spin 1s linear infinite; margin: 0 auto 20px; }
  @keyframes spin { to { transform: rotate(360deg); } }
  .status { color: #888; font-size: 14px; }
</style>
</head>
<body>
<div class="container">
  <div class="spinner"></div>
  <h2>Waiting for service on port %d...</h2>
  <p class="status" id="status">Retrying...</p>
</div>
<script>
  let retries = 0;
  async function check() {
    retries++;
    document.getElementById('status').textContent = 'Retry #' + retries + '...';
    try {
      const res = await fetch(window.location.href, { method: 'HEAD' });
      if (res.ok) { window.location.reload(); return; }
    } catch {}
    setTimeout(check, 2000);
  }
  setTimeout(check, 2000);
</script>
</body>
</html>`, port)
}

// proxyErrorHTML returns an HTML error page for proxy errors.
func proxyErrorHTML(port int, err error) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>Proxy Error</title>
<style>
  body { font-family: system-ui, sans-serif; display: flex; justify-content: center; align-items: center; min-height: 100vh; margin: 0; background: #f5f5f5; color: #333; }
  @media (prefers-color-scheme: dark) { body { background: #1a1a1a; color: #e0e0e0; } }
</style>
</head>
<body>
<div>
  <h2>502 Bad Gateway</h2>
  <p>Error connecting to service on port %d: %s</p>
</div>
</body>
</html>`, port, err.Error())
}
