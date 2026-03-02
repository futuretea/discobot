package handler

import (
	"net/http"
)

// ListThreads handles GET /threads — lists all threads.
func (h *Handler) ListThreads(w http.ResponseWriter, _ *http.Request) {
	threads, err := h.completions.ListThreads()
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if threads == nil {
		threads = []string{}
	}
	h.JSON(w, http.StatusOK, map[string]any{"threads": threads})
}
