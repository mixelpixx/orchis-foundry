package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GET /v1/me/sessions — the user's active sessions (current one flagged).
func (s *Server) handleListSessions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	current := s.sessions.SessionID(r)
	list, err := s.sessions.ListSessions(r.Context(), u.ID, current)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list sessions")
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, si := range list {
		out = append(out, map[string]any{
			"id":       si.PubID,
			"created":  relativeTime(si.CreatedAt),
			"lastSeen": relativeTime(si.LastSeenAt),
			"current":  si.Current,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// DELETE /v1/me/sessions/{id} — revoke one session by its public handle.
func (s *Server) handleRevokeSession(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	_ = s.sessions.RevokeSessionByPub(r.Context(), u.ID, chi.URLParam(r, "id"))
	w.WriteHeader(http.StatusNoContent)
}

// POST /v1/me/sessions/revoke-others — sign out everywhere except this device.
func (s *Server) handleRevokeOtherSessions(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	current := s.sessions.SessionID(r)
	_ = s.sessions.RevokeOtherSessions(r.Context(), u.ID, current)
	w.WriteHeader(http.StatusNoContent)
}
