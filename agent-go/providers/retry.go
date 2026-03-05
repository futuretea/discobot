package providers

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"
)

// RetryConfig holds parameters for exponential-backoff HTTP retries.
type RetryConfig struct {
	// MaxAttempts is the total number of attempts including the first (default 5).
	MaxAttempts int
	// InitialDelay is the delay before the second attempt (default 1s).
	InitialDelay time.Duration
	// MaxDelay caps the inter-attempt delay (default 30s).
	MaxDelay time.Duration
	// NotifyAfter is the number of failed attempts before onRetry receives a
	// non-empty message for user-visible notification (default 2).
	NotifyAfter int
}

// DefaultRetry is the recommended RetryConfig for provider HTTP calls.
var DefaultRetry = RetryConfig{
	MaxAttempts:  5,
	InitialDelay: time.Second,
	MaxDelay:     30 * time.Second,
	NotifyAfter:  2,
}

// DoWithRetry executes do() with exponential-backoff retry on transient failures.
//
// Retriable conditions:
//   - Network/transport errors that are not due to context cancellation
//   - HTTP 429 Too Many Requests (Retry-After header is respected when present)
//   - HTTP 5xx server errors
//
// Non-retriable conditions (returned immediately):
//   - Context cancellation or deadline exceeded
//   - HTTP 4xx errors other than 429 (e.g., 400, 401, 403, 404)
//   - parseError returning retriable=false
//
// parseError is called on every non-200 response. It receives the status code
// and response body and returns a descriptive error and whether to retry.
//
// onRetry is called before each retry sleep. attempt is 1-based (1 = first failure).
// For attempt < cfg.NotifyAfter, msg is empty so callers stay quiet while waiting
// on early retries. Starting at cfg.NotifyAfter, msg carries the last error text
// so callers can surface a visible warning to the user. Return false to abort.
func DoWithRetry(
	ctx context.Context,
	cfg RetryConfig,
	do func() (*http.Response, error),
	parseError func(statusCode int, body []byte) (retriable bool, err error),
	onRetry func(attempt int, msg string) bool,
) (*http.Response, error) {
	var lastErr error
	delay := cfg.InitialDelay

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := do()
		if err != nil {
			// Context cancellation: not retriable.
			if ctx.Err() != nil {
				return nil, err
			}
			lastErr = fmt.Errorf("request failed: %w", err)
		} else if resp.StatusCode == http.StatusOK {
			return resp, nil
		} else {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()

			retriable, parsedErr := parseError(resp.StatusCode, body)
			if parsedErr == nil {
				// Fallback: build a minimal error from status + body.
				parsedErr = fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
				retriable = resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
			}
			if !retriable {
				return nil, parsedErr
			}

			// For 429, respect Retry-After if it fits within MaxDelay.
			if resp.StatusCode == http.StatusTooManyRequests {
				if ra := resp.Header.Get("Retry-After"); ra != "" {
					if secs, parseErr := strconv.Atoi(ra); parseErr == nil {
						if d := time.Duration(secs) * time.Second; d <= cfg.MaxDelay {
							delay = d
						}
					}
				}
			}
			lastErr = parsedErr
		}

		// On the last attempt, stop without sleeping or notifying.
		if attempt == cfg.MaxAttempts {
			break
		}

		// Notify the caller. Suppress the message on early attempts so minor
		// blips don't surface visible warnings.
		msg := ""
		if attempt >= cfg.NotifyAfter && lastErr != nil {
			msg = lastErr.Error()
		}
		if onRetry != nil && !onRetry(attempt, msg) {
			return nil, lastErr
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}

		// Exponential backoff capped at MaxDelay.
		delay = time.Duration(math.Min(float64(delay*2), float64(cfg.MaxDelay)))
	}

	return nil, lastErr
}
