package tools

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type grepInput struct {
	Pattern         string `json:"pattern"`
	Path            string `json:"path"`
	Type            string `json:"type"`        // rg --type
	Glob            string `json:"glob"`        // rg --glob
	OutputMode      string `json:"output_mode"` // "content", "files_with_matches", "count"
	CaseInsensitive bool   `json:"i"`
	Context         int    `json:"context"` // -C lines
	After           int    `json:"A"`       // lines after
	Before          int    `json:"B"`       // lines before
	LineNumbers     bool   `json:"n"`       // show line numbers (default true)
	HeadLimit       int    `json:"head_limit"`
	Offset          int    `json:"offset"`
	Multiline       bool   `json:"multiline"`
}

func (e *Executor) executeGrep(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input grepInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Pattern == "" {
		return errResult(call, "pattern is required"), nil
	}

	// Determine search path.
	searchPath := e.cwd
	if input.Path != "" {
		searchPath = resolvePath(e.cwd, input.Path)
	}

	args := buildRgArgs(input, searchPath)

	cmd := exec.Command("rg", args...)
	cmd.Dir = e.cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	// rg exits 1 when no matches, 2 on error.
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if ok && exitErr.ExitCode() == 1 {
			return textResult(call, "No matches found"), nil
		}
		if ok && exitErr.ExitCode() == 2 {
			errMsg := strings.TrimSpace(stderr.String())
			if errMsg == "" {
				errMsg = err.Error()
			}
			return errResult(call, "grep error: "+errMsg), nil
		}
		// rg not installed — fall back to a message.
		return errResult(call, fmt.Sprintf("rg (ripgrep) not available: %v", err)), nil
	}

	output := stdout.String()

	// Apply offset + head_limit filtering.
	if input.Offset > 0 || input.HeadLimit > 0 {
		output = applyOffsetLimit(output, input.Offset, input.HeadLimit)
	}

	if output == "" {
		return textResult(call, "No matches found"), nil
	}
	return textResult(call, output), nil
}

// buildRgArgs constructs the ripgrep command arguments.
func buildRgArgs(input grepInput, searchPath string) []string {
	var args []string

	// Output mode.
	switch input.OutputMode {
	case "files_with_matches":
		args = append(args, "-l")
	case "count":
		args = append(args, "--count")
	default: // "content" or empty
		// default rg behavior: show matching lines
	}

	// Case insensitive.
	if input.CaseInsensitive {
		args = append(args, "-i")
	}

	// Line numbers (default on for content mode).
	if input.OutputMode == "" || input.OutputMode == "content" {
		// Always include line numbers for content mode; -n is on by default in rg
		// when output is not a tty, but we force it.
		args = append(args, "-n")
	}

	// Context flags.
	if input.Context > 0 {
		args = append(args, fmt.Sprintf("-C%d", input.Context))
	} else {
		if input.Before > 0 {
			args = append(args, fmt.Sprintf("-B%d", input.Before))
		}
		if input.After > 0 {
			args = append(args, fmt.Sprintf("-A%d", input.After))
		}
	}

	// Multiline.
	if input.Multiline {
		args = append(args, "-U", "--multiline-dotall")
	}

	// File type filter.
	if input.Type != "" {
		args = append(args, "--type", input.Type)
	}

	// Glob filter.
	if input.Glob != "" {
		args = append(args, "--glob", input.Glob)
	}

	// Pattern.
	args = append(args, input.Pattern)

	// Search path.
	args = append(args, searchPath)

	return args
}

// applyOffsetLimit skips the first `offset` lines and returns at most `limit` lines.
func applyOffsetLimit(output string, offset, limit int) string {
	lines := strings.Split(output, "\n")
	// Remove trailing empty line.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	if offset > 0 && offset < len(lines) {
		lines = lines[offset:]
	} else if offset >= len(lines) {
		return ""
	}

	if limit > 0 && limit < len(lines) {
		lines = lines[:limit]
	}

	return strings.Join(lines, "\n") + "\n"
}
