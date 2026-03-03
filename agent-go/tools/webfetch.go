package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type webFetchInput struct {
	URL    string `json:"url"`
	Prompt string `json:"prompt"`
}

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
	CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("too many redirects")
		}
		return nil
	},
}

func (e *Executor) executeWebFetch(ctx context.Context, call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input webFetchInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.URL == "" {
		return errResult(call, "url is required"), nil
	}

	// Upgrade http to https.
	rawURL := input.URL
	if strings.HasPrefix(rawURL, "http://") {
		rawURL = "https://" + strings.TrimPrefix(rawURL, "http://")
	}

	pageURL, err := url.Parse(rawURL)
	if err != nil {
		return errResult(call, fmt.Sprintf("invalid URL: %v", err)), nil
	}

	req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
	if err != nil {
		return errResult(call, fmt.Sprintf("invalid URL: %v", err)), nil
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Discobot/1.0)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.9,*/*;q=0.8")

	resp, err := httpClient.Do(req)
	if err != nil {
		return errResult(call, fmt.Sprintf("request failed: %v", err)), nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return errResult(call, fmt.Sprintf("HTTP %d: %s", resp.StatusCode, resp.Status)), nil
	}

	const maxBody = 5 * 1024 * 1024 // 5 MB
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return errResult(call, fmt.Sprintf("failed to read response: %v", err)), nil
	}

	contentType := resp.Header.Get("Content-Type")
	var markdown string
	if strings.Contains(contentType, "text/html") || contentType == "" {
		markdown, err = htmlToMarkdown(body, pageURL)
		if err != nil {
			return errResult(call, fmt.Sprintf("failed to convert page: %v", err)), nil
		}
	} else {
		// Plain text or other — return as-is.
		markdown = string(body)
	}

	// Trim to a reasonable length.
	const maxContent = 100_000
	if len(markdown) > maxContent {
		markdown = markdown[:maxContent] + "\n\n[Content truncated]"
	}

	return textResult(call, markdown), nil
}

// htmlToMarkdown converts raw HTML to Markdown using a two-step pipeline:
// 1. go-readability extracts the main article content (strips nav, ads, scripts).
// 2. html-to-markdown converts the cleaned HTML to Markdown.
func htmlToMarkdown(body []byte, pageURL *url.URL) (string, error) {
	// Step 1: Extract readable content.
	article, err := readability.FromReader(bytes.NewReader(body), pageURL)
	if err != nil {
		// If readability fails, fall back to converting the raw HTML.
		return convertHTMLToMarkdown(body)
	}

	// Build a clean HTML fragment from the article.
	var extracted bytes.Buffer
	if title := article.Title(); title != "" {
		fmt.Fprintf(&extracted, "<h1>%s</h1>\n", title)
	}
	if err := article.RenderHTML(&extracted); err != nil {
		return convertHTMLToMarkdown(body)
	}

	// Step 2: Convert the clean HTML to Markdown.
	return convertHTMLToMarkdown(extracted.Bytes())
}

// convertHTMLToMarkdown converts raw HTML bytes to Markdown using
// JohannesKaufmann/html-to-markdown/v2.
func convertHTMLToMarkdown(html []byte) (string, error) {
	md, err := htmltomarkdown.ConvertReader(bytes.NewReader(html))
	if err != nil {
		return "", err
	}
	return string(md), nil
}
