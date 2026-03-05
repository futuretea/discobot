package integration_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/tools"
)

func newExecutor(t *testing.T) *tools.Executor {
	t.Helper()
	return tools.New(t.TempDir(), t.TempDir(), t.Name())
}

func fetchURL(t *testing.T, e *tools.Executor, url string) string {
	t.Helper()
	input, _ := json.Marshal(map[string]string{"url": url, "prompt": "summarise"})
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "test-fetch",
		ToolName:   "WebFetch",
		Input:      input,
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	return outputText(result.Result.Output)
}

func outputText(out message.ToolResultOutput) string {
	switch v := out.(type) {
	case message.TextOutput:
		return v.Value
	case message.ErrorTextOutput:
		return v.Value
	}
	return ""
}

// TestWebFetch_ExampleCom verifies basic fetch + markdown conversion works
// against the simplest possible stable page.
func TestWebFetch_ExampleCom(t *testing.T) {
	t.Parallel()
	out := fetchURL(t, newExecutor(t), "https://example.com")

	if !strings.Contains(out, "Example Domain") {
		t.Errorf("expected 'Example Domain' in output, got:\n%s", out)
	}
}

// TestWebFetch_NoRawHTML verifies the output is Markdown, not raw HTML.
func TestWebFetch_NoRawHTML(t *testing.T) {
	t.Parallel()
	out := fetchURL(t, newExecutor(t), "https://example.com")

	if strings.Contains(out, "<html") || strings.Contains(out, "<body") || strings.Contains(out, "<div") {
		t.Errorf("output contains raw HTML tags — converter not working:\n%s", out)
	}
}

// TestWebFetch_ArticleExtraction fetches a structured article page and checks
// that readability extraction produced meaningful content without nav/scripts.
func TestWebFetch_ArticleExtraction(t *testing.T) {
	t.Parallel()
	out := fetchURL(t, newExecutor(t), "https://go.dev/blog/go1.21")

	if len(out) < 200 {
		t.Errorf("expected substantial content (>200 chars), got %d chars", len(out))
	}
	if strings.Contains(out, "<script") || strings.Contains(out, "<nav") {
		t.Errorf("readability failed to strip nav/script:\n%.500s", out)
	}
}

// TestWebFetch_404 checks that a 404 response produces an error result, not a panic.
func TestWebFetch_404(t *testing.T) {
	t.Parallel()
	e := newExecutor(t)
	input, _ := json.Marshal(map[string]string{"url": "https://httpbin.org/status/404"})
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "test-404",
		ToolName:   "WebFetch",
		Input:      input,
	})
	if err != nil {
		t.Fatalf("Execute should return error result, not a Go error: %v", err)
	}
	errOut, ok := result.Result.Output.(message.ErrorTextOutput)
	if !ok {
		t.Fatalf("expected ErrorTextOutput for HTTP 404, got %T", result.Result.Output)
	}
	if !strings.Contains(errOut.Value, "404") {
		t.Errorf("expected '404' in error text, got: %s", errOut.Value)
	}
}

// TestWebFetch_BadURL checks that an unreachable host produces an error result.
func TestWebFetch_BadURL(t *testing.T) {
	t.Parallel()
	e := newExecutor(t)
	input, _ := json.Marshal(map[string]string{"url": "https://this-host-does-not-exist.invalid/"})
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "test-bad-url",
		ToolName:   "WebFetch",
		Input:      input,
	})
	if err != nil {
		t.Fatalf("Execute should return error result, not a Go error: %v", err)
	}
	if _, ok := result.Result.Output.(message.ErrorTextOutput); !ok {
		t.Errorf("expected ErrorTextOutput for bad host, got %T", result.Result.Output)
	}
}

// TestWebSearch_Tavily exercises the full Tavily search pipeline.
// Requires TAVILY_API_KEY to be set; skipped otherwise.
func TestWebSearch_Tavily(t *testing.T) {
	t.Parallel()
	if os.Getenv("TAVILY_API_KEY") == "" {
		t.Skip("TAVILY_API_KEY not set")
	}

	e := newExecutor(t)
	input, _ := json.Marshal(map[string]string{"query": "Go programming language"})
	result, err := e.Execute(context.Background(), nil, message.ToolCallPart{
		ToolCallID: "test-search",
		ToolName:   "WebSearch",
		Input:      input,
	})
	if err != nil {
		t.Fatalf("Execute(WebSearch) error: %v", err)
	}
	out, ok := result.Result.Output.(message.TextOutput)
	if !ok {
		t.Fatalf("expected TextOutput, got %T: %s", result.Result.Output, outputText(result.Result.Output))
	}
	if !strings.Contains(out.Value, "Result") {
		t.Errorf("expected formatted results in output, got:\n%s", out.Value)
	}
}
