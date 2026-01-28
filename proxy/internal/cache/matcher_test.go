package cache

import (
	"net/http"
	"testing"
)

func TestMatcher_ShouldCache(t *testing.T) {
	patterns := []string{
		`^/v2/.*/blobs/sha256:.*`,
		`^/v2/.*/manifests/sha256:.*`,
	}

	matcher, err := NewMatcher(patterns)
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
			query:    "",
			expected: true,
		},
		{
			name:     "docker manifest by digest - should cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/manifests/sha256:xyz789",
			query:    "",
			expected: true,
		},
		{
			name:     "docker manifest by tag - should not cache",
			method:   "GET",
			path:     "/v2/library/ubuntu/manifests/latest",
			query:    "",
			expected: false,
		},
		{
			name:     "POST request - should not cache",
			method:   "POST",
			path:     "/v2/library/ubuntu/blobs/sha256:abc123",
			query:    "",
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
			query:    "",
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

func TestMatcher_ShouldCacheResponse(t *testing.T) {
	matcher, err := NewMatcher([]string{`.*`})
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
	matcher, err := NewMatcher([]string{`.*`})
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

func TestDefaultDockerPatterns(t *testing.T) {
	patterns := DefaultDockerPatterns()

	if len(patterns) != 2 {
		t.Errorf("expected 2 default patterns, got %d", len(patterns))
	}

	// Test that patterns compile
	matcher, err := NewMatcher(patterns)
	if err != nil {
		t.Fatalf("default patterns failed to compile: %v", err)
	}

	// Test blob pattern
	req1, _ := http.NewRequest("GET", "http://r.io/v2/ubuntu/blobs/sha256:abc", nil)
	if !matcher.ShouldCache(req1) {
		t.Error("blob pattern should match")
	}

	// Test manifest pattern
	req2, _ := http.NewRequest("GET", "http://r.io/v2/ubuntu/manifests/sha256:def", nil)
	if !matcher.ShouldCache(req2) {
		t.Error("manifest pattern should match")
	}

	// Test non-digest manifest (should not match)
	req3, _ := http.NewRequest("GET", "http://r.io/v2/ubuntu/manifests/latest", nil)
	if matcher.ShouldCache(req3) {
		t.Error("tag-based manifest should not match")
	}
}

func TestMatcher_InvalidPattern(t *testing.T) {
	_, err := NewMatcher([]string{"[invalid"})
	if err == nil {
		t.Error("expected error for invalid regex pattern")
	}
}
