package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/orchis-ai/foundry/internal/auth"
)

// POST /v1/repos/:org/:name/pulls/:num/comments
// { body, path?, line?, side?, in_reply_to? }
func (s *Server) handleCreateComment(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var in struct {
		Body      string `json:"body"`
		Path      string `json:"path"`
		Line      *int   `json:"line"`
		Side      string `json:"side"`
		InReplyTo *int64 `json:"in_reply_to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Body) == "" {
		writeError(w, http.StatusBadRequest, "comment body required")
		return
	}
	var side any
	if in.Side == "left" || in.Side == "right" {
		side = in.Side
	}
	var pathVal any
	if in.Path != "" {
		pathVal = in.Path
	}
	var lineVal any
	if in.Line != nil {
		lineVal = *in.Line
	}
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO pull_comments (pull_id, author_id, body, path, line, side, in_reply_to)
		 VALUES (?,?,?,?,?,?,?)`,
		pullID, u.ID, in.Body, pathVal, lineVal, side, in.InReplyTo)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not post comment")
		return
	}
	s.db.ExecContext(r.Context(), `UPDATE pulls SET updated_at = datetime('now') WHERE id = ?`, pullID)
	id, _ := res.LastInsertId()
	s.logActivity(r.Context(), u.ID, "pr_commented", row.ID, row.OwnerHandle+"/"+row.Name+"#"+strconv.Itoa(num), in.Body)
	s.publish(pullTopic(row.OwnerHandle, row.Name, num), "pull.commented", map[string]any{
		"number": num, "repo": row.OwnerHandle + "/" + row.Name, "author": u.Handle,
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"id": id, "author": s.userBrief(u.ID), "body": in.Body,
		"path": in.Path, "line": in.Line, "side": in.Side, "when": "just now",
	})
}

// POST /v1/repos/:org/:name/pulls/:num/reviews
// { verdict: comment|approve|changes, body, comments: [{body,path,line,side}] }
func (s *Server) handleSubmitReview(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var in struct {
		Verdict  string `json:"verdict"`
		Body     string `json:"body"`
		Comments []struct {
			Body string `json:"body"`
			Path string `json:"path"`
			Line *int   `json:"line"`
			Side string `json:"side"`
		} `json:"comments"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Verdict != "comment" && in.Verdict != "approve" && in.Verdict != "changes" {
		writeError(w, http.StatusBadRequest, "verdict must be comment|approve|changes")
		return
	}
	if _, err := s.db.ExecContext(r.Context(),
		`INSERT INTO pull_reviews (pull_id, author_id, verdict, body) VALUES (?,?,?,?)`,
		pullID, u.ID, in.Verdict, in.Body); err != nil {
		writeError(w, http.StatusInternalServerError, "could not submit review")
		return
	}
	for _, c := range in.Comments {
		if strings.TrimSpace(c.Body) == "" {
			continue
		}
		var side, pathVal, lineVal any
		if c.Side == "left" || c.Side == "right" {
			side = c.Side
		}
		if c.Path != "" {
			pathVal = c.Path
		}
		if c.Line != nil {
			lineVal = *c.Line
		}
		s.db.ExecContext(r.Context(),
			`INSERT INTO pull_comments (pull_id, author_id, body, path, line, side) VALUES (?,?,?,?,?,?)`,
			pullID, u.ID, c.Body, pathVal, lineVal, side)
	}
	s.db.ExecContext(r.Context(), `UPDATE pulls SET updated_at = datetime('now') WHERE id = ?`, pullID)
	s.logActivity(r.Context(), u.ID, "pr_reviewed", row.ID, row.OwnerHandle+"/"+row.Name+"#"+strconv.Itoa(num), in.Verdict)
	s.publish(pullTopic(row.OwnerHandle, row.Name, num), "pull.reviewed", map[string]any{
		"number": num, "repo": row.OwnerHandle + "/" + row.Name, "verdict": in.Verdict, "author": u.Handle,
	})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "verdict": in.Verdict})
}

// POST /v1/repos/:org/:name/pulls/:num/merge
// { strategy: merge|squash|rebase, commit_title?, commit_body? }
func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	// Merge requires write access (owner or write/admin collaborator).
	if !s.requireWrite(w, row) {
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var in struct {
		Strategy    string `json:"strategy"`
		CommitTitle string `json:"commit_title"`
		CommitBody  string `json:"commit_body"`
	}
	_ = json.NewDecoder(r.Body).Decode(&in)
	if in.Strategy == "" {
		in.Strategy = "merge"
	}

	var head, base, state, prTitle string
	s.db.QueryRowContext(r.Context(), `SELECT title, head_branch, base_branch, state FROM pulls WHERE id = ?`, pullID).
		Scan(&prTitle, &head, &base, &state)
	if state != "open" {
		writeError(w, http.StatusConflict, "pull request is not open")
		return
	}

	title := in.CommitTitle
	if title == "" {
		title = fmt.Sprintf("Merge branch '%s' into '%s' (#%d)", head, base, num)
	}
	msg := title
	if in.CommitBody != "" {
		msg += "\n\n" + in.CommitBody
	}

	if err := s.mergeBranches(row.ID, base, head, in.Strategy, msg); err != nil {
		writeError(w, http.StatusConflict, "merge failed: "+err.Error())
		return
	}

	s.db.ExecContext(r.Context(), `UPDATE pulls SET state='merged', updated_at=datetime('now') WHERE id = ?`, pullID)
	s.db.ExecContext(r.Context(), `UPDATE repos SET pushed_at=datetime('now') WHERE id = ?`, row.ID)
	s.logActivity(r.Context(), u.ID, "pr_merged", row.ID, row.OwnerHandle+"/"+row.Name+"#"+strconv.Itoa(num), prTitle)
	if s.webhooks != nil {
		s.webhooks.Fire(r.Context(), row.ID, "pull_request", map[string]any{
			"event": "pull_request", "action": "merged", "repo": row.OwnerHandle + "/" + row.Name,
			"number": num, "title": prTitle, "strategy": in.Strategy, "merged_by": u.Handle,
		})
	}
	s.publish(pullTopic(row.OwnerHandle, row.Name, num), "pull.merged", map[string]any{
		"number": num, "repo": row.OwnerHandle + "/" + row.Name, "merged_by": u.Handle,
	})
	writeJSON(w, http.StatusOK, map[string]any{"merged": true, "strategy": in.Strategy})
}

// canManageReviewers reports whether u may add/remove reviewers on a pull:
// a write-access collaborator (owner/admin/write) or the PR author.
func (s *Server) canManageReviewers(r *http.Request, row *repoRow, pullID int64, u *auth.User) bool {
	if u == nil {
		return false
	}
	if canWrite(row) {
		return true
	}
	var authorID int64
	s.db.QueryRowContext(r.Context(), `SELECT author_id FROM pulls WHERE id = ?`, pullID).Scan(&authorID)
	return authorID == u.ID
}

// POST /v1/repos/:org/:name/pulls/:num/request-review { reviewer: "<handle>" }
func (s *Server) handleRequestReview(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	u := userFrom(r)
	if !s.canManageReviewers(r, row, pullID, u) {
		writeError(w, http.StatusForbidden, "only the repo owner or PR author can request reviews")
		return
	}
	var in struct {
		Reviewer string `json:"reviewer"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Reviewer) == "" {
		writeError(w, http.StatusBadRequest, "reviewer handle required")
		return
	}
	var reviewerID int64
	if err := s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, strings.TrimSpace(in.Reviewer)).Scan(&reviewerID); err != nil {
		writeError(w, http.StatusNotFound, "no such user: "+in.Reviewer)
		return
	}
	if _, err := s.db.ExecContext(r.Context(),
		`INSERT INTO pull_reviewers (pull_id, user_id) VALUES (?,?) ON CONFLICT DO NOTHING`, pullID, reviewerID); err != nil {
		writeError(w, http.StatusInternalServerError, "could not request review")
		return
	}
	s.db.ExecContext(r.Context(), `UPDATE pulls SET updated_at = datetime('now') WHERE id = ?`, pullID)

	// Live: the reviewer's dashboard inbox + the PR's reviewer chips.
	repo := row.OwnerHandle + "/" + row.Name
	var title string
	var adds, dels int
	s.db.QueryRowContext(r.Context(), `SELECT title, additions, deletions FROM pulls WHERE id = ?`, pullID).Scan(&title, &adds, &dels)
	s.publish("inbox:"+strconv.FormatInt(reviewerID, 10), "inbox.new", map[string]any{
		"icon": "pr", "kind": "accent", "title": "Review requested",
		"repo": repo, "detail": "#" + strconv.Itoa(num) + " " + title,
		"meta":  []map[string]any{{"label": "+" + strconv.Itoa(adds) + " −" + strconv.Itoa(dels), "mono": true}},
		"cta":   "Open review", "route": map[string]any{"view": "pr", "pr": num, "repo": repo},
	})
	s.publish(pullTopic(row.OwnerHandle, row.Name, num), "pull.reviewers-changed", map[string]any{"number": num, "repo": repo})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "reviewer": in.Reviewer})
}

// DELETE /v1/repos/:org/:name/pulls/:num/request-review/{handle}
func (s *Server) handleRemoveReviewer(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	u := userFrom(r)
	if !s.canManageReviewers(r, row, pullID, u) {
		writeError(w, http.StatusForbidden, "only the repo owner or PR author can manage reviewers")
		return
	}
	handle := chi.URLParam(r, "handle")
	var reviewerID int64
	if s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, handle).Scan(&reviewerID) == nil {
		s.db.ExecContext(r.Context(), `DELETE FROM pull_reviewers WHERE pull_id = ? AND user_id = ?`, pullID, reviewerID)
		s.publish(pullTopic(row.OwnerHandle, row.Name, num), "pull.reviewers-changed", map[string]any{"number": num, "repo": row.OwnerHandle + "/" + row.Name})
	}
	w.WriteHeader(http.StatusNoContent)
}

// mergeBranches performs the merge in a throwaway worktree of the bare repo.
// The worktree shares the object store + refs, so updating <base> there
// updates it in the bare repo. Handles merge / squash / rebase.
func (s *Server) mergeBranches(repoID int64, base, head, strategy, msg string) error {
	dir := s.git.Path(repoID)
	wt, err := os.MkdirTemp("", "orchis-merge-")
	if err != nil {
		return err
	}
	defer func() {
		exec.Command("git", "-C", dir, "worktree", "remove", "--force", wt).Run()
		os.RemoveAll(wt)
	}()

	run := func(args ...string) (string, error) {
		c := exec.Command("git", args...)
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=orchis", "GIT_AUTHOR_EMAIL=merge@orchis.ai",
			"GIT_COMMITTER_NAME=orchis", "GIT_COMMITTER_EMAIL=merge@orchis.ai")
		out, err := c.CombinedOutput()
		return string(out), err
	}

	if out, err := run("-C", dir, "worktree", "add", "--force", wt, base); err != nil {
		return fmt.Errorf("worktree: %s", strings.TrimSpace(out))
	}

	switch strategy {
	case "squash":
		if out, err := run("-C", wt, "merge", "--squash", head); err != nil {
			run("-C", wt, "merge", "--abort")
			return fmt.Errorf("conflicts: %s", lastLine(out))
		}
		if out, err := run("-C", wt, "commit", "-m", msg); err != nil {
			return fmt.Errorf("commit: %s", lastLine(out))
		}
	case "rebase":
		// Rebase head onto base, then fast-forward base.
		if out, err := run("-C", wt, "rebase", head); err != nil {
			run("-C", wt, "rebase", "--abort")
			return fmt.Errorf("rebase conflicts: %s", lastLine(out))
		}
	default: // merge (no-ff)
		if out, err := run("-C", wt, "merge", "--no-ff", "-m", msg, head); err != nil {
			run("-C", wt, "merge", "--abort")
			return fmt.Errorf("conflicts: %s", lastLine(out))
		}
	}
	return nil
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}
