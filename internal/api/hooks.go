package api

import (
	"context"
	"encoding/json"
	"net/http"
)

// RefUpdate is one ref movement (branch push, browser commit, or accepted AI
// proposal). Old is "" for a freshly created ref.
type RefUpdate struct {
	Ref string `json:"ref"`
	Old string `json:"old"`
	New string `json:"new"`
}

// handlePushHook is called by each repo's post-receive hook after a push.
// Guarded by a shared secret (the config session key) since it's reachable
// via the same loopback the proxy uses.
func (s *Server) handlePushHook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Hook-Secret") != s.cfg.SessionKey || s.cfg.SessionKey == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var in struct {
		RepoID int64       `json:"repo_id"`
		Refs   []RefUpdate `json:"refs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.RepoID == 0 {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}
	s.onRefUpdated(r.Context(), in.RepoID, in.Refs)
	w.WriteHeader(http.StatusNoContent)
}

// onRefUpdated runs the post-push fan-out for a set of ref movements: refresh
// pushed_at + language, refresh open PRs (SHAs/stats + re-scan), and fan out a
// `push` event to webhooks + SSE. Shared by the HTTP push hook and any in-process
// write path (browser commits, accepted AI proposals) so they all inherit the
// same bookkeeping and the supply-chain auto-audit.
func (s *Server) onRefUpdated(ctx context.Context, repoID int64, refs []RefUpdate) {
	// Update pushed_at and re-detect the dominant language.
	lang := s.git.DetectLanguage(repoID)
	_, _ = s.db.ExecContext(ctx,
		`UPDATE repos SET pushed_at = datetime('now'), language = ? WHERE id = ?`, lang, repoID)

	// Refresh open PRs in this repo: if a PR's head branch moved, update its
	// SHAs/stats and re-enqueue a supply-chain scan on the new head.
	rows, _ := s.db.QueryContext(ctx,
		`SELECT id, head_branch, base_branch, head_sha FROM pulls WHERE repo_id = ? AND state = 'open'`, repoID)
	type pr struct {
		id                  int64
		head, base, oldHead string
	}
	var prs []pr
	if rows != nil {
		for rows.Next() {
			var p pr
			if rows.Scan(&p.id, &p.head, &p.base, &p.oldHead) == nil {
				prs = append(prs, p)
			}
		}
		rows.Close()
	}
	for _, p := range prs {
		newHead, err := s.git.RevParse(repoID, p.head)
		if err != nil || newHead == p.oldHead {
			continue
		}
		baseSHA, e := s.git.MergeBase(repoID, p.base, p.head)
		if e != nil || baseSHA == "" {
			baseSHA, _ = s.git.RevParse(repoID, p.base)
		}
		adds, dels, files := s.git.DiffStat(repoID, baseSHA, newHead)
		commits := s.git.CountCommits(repoID, baseSHA, newHead)
		s.db.ExecContext(ctx,
			`UPDATE pulls SET head_sha=?, base_sha=?, additions=?, deletions=?, files_count=?, commits_count=?, updated_at=datetime('now') WHERE id=?`,
			newHead, baseSHA, adds, dels, files, commits, p.id)
		if s.scan != nil {
			s.scan.Enqueue(p.id, repoID, newHead)
		}
	}

	// Fan out a `push` event to webhook subscribers and live SSE listeners.
	var owner, name string
	s.db.QueryRowContext(ctx,
		`SELECT u.handle, rp.name FROM repos rp JOIN users u ON u.id = rp.owner_user_id WHERE rp.id = ?`,
		repoID).Scan(&owner, &name)
	outRefs := make([]map[string]string, 0, len(refs))
	for _, rf := range refs {
		outRefs = append(outRefs, map[string]string{"ref": rf.Ref, "old": rf.Old, "new": rf.New})
	}
	if s.webhooks != nil {
		s.webhooks.Fire(ctx, repoID, "push", map[string]any{
			"event": "push", "repo": owner + "/" + name, "repoId": repoID, "refs": outRefs,
		})
	}
	s.publish(repoTopic(owner, name), "push", map[string]any{"repo": owner + "/" + name, "refs": outRefs})

	s.log.Info("ref updated", "repo_id", repoID, "language", lang, "prs_refreshed", len(prs))
}
