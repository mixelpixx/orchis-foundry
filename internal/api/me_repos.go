package api

import (
	"database/sql"
	"net/http"
)

// PUT /v1/me/pinned/{org}/{name}
func (s *Server) handlePinRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	var ord int
	s.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(ord),0)+1 FROM user_repo_pins WHERE user_id = ?`, u.ID).Scan(&ord)
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO user_repo_pins (user_id, repo_id, ord) VALUES (?,?,?)
		 ON CONFLICT(user_id, repo_id) DO NOTHING`, u.ID, row.ID, ord)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not pin repo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/me/pinned/{org}/{name}
func (s *Server) handleUnpinRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	s.db.ExecContext(r.Context(), `DELETE FROM user_repo_pins WHERE user_id = ? AND repo_id = ?`, u.ID, row.ID)
	w.WriteHeader(http.StatusNoContent)
}

// PUT /v1/me/stars/{org}/{name}
func (s *Server) handleStarRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO user_repo_stars (user_id, repo_id) VALUES (?,?)
		 ON CONFLICT(user_id, repo_id) DO NOTHING`, u.ID, row.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not star repo")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DELETE /v1/me/stars/{org}/{name}
func (s *Server) handleUnstarRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	s.db.ExecContext(r.Context(), `DELETE FROM user_repo_stars WHERE user_id = ? AND repo_id = ?`, u.ID, row.ID)
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/me/pinned — repos the current user has pinned, in pin order.
func (s *Server) handleListPinned(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT rp.id, rp.owner_user_id, uo.handle, rp.name, rp.description, rp.visibility,
		        rp.default_branch, rp.language, rp.created_at, rp.pushed_at,
		        (SELECT COUNT(*) FROM user_repo_stars s WHERE s.repo_id = rp.id) AS stars,
		        EXISTS(SELECT 1 FROM user_repo_stars s WHERE s.repo_id = rp.id AND s.user_id = ?) AS starred
		 FROM user_repo_pins p
		 JOIN repos rp ON rp.id = p.repo_id
		 JOIN users uo ON uo.id = rp.owner_user_id
		 WHERE p.user_id = ?
		 ORDER BY p.ord ASC`, u.ID, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list pinned repos")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		row := &repoRow{Pinned: true}
		var pushedAt sql.NullString
		var starred int
		if rows.Scan(&row.ID, &row.OwnerUserID, &row.OwnerHandle, &row.Name, &row.Description,
			&row.Visibility, &row.DefaultBranch, &row.Language, &row.CreatedAt, &pushedAt, &row.Stars, &starred) != nil {
			continue
		}
		row.PushedAt = pushedAt
		row.Starred = starred == 1
		out = append(out, repoJSON(row))
	}
	writeJSON(w, http.StatusOK, out)
}
