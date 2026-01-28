package cache

import (
	"net/http"
	"regexp"
	"strings"
)

// Matcher determines if a request should be cached.
type Matcher struct {
	patterns []*regexp.Regexp
}

// NewMatcher creates a new cache matcher.
func NewMatcher(patterns []string) (*Matcher, error) {
	m := &Matcher{
		patterns: make([]*regexp.Regexp, 0, len(patterns)),
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		m.patterns = append(m.patterns, re)
	}

	return m, nil
}

// ShouldCache determines if a request should be cached.
func (m *Matcher) ShouldCache(req *http.Request) bool {
	// Only cache GET requests
	if req.Method != http.MethodGet {
		return false
	}

	// Don't cache requests with query parameters (unless specifically configured)
	if req.URL.RawQuery != "" {
		return false
	}

	// Check if path matches any pattern
	path := req.URL.Path
	for _, pattern := range m.patterns {
		if pattern.MatchString(path) {
			return true
		}
	}

	return false
}

// ShouldCacheResponse determines if a response should be cached.
func (m *Matcher) ShouldCacheResponse(resp *http.Response) bool {
	// Only cache successful responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	// Check for Cache-Control: no-store
	cacheControl := resp.Header.Get("Cache-Control")
	return !strings.Contains(strings.ToLower(cacheControl), "no-store")
}

// GenerateKey generates a cache key from a request.
func (m *Matcher) GenerateKey(req *http.Request) string {
	// Use full URL path as key (query params excluded in ShouldCache)
	return req.URL.Host + req.URL.Path
}

// DefaultDockerPatterns returns default patterns for Docker registry caching.
func DefaultDockerPatterns() []string {
	return []string{
		`^/v2/.*/blobs/sha256:.*`,     // Docker blob layers (immutable)
		`^/v2/.*/manifests/sha256:.*`, // Docker manifests by digest (immutable)
	}
}
