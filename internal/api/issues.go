package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// issueJSON builds the Issue shape consumed by the frontend split panel.
func (s *Server) issueJSON(r *http.Request, id int64) map[string]any {
	var (
		number, authorID int64
		title, body, state, created, updated string
		assignee sql.NullInt64
		closedAt sql.NullString
	)
	err := s.db.QueryRowContext(r.Context(),
		`SELECT number, title, body, author_id, assignee_id, state, created_at, updated_at, closed_at
		 FROM issues WHERE id = ?`, id).
		Scan(&number, &title, &body, &authorID, &assignee, &state, &created, &updated, &closedAt)
	if err != nil {
		return nil
	}
	var commentCount int
	s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM issue_comments WHERE issue_id = ?`, id).Scan(&commentCount)

	var assigneeJSON any
	if assignee.Valid {
		assigneeJSON = s.userBrief(assignee.Int64)
	}
	return map[string]any{
		"id":       number,
		"number":   number,
		"title":    title,
		"body":     body,
		"author":   s.userBrief(authorID),
		"assignee": assigneeJSON,
		"state":    state,
		"closed":   state == "closed",
		"comments": commentCount,
		"created":  relativeTime(created),
		"updated":  relativeTime(updated),
	}
}

func (s *Server) issueIDByNumber(repoID int64, number int) (int64, bool) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM issues WHERE repo_id = ? AND number = ?`, repoID, number).Scan(&id)
	return id, err == nil
}

// GET /v1/repos/{org}/{name}/issues?state=open|closed|all
func (s *Server) handleListIssues(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	state := r.URL.Query().Get("state")
	q := `SELECT id FROM issues WHERE repo_id = ?`
	args := []any{row.ID}
	if state == "open" || state == "closed" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY number DESC`
	rows, err := s.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list issues")
		return
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			ids = append(ids, id)
		}
	}
	rows.Close()
	out := []map[string]any{}
	for _, id := range ids {
		if ij := s.issueJSON(r, id); ij != nil {
			out = append(out, ij)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/repos/{org}/{name}/issues { title, body }
func (s *Server) handleCreateIssue(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	var in struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}
	var number int
	s.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(number),0)+1 FROM issues WHERE repo_id = ?`, row.ID).Scan(&number)
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO issues (repo_id, number, title, body, author_id) VALUES (?,?,?,?,?)`,
		row.ID, number, in.Title, in.Body, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create issue")
		return
	}
	id, _ := res.LastInsertId()
	target := row.OwnerHandle + "/" + row.Name + "#" + strconv.Itoa(number)
	s.logActivity(r.Context(), u.ID, "issue_opened", row.ID, target, in.Title)
	if s.webhooks != nil {
		s.webhooks.Fire(r.Context(), row.ID, "issue", map[string]any{
			"event": "issue", "action": "opened", "repo": row.OwnerHandle + "/" + row.Name,
			"number": number, "title": in.Title, "author": u.Handle,
		})
	}
	writeJSON(w, http.StatusOK, s.issueJSON(r, id))
}

func (s *Server) handleGetIssue(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.issueIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	writeJSON(w, http.StatusOK, s.issueJSON(r, id))
}

// PATCH /v1/repos/{org}/{name}/issues/{num} { title?, body?, state?, assignee? }
func (s *Server) handleUpdateIssue(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.issueIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	u := userFrom(r)
	var in struct {
		Title    *string `json:"title"`
		Body     *string `json:"body"`
		State    *string `json:"state"`
		Assignee *string `json:"assignee"` // handle, or "" to clear
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Title != nil && *in.Title != "" {
		s.db.ExecContext(r.Context(), `UPDATE issues SET title = ? WHERE id = ?`, *in.Title, id)
	}
	if in.Body != nil {
		s.db.ExecContext(r.Context(), `UPDATE issues SET body = ? WHERE id = ?`, *in.Body, id)
	}
	if in.Assignee != nil {
		if *in.Assignee == "" {
			s.db.ExecContext(r.Context(), `UPDATE issues SET assignee_id = NULL WHERE id = ?`, id)
		} else {
			var aid int64
			if s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, *in.Assignee).Scan(&aid) == nil {
				s.db.ExecContext(r.Context(), `UPDATE issues SET assignee_id = ? WHERE id = ?`, aid, id)
			}
		}
	}
	if in.State != nil && (*in.State == "open" || *in.State == "closed") {
		if *in.State == "closed" {
			s.db.ExecContext(r.Context(), `UPDATE issues SET state='closed', closed_at=datetime('now') WHERE id = ?`, id)
			target := row.OwnerHandle + "/" + row.Name + "#" + strconv.Itoa(num)
			s.logActivity(r.Context(), u.ID, "issue_closed", row.ID, target, "")
			if s.webhooks != nil {
				s.webhooks.Fire(r.Context(), row.ID, "issue", map[string]any{
					"event": "issue", "action": "closed", "repo": row.OwnerHandle + "/" + row.Name, "number": num, "closed_by": u.Handle,
				})
			}
		} else {
			s.db.ExecContext(r.Context(), `UPDATE issues SET state='open', closed_at=NULL WHERE id = ?`, id)
		}
	}
	s.db.ExecContext(r.Context(), `UPDATE issues SET updated_at=datetime('now') WHERE id = ?`, id)
	writeJSON(w, http.StatusOK, s.issueJSON(r, id))
}

func (s *Server) handleListIssueComments(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.issueIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT author_id, body, created_at FROM issue_comments WHERE issue_id = ? ORDER BY id`, id)
	type raw struct {
		aid          int64
		body, created string
	}
	var raws []raw
	if rows != nil {
		for rows.Next() {
			var rw raw
			if rows.Scan(&rw.aid, &rw.body, &rw.created) == nil {
				raws = append(raws, rw)
			}
		}
		rows.Close()
	}
	out := []map[string]any{}
	for _, rw := range raws {
		out = append(out, map[string]any{"author": s.userBrief(rw.aid), "body": rw.body, "when": relativeTime(rw.created)})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/{org}/{name}/checks?limit= — recent check runs across the repo.
// Foundry doesn't run CI itself; these are reported by the supply-chain scanner
// and any external CI via the checks/webhooks API.
func (s *Server) handleRepoChecks(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	limit := 30
	if v, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && v > 0 && v <= 100 {
		limit = v
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT name, status, sha, detail, COALESCE(external_url,''), COALESCE(finished_at, started_at, '')
		 FROM checks WHERE repo_id = ? ORDER BY id DESC LIMIT ?`, row.ID, limit)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, status, sha, detail, url, when string
			if rows.Scan(&name, &status, &sha, &detail, &url, &when) == nil {
				short := sha
				if len(short) > 7 {
					short = short[:7]
				}
				item := map[string]any{
					"name": name, "status": status, "sha": short, "detail": detail, "externalUrl": url,
				}
				if when != "" {
					item["when"] = relativeTime(when)
				} else {
					item["when"] = ""
				}
				out = append(out, item)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/repos/{org}/{name}/issues/{num}/comments { body }
func (s *Server) handleCreateIssueComment(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.issueIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "issue not found")
		return
	}
	u := userFrom(r)
	var in struct {
		Body string `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Body == "" {
		writeError(w, http.StatusBadRequest, "body is required")
		return
	}
	_, err := s.db.ExecContext(r.Context(),
		`INSERT INTO issue_comments (issue_id, author_id, body) VALUES (?,?,?)`, id, u.ID, in.Body)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not add comment")
		return
	}
	s.db.ExecContext(r.Context(), `UPDATE issues SET updated_at=datetime('now') WHERE id = ?`, id)
	writeJSON(w, http.StatusOK, map[string]any{"author": s.userBrief(u.ID), "body": in.Body, "when": "just now"})
}
