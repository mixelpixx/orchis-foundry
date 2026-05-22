package api

import (
	"net/http"
	"strconv"
	"strings"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

// GET /v1/search/palette?q= — ranked hits across repos, pulls, and files for
// the ⌘K command palette. Quick-action and navigation groups are built on the
// client (they need local navigation callbacks); this endpoint supplies the
// live data groups: Repositories, Pull requests, Files.
//
// Response shape (consumed by src/palette.jsx):
//
//	{
//	  "repos": [ { id, label, sub, route:{view,repo} } ],
//	  "pulls": [ { id, label, sub, route:{view,pr,repo} } ],
//	  "files": [ { id, label, sub, route:{view,repo,file} } ]
//	}
func (s *Server) handlePaletteSearch(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))

	repos := s.searchRepos(r, u.ID, q)
	pulls := s.searchPulls(r, u.ID, q)
	files := []map[string]any{}
	if q != "" {
		// Files matching the query by filename, scanned across the user's few
		// most-recently-active repos (cap 3 trees walked, 20 hits total). This
		// decouples file search from repo-name matching so typing a filename
		// finds it regardless of which repo it lives in.
		files = s.searchFiles(r, u.ID, q)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"repos": repos,
		"pulls": pulls,
		"files": files,
	})
}

func (s *Server) searchRepos(r *http.Request, userID int64, q string) []map[string]any {
	like := "%" + q + "%"
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT u.handle, rp.name, rp.description
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE (rp.owner_user_id = ? OR rp.visibility = 'public')
		   AND (? = '' OR rp.name LIKE ? OR rp.description LIKE ?)
		 ORDER BY CASE WHEN rp.name LIKE ? THEN 0 ELSE 1 END,
		          COALESCE(rp.pushed_at, rp.created_at) DESC
		 LIMIT 6`,
		userID, q, like, like, q+"%")
	out := []map[string]any{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var owner, name, desc string
		if rows.Scan(&owner, &name, &desc) == nil {
			id := owner + "/" + name
			out = append(out, map[string]any{
				"id": id, "label": id, "sub": desc,
				"route": map[string]any{"view": "repo", "repo": id},
			})
		}
	}
	return out
}

func (s *Server) searchPulls(r *http.Request, userID int64, q string) []map[string]any {
	like := "%" + q + "%"
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT p.number, p.title, u.handle, rp.name
		 FROM pulls p
		 JOIN repos rp ON rp.id = p.repo_id
		 JOIN users u ON u.id = rp.owner_user_id
		 WHERE (rp.owner_user_id = ? OR rp.visibility = 'public')
		   AND (? = '' OR p.title LIKE ?)
		 ORDER BY p.updated_at DESC
		 LIMIT 6`,
		userID, q, like)
	out := []map[string]any{}
	if err != nil {
		return out
	}
	defer rows.Close()
	for rows.Next() {
		var num int
		var title, owner, name string
		if rows.Scan(&num, &title, &owner, &name) == nil {
			repo := owner + "/" + name
			out = append(out, map[string]any{
				"id":    repo + "#" + strconv.Itoa(num),
				"label": "#" + strconv.Itoa(num) + " " + title,
				"sub":   repo,
				"route": map[string]any{"view": "pr", "pr": num, "repo": repo},
			})
		}
	}
	return out
}

// searchFiles scans the user's 3 most-recently-active visible repos for files
// whose name contains q (case-insensitive). Capped at 20 hits total.
func (s *Server) searchFiles(r *http.Request, userID int64, q string) []map[string]any {
	type cand struct {
		id    int64
		label string
	}
	var cands []cand
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT rp.id, u.handle, rp.name
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE rp.owner_user_id = ? OR rp.visibility = 'public'
		 ORDER BY COALESCE(rp.pushed_at, rp.created_at) DESC
		 LIMIT 3`, userID)
	if err != nil {
		return []map[string]any{}
	}
	for rows.Next() {
		var id int64
		var owner, name string
		if rows.Scan(&id, &owner, &name) == nil {
			cands = append(cands, cand{id: id, label: owner + "/" + name})
		}
	}
	rows.Close()

	ql := strings.ToLower(q)
	out := []map[string]any{}
	for _, c := range cands {
		if len(out) >= 20 {
			break
		}
		nodes, terr := s.git.Tree(c.id, "")
		if terr != nil {
			continue
		}
		var walk func([]*gitstore.Node)
		walk = func(ns []*gitstore.Node) {
			for _, n := range ns {
				if len(out) >= 20 {
					return
				}
				if n.Type == "file" && strings.Contains(strings.ToLower(n.Name), ql) {
					out = append(out, map[string]any{
						"id":    c.label + ":" + n.Path,
						"label": n.Name,
						"sub":   c.label + " · " + n.Path,
						"route": map[string]any{"view": "repo", "repo": c.label, "file": n.Path},
					})
				}
				if n.Children != nil {
					walk(n.Children)
				}
			}
		}
		walk(nodes)
	}
	return out
}
