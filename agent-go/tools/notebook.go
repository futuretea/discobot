package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/obot-platform/discobot/agent-go/message"
	"github.com/obot-platform/discobot/agent-go/thread"
)

type notebookEditInput struct {
	NotebookPath string `json:"notebook_path"`
	NewSource    string `json:"new_source"`
	CellID       string `json:"cell_id"`   // optional cell ID or 0-based index (as string)
	CellType     string `json:"cell_type"` // "code" or "markdown"
	EditMode     string `json:"edit_mode"` // "replace" (default), "insert", "delete"
}

// Jupyter notebook structures.
type notebook struct {
	NBFormat      int             `json:"nbformat"`
	NBFormatMinor int             `json:"nbformat_minor"`
	Metadata      json.RawMessage `json:"metadata"`
	Cells         []notebookCell  `json:"cells"`
}

type notebookCell struct {
	CellType       string            `json:"cell_type"`
	ID             string            `json:"id,omitempty"`
	Source         cellSource        `json:"source"`
	Metadata       json.RawMessage   `json:"metadata,omitempty"`
	ExecutionCount *int              `json:"execution_count,omitempty"`
	Outputs        []json.RawMessage `json:"outputs,omitempty"`
}

// cellSource handles both string and []string notebook cell sources.
type cellSource []string

func (cs *cellSource) UnmarshalJSON(data []byte) error {
	// Try array first.
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*cs = arr
		return nil
	}
	// Try string.
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*cs = []string{s}
	return nil
}

func (e *Executor) executeNotebookEdit(call message.ToolCallPart) (thread.ToolExecuteResult, error) {
	var input notebookEditInput
	if err := unmarshalInput(call, &input); err != nil {
		return errResult(call, err.Error()), nil
	}
	if input.NotebookPath == "" {
		return errResult(call, "notebook_path is required"), nil
	}

	path := resolvePath(e.cwd, input.NotebookPath)

	// Read existing notebook or create a new one.
	var nb notebook
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Create a minimal empty notebook.
			nb = notebook{
				NBFormat:      4,
				NBFormatMinor: 5,
				Metadata:      json.RawMessage(`{}`),
			}
		} else {
			return errResult(call, fmt.Sprintf("failed to read notebook: %v", err)), nil
		}
	} else {
		if err := json.Unmarshal(data, &nb); err != nil {
			return errResult(call, fmt.Sprintf("failed to parse notebook JSON: %v", err)), nil
		}
	}

	editMode := input.EditMode
	if editMode == "" {
		editMode = "replace"
	}

	cellType := input.CellType
	if cellType == "" {
		cellType = "code"
	}

	// Find the target cell index.
	targetIdx := -1
	if input.CellID != "" {
		for i, cell := range nb.Cells {
			if cell.ID == input.CellID {
				targetIdx = i
				break
			}
		}
		if targetIdx < 0 {
			return errResult(call, fmt.Sprintf("cell with id %q not found", input.CellID)), nil
		}
	}

	switch editMode {
	case "replace":
		if targetIdx < 0 {
			// Replace the last cell if no ID given.
			if len(nb.Cells) == 0 {
				return errResult(call, "no cells in notebook to replace; use insert mode to add a cell"), nil
			}
			targetIdx = len(nb.Cells) - 1
		}
		nb.Cells[targetIdx].Source = cellSource{input.NewSource}
		if input.CellType != "" {
			nb.Cells[targetIdx].CellType = cellType
		}

	case "insert":
		newCell := notebookCell{
			CellType: cellType,
			Source:   cellSource{input.NewSource},
			Metadata: json.RawMessage(`{}`),
		}
		if cellType == "code" {
			newCell.Outputs = []json.RawMessage{}
		}
		if targetIdx < 0 {
			// Append at end.
			nb.Cells = append(nb.Cells, newCell)
		} else {
			// Insert after targetIdx.
			nb.Cells = append(nb.Cells, notebookCell{})
			copy(nb.Cells[targetIdx+2:], nb.Cells[targetIdx+1:])
			nb.Cells[targetIdx+1] = newCell
		}

	case "delete":
		if targetIdx < 0 {
			return errResult(call, "cell_id is required for delete mode"), nil
		}
		nb.Cells = append(nb.Cells[:targetIdx], nb.Cells[targetIdx+1:]...)

	default:
		return errResult(call, fmt.Sprintf("unknown edit_mode: %q (use replace, insert, or delete)", editMode)), nil
	}

	// Write the notebook back.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errResult(call, fmt.Sprintf("failed to create parent directory: %v", err)), nil
	}

	out, err := json.MarshalIndent(nb, "", " ")
	if err != nil {
		return errResult(call, fmt.Sprintf("failed to marshal notebook: %v", err)), nil
	}

	if err := os.WriteFile(path, out, 0o644); err != nil {
		return errResult(call, fmt.Sprintf("failed to write notebook: %v", err)), nil
	}

	return textResult(call, fmt.Sprintf("Successfully edited notebook %s (%s mode)", input.NotebookPath, editMode)), nil
}
