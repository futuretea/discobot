package cache

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

// dockerMediaTypePrefixes are the OCI/Docker vendor media type prefixes.
// Content at URLs with sha256 digests and these Accept headers is immutable
// content-addressable storage safe to cache indefinitely.
var dockerMediaTypePrefixes = []string{
	"application/vnd.docker.",
	"application/vnd.oci.",
}

// sha256DigestRe extracts a sha256 hex digest from a URL path in either format:
//   - OCI standard:          "sha256:HEX64"       (e.g. ghcr.io, docker.io)
//   - Registry storage path: "/HEX64/"            (e.g. Cloudflare R2 /registry-v2/.../sha256/ab/HEX64/data)
var sha256DigestRe = regexp.MustCompile(`sha256:([a-fA-F0-9]{64})|/([a-fA-F0-9]{64})/`)

// Matcher determines if a request should be cached.
type Matcher struct {
	patterns     []*regexp.Regexp
	contentAware bool // detect Docker/OCI CAS blobs by URL digest + request/response headers
}

// NewMatcher creates a new cache matcher.
//
// When contentAware is true, requests are cached when the URL path contains a
// sha256 digest and the request/response headers indicate Docker/OCI registry
// content.
//
// Explicit patterns are evaluated alongside content-aware detection — a request
// is cached when either check passes. This lets callers cover CDN backends (e.g.
// Cloudflare R2 registry storage) whose URLs don't carry standard OCI Accept
// headers while still using content-aware detection for normal registry traffic.
func NewMatcher(patterns []string, contentAware bool) (*Matcher, error) {
	m := &Matcher{
		contentAware: contentAware,
		patterns:     make([]*regexp.Regexp, 0, len(patterns)),
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
// Returns true if the content-aware check passes OR any explicit pattern matches.
func (m *Matcher) ShouldCache(req *http.Request) bool {
	if req.Method != http.MethodGet {
		return false
	}

	path := req.URL.Path

	// Content-aware: URL must contain a sha256 digest and request must carry
	// Docker/OCI Accept headers confirming this is registry content.
	// Query params are permitted here — they are auth tokens (e.g. signed URLs)
	// and the sha256 digest in the path already guarantees content identity.
	if m.contentAware && strings.Contains(path, "sha256:") && hasDockerAccept(req) {
		return true
	}

	// Pattern-based: explicit URL patterns (e.g. CDN storage paths).
	for _, pattern := range m.patterns {
		if pattern.MatchString(path) {
			// Allow query params only when the path contains a sha256 digest —
			// the hash guarantees content identity so query params are auth-only
			// (e.g. R2/S3 presigned URLs).
			if req.URL.RawQuery != "" && sha256DigestRe.FindString(path) == "" {
				return false
			}
			return true
		}
	}

	// Don't cache unmatched requests that have query parameters.
	if req.URL.RawQuery != "" {
		return false
	}

	return false
}

// ShouldCacheResponse determines if a response should be cached.
func (m *Matcher) ShouldCacheResponse(resp *http.Response) bool {
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return false
	}

	cacheControl := resp.Header.Get("Cache-Control")
	if strings.Contains(strings.ToLower(cacheControl), "no-store") {
		return false
	}

	if m.contentAware {
		return isDockerResponse(resp)
	}

	return true
}

// hasDockerAccept checks whether the request Accept header includes Docker/OCI media types.
func hasDockerAccept(req *http.Request) bool {
	accept := req.Header.Get("Accept")
	if accept == "" {
		return false
	}
	for _, prefix := range dockerMediaTypePrefixes {
		if strings.Contains(accept, prefix) {
			return true
		}
	}
	return false
}

// isDockerResponse checks whether the response looks like Docker/OCI registry content.
func isDockerResponse(resp *http.Response) bool {
	// Docker-Content-Digest is set by OCI-compliant registries on all blob/manifest responses.
	if resp.Header.Get("Docker-Content-Digest") != "" {
		return true
	}

	ct := resp.Header.Get("Content-Type")
	for _, prefix := range dockerMediaTypePrefixes {
		if strings.Contains(ct, prefix) {
			return true
		}
	}

	// CDN blob delivery (e.g. pkg-containers.githubusercontent.com after a ghcr.io redirect,
	// or Cloudflare R2 registry storage) serves blobs as application/octet-stream without
	// Docker-specific headers. The request-side checks are sufficient to confirm the content
	// is legitimate immutable blob data.
	return strings.HasPrefix(ct, "application/octet-stream")
}

// GenerateKey generates a cache key from a request.
func (m *Matcher) GenerateKey(req *http.Request) string {
	return req.URL.Host + req.URL.Path
}

// VerifyDigest checks that body's sha256 hash matches the digest embedded in the
// URL path. Recognises two formats:
//   - OCI standard:          "sha256:HEX64"   (e.g. /v2/…/blobs/sha256:abc…)
//   - Registry storage path: "/HEX64/"        (e.g. /registry-v2/…/sha256/ab/HEX64/data)
//
// Returns nil if no digest is found in the path or the digest matches.
// Returns an error describing the mismatch otherwise.
func (m *Matcher) VerifyDigest(path string, body []byte) error {
	matches := sha256DigestRe.FindStringSubmatch(path)
	if len(matches) < 2 {
		return nil // no digest in path, nothing to verify
	}

	// matches[1] = OCI colon format, matches[2] = path-component format
	expected := strings.ToLower(matches[1])
	if expected == "" && len(matches) > 2 {
		expected = strings.ToLower(matches[2])
	}
	if expected == "" {
		return nil
	}

	actual := fmt.Sprintf("%x", sha256.Sum256(body))
	if expected != actual {
		return fmt.Errorf("sha256 mismatch: URL claims %s, body hashes to %s", expected, actual)
	}
	return nil
}
