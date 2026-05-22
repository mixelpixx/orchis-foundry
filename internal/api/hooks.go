package api

import (
	"encoding/json"
	"net/http"
)

// handlePushHook is called by each repo's post-receive hook after a push.
// Guarded by a shared secret (the config session key) since it's reachable
// via the same loopback the proxy uses.
func (s *Server) handlePushHook(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Hook-Secret") != s.cfg.SessionKey || s.cfg.SessionKey == "" {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	var in struct {
		RepoID int64 `json:"repo_id"`
		Refs   []struct {
			Ref string `json:"ref"`
			Old string `json:"old"`
			New string `json:"new"`
		} `json:"refs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.RepoID == 0 {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}

	// Update pushed_at and re-detect the dominant language.
	lang := s.git.DetectLanguage(in.RepoID)
	_, _ = s.db.ExecContext(r.Context(),
		`UPDATE repos SET pushed_at = datetime('now'), language = ? WHERE id = ?`, lang, in.RepoID)

	// Refresh open PRs in this repo: if a PR's head branch moved, update its
	// SHAs/stats and re-enqueue a supply-chain scan on the new head.
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT id, head_branch, base_branch, head_sha FROM pulls WHERE repo_id = ? AND state = 'open'`, in.RepoID)
	type pr struct {
		id                          int64
		head, base, oldHead         string
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
		newHead, err := s.git.RevParse(in.RepoID, p.head)
		if err != nil || newHead == p.oldHead {
			continue
		}
		baseSHA, e := s.git.MergeBase(in.RepoID, p.base, p.head)
		if e != nil || baseSHA == "" {
			baseSHA, _ = s.git.RevParse(in.RepoID, p.base)
		}
		adds, dels, files := s.git.DiffStat(in.RepoID, baseSHA, newHead)
		commits := s.git.CountCommits(in.RepoID, baseSHA, newHead)
		s.db.ExecContext(r.Context(),
			`UPDATE pulls SET head_sha=?, base_sha=?, additions=?, deletions=?, files_count=?, commits_count=?, updated_at=datetime('now') WHERE id=?`,
			newHead, baseSHA, adds, dels, files, commits, p.id)
		if s.scan != nil {
			s.scan.Enqueue(p.id, in.RepoID, newHead)
		}
	}

	// Fan out a `push` webhook event to subscribers of this repo.
	if s.webhooks != nil {
		var owner, name string
		s.db.QueryRowContext(r.Context(),
			`SELECT u.handle, rp.name FROM repos rp JOIN users u ON u.id = rp.owner_user_id WHERE rp.id = ?`,
			in.RepoID).Scan(&owner, &name)
		refs := make([]map[string]string, 0, len(in.Refs))
		for _, rf := range in.Refs {
			refs = append(refs, map[string]string{"ref": rf.Ref, "old": rf.Old, "new": rf.New})
		}
		s.webhooks.Fire(r.Context(), in.RepoID, "push", map[string]any{
			"event": "push", "repo": owner + "/" + name, "repoId": in.RepoID, "refs": refs,
		})
	}

	s.log.Info("push received", "repo_id", in.RepoID, "language", lang, "prs_refreshed", len(prs))
	w.WriteHeader(http.StatusNoContent)
}
