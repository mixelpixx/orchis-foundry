package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/orchis-ai/foundry/internal/auth"
)

// handleOIDCStart kicks off provider sign-in: mint state, redirect to provider.
func (s *Server) handleOIDCStart(w http.ResponseWriter, r *http.Request) {
	provider := chi.URLParam(r, "provider")
	switch provider {
	case "github":
		if s.github == nil || !s.github.Configured() {
			writeError(w, http.StatusNotImplemented, "GitHub sign-in is not configured")
			return
		}
		state, err := s.sessions.NewOAuthState(r.Context(), "github")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "could not start sign-in")
			return
		}
		http.Redirect(w, r, s.github.AuthURL(state), http.StatusFound)
	default:
		writeError(w, http.StatusNotFound, "unknown provider")
	}
}

// handleOIDCCallback completes sign-in for whichever provider the state names.
func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		writeError(w, http.StatusBadRequest, "missing code or state")
		return
	}
	provider, ok := s.sessions.ConsumeOAuthState(r.Context(), state)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid or expired state")
		return
	}

	var profile *struct {
		Provider, Subject, Handle, Name, Email, Avatar string
	}
	switch provider {
	case "github":
		p, err := s.github.Exchange(r.Context(), code)
		if err != nil {
			s.log.Error("github exchange failed", "err", err)
			writeError(w, http.StatusBadGateway, "sign-in failed talking to GitHub")
			return
		}
		profile = &struct{ Provider, Subject, Handle, Name, Email, Avatar string }{
			p.Provider, p.Subject, p.Handle, p.Name, p.Email, p.Avatar,
		}
	default:
		writeError(w, http.StatusBadRequest, "unknown provider")
		return
	}

	userID, err := s.sessions.UpsertOIDCUser(r.Context(),
		profile.Provider, profile.Subject, profile.Handle, profile.Name, profile.Email, profile.Avatar)
	if err != nil {
		s.log.Error("upsert user failed", "err", err)
		writeError(w, http.StatusInternalServerError, "could not create your account")
		return
	}
	if err := s.sessions.CreateSession(r.Context(), w, r, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not create session")
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.sessions.Destroy(r.Context(), w, r)
	w.WriteHeader(http.StatusNoContent)
}

// handleMe returns the current user or 401.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil {
		writeError(w, http.StatusUnauthorized, "not signed in")
		return
	}
	writeJSON(w, http.StatusOK, meResponse(u))
}

// meResponse shapes the user for the frontend (camelCase, plus UI helpers the
// sidebar mock used: initials + a deterministic color).
func meResponse(u *auth.User) map[string]any {
	return map[string]any{
		"id":        u.Handle,
		"handle":    u.Handle,
		"name":      u.Name,
		"email":     u.Email,
		"avatarUrl": u.AvatarURL,
		"isAdmin":   u.IsAdmin,
		"initials":  initials(u.Name, u.Handle),
		"color":     colorFor(u.Handle),
	}
}

func (s *Server) currentUser(r *http.Request) *auth.User {
	return s.sessions.UserFromRequest(r.Context(), r)
}

func initials(name, handle string) string {
	src := name
	if src == "" {
		src = handle
	}
	parts := splitWords(src)
	switch len(parts) {
	case 0:
		return "?"
	case 1:
		if len(parts[0]) >= 2 {
			return upper(parts[0][:2])
		}
		return upper(parts[0])
	default:
		return upper(string(parts[0][0]) + string(parts[len(parts)-1][0]))
	}
}

func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' || r == '-' || r == '_' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func upper(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

// colorFor returns a stable oklch color string based on the handle, matching
// the prototype's avatar color style.
func colorFor(handle string) string {
	var sum int
	for _, c := range handle {
		sum += int(c)
	}
	hue := sum % 360
	return "oklch(60% 0.14 " + itoa(hue) + ")"
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b [20]byte
	p := len(b)
	for i > 0 {
		p--
		b[p] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		p--
		b[p] = '-'
	}
	return string(b[p:])
}
