package handler

import (
	"net/http"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/files"
	"github.com/obot-platform/discobot/agent-go/internal/gitops"
)

// GetDiff handles GET /diff — returns workspace diff.
// Query params:
//   - path: optional, single file diff
//   - format: optional, "full" (default) or "files"
func (h *Handler) GetDiff(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	format := r.URL.Query().Get("format")

	// Validate single file path if provided
	if path != "" {
		if _, err := files.ValidatePath(path, h.agentCwd); err != nil {
			h.Error(w, http.StatusBadRequest, "Invalid path")
			return
		}
	}

	diff := gitops.GetDiff(h.agentCwd, path)

	// Handle single file request
	if path != "" {
		var file *gitops.FileDiffEntry
		for i := range diff.Files {
			if diff.Files[i].Path == path {
				file = &diff.Files[i]
				break
			}
		}

		if file == nil {
			h.JSON(w, http.StatusOK, api.SingleFileDiffResponse{
				Path:   path,
				Status: "unchanged",
				Patch:  "",
			})
			return
		}

		h.JSON(w, http.StatusOK, api.SingleFileDiffResponse{
			Path:      file.Path,
			Status:    file.Status,
			OldPath:   file.OldPath,
			Additions: file.Additions,
			Deletions: file.Deletions,
			Binary:    file.Binary,
			Patch:     file.Patch,
		})
		return
	}

	// Handle format=files — return file entries with status only
	if format == "files" {
		fileEntries := make([]api.DiffFileEntry, len(diff.Files))
		for i, f := range diff.Files {
			fileEntries[i] = api.DiffFileEntry{
				Path:    f.Path,
				Status:  f.Status,
				OldPath: f.OldPath,
			}
		}

		h.JSON(w, http.StatusOK, api.DiffFilesResponse{
			Files: fileEntries,
			Stats: api.DiffStats{
				FilesChanged: diff.Stats.FilesChanged,
				Additions:    diff.Stats.Additions,
				Deletions:    diff.Stats.Deletions,
			},
		})
		return
	}

	// Full diff response
	fileEntries := make([]api.FileDiffEntry, len(diff.Files))
	for i, f := range diff.Files {
		fileEntries[i] = api.FileDiffEntry{
			Path:      f.Path,
			Status:    f.Status,
			OldPath:   f.OldPath,
			Additions: f.Additions,
			Deletions: f.Deletions,
			Binary:    f.Binary,
			Patch:     f.Patch,
		}
	}

	h.JSON(w, http.StatusOK, api.DiffResponse{
		Files: fileEntries,
		Stats: api.DiffStats{
			FilesChanged: diff.Stats.FilesChanged,
			Additions:    diff.Stats.Additions,
			Deletions:    diff.Stats.Deletions,
		},
	})
}
