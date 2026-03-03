package tools

import (
	"fmt"
	"os"
	"strings"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type readInput struct {
	FilePath string `json:"file_path"`
	Offset   int    `json:"offset"` // 1-based line number to start reading from
	Limit    int    `json:"limit"`  // number of lines to read
}

func (e *Executor) executeRead(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input readInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.FilePath == "" {
		return errResult(call, "file_path is required"), nil
	}

	// Resolve path: absolute paths are allowed for the agent.
	path := resolvePath(e.cwd, input.FilePath)

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return errResult(call, fmt.Sprintf("file not found: %s", input.FilePath)), nil
		}
		return errResult(call, err.Error()), nil
	}
	if info.IsDir() {
		return errResult(call, fmt.Sprintf("%s is a directory", input.FilePath)), nil
	}

	const maxBytes = 10 * 1024 * 1024 // 10 MB
	if info.Size() > maxBytes {
		return errResult(call, fmt.Sprintf("file too large (%d bytes, max %d)", info.Size(), maxBytes)), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errResult(call, err.Error()), nil
	}

	// Record the mtime+size so Write/Edit know the file was read.
	e.recordFileRead(path, info)

	content := string(data)

	// Apply line offset and limit if requested.
	if input.Offset > 0 || input.Limit > 0 {
		content = sliceLines(content, input.Offset, input.Limit)
	}

	// Format output with line numbers (matching Claude Code's cat -n style).
	output := addLineNumbers(content, maxOf(input.Offset, 1))

	return textResult(call, output), nil
}

// sliceLines extracts lines [offset, offset+limit) from content.
// offset is 1-based; 0 means start from beginning.
func sliceLines(content string, offset, limit int) string {
	lines := strings.Split(content, "\n")

	start := 0
	if offset > 1 {
		start = offset - 1
	}
	if start >= len(lines) {
		return ""
	}

	end := len(lines)
	if limit > 0 && start+limit < end {
		end = start + limit
	}

	return strings.Join(lines[start:end], "\n")
}

// addLineNumbers prefixes each line with its line number (cat -n style).
// startLine is the 1-based line number of the first line.
func addLineNumbers(content string, startLine int) string {
	if startLine < 1 {
		startLine = 1
	}
	lines := strings.Split(content, "\n")
	// Remove trailing empty line from final newline split.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	var sb strings.Builder
	for i, line := range lines {
		fmt.Fprintf(&sb, "%6d\t%s\n", startLine+i, line)
	}
	return sb.String()
}

func maxOf(a, b int) int {
	if a > b {
		return a
	}
	return b
}
