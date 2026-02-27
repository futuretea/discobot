package middleware

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/store"
)

const SessionKey contextKey = "session"

// SessionBelongsToProject middleware validates that the session identified by
// the {sessionId} URL parameter belongs to the project identified by the
// {projectId} URL parameter (already validated and stored in context by
// ProjectMember). This prevents IDOR attacks where a user could access
// sessions from other projects by guessing session IDs.
//
// Must be mounted inside a route that defines {sessionId}, e.g.:
//
//	r.Route("/{sessionId}", func(r chi.Router) {
//	    r.Use(middleware.SessionBelongsToProject(s))
//	    ...
//	})
func SessionBelongsToProject(s *store.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := chi.URLParam(r, "sessionId")
			if sessionID == "" {
				http.Error(w, `{"error":"Session ID required"}`, http.StatusBadRequest)
				return
			}

			projectID := GetProjectID(r.Context())
			if projectID == "" {
				http.Error(w, `{"error":"Project ID required"}`, http.StatusBadRequest)
				return
			}

			session, err := s.GetSessionByID(r.Context(), sessionID)
			if err != nil {
				http.Error(w, `{"error":"Session not found"}`, http.StatusNotFound)
				return
			}

			if session.ProjectID != projectID {
				http.Error(w, `{"error":"Session does not belong to this project"}`, http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), SessionKey, session)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetSession extracts the session from context (set by SessionBelongsToProject middleware).
func GetSession(ctx context.Context) *model.Session {
	if s, ok := ctx.Value(SessionKey).(*model.Session); ok {
		return s
	}
	return nil
}
