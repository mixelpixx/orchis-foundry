package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

// Change proposals are the enterprise human-in-the-loop gate: an LLM (or any
// client) proposes a code change, but it is never applied to a branch or turned
// into a PR until a human with write access clicks Accept.

type proposalChange struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Delete  bool   `json:"delete"`
}

func (s *Server) proposalJSON(id, repoID, authorID int64, title, summary, baseSHA, targetBranch, patch, changes, status, appliedSHA string, pullNumber sql.NullInt64, created string) map[string]any {
	out := map[string]any{
		"id": id, "title": title, "summary": summary, "baseSha": baseSHA,
		"targetBranch": targetBranch, "patch": patch, "status": status,
		"appliedSha": appliedSHA, "author": s.userBrief(authorID),
		"when": relativeTime(created),
	}
	if changes != "" {
		var parsed []proposalChange
		if json.Unmarshal([]byte(changes), &parsed) == nil {
			out["changes"] = parsed
		}
	}
	if pullNumber.Valid {
		out["pullNumber"] = pullNumber.Int64
	}
	return out
}

// POST /v1/repos/{org}/{name}/proposals
// { title, summary?, patch? | changes?, baseSha?, targetBranch? }
func (s *Server) handleCreateProposal(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireWrite(w, row) {
		return
	}
	u := userFrom(r)
	var in struct {
		Title        string           `json:"title"`
		Summary      string           `json:"summary"`
		Patch        string           `json:"patch"`
		Changes      []proposalChange `json:"changes"`
		BaseSHA      string           `json:"baseSha"`
		TargetBranch string           `json:"targetBranch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if strings.TrimSpace(in.Title) == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	if strings.TrimSpace(in.Patch) == "" && len(in.Changes) == 0 {
		writeError(w, http.StatusBadRequest, "a patch or changes are required")
		return
	}
	changesJSON := ""
	if len(in.Changes) > 0 {
		b, _ := json.Marshal(in.Changes)
		changesJSON = string(b)
	}
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO change_proposals (repo_id, author_id, title, summary, base_sha, target_branch, patch, changes)
		 VALUES (?,?,?,?,?,?,?,?)`,
		row.ID, u.ID, in.Title, in.Summary, in.BaseSHA, in.TargetBranch, in.Patch, changesJSON)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save proposal")
		return
	}
	id, _ := res.LastInsertId()
	s.publish(repoTopic(row.OwnerHandle, row.Name), "proposal.created", map[string]any{
		"repo": row.OwnerHandle + "/" + row.Name, "id": id, "title": in.Title,
	})
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "status": "pending"})
}

// GET /v1/repos/{org}/{name}/proposals?status=
func (s *Server) handleListProposals(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	q := `SELECT id, repo_id, author_id, title, summary, base_sha, target_branch, patch, changes, status, applied_sha, pull_number, created_at
	      FROM change_proposals WHERE repo_id = ?`
	args := []any{row.ID}
	if st := r.URL.Query().Get("status"); st == "pending" || st == "accepted" || st == "rejected" {
		q += " AND status = ?"
		args = append(args, st)
	}
	q += " ORDER BY id DESC LIMIT 100"
	rows, err := s.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list proposals")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id, repoID, authorID int64
		var title, summary, baseSHA, targetBranch, patch, changes, status, appliedSHA, created string
		var pullNumber sql.NullInt64
		if rows.Scan(&id, &repoID, &authorID, &title, &summary, &baseSHA, &targetBranch, &patch, &changes, &status, &appliedSHA, &pullNumber, &created) == nil {
			out = append(out, s.proposalJSON(id, repoID, authorID, title, summary, baseSHA, targetBranch, patch, changes, status, appliedSHA, pullNumber, created))
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// loadProposal fetches one proposal scoped to the repo.
func (s *Server) loadProposal(r *http.Request, repoID int64) (map[string]any, string, []proposalChange, string, string, bool) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var pid, aid int64
	var title, summary, baseSHA, targetBranch, patch, changes, status, appliedSHA, created string
	var pullNumber sql.NullInt64
	err := s.db.QueryRowContext(r.Context(),
		`SELECT id, author_id, title, summary, base_sha, target_branch, patch, changes, status, applied_sha, pull_number, created_at
		 FROM change_proposals WHERE id = ? AND repo_id = ?`, id, repoID).
		Scan(&pid, &aid, &title, &summary, &baseSHA, &targetBranch, &patch, &changes, &status, &appliedSHA, &pullNumber, &created)
	if err != nil {
		return nil, "", nil, "", "", false
	}
	var parsed []proposalChange
	if changes != "" {
		_ = json.Unmarshal([]byte(changes), &parsed)
	}
	return s.proposalJSON(pid, repoID, aid, title, summary, baseSHA, targetBranch, patch, changes, status, appliedSHA, pullNumber, created), status, parsed, patch, targetBranch, true
}

// GET /v1/repos/{org}/{name}/proposals/{id}
func (s *Server) handleGetProposal(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	obj, _, _, _, _, found := s.loadProposal(r, row.ID)
	if !found {
		writeError(w, http.StatusNotFound, "proposal not found")
		return
	}
	writeJSON(w, http.StatusOK, obj)
}

// POST /v1/repos/{org}/{name}/proposals/{id}/accept
// Applies the proposal onto a feature branch (never the default branch), opens a
// PR, and runs the shared post-push fan-out (so the auto-audit scan runs too).
func (s *Server) handleAcceptProposal(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireWrite(w, row) {
		return
	}
	u := userFrom(r)
	obj, status, changes, patch, targetBranch, found := s.loadProposal(r, row.ID)
	if !found {
		writeError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if status != "pending" {
		writeError(w, http.StatusConflict, "proposal already "+status)
		return
	}
	id := int64(obj["id"].(int64))
	baseSHA, _ := obj["baseSha"].(string)
	title, _ := obj["title"].(string)
	summary, _ := obj["summary"].(string)

	// Never apply straight onto the default branch — force a feature branch so
	// the change always lands in review.
	branch := targetBranch
	if branch == "" || branch == row.DefaultBranch {
		branch = "ai/proposal-" + strconv.FormatInt(id, 10)
	}

	name := u.Name
	if name == "" {
		name = u.Handle
	}
	email := u.Email
	if email == "" {
		email = u.Handle + "@users.noreply.orchis"
	}
	msg := title
	if summary != "" {
		msg += "\n\n" + summary
	}

	var newSHA, oldSHA string
	var err error
	if len(changes) > 0 {
		fc := make([]gitstore.FileChange, 0, len(changes))
		for _, c := range changes {
			fc = append(fc, gitstore.FileChange{Path: c.Path, Content: []byte(c.Content), Delete: c.Delete})
		}
		newSHA, oldSHA, err = s.git.CommitFiles(row.ID, branch, baseSHA, msg, name, email, fc)
	} else {
		newSHA, oldSHA, err = s.git.ApplyPatchCommit(row.ID, branch, baseSHA, patch, msg, name, email)
	}
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Shared fan-out (PR refresh, webhooks, supply-chain auto-audit).
	s.onRefUpdated(r.Context(), row.ID, []RefUpdate{
		{Ref: "refs/heads/" + branch, Old: oldSHA, New: newSHA},
	})

	// Open a PR so the change goes through normal review.
	body := summary
	if body == "" {
		body = "Applied from change proposal #" + strconv.FormatInt(id, 10) + "."
	}
	_, number, perr := s.createPull(r.Context(), row, u.ID, u.Handle, title, body, branch, row.DefaultBranch)

	var pullNum any
	if perr == nil {
		pullNum = number
		s.db.ExecContext(r.Context(),
			`UPDATE change_proposals SET status='accepted', applied_sha=?, pull_number=?, resolved_at=datetime('now') WHERE id=?`,
			newSHA, number, id)
	} else {
		// The commit landed even if PR creation hiccupped; still mark accepted.
		s.db.ExecContext(r.Context(),
			`UPDATE change_proposals SET status='accepted', applied_sha=?, resolved_at=datetime('now') WHERE id=?`,
			newSHA, id)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status": "accepted", "branch": branch, "sha": newSHA, "pullNumber": pullNum,
	})
}

// POST /v1/repos/{org}/{name}/proposals/{id}/reject
func (s *Server) handleRejectProposal(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireWrite(w, row) {
		return
	}
	_, status, _, _, _, found := s.loadProposal(r, row.ID)
	if !found {
		writeError(w, http.StatusNotFound, "proposal not found")
		return
	}
	if status != "pending" {
		writeError(w, http.StatusConflict, "proposal already "+status)
		return
	}
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s.db.ExecContext(r.Context(),
		`UPDATE change_proposals SET status='rejected', resolved_at=datetime('now') WHERE id=? AND repo_id=?`, id, row.ID)
	writeJSON(w, http.StatusOK, map[string]any{"status": "rejected"})
}

// POST /v1/repos/{org}/{name}/commits/draft-message { diff? | changes? }
// Generates a Conventional-Commits message from a diff or a set of file edits,
// using the requesting user's configured model.
func (s *Server) handleDraftCommitMessage(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	st, usable := s.modelSettings(r.Context(), u.ID)
	if !usable {
		writeError(w, http.StatusConflict, "no model configured — set one up in Developer settings → Scanner")
		return
	}
	var in struct {
		Diff    string           `json:"diff"`
		Changes []proposalChange `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	content := in.Diff
	if content == "" && len(in.Changes) > 0 {
		var b strings.Builder
		for _, c := range in.Changes {
			if c.Delete {
				b.WriteString("deleted: " + c.Path + "\n")
				continue
			}
			b.WriteString("--- file: " + c.Path + " ---\n" + c.Content + "\n")
		}
		content = b.String()
	}
	if strings.TrimSpace(content) == "" {
		writeError(w, http.StatusBadRequest, "diff or changes required")
		return
	}
	if budget := st.InputBudgetBytes(); len(content) > budget {
		content = content[:budget] + "\n\n[truncated]\n"
	}
	system := "You write a single Conventional Commits message for the given change. Format: `type(scope): summary` on the first line (type ∈ feat|fix|docs|refactor|test|chore|perf|build|ci; scope optional), then an optional blank line and short body. Output ONLY the commit message, no code fences, no preamble."
	out, code, msg := runChat(r.Context(), st, system, "Change:\n"+content)
	if code != 0 {
		writeError(w, code, msg)
		return
	}
	_ = row
	writeJSON(w, http.StatusOK, map[string]string{"message": strings.TrimSpace(out)})
}
