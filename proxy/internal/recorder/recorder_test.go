package recorder

import (
	"bytes"
	"io"
	"net/http"
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

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		ct   string
		want bool
	}{
		{"image/png", true},
		{"image/jpeg", true},
		{"image/webp", true},
		{"video/mp4", true},
		{"audio/mpeg", true},
		{"font/woff2", true},
		{"application/octet-stream", true},
		{"application/zip", true},
		{"application/gzip", true},
		{"application/x-gzip", true},
		{"application/x-tar", true},
		{"application/x-bz2", true},
		{"application/x-xz", true},
		{"application/zstd", true},
		{"application/x-zstd", true},
		{"application/pdf", true},
		{"application/wasm", true},
		{"application/vnd.docker.image.rootfs.diff.tar.gzip", true},
		{"application/vnd.oci.image.layer.v1.tar+gzip", true},
		// Text / structured types must NOT be skipped.
		{"application/json", false},
		{"application/json; charset=utf-8", false},
		{"text/plain", false},
		{"text/html; charset=utf-8", false},
		{"application/x-www-form-urlencoded", false},
		{"application/xml", false},
		// Empty content-type should not be treated as binary.
		{"", false},
		// Case-insensitive.
		{"IMAGE/PNG", true},
		{"Application/Zip", true},
	}
	for _, tt := range tests {
		got := isBinaryContentType(tt.ct)
		if got != tt.want {
			t.Errorf("isBinaryContentType(%q) = %v, want %v", tt.ct, got, tt.want)
		}
	}
}

func TestCaptureRequestBody_SkipsBinaryContentType(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("PNG\x89binary data")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "image/png")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured for binary content-type, got %d bytes", len(entry.Request.Body))
	}
	// Body stream must still be intact for downstream use.
	got, _ := io.ReadAll(req.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("request body not restored: got %q, want %q", got, body)
	}
}

func TestCaptureRequestBody_SkipsNullBytes(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("some\x00binary\x00data")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/octet-stream")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured when null bytes present, got %d bytes", len(entry.Request.Body))
	}
}

func TestCaptureRequestBody_NullBytesStreamRestored(t *testing.T) {
	// Even when null bytes cause the capture to be discarded, the request body
	// must remain readable by downstream handlers.
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("hello\x00world")
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "text/plain") // not binary MIME, so passes first check
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if len(entry.Request.Body) != 0 {
		t.Errorf("expected no body captured when null bytes present, got %d bytes", len(entry.Request.Body))
	}
	got, _ := io.ReadAll(req.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("request body not restored after null-byte discard: got %q, want %q", got, body)
	}
}

func TestCaptureResponseBody_SkipsBinaryContentType(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte("\x1f\x8b binary gz content")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/gzip"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
	}
	entry := &Entry{Response: &ResponseInfo{}}

	r.CaptureResponseBody(entry, resp)

	if len(entry.Response.Body) != 0 {
		t.Errorf("expected no body captured for binary content-type, got %d bytes", len(entry.Response.Body))
	}
	got, _ := io.ReadAll(resp.Body)
	if !bytes.Equal(got, body) {
		t.Errorf("response body not restored: got %q, want %q", got, body)
	}
}

func TestCaptureRequestBody_TextNotSkipped(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	body := []byte(`{"key":"value"}`)
	req, _ := http.NewRequest(http.MethodPost, "http://example.com", io.NopCloser(bytes.NewReader(body)))
	req.Header.Set("Content-Type", "application/json")
	entry := NewEntry(req)

	r.CaptureRequestBody(entry, req)

	if !bytes.Equal(entry.Request.Body, body) {
		t.Errorf("expected body captured for text content-type, got %q", entry.Request.Body)
	}
}

func TestBeginResponseCapture_StreamsWithinLimit(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: 5}}
	entry := &Entry{Response: &ResponseInfo{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	capture := r.BeginResponseCapture(entry, resp)
	if capture == nil {
		t.Fatal("expected streaming response capture")
	}

	capture.Write([]byte("he"))
	capture.Write([]byte("llo"))
	capture.Write([]byte(" world"))
	capture.Finish()

	if !bytes.Equal(entry.Response.Body, []byte("hello")) {
		t.Fatalf("captured body = %q, want %q", entry.Response.Body, "hello")
	}
	if !entry.Response.BodyTruncated {
		t.Fatal("expected truncated body flag after exceeding limit")
	}
}

func TestBeginResponseCapture_DropsBinaryPayloadsDetectedByNullByte(t *testing.T) {
	r := &Recorder{cfg: Config{Enabled: true, MaxBodySize: -1}}
	entry := &Entry{Response: &ResponseInfo{}}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/plain"}},
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}

	capture := r.BeginResponseCapture(entry, resp)
	if capture == nil {
		t.Fatal("expected streaming response capture")
	}

	capture.Write([]byte("hello"))
	capture.Write([]byte{'\x00'})
	capture.Write([]byte("world"))
	capture.Finish()

	if len(entry.Response.Body) != 0 {
		t.Fatalf("expected null-byte payload to be discarded, got %q", entry.Response.Body)
	}
	if entry.Response.BodyTruncated {
		t.Fatal("expected truncated flag to remain false when payload is discarded")
	}
}
