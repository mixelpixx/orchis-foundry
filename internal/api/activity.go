package api

import (
	"context"
	"database/sql"
	"net/http"
	"strconv"
)

// logActivity records a feed event. Best-effort; never blocks the caller path.
func (s *Server) logActivity(ctx context.Context, actorID int64, kind string, repoID int64, target, title string) {
	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO activity (actor_id, kind, repo_id, target, title) VALUES (?,?,?,?,?)`,
		actorID, kind, repoID, target, title)
}

// GET /v1/me/activity?limit= — recent events across repos the user can see
// (repos they own or public repos) plus their own actions.
func (s *Server) handleActivity(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT a.id, a.actor_id, a.kind, a.target, a.title, a.created_at
		 FROM activity a
		 LEFT JOIN repos rp ON rp.id = a.repo_id
		 WHERE a.actor_id = ? OR rp.owner_user_id = ? OR rp.visibility = 'public'
		 ORDER BY a.id DESC LIMIT ?`, u.ID, u.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load activity")
		return
	}
	// Collect rows first, then resolve actor briefs. Calling userBrief (a nested
	// query) inside the open rows loop would deadlock the single SQLite conn.
	type rawAct struct {
		id      int64
		actorID sql.NullInt64
		kind, target, title, created string
	}
	var raws []rawAct
	for rows.Next() {
		var a rawAct
		if rows.Scan(&a.id, &a.actorID, &a.kind, &a.target, &a.title, &a.created) == nil {
			raws = append(raws, a)
		}
	}
	rows.Close()

	out := []map[string]any{}
	for _, a := range raws {
		actor := map[string]any{"name": "system", "initials": "·", "color": "var(--fg-3)", "handle": ""}
		if a.actorID.Valid {
			actor = s.userBrief(a.actorID.Int64)
		}
		out = append(out, map[string]any{
			"id": a.id, "actor": actor, "kind": a.kind, "target": a.target, "title": a.title, "when": relativeTime(a.created),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/{org}/{name}/activity?limit= — recent events for a single repo.
// ACL piggybacks on loadRepo (public, or owner for private/internal).
func (s *Server) handleRepoActivity(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT a.id, a.actor_id, a.kind, a.target, a.title, a.created_at
		 FROM activity a
		 WHERE a.repo_id = ?
		 ORDER BY a.id DESC LIMIT ?`, row.ID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load activity")
		return
	}
	// Collect rows first, then resolve actor briefs — calling userBrief (a nested
	// query) inside the open rows loop would deadlock the single SQLite conn.
	type rawAct struct {
		id      int64
		actorID sql.NullInt64
		kind, target, title, created string
	}
	var raws []rawAct
	for rows.Next() {
		var a rawAct
		if rows.Scan(&a.id, &a.actorID, &a.kind, &a.target, &a.title, &a.created) == nil {
			raws = append(raws, a)
		}
	}
	rows.Close()

	out := []map[string]any{}
	for _, a := range raws {
		actor := map[string]any{"name": "system", "initials": "·", "color": "var(--fg-3)", "handle": ""}
		if a.actorID.Valid {
			actor = s.userBrief(a.actorID.Int64)
		}
		out = append(out, map[string]any{
			"id": a.id, "actor": actor, "kind": a.kind, "target": a.target, "title": a.title, "when": relativeTime(a.created),
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/me/inbox — derived "needs your attention" items (no separate table).
func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	items := []map[string]any{}

	// 1. Review requested on open PRs.
	if rows, err := s.db.QueryContext(r.Context(),
		`SELECT p.number, p.title, p.additions, p.deletions, uo.handle, rp.name
		 FROM pulls p
		 JOIN pull_reviewers pr ON pr.pull_id = p.id
		 JOIN repos rp ON rp.id = p.repo_id
		 JOIN users uo ON uo.id = rp.owner_user_id
		 WHERE pr.user_id = ? AND p.state = 'open'
		 ORDER BY p.updated_at DESC LIMIT 10`, u.ID); err == nil {
		for rows.Next() {
			var num, adds, dels int
			var title, owner, name string
			if rows.Scan(&num, &title, &adds, &dels, &owner, &name) == nil {
				repo := owner + "/" + name
				items = append(items, map[string]any{
					"icon": "pr", "kind": "accent", "title": "Review requested",
					"repo": repo, "detail": "#" + strconv.Itoa(num) + " " + title,
					"meta": []map[string]any{{"label": "+" + strconv.Itoa(adds) + " −" + strconv.Itoa(dels), "mono": true}},
					"cta":  "Open review", "route": map[string]any{"view": "pr", "pr": num, "repo": repo},
				})
			}
		}
		rows.Close()
	}

	// 2. Tokens expiring within 14 days.
	if rows, err := s.db.QueryContext(r.Context(),
		`SELECT name, scopes, expires_at FROM personal_access_tokens
		 WHERE user_id = ? AND expires_at IS NOT NULL
		   AND expires_at > datetime('now') AND expires_at <= datetime('now','+14 days')
		 ORDER BY expires_at ASC`, u.ID); err == nil {
		for rows.Next() {
			var name, scopes, exp string
			if rows.Scan(&name, &scopes, &exp) == nil {
				items = append(items, map[string]any{
					"icon": "key", "kind": "warn", "title": "Token expires soon",
					"repo": name, "detail": "Rotate before " + exp[:10] + ", or automation using it will stall.",
					"meta": []map[string]any{{"label": "scopes: " + scopes, "mono": true}},
					"cta":  "Rotate token", "route": map[string]any{"view": "settings", "tab": "tokens"},
				})
			}
		}
		rows.Close()
	}

	// 3. Your open PRs flagged by the supply-chain scanner.
	if rows, err := s.db.QueryContext(r.Context(),
		`SELECT p.number, p.title, c.detail, uo.handle, rp.name
		 FROM pulls p
		 JOIN repos rp ON rp.id = p.repo_id
		 JOIN users uo ON uo.id = rp.owner_user_id
		 JOIN checks c ON c.repo_id = p.repo_id AND c.sha = p.head_sha AND c.name = 'orchis-scan' AND c.status = 'fail'
		 WHERE p.author_id = ? AND p.state = 'open'
		 ORDER BY p.updated_at DESC LIMIT 10`, u.ID); err == nil {
		for rows.Next() {
			var num int
			var title, detail, owner, name string
			if rows.Scan(&num, &title, &detail, &owner, &name) == nil {
				repo := owner + "/" + name
				items = append(items, map[string]any{
					"icon": "alert", "kind": "warn", "title": "Supply-chain scan flagged your PR",
					"repo": repo, "detail": "#" + strconv.Itoa(num) + " " + title,
					"meta": []map[string]any{},
					"cta":  "Open PR", "route": map[string]any{"view": "pr", "pr": num, "repo": repo},
				})
			}
		}
		rows.Close()
	}

	writeJSON(w, http.StatusOK, items)
}
