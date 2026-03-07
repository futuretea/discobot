package tools

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type globInput struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"` // optional directory to search in
}

func (e *Executor) executeGlob(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input globInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.Pattern == "" {
		return errResult(call, "pattern is required"), nil
	}

	// Determine search root.
	root := e.cwd
	if input.Path != "" {
		root = resolvePath(e.cwd, input.Path)
	}

	// Build the full glob pattern.
	var pattern string
	if filepath.IsAbs(input.Pattern) {
		pattern = input.Pattern
	} else {
		pattern = filepath.Join(root, input.Pattern)
	}

	matches, err := doublestar.FilepathGlob(pattern)
	if err != nil {
		return errResult(call, "invalid glob pattern: "+err.Error()), nil
	}

	if len(matches) == 0 {
		return textResult(call, "No files found"), nil
	}

	type entry struct {
		path string
	}
	entries := make([]entry, 0, len(matches))
	for _, m := range matches {
		// Convert to relative path if inside root.
		rel, relErr := filepath.Rel(root, m)
		if relErr != nil {
			rel = m
		}
		entries = append(entries, entry{path: rel})
	}

	// Simple alphabetical sort as fallback (stat calls add overhead).
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].path < entries[j].path
	})

	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(e.path)
		sb.WriteByte('\n')
	}

	return textResult(call, strings.TrimRight(sb.String(), "\n")), nil
}
