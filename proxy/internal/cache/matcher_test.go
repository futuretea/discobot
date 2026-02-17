package cache

import (
	"net/http"
	"testing"
)

// dockerAccept is a representative Docker client Accept header for blob pulls.
const dockerAccept = "application/vnd.docker.image.rootfs.diff.tar.gzip, application/vnd.oci.image.layer.v1.tar+gzip, application/vnd.oci.image.layer.v1.tar+zstd, application/octet-stream, */*"

// manifestAccept is a representative Docker client Accept header for manifest pulls.
const manifestAccept = "application/vnd.docker.distribution.manifest.v2+json, application/vnd.docker.distribution.manifest.list.v2+json, application/vnd.oci.image.index.v1+json, application/vnd.oci.image.manifest.v1+json, */*"

func TestMatcher_ContentAware_ShouldCache(t *testing.T) {
	matcher, err := NewMatcher(nil, true)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	tests := []struct {
		name     string
		method   string
		url      string
		accept   string
		expected bool
	}{
		{
			name:     "docker blob with OCI accept - should cache",
			method:   "GET",
			url:      "https://registry-1.docker.io/v2/library/ubuntu/blobs/sha256:abc123def456",
			accept:   dockerAccept,
			expected: true,
		},
		{
			name:     "docker manifest by digest with manifest accept - should cache",
			method:   "GET",
			url:      "https://registry-1.docker.io/v2/library/ubuntu/manifests/sha256:xyz789",
			accept:   manifestAccept,
			expected: true,
		},
		{
			name:     "ghcr.io blob with OCI accept - should cache",
			method:   "GET",
			url:      "https://ghcr.io/v2/obot-platform/discobot/blobs/sha256:bd9ddc54bea929a22b334e73e026d4136e5b73f5cc29942896c72e4ece69b13d",
			accept:   dockerAccept,
			expected: true,
		},
		{
			name:     "ghcr.io CDN redirect blob with OCI accept - should cache",
			method:   "GET",
			url:      "https://pkg-containers.githubusercontent.com/ghcr1/blobs/sha256:bd9ddc54bea929a22b334e73e026d4136e5b73f5cc29942896c72e4ece69b13d",
			accept:   dockerAccept,
			expected: true,
		},
		{
			name:     "ECR blob with OCI accept - should cache",
			method:   "GET",
			url:      "https://123456789.dkr.ecr.us-east-1.amazonaws.com/v2/myapp/blobs/sha256:abc123",
			accept:   dockerAccept,
			expected: true,
		},
		{
			name:     "no sha256 in path - should not cache",
			method:   "GET",
			url:      "https://registry.io/v2/ubuntu/manifests/latest",
			accept:   manifestAccept,
			expected: false,
		},
		{
			name:     "sha256 in path but no docker accept - should not cache",
			method:   "GET",
			url:      "https://example.com/files/sha256:abc123",
			accept:   "text/html",
			expected: false,
		},
		{
			name:     "sha256 in path but no accept header - should not cache",
			method:   "GET",
			url:      "https://registry.io/v2/ubuntu/blobs/sha256:abc123",
			accept:   "",
			expected: false,
		},
		{
			name:     "POST request - should not cache",
			method:   "POST",
			url:      "https://registry.io/v2/ubuntu/blobs/sha256:abc123",
			accept:   dockerAccept,
			expected: false,
		},
		{
			// sha256 in path guarantees content identity; query params are auth tokens.
			name:     "sha256 blob with query params (signed URL) - should cache",
			method:   "GET",
			url:      "https://registry.io/v2/ubuntu/blobs/sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abcd?token=xyz",
			accept:   dockerAccept,
			expected: true,
		},
		{
			// No sha256 in path, query params may affect content — don't cache.
			name:     "non-CAS URL with query params - should not cache",
			method:   "GET",
			url:      "https://registry.io/v2/ubuntu/manifests/latest?foo=bar",
			accept:   manifestAccept,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, tt.url, nil)
			if tt.accept != "" {
				req.Header.Set("Accept", tt.accept)
			}

			result := matcher.ShouldCache(req)
			if result != tt.expected {
				t.Errorf("ShouldCache() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatcher_ContentAware_ShouldCacheResponse(t *testing.T) {
	matcher, err := NewMatcher(nil, true)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	tests := []struct {
		name                string
		statusCode          int
		contentType         string
		dockerContentDigest string
		cacheControl        string
		expected            bool
	}{
		{
			name:                "registry blob response with Docker-Content-Digest - should cache",
			statusCode:          200,
			contentType:         "application/octet-stream",
			dockerContentDigest: "sha256:abc123",
			expected:            true,
		},
		{
			name:        "OCI manifest content-type - should cache",
			statusCode:  200,
			contentType: "application/vnd.oci.image.manifest.v1+json",
			expected:    true,
		},
		{
			name:        "Docker manifest content-type - should cache",
			statusCode:  200,
			contentType: "application/vnd.docker.distribution.manifest.v2+json",
			expected:    true,
		},
		{
			name:        "CDN blob delivery (octet-stream, no Docker headers) - should cache",
			statusCode:  200,
			contentType: "application/octet-stream",
			expected:    true,
		},
		{
			name:        "307 redirect - should not cache",
			statusCode:  307,
			contentType: "application/octet-stream",
			expected:    false,
		},
		{
			name:         "200 with no-store - should not cache",
			statusCode:   200,
			contentType:  "application/octet-stream",
			cacheControl: "no-store",
			expected:     false,
		},
		{
			name:       "404 not found - should not cache",
			statusCode: 404,
			expected:   false,
		},
		{
			name:       "500 server error - should not cache",
			statusCode: 500,
			expected:   false,
		},
		{
			name:        "html response (non-docker) - should not cache",
			statusCode:  200,
			contentType: "text/html",
			expected:    false,
		},
		{
			name:        "json response (non-docker) - should not cache",
			statusCode:  200,
			contentType: "application/json",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{},
			}
			if tt.contentType != "" {
				resp.Header.Set("Content-Type", tt.contentType)
			}
			if tt.dockerContentDigest != "" {
				resp.Header.Set("Docker-Content-Digest", tt.dockerContentDigest)
			}
			if tt.cacheControl != "" {
				resp.Header.Set("Cache-Control", tt.cacheControl)
			}

			result := matcher.ShouldCacheResponse(resp)
			if result != tt.expected {
				t.Errorf("ShouldCacheResponse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatcher_ContentAware_GitHubRegistryFlow(t *testing.T) {
	// Validates the real-world ghcr.io flow from the proxy logs:
	// 1. GET ghcr.io/v2/.../blobs/sha256:... → 307 redirect (not cached)
	// 2. GET pkg-containers.githubusercontent.com/ghcr1/blobs/sha256:... → 200 (cached)
	matcher, err := NewMatcher(nil, true)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	digest := "sha256:bd9ddc54bea929a22b334e73e026d4136e5b73f5cc29942896c72e4ece69b13d"

	// Step 1: initial request to ghcr.io
	req1, _ := http.NewRequest("GET", "https://ghcr.io/v2/obot-platform/discobot/blobs/"+digest, nil)
	req1.Header.Set("Accept", dockerAccept)
	if !matcher.ShouldCache(req1) {
		t.Error("initial ghcr.io request should be cacheable")
	}
	// The 307 redirect response itself should not be cached
	resp307 := &http.Response{StatusCode: 307, Header: http.Header{}}
	if matcher.ShouldCacheResponse(resp307) {
		t.Error("307 redirect should not be cached")
	}

	// Step 2: redirected request to CDN
	req2, _ := http.NewRequest("GET", "https://pkg-containers.githubusercontent.com/ghcr1/blobs/"+digest, nil)
	req2.Header.Set("Accept", dockerAccept)
	if !matcher.ShouldCache(req2) {
		t.Error("CDN redirect request should be cacheable")
	}
	// CDN serves blob as octet-stream without Docker-specific headers
	resp200 := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/octet-stream"}},
	}
	if !matcher.ShouldCacheResponse(resp200) {
		t.Error("CDN blob 200 response should be cached")
	}
}

func TestMatcher_PatternMode_ShouldCache(t *testing.T) {
	// Explicit patterns work without header checking.
	patterns := []string{
		`^/v2/.*/blobs/sha256:.*`,
		`^/v2/.*/manifests/sha256:.*`,
	}
	matcher, err := NewMatcher(patterns, false)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	tests := []struct {
		name     string
		method   string
		path     string
		query    string
		expected bool
	}{
		{
			name:     "docker blob - should cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/blobs/sha256:abc123def456",
			expected: true,
		},
		{
			name:     "docker manifest by digest - should cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/manifests/sha256:xyz789",
			expected: true,
		},
		{
			name:     "docker manifest by tag - should not cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/manifests/latest",
			expected: false,
		},
		{
			name:     "POST request - should not cache",
			method:   "POST",
			path:     "/v2/library/ubuntu/blobs/sha256:abc123",
			expected: false,
		},
		{
			name:     "with query params - should not cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/blobs/sha256:abc123",
			query:    "token=xyz",
			expected: false,
		},
		{
			name:     "non-matching path - should not cache",
			method:   "GET",
			path:     "/api/v1/images",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, "http://registry.example.com"+tt.path, nil)
			if tt.query != "" {
				req.URL.RawQuery = tt.query
			}

			result := matcher.ShouldCache(req)
			if result != tt.expected {
				t.Errorf("ShouldCache() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatcher_PatternMode_ShouldCacheResponse(t *testing.T) {
	// In pattern mode, only status code and Cache-Control matter.
	matcher, err := NewMatcher([]string{`.*`}, false)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	tests := []struct {
		name         string
		statusCode   int
		cacheControl string
		expected     bool
	}{
		{
			name:       "200 OK - should cache",
			statusCode: 200,
			expected:   true,
		},
		{
			name:       "404 Not Found - should not cache",
			statusCode: 404,
			expected:   false,
		},
		{
			name:       "500 Internal Error - should not cache",
			statusCode: 500,
			expected:   false,
		},
		{
			name:         "200 with no-store - should not cache",
			statusCode:   200,
			cacheControl: "no-store",
			expected:     false,
		},
		{
			name:         "200 with public - should cache",
			statusCode:   200,
			cacheControl: "public, max-age=3600",
			expected:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Header:     http.Header{},
			}
			if tt.cacheControl != "" {
				resp.Header.Set("Cache-Control", tt.cacheControl)
			}

			result := matcher.ShouldCacheResponse(resp)
			if result != tt.expected {
				t.Errorf("ShouldCacheResponse() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatcher_GenerateKey(t *testing.T) {
	matcher, err := NewMatcher([]string{`.*`}, false)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	tests := []struct {
		url      string
		expected string
	}{
		{
			url:      "http://registry.example.com/v2/library/ubuntu/blobs/sha256:abc123",
			expected: "registry.example.com/v2/library/ubuntu/blobs/sha256:abc123",
		},
		{
			url:      "https://registry.io/v2/foo/manifests/sha256:def456",
			expected: "registry.io/v2/foo/manifests/sha256:def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			req, _ := http.NewRequest("GET", tt.url, nil)
			result := matcher.GenerateKey(req)
			if result != tt.expected {
				t.Errorf("GenerateKey() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestMatcher_InvalidPattern(t *testing.T) {
	_, err := NewMatcher([]string{"[invalid"}, false)
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}

func TestMatcher_CombinedMode(t *testing.T) {
	// content_aware + explicit patterns: either check can match.
	r2Pattern := `^/registry-v2/docker/registry/v2/blobs/sha256/`
	matcher, err := NewMatcher([]string{r2Pattern}, true)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}

	// Content-aware path (OCI format) — matched by content-aware check.
	req1, _ := http.NewRequest("GET", "https://ghcr.io/v2/foo/blobs/sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abcd", nil)
	req1.Header.Set("Accept", dockerAccept)
	if !matcher.ShouldCache(req1) {
		t.Error("OCI blob should be cached via content-aware check")
	}

	// Cloudflare R2 path — matched by explicit pattern (no OCI Accept header).
	req2, _ := http.NewRequest("GET", "https://docker-images-prod.example.r2.cloudflarestorage.com/registry-v2/docker/registry/v2/blobs/sha256/49/493218ed0f404132311952996fea8ce85e50c49f5a717f26f25c52a25fcb2e56/data", nil)
	if !matcher.ShouldCache(req2) {
		t.Error("R2 registry path should be cached via explicit pattern")
	}

	// Non-matching path — neither check passes.
	req3, _ := http.NewRequest("GET", "https://example.com/some/random/path", nil)
	req3.Header.Set("Accept", dockerAccept)
	if matcher.ShouldCache(req3) {
		t.Error("unrelated path should not be cached")
	}

	// R2 presigned URL — has query params but sha256 in path guarantees identity.
	req4, _ := http.NewRequest("GET", "https://docker-images-prod.example.r2.cloudflarestorage.com/registry-v2/docker/registry/v2/blobs/sha256/a3/a3629ac5b9f4680dc2032439ff2354e73b06aecc2e68f0035a2d7c001c8b4114/data?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Signature=abc123", nil)
	if !matcher.ShouldCache(req4) {
		t.Error("R2 presigned URL with sha256 in path should be cached despite query params")
	}

	// OCI blob presigned URL — content-aware with query params should also work.
	req5, _ := http.NewRequest("GET", "https://ghcr.io/v2/foo/blobs/sha256:abc123def456abc123def456abc123def456abc123def456abc123def456abcd?token=xyz", nil)
	req5.Header.Set("Accept", dockerAccept)
	if !matcher.ShouldCache(req5) {
		t.Error("OCI blob URL with query params should be cached via content-aware check")
	}
}

func TestMatcher_VerifyDigest(t *testing.T) {
	body := []byte("hello world")
	// pre-computed: echo -n "hello world" | sha256sum
	correctDigest := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	wrongDigest := "0000000000000000000000000000000000000000000000000000000000000000"

	t.Run("OCI colon format - correct digest", func(t *testing.T) {
		m, _ := NewMatcher(nil, true)
		if err := m.VerifyDigest("/v2/foo/blobs/sha256:"+correctDigest, body); err != nil {
			t.Errorf("expected no error for correct digest, got: %v", err)
		}
	})

	t.Run("OCI colon format - wrong digest", func(t *testing.T) {
		m, _ := NewMatcher(nil, true)
		if err := m.VerifyDigest("/v2/foo/blobs/sha256:"+wrongDigest, body); err == nil {
			t.Error("expected error for wrong digest, got nil")
		}
	})

	t.Run("registry storage path format - correct digest", func(t *testing.T) {
		// Cloudflare R2 format: /registry-v2/.../sha256/PREFIX/HEX64/data
		m, _ := NewMatcher(nil, false)
		path := "/registry-v2/docker/registry/v2/blobs/sha256/b9/" + correctDigest + "/data"
		if err := m.VerifyDigest(path, body); err != nil {
			t.Errorf("expected no error for R2 path with correct digest, got: %v", err)
		}
	})

	t.Run("registry storage path format - wrong digest", func(t *testing.T) {
		m, _ := NewMatcher(nil, false)
		path := "/registry-v2/docker/registry/v2/blobs/sha256/00/" + wrongDigest + "/data"
		if err := m.VerifyDigest(path, body); err == nil {
			t.Error("expected error for R2 path with wrong digest, got nil")
		}
	})

	t.Run("CDN redirect path with correct digest", func(t *testing.T) {
		m, _ := NewMatcher(nil, true)
		if err := m.VerifyDigest("/ghcr1/blobs/sha256:"+correctDigest, body); err != nil {
			t.Errorf("expected no error for correct digest in CDN path, got: %v", err)
		}
	})

	t.Run("no digest in path skips verification", func(t *testing.T) {
		m, _ := NewMatcher(nil, true)
		if err := m.VerifyDigest("/v2/ubuntu/manifests/latest", body); err != nil {
			t.Errorf("expected no error when path has no digest, got: %v", err)
		}
	})
}

func TestNewMatcher_ContentAwareParam(t *testing.T) {
	m1, _ := NewMatcher(nil, true)
	if !m1.contentAware {
		t.Error("contentAware=true should enable content-aware mode")
	}

	m2, _ := NewMatcher(nil, false)
	if m2.contentAware {
		t.Error("contentAware=false should not enable content-aware mode")
	}

	// Patterns are always compiled, even alongside content-aware mode.
	m3, err := NewMatcher([]string{`^/registry-v2/`}, true)
	if err != nil {
		t.Fatalf("NewMatcher failed: %v", err)
	}
	if !m3.contentAware {
		t.Error("contentAware=true should be set with patterns")
	}
	if len(m3.patterns) != 1 {
		t.Errorf("expected 1 pattern, got %d", len(m3.patterns))
	}
}
