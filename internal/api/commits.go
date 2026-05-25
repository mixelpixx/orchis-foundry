package api

import (
	"encoding/json"
	"net/http"
	"strings"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

// GET /v1/repos/{org}/{name}/blame?ref=&path= — per-line authorship for a file.
func (s *Server) handleRepoBlame(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	lines, err := s.git.Blame(row.ID, r.URL.Query().Get("ref"), path)
	if err != nil {
		writeError(w, http.StatusNotFound, "could not blame file")
		return
	}
	out := make([]map[string]any, 0, len(lines))
	for _, l := range lines {
		out = append(out, map[string]any{
			"sha": l.SHA, "short": l.Short, "author": l.Author,
			"when": relativeTime(l.Date), "summary": l.Summary,
			"line": l.Line, "text": l.Text,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/{org}/{name}/outline?ref=&path= — structural symbols for a file
// (AST-aware chunking affordance for LLMs and the UI).
func (s *Server) handleRepoOutline(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, http.StatusBadRequest, "path required")
		return
	}
	syms, err := s.git.Outline(row.ID, r.URL.Query().Get("ref"), path)
	if err != nil {
		writeError(w, http.StatusNotFound, "could not read file")
		return
	}
	writeJSON(w, http.StatusOK, syms)
}

// POST /v1/repos/{org}/{name}/commits — commit file edits straight from the
// browser (no local checkout). Requires write access. Commits onto `branch`,
// creating it from baseSha/HEAD when it doesn't exist, then runs the shared
// post-push fan-out (PR refresh, webhooks, auto-audit scan).
func (s *Server) handleBrowserCommit(w http.ResponseWriter, r *http.Request) {
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
		Branch  string `json:"branch"`
		BaseSHA string `json:"baseSha"`
		Message string `json:"message"`
		Changes []struct {
			Path    string `json:"path"`
			Content string `json:"content"`
			Delete  bool   `json:"delete"`
		} `json:"changes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if strings.TrimSpace(in.Branch) == "" {
		writeError(w, http.StatusBadRequest, "branch required")
		return
	}
	if strings.TrimSpace(in.Message) == "" {
		writeError(w, http.StatusBadRequest, "commit message required")
		return
	}
	if len(in.Changes) == 0 {
		writeError(w, http.StatusBadRequest, "no changes")
		return
	}
	changes := make([]gitstore.FileChange, 0, len(in.Changes))
	for _, c := range in.Changes {
		changes = append(changes, gitstore.FileChange{
			Path: c.Path, Content: []byte(c.Content), Delete: c.Delete,
		})
	}

	name := u.Name
	if name == "" {
		name = u.Handle
	}
	email := u.Email
	if email == "" {
		email = u.Handle + "@users.noreply.orchis"
	}

	newSHA, oldSHA, err := s.git.CommitFiles(row.ID, in.Branch, in.BaseSHA, in.Message, name, email, changes)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Shared fan-out: refresh PRs, fire webhooks, enqueue the supply-chain scan.
	s.onRefUpdated(r.Context(), row.ID, []RefUpdate{
		{Ref: "refs/heads/" + in.Branch, Old: oldSHA, New: newSHA},
	})
	s.logActivity(r.Context(), u.ID, "commit", row.ID,
		row.OwnerHandle+"/"+row.Name, strings.SplitN(in.Message, "\n", 2)[0])

	writeJSON(w, http.StatusOK, map[string]any{
		"sha": newSHA, "branch": in.Branch, "created": oldSHA == "",
	})
}
