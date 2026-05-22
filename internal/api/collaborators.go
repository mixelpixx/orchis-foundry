package api

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

var validRoles = map[string]bool{roleRead: true, roleWrite: true, roleAdmin: true}

// GET /v1/repos/{org}/{name}/collaborators — list collaborators (members who
// can read the repo can see the roster). Includes each one's role.
func (s *Server) handleListCollaborators(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT u.id, u.handle, u.name, c.role
		 FROM repo_collaborators c JOIN users u ON u.id = c.user_id
		 WHERE c.repo_id = ? ORDER BY u.handle`, row.ID)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var id int64
			var handle, name, role string
			if rows.Scan(&id, &handle, &name, &role) == nil {
				out = append(out, map[string]any{
					"handle": handle, "name": name, "role": role,
					"initials": initials(name, handle), "color": colorFor(handle),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// PUT /v1/repos/{org}/{name}/collaborators/{handle} { role } — add/update a
// collaborator (admin only).
func (s *Server) handleSetCollaborator(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireAdmin(w, row) {
		return
	}
	handle := chi.URLParam(r, "handle")
	var in struct {
		Role string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	in.Role = strings.TrimSpace(in.Role)
	if !validRoles[in.Role] {
		writeError(w, http.StatusBadRequest, "role must be read, write, or admin")
		return
	}
	var uid int64
	if err := s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, handle).Scan(&uid); err != nil {
		writeError(w, http.StatusNotFound, "no such user: "+handle)
		return
	}
	if row.OwnerUserID.Valid && row.OwnerUserID.Int64 == uid {
		writeError(w, http.StatusBadRequest, "the owner already has full access")
		return
	}
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO repo_collaborators (repo_id, user_id, role) VALUES (?,?,?)
		 ON CONFLICT(repo_id, user_id) DO UPDATE SET role = excluded.role`, row.ID, uid, in.Role)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not set collaborator")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"handle": handle, "role": in.Role})
}

// DELETE /v1/repos/{org}/{name}/collaborators/{handle} — remove a collaborator
// (admin only).
func (s *Server) handleRemoveCollaborator(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireAdmin(w, row) {
		return
	}
	handle := chi.URLParam(r, "handle")
	var uid int64
	if s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, handle).Scan(&uid) == nil {
		s.db.ExecContext(r.Context(), `DELETE FROM repo_collaborators WHERE repo_id = ? AND user_id = ?`, row.ID, uid)
	}
	w.WriteHeader(http.StatusNoContent)
}
