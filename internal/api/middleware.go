package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/orchis-ai/foundry/internal/auth"
)

type ctxKey int

const (
	ctxUserKey ctxKey = iota
	ctxScopesKey
)

// authClaims resolves either a session cookie or a Bearer PAT to a user.
// Stores the user (and scopes, for PATs) in the request context. Does not
// reject unauthenticated requests — handlers/requireUser decide that.
func (s *Server) authClaims(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			tok := strings.TrimSpace(strings.TrimPrefix(h, "Bearer "))
			if ta, err := s.sessions.ValidateToken(ctx, tok); err == nil && ta.User != nil && !ta.User.Disabled {
				ctx = context.WithValue(ctx, ctxUserKey, ta.User)
				ctx = context.WithValue(ctx, ctxScopesKey, ta.Scopes)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		if u := s.sessions.UserFromRequest(ctx, r); u != nil {
			ctx = context.WithValue(ctx, ctxUserKey, u)
			// Session cookies carry full authority (no scope restriction).
			ctx = context.WithValue(ctx, ctxScopesKey, []string{"*"})
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func userFrom(r *http.Request) *auth.User {
	u, _ := r.Context().Value(ctxUserKey).(*auth.User)
	return u
}

func scopesFrom(r *http.Request) []string {
	s, _ := r.Context().Value(ctxScopesKey).([]string)
	return s
}

// requireUser wraps a handler, returning 401 if not authenticated.
func (s *Server) requireUser(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if userFrom(r) == nil {
			writeError(w, http.StatusUnauthorized, "not signed in")
			return
		}
		h(w, r)
	}
}

// requireScope wraps a handler, enforcing that the caller's token grants the
// scope. Session cookies (scope "*") always pass.
func (s *Server) requireScope(scope string, h http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		if !hasScope(scopesFrom(r), scope) {
			writeError(w, http.StatusForbidden, "token missing required scope: "+scope)
			return
		}
		h(w, r)
	})
}

// requireInstanceAdmin wraps a handler, requiring an authenticated instance
// admin (users.is_admin) — distinct from the per-repo admin role in acl.go.
func (s *Server) requireInstanceAdmin(h http.HandlerFunc) http.HandlerFunc {
	return s.requireUser(func(w http.ResponseWriter, r *http.Request) {
		u := userFrom(r)
		if u == nil || !u.IsAdmin {
			writeError(w, http.StatusForbidden, "admin access required")
			return
		}
		h(w, r)
	})
}

func hasScope(scopes []string, want string) bool {
	for _, s := range scopes {
		if s == "*" || s == want {
			return true
		}
	}
	return false
}
