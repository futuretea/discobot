package tools

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type writeInput struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

func (e *Executor) executeWrite(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input writeInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.FilePath == "" {
		return errResult(call, "file_path is required"), nil
	}

	path := resolvePath(e.cwd, input.FilePath)

	// Ensure parent directory exists.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errResult(call, fmt.Sprintf("failed to create parent directory: %v", err)), nil
	}

	if err := os.WriteFile(path, []byte(input.Content), 0o644); err != nil {
		return errResult(call, fmt.Sprintf("failed to write file: %v", err)), nil
	}

	return textResult(call, fmt.Sprintf("Successfully wrote to %s", input.FilePath)), nil
}
