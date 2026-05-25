package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// Instance administration: auth-method configuration + user lifecycle.
// All handlers are mounted behind requireAdmin.

func boolStr(b bool) string {
	if b {
		return "on"
	}
	return "off"
}

// GET /v1/admin/settings — current auth configuration for the admin panel.
func (s *Server) handleAdminGetSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	providers := []map[string]any{}
	for _, p := range s.cfg.OIDC {
		if p.ID == "" || p.ClientID == "" {
			continue
		}
		providers = append(providers, map[string]any{
			"id":      p.ID,
			"label":   providerLabel(p.ID),
			"enabled": s.settingBool(ctx, "auth.provider."+p.ID, true),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"localLogin":  s.settingBool(ctx, "auth.local_login", true),
		"localSignup": s.settingBool(ctx, "auth.local_signup", false),
		"providers":   providers,
	})
}

// PUT /v1/admin/settings { localLogin?, localSignup?, providers?: {id: bool} }
func (s *Server) handleAdminPutSettings(w http.ResponseWriter, r *http.Request) {
	var in struct {
		LocalLogin  *bool           `json:"localLogin"`
		LocalSignup *bool           `json:"localSignup"`
		Providers   map[string]bool `json:"providers"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	ctx := r.Context()
	if in.LocalLogin != nil {
		s.setSetting(ctx, "auth.local_login", boolStr(*in.LocalLogin))
	}
	if in.LocalSignup != nil {
		s.setSetting(ctx, "auth.local_signup", boolStr(*in.LocalSignup))
	}
	// Only accept toggles for providers that actually exist in config.
	known := map[string]bool{}
	for _, p := range s.cfg.OIDC {
		known[p.ID] = true
	}
	for id, on := range in.Providers {
		if known[id] {
			s.setSetting(ctx, "auth.provider."+id, boolStr(on))
		}
	}
	s.handleAdminGetSettings(w, r)
}

// GET /v1/admin/users — list all users for management.
func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT id, handle, email, name, is_admin, disabled, password_hash != '' AS has_pw, created_at, COALESCE(last_login_at,'')
		 FROM users ORDER BY id`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list users")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		var handle, email, name, created, lastLogin string
		var admin, disabled, hasPw int
		if rows.Scan(&id, &handle, &email, &name, &admin, &disabled, &hasPw, &created, &lastLogin) == nil {
			out = append(out, map[string]any{
				"id": id, "handle": handle, "email": email, "name": name,
				"isAdmin": admin == 1, "disabled": disabled == 1, "hasPassword": hasPw == 1,
				"created": relativeTime(created), "lastLogin": lastLogin,
			})
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/admin/users { email, handle?, name?, password } — provision a local account.
func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email, Handle, Name, Password string
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
	writeJSON(w, http.StatusOK, map[string]any{"id": uid})
}

// countActiveAdmins returns how many enabled admins exist (for last-admin guards).
func (s *Server) countActiveAdmins(r *http.Request) int {
	var n int
	s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM users WHERE is_admin = 1 AND disabled = 0`).Scan(&n)
	return n
}

// PATCH /v1/admin/users/{id} { isAdmin?, disabled?, password? }
func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var cur struct {
		admin, disabled int
	}
	if s.db.QueryRowContext(r.Context(), `SELECT is_admin, disabled FROM users WHERE id = ?`, id).
		Scan(&cur.admin, &cur.disabled) != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	var in struct {
		IsAdmin  *bool   `json:"isAdmin"`
		Disabled *bool   `json:"disabled"`
		Password *string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// Guard: never strip the last remaining active admin (demote or disable).
	losingAdmin := (in.IsAdmin != nil && !*in.IsAdmin && cur.admin == 1) ||
		(in.Disabled != nil && *in.Disabled && cur.admin == 1)
	if losingAdmin && s.countActiveAdmins(r) <= 1 {
		writeError(w, http.StatusConflict, "cannot remove the last active admin")
		return
	}
	if in.IsAdmin != nil {
		s.db.ExecContext(r.Context(), `UPDATE users SET is_admin = ? WHERE id = ?`, b2i(*in.IsAdmin), id)
	}
	if in.Disabled != nil {
		s.db.ExecContext(r.Context(), `UPDATE users SET disabled = ? WHERE id = ?`, b2i(*in.Disabled), id)
		if *in.Disabled {
			// Revoke active sessions on disable (immediate deprovision).
			s.db.ExecContext(r.Context(), `DELETE FROM sessions WHERE user_id = ?`, id)
		}
	}
	if in.Password != nil {
		if err := s.sessions.SetPassword(r.Context(), id, *in.Password); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// DELETE /v1/admin/users/{id}
func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	u := userFrom(r)
	if u != nil && u.ID == id {
		writeError(w, http.StatusConflict, "you can't delete your own account")
		return
	}
	var admin int
	s.db.QueryRowContext(r.Context(), `SELECT is_admin FROM users WHERE id = ?`, id).Scan(&admin)
	if admin == 1 && s.countActiveAdmins(r) <= 1 {
		writeError(w, http.StatusConflict, "cannot delete the last active admin")
		return
	}
	s.db.ExecContext(r.Context(), `DELETE FROM users WHERE id = ?`, id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}
