package providers_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/obot-platform/discobot/agent-go/providers"
)

// fastRetry is a tight RetryConfig for tests that avoids real sleeps.
var fastRetry = providers.RetryConfig{
	MaxAttempts:  4,
	InitialDelay: time.Millisecond,
	MaxDelay:     10 * time.Millisecond,
	NotifyAfter:  2,
}

// noopParseError always returns (retriable based on status, error).
func noopParseError(statusCode int, body []byte) (bool, error) {
	retriable := statusCode == http.StatusTooManyRequests || statusCode >= 500
	return retriable, fmt.Errorf("API error %d: %s", statusCode, string(body))
}

func TestDoWithRetry_SuccessFirstAttempt(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	calls := 0
	resp, err := providers.DoWithRetry(context.Background(), fastRetry,
		func() (*http.Response, error) {
			calls++
			return http.Get(srv.URL) //nolint:noctx
		},
		noopParseError,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("expected body 'ok', got %q", string(body))
	}
}

func TestDoWithRetry_RetryOn5xx(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, "unavailable")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	notified := 0
	resp, err := providers.DoWithRetry(context.Background(), fastRetry,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		noopParseError,
		func(_ int, msg string) bool {
			if msg != "" {
				notified++
			}
			return true
		},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
	// First failure is silent; second triggers notification (NotifyAfter=2).
	if notified != 1 {
		t.Errorf("expected 1 notification, got %d", notified)
	}
}

func TestDoWithRetry_NoRetryOn4xx(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid key"}`)
	}))
	defer srv.Close()

	_, err := providers.DoWithRetry(context.Background(), fastRetry,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		func(statusCode int, body []byte) (bool, error) {
			return false, fmt.Errorf("API error %d: %s", statusCode, string(body)) // non-retriable
		},
		nil,
	)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for 4xx), got %d", calls)
	}
}

func TestDoWithRetry_ExhaustsMaxAttempts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
		fmt.Fprint(w, "still down")
	}))
	defer srv.Close()

	cfg := providers.RetryConfig{
		MaxAttempts:  3,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
		NotifyAfter:  1,
	}
	_, err := providers.DoWithRetry(context.Background(), cfg,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		noopParseError,
		func(_ int, _ string) bool { return true },
	)
	if err == nil {
		t.Fatal("expected error after exhausting attempts")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	if !strings.Contains(err.Error(), "503") && !strings.Contains(err.Error(), "still down") {
		t.Errorf("expected error to mention failure, got: %v", err)
	}
}

func TestDoWithRetry_RetryOn429WithRetryAfter(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, "rate limited")
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	}))
	defer srv.Close()

	resp, err := providers.DoWithRetry(context.Background(), fastRetry,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		noopParseError,
		func(_ int, _ string) bool { return true },
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestDoWithRetry_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		cancel() // cancel before responding
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	_, err := providers.DoWithRetry(ctx, fastRetry,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		noopParseError,
		func(_ int, _ string) bool { return true },
	)
	if err == nil {
		t.Fatal("expected error from context cancellation")
	}
}

func TestDoWithRetry_OnRetryAbort(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	cfg := providers.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
		MaxDelay:     time.Millisecond,
		NotifyAfter:  1,
	}
	_, err := providers.DoWithRetry(context.Background(), cfg,
		func() (*http.Response, error) { return http.Get(srv.URL) }, //nolint:noctx
		noopParseError,
		func(attempt int, _ string) bool {
			return attempt < 2 // abort after 2nd failure
		},
	)
	if err == nil {
		t.Fatal("expected error after abort")
	}
	if calls != 2 {
		t.Errorf("expected 2 calls before abort, got %d", calls)
	}
}
