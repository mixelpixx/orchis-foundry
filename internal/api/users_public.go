package api

import (
	"database/sql"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// GET /v1/users/{handle} — a public profile: identity + the user's public
// repos. Read-only; no auth required (public info only).
func (s *Server) handleUserProfile(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	var (
		id            int64
		name, avatar  string
		bio, created  string
	)
	err := s.db.QueryRowContext(r.Context(),
		`SELECT id, name, avatar_url, bio, created_at FROM users WHERE handle = ?`, handle).
		Scan(&id, &name, &avatar, &bio, &created)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}

	// Public repos owned by this user, newest activity first.
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT rp.name, rp.description, rp.visibility, rp.default_branch, rp.language,
		        rp.created_at, rp.pushed_at,
		        (SELECT COUNT(*) FROM user_repo_stars st WHERE st.repo_id = rp.id) AS stars
		 FROM repos rp
		 WHERE rp.owner_user_id = ? AND rp.visibility = 'public'
		 ORDER BY COALESCE(rp.pushed_at, rp.created_at) DESC`, id)
	repos := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			row := &repoRow{OwnerHandle: handle}
			var pushedAt sql.NullString
			if rows.Scan(&row.Name, &row.Description, &row.Visibility, &row.DefaultBranch,
				&row.Language, &row.CreatedAt, &pushedAt, &row.Stars) == nil {
				row.PushedAt = pushedAt
				repos = append(repos, repoJSON(row))
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"handle":      handle,
		"name":        name,
		"bio":         bio,
		"avatarUrl":   avatar,
		"initials":    initials(name, handle),
		"color":       colorFor(handle),
		"memberSince": relativeTime(created),
		"repos":       repos,
	})
}
