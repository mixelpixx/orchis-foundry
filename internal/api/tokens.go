package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/orchis-ai/foundry/internal/auth"
)

// validScopes mirrors NewTokenForm in src/views/devsettings.jsx.
var validScopes = map[string]bool{
	"repo:read": true, "repo:write": true, "repo:admin": true,
	"actions:read": true, "packages:write": true, "user:read": true,
}

func (s *Server) handleListTokens(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	toks, err := s.sessions.ListTokens(r.Context(), u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list tokens")
		return
	}
	if toks == nil {
		toks = []auth.TokenInfo{}
	}
	writeJSON(w, http.StatusOK, toks)
}

func (s *Server) handleCreateToken(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var in struct {
		Name          string   `json:"name"`
		Scopes        []string `json:"scopes"`
		ExpiresInDays int      `json:"expires_in_days"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	clean := make([]string, 0, len(in.Scopes))
	for _, sc := range in.Scopes {
		if validScopes[sc] {
			clean = append(clean, sc)
		}
	}
	created, err := s.sessions.CreateToken(r.Context(), u.ID, in.Name, clean, in.ExpiresInDays)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create token")
		return
	}
	writeJSON(w, http.StatusOK, created)
}

func (s *Server) handleRotateToken(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	// Rotate = read old metadata, delete, recreate with same name+scopes.
	toks, _ := s.sessions.ListTokens(r.Context(), u.ID)
	var name string
	var scopes []string
	found := false
	for _, t := range toks {
		if t.ID == id {
			name, scopes, found = t.Name, t.Scopes, true
			break
		}
	}
	if !found {
		writeError(w, http.StatusNotFound, "token not found")
		return
	}
	_ = s.sessions.DeleteToken(r.Context(), u.ID, id)
	created, err := s.sessions.CreateToken(r.Context(), u.ID, name, scopes, 0)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not rotate token")
		return
	}
	writeJSON(w, http.StatusOK, created)
}

func (s *Server) handleDeleteToken(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := s.sessions.DeleteToken(r.Context(), u.ID, id); err != nil {
		writeError(w, http.StatusInternalServerError, "could not delete token")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
