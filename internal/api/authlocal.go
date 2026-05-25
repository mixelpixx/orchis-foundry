package api

import (
	"encoding/json"
	"net/http"
)

// GET /v1/auth/methods — public. Tells the sign-in screen which options to show.
func (s *Server) handleAuthMethods(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"localLogin":  s.settingBool(r.Context(), "auth.local_login", true),
		"localSignup": s.settingBool(r.Context(), "auth.local_signup", false),
		"providers":   s.enabledProviders(r.Context()),
	})
}

// POST /v1/auth/login { identifier, password } — public. identifier is email or handle.
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !s.settingBool(r.Context(), "auth.local_login", true) {
		writeError(w, http.StatusForbidden, "password sign-in is disabled on this instance")
		return
	}
	var in struct {
		Identifier string `json:"identifier"`
		Email      string `json:"email"`
		Password   string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	id := in.Identifier
	if id == "" {
		id = in.Email
	}
	u, err := s.sessions.VerifyLogin(r.Context(), id, in.Password)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err := s.sessions.CreateSession(r.Context(), w, r, u.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not start session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "handle": u.Handle})
}

// POST /v1/auth/signup { email, handle?, name?, password } — public, only when
// open signup is enabled by an admin.
func (s *Server) handleSignup(w http.ResponseWriter, r *http.Request) {
	if !s.settingBool(r.Context(), "auth.local_signup", false) {
		writeError(w, http.StatusForbidden, "open sign-up is disabled on this instance")
		return
	}
	var in struct {
		Email    string `json:"email"`
		Handle   string `json:"handle"`
		Name     string `json:"name"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	uid, err := s.sessions.CreateLocalUser(r.Context(), in.Email, in.Handle, in.Name, in.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.sessions.CreateSession(r.Context(), w, r, uid); err != nil {
		writeError(w, http.StatusInternalServerError, "account created but session failed; try signing in")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
