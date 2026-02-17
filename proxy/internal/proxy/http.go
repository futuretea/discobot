package proxy

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"net/http"
	"time"

	"github.com/elazarl/goproxy"

	"github.com/obot-platform/discobot/proxy/internal/cache"
	"github.com/obot-platform/discobot/proxy/internal/cert"
	"github.com/obot-platform/discobot/proxy/internal/filter"
	"github.com/obot-platform/discobot/proxy/internal/injector"
	"github.com/obot-platform/discobot/proxy/internal/logger"
)

// HTTPProxy wraps goproxy for HTTP/HTTPS proxying.
type HTTPProxy struct {
	proxy        *goproxy.ProxyHttpServer
	injector     *injector.Injector
	filter       *filter.Filter
	logger       *logger.Logger
	cache        *cache.Cache
	cacheMatcher *cache.Matcher
}

// requestMeta is stored in goproxy's ctx.UserData to carry per-request state
// between the request and response handlers.
type requestMeta struct {
	startTime time.Time
	cacheHit  bool
}

// NewHTTPProxy creates a new HTTP proxy.
func NewHTTPProxy(certMgr *cert.Manager, inj *injector.Injector, flt *filter.Filter, log *logger.Logger, c *cache.Cache, matcher *cache.Matcher) *HTTPProxy {
	proxy := goproxy.NewProxyHttpServer()
	proxy.Verbose = false

	h := &HTTPProxy{
		proxy:        proxy,
		injector:     inj,
		filter:       flt,
		logger:       log,
		cache:        c,
		cacheMatcher: matcher,
	}

	h.setupMITM(certMgr)
	h.setupHandlers()

	return h
}

func (h *HTTPProxy) setupMITM(certMgr *cert.Manager) {
	ca := certMgr.GetCA()

	// Parse the x509 certificate from the tls.Certificate
	x509Cert, err := x509.ParseCertificate(ca.Certificate[0])
	if err != nil {
		h.logger.Error("failed to parse CA certificate")
		return
	}

	// Set up goproxy CA
	goproxy.GoproxyCa = *ca

	// Create TLS config that uses our CA to sign certificates
	tlsConfig := func(host string, _ *goproxy.ProxyCtx) (*tls.Config, error) {
		// InsecureSkipVerify is required for MITM proxy functionality - the proxy
		// decrypts traffic from clients and re-encrypts it to upstream servers.
		// This allows the proxy to inspect and modify HTTP traffic over TLS.
		config := &tls.Config{
			InsecureSkipVerify: true, //#nosec G402 -- Required for MITM proxy
		}

		// Strip port from host if present (goproxy may pass host:port)
		// For example: "registry-1.docker.io:443" -> "registry-1.docker.io"
		hostname := host
		if h, _, err := net.SplitHostPort(host); err == nil {
			hostname = h
		}

		// Generate certificate for this host signed by our CA
		cert, err := signHost(*ca, x509Cert, []string{hostname})
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
		return config, nil
	}

	// Configure CONNECT handling to use MITM
	connectAction := &goproxy.ConnectAction{
		Action:    goproxy.ConnectMitm,
		TLSConfig: tlsConfig,
	}
	goproxy.OkConnect = connectAction
	goproxy.MitmConnect = connectAction
	goproxy.RejectConnect = &goproxy.ConnectAction{Action: goproxy.ConnectReject}
}

func (h *HTTPProxy) setupHandlers() {
	// Handle CONNECT requests (HTTPS)
	h.proxy.OnRequest().HandleConnectFunc(func(host string, _ *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		if !h.filter.AllowHost(host) {
			h.logger.LogBlocked(host, "filter")
			return goproxy.RejectConnect, host
		}
		return goproxy.MitmConnect, host
	})

	// Handle all requests (after MITM decryption for HTTPS)
	h.proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		meta := &requestMeta{startTime: time.Now()}
		ctx.UserData = meta

		// Filter check (for plain HTTP)
		if !h.filter.AllowHost(req.Host) {
			h.logger.LogBlocked(req.Host, "filter")
			return req, goproxy.NewResponse(req, goproxy.ContentTypeText, http.StatusForbidden, "Blocked by proxy")
		}

		// Check cache
		if h.cacheMatcher != nil && h.cacheMatcher.ShouldCache(req) {
			key := h.cacheMatcher.GenerateKey(req)
			if entry, err := h.cache.Get(key); err == nil {
				meta.cacheHit = true
				h.logger.Info("cache hit",
					"host", req.Host,
					"path", req.URL.Path,
					"size", entry.Size,
					"cached_at", entry.CachedAt.Format(time.RFC3339),
				)
				return req, cache.RestoreResponse(entry, req)
			}
			h.logger.Debug("cache miss", "host", req.Host, "path", req.URL.Path)
		}

		// Inject headers
		match := h.injector.Apply(req)
		if match.Matched {
			h.logger.LogHeaderInjection(match.Host, match.Pattern, match.Headers)
		}

		// Log request
		h.logger.LogRequest(req)

		return req, nil
	})

	// Log responses and cache if applicable
	h.proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if resp == nil || ctx.Req == nil {
			return resp
		}

		meta, _ := ctx.UserData.(*requestMeta)

		// Cache hits were already logged in the request handler and never
		// contacted upstream — nothing more to do here.
		if meta != nil && meta.cacheHit {
			return resp
		}

		var duration time.Duration
		if meta != nil {
			duration = time.Since(meta.startTime)
		}
		h.logger.LogResponse(resp, ctx.Req, duration)

		// Cache response if applicable
		if h.cacheMatcher != nil && h.cacheMatcher.ShouldCache(ctx.Req) {
			if !h.cacheMatcher.ShouldCacheResponse(resp) {
				h.logger.Debug("response not cacheable",
					"path", ctx.Req.URL.Path,
					"status", resp.StatusCode,
					"content_type", resp.Header.Get("Content-Type"),
					"cache_control", resp.Header.Get("Cache-Control"),
				)
			} else {
				entry, err := cache.CaptureResponse(resp)
				if err != nil {
					h.logger.Warn("failed to read response for caching", "path", ctx.Req.URL.Path, "error", err.Error())
				} else if err := h.cacheMatcher.VerifyDigest(ctx.Req.URL.Path, entry.Body); err != nil {
					h.logger.Warn("digest verification failed, not caching", "path", ctx.Req.URL.Path, "error", err.Error())
				} else {
					key := h.cacheMatcher.GenerateKey(ctx.Req)
					if err := h.cache.Put(key, entry); err != nil {
						h.logger.Warn("cache store failed", "path", ctx.Req.URL.Path, "error", err.Error())
					} else {
						h.logger.Info("cached response", "path", ctx.Req.URL.Path, "size", entry.Size)
					}
				}
			}
		}

		return resp
	})
}

// ServeConn serves an HTTP connection.
func (h *HTTPProxy) ServeConn(conn *PeekedConn) {
	// Create a listener that returns this single connection
	listener := &singleConnListener{
		conn: conn,
		done: make(chan struct{}),
	}
	server := &http.Server{
		Handler:           h.proxy,
		ReadHeaderTimeout: 10 * time.Second,
	}
	_ = server.Serve(listener)
}

// GetProxy returns the underlying goproxy instance.
func (h *HTTPProxy) GetProxy() *goproxy.ProxyHttpServer {
	return h.proxy
}

// singleConnListener is a net.Listener that returns one connection then blocks forever.
type singleConnListener struct {
	conn   net.Conn
	served bool
	done   chan struct{}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	if l.served {
		// Block until Close is called
		<-l.done
		return nil, net.ErrClosed
	}
	l.served = true
	return l.conn, nil
}

func (l *singleConnListener) Close() error {
	select {
	case <-l.done:
		// Already closed
	default:
		close(l.done)
	}
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	if l.conn != nil {
		return l.conn.LocalAddr()
	}
	return nil
}
