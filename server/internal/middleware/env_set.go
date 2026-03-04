package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/store"
)

// EnvSetOwnedByProject validates that the env set identified by the {envSetId}
// URL parameter belongs to the current project. Returns 404 if the env set
// does not exist or belongs to a different project.
//
// Must be applied inside a route that has ProjectMember middleware (so that
// GetProjectID returns a value) and that uses the {envSetId} URL parameter.
func EnvSetOwnedByProject(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			envSetID := chi.URLParam(r, "envSetId")
			projectID := GetProjectID(r.Context())

			envSet, err := s.GetEnvSetByID(r.Context(), envSetID)
			if err != nil || envSet.ProjectID != projectID {
				http.Error(w, `{"error":"Env set not found"}`, http.StatusNotFound)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// EnvSetsOwnedByProject validates that all env set IDs in a request body
// belong to the current project before the request reaches the handler.
// This prevents privilege escalation via cross-project env set references.
//
// It reads the body, checks ownership, then restores the body so the handler
// can decode it normally. Must be applied inside a route that has ProjectMember
// middleware (so that GetProjectID returns a value).
func EnvSetsOwnedByProject(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read and immediately restore the body so the handler can still decode it.
			body, err := io.ReadAll(r.Body)
			r.Body.Close()
			if err != nil {
				http.Error(w, `{"error":"Failed to read request body"}`, http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(body))

			var req struct {
				EnvSetIDs []string `json:"envSetIds"`
			}
			if err := json.Unmarshal(body, &req); err != nil || len(req.EnvSetIDs) == 0 {
				// Nothing to validate — let the handler deal with malformed or empty body.
				next.ServeHTTP(w, r)
				return
			}

			projectID := GetProjectID(r.Context())
			for _, id := range req.EnvSetIDs {
				envSet, err := s.GetEnvSetByID(r.Context(), id)
				if err != nil || envSet.ProjectID != projectID {
					http.Error(w, `{"error":"One or more env sets not found"}`, http.StatusNotFound)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}
