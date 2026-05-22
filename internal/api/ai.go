package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	gitstore "github.com/orchis-ai/foundry/internal/git"
	"github.com/orchis-ai/foundry/internal/scan"
)

const aiMaxDiffBytes = 120 * 1024

// renderDiff turns parsed FileDiffs back into a unified-diff string for prompts.
func renderDiff(diffs []gitstore.FileDiff) string {
	var b strings.Builder
	for _, fd := range diffs {
		b.WriteString("diff --git a/" + fd.Path + " b/" + fd.Path + "\n")
		for _, h := range fd.Hunks {
			b.WriteString(h.Header + "\n")
			for _, ln := range h.Lines {
				p := " "
				if ln.Type == "add" {
					p = "+"
				} else if ln.Type == "del" {
					p = "-"
				}
				b.WriteString(p + ln.Text + "\n")
			}
		}
	}
	out := b.String()
	if len(out) > aiMaxDiffBytes {
		out = out[:aiMaxDiffBytes] + "\n\n[diff truncated]\n"
	}
	return out
}

// aiPrep resolves the repo + PR, loads the requesting user's model, and builds
// the PR diff. Writes the appropriate error and returns ok=false on failure.
func (s *Server) aiPrep(w http.ResponseWriter, r *http.Request) (st scan.Settings, title, body, diff string, ok bool) {
	row, found := s.loadRepo(r)
	if !found {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	settings, usable := s.modelSettings(r.Context(), u.ID)
	if !usable {
		writeError(w, http.StatusConflict, "no model configured — set one up in Developer settings → Scanner")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	pullID, exists := s.pullIDByNumber(row.ID, num)
	if !exists {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var baseSHA, headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT title, body, base_sha, head_sha FROM pulls WHERE id = ?`, pullID).
		Scan(&title, &body, &baseSHA, &headSHA)
	diffs, _ := s.git.Diff(row.ID, baseSHA, headSHA)
	return settings, title, body, renderDiff(diffs), true
}

func runChat(ctx context.Context, st scan.Settings, system, user string) (string, int, string) {
	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	out, err := scan.Chat(cctx, st, system, user)
	if err != nil {
		return "", http.StatusBadGateway, "model call failed: " + err.Error()
	}
	return strings.TrimSpace(out), 0, ""
}

// POST .../pulls/:num/summarize → { summary }
func (s *Server) handleSummarizePR(w http.ResponseWriter, r *http.Request) {
	st, title, body, diff, ok := s.aiPrep(w, r)
	if !ok {
		return
	}
	system := "You are a senior engineer. Summarize a pull request for a reviewer in concise markdown: what changed, why it matters, and anything risky to look at. Be specific and brief. Do not invent details not in the diff."
	user := "PR title: " + title + "\n\nPR description:\n" + body + "\n\nUnified diff:\n" + diff
	out, code, msg := runChat(r.Context(), st, system, user)
	if code != 0 {
		writeError(w, code, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"summary": out})
}

// POST .../pulls/:num/explain?file= → { explanation }
func (s *Server) handleExplainPR(w http.ResponseWriter, r *http.Request) {
	st, title, _, diff, ok := s.aiPrep(w, r)
	if !ok {
		return
	}
	file := r.URL.Query().Get("file")
	scopeNote := "the whole diff"
	if file != "" {
		scopeNote = "the file " + file
	}
	system := "You are a patient code reviewer. Explain " + scopeNote + " in this pull request, step by step, in plain markdown so a teammate can understand the change quickly. Reference concrete lines/identifiers. Don't speculate beyond the diff."
	user := "PR title: " + title + "\n\nUnified diff:\n" + diff
	out, code, msg := runChat(r.Context(), st, system, user)
	if code != 0 {
		writeError(w, code, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"explanation": out})
}

// POST .../pulls/:num/draft-description → { description }
func (s *Server) handleDraftDescription(w http.ResponseWriter, r *http.Request) {
	st, title, _, diff, ok := s.aiPrep(w, r)
	if !ok {
		return
	}
	system := "You write clear, conventional pull-request descriptions in markdown. Given the title and diff, produce a description with a short summary, a bulleted list of notable changes, and a 'Testing' note if implied. Keep it tight. Output only the description body, no preamble."
	user := "PR title: " + title + "\n\nUnified diff:\n" + diff
	out, code, msg := runChat(r.Context(), st, system, user)
	if code != 0 {
		writeError(w, code, msg)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"description": out})
}

// PATCH /v1/repos/:org/:name/pulls/:num  { body }  → apply a description.
func (s *Server) handleUpdatePull(w http.ResponseWriter, r *http.Request) {
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
	// The PR author or a write-access collaborator may edit.
	var authorID int64
	s.db.QueryRowContext(r.Context(), `SELECT author_id FROM pulls WHERE id = ?`, pullID).Scan(&authorID)
	if authorID != u.ID && !canWrite(row) {
		writeError(w, http.StatusForbidden, "only the PR author or a write collaborator can edit")
		return
	}
	var in struct {
		Body  *string `json:"body"`
		Title *string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Body != nil {
		s.db.ExecContext(r.Context(), `UPDATE pulls SET body = ?, updated_at = datetime('now') WHERE id = ?`, *in.Body, pullID)
	}
	if in.Title != nil && *in.Title != "" {
		s.db.ExecContext(r.Context(), `UPDATE pulls SET title = ?, updated_at = datetime('now') WHERE id = ?`, *in.Title, pullID)
	}
	writeJSON(w, http.StatusOK, s.pullSummary(r, pullID))
}
