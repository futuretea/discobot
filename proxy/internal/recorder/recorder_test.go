package recorder

import (
	"bytes"
	"io"
	"testing"
)

func TestCaptureStream_UnderLimit(t *testing.T) {
	body := []byte("hello world")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, 100)

	if truncated {
		t.Error("expected truncated=false for body smaller than limit")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

func TestCaptureStream_ExactLimit(t *testing.T) {
	body := []byte("exactly ten!")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, int64(len(body)))

	if truncated {
		t.Error("expected truncated=false for body equal to limit")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

// TestCaptureStream_OverLimit is the regression test for the bug where the
// restored stream was missing the first maxSize bytes when the body exceeded
// the capture limit, causing downstream clients (e.g. Docker) to receive a
// truncated body and fail with "unexpected EOF".
func TestCaptureStream_OverLimit(t *testing.T) {
	body := []byte("abcdefghij") // 10 bytes
	rc := io.NopCloser(bytes.NewReader(body))
	const maxSize = 4

	captured, restored, truncated := captureStream(rc, maxSize)

	if !truncated {
		t.Error("expected truncated=true for body larger than limit")
	}
	if !bytes.Equal(captured, body[:maxSize]) {
		t.Errorf("captured = %q, want %q", captured, body[:maxSize])
	}
	// The restored stream must contain the full original body, not just the
	// bytes from position maxSize onward.
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want full body %q", got, body)
	}
}

func TestCaptureStream_Unlimited(t *testing.T) {
	body := []byte("some large content")
	rc := io.NopCloser(bytes.NewReader(body))

	captured, restored, truncated := captureStream(rc, -1)

	if truncated {
		t.Error("expected truncated=false for unlimited capture")
	}
	if !bytes.Equal(captured, body) {
		t.Errorf("captured = %q, want %q", captured, body)
	}
	got, _ := io.ReadAll(restored)
	if !bytes.Equal(got, body) {
		t.Errorf("restored stream = %q, want %q", got, body)
	}
}

func TestCaptureStream_Empty(t *testing.T) {
	rc := io.NopCloser(bytes.NewReader(nil))

	captured, restored, truncated := captureStream(rc, 100)

	if truncated {
		t.Error("expected truncated=false for empty body")
	}
	if len(captured) != 0 {
		t.Errorf("captured = %q, want empty", captured)
	}
	got, _ := io.ReadAll(restored)
	if len(got) != 0 {
		t.Errorf("restored stream = %q, want empty", got)
	}
}
