package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/orchis-ai/foundry/internal/auth"
	gitstore "github.com/orchis-ai/foundry/internal/git"
)

// langPathspecs maps a ?lang= value to git pathspec globs. The globs are
// server-controlled (never user input) so they can't be used to traverse or
// inject. Unknown languages return nil (search everything).
var langPathspecs = map[string][]string{
	"go":   {"*.go"},
	"js":   {"*.js", "*.jsx", "*.mjs"},
	"ts":   {"*.ts", "*.tsx"},
	"py":   {"*.py"},
	"rs":   {"*.rs"},
	"rb":   {"*.rb"},
	"java": {"*.java"},
	"c":    {"*.c", "*.h"},
	"cpp":  {"*.cc", "*.cpp", "*.hpp", "*.cxx"},
	"sh":   {"*.sh"},
	"md":   {"*.md"},
	"json": {"*.json"},
	"yaml": {"*.yaml", "*.yml"},
	"html": {"*.html", "*.htm"},
	"css":  {"*.css"},
}

const (
	codeSearchTimeout  = 8 * time.Second
	codeSearchMaxHits  = 100
	codeSearchMaxRepos = 5
)

// GET /v1/search/code?q=&repo=&ref=&lang= — full-text code search via git grep.
//
// Security: the query is searched literally (fixed-string) and bound to git's
// -e option so it can't be read as a flag; refs are resolved to a hex sha
// before reaching git; repos are addressed by integer id (no user path); lang
// maps only to a server-side glob allowlist; results, per-file matches, and
// wall-clock are all bounded. Single-repo searches go through the same ACL as
// other repo reads (public, or owner for private).
func (s *Server) handleCodeSearch(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(q) < 2 {
		writeJSON(w, http.StatusOK, []map[string]any{})
		return
	}
	pathspecs := langPathspecs[strings.ToLower(strings.TrimSpace(r.URL.Query().Get("lang")))]

	ctx, cancel := context.WithTimeout(r.Context(), codeSearchTimeout)
	defer cancel()

	out := []map[string]any{}

	repoParam := strings.TrimSpace(r.URL.Query().Get("repo"))
	if repoParam != "" {
		parts := strings.SplitN(repoParam, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "repo must be org/name")
			return
		}
		row, err := s.repoForOwnerName(parts[0], parts[1])
		if err != nil || !codeSearchReadable(row, u) {
			writeError(w, http.StatusNotFound, "repo not found")
			return
		}
		ref := strings.TrimSpace(r.URL.Query().Get("ref"))
		if ref == "" {
			ref = row.DefaultBranch
		}
		sha, err := s.git.RevParse(row.ID, ref)
		if err != nil {
			writeError(w, http.StatusBadRequest, "ref not found: "+ref)
			return
		}
		s.grepInto(ctx, &out, row.ID, row.OwnerHandle+"/"+row.Name, sha, ref, q, pathspecs)
		writeJSON(w, http.StatusOK, out)
		return
	}

	// No repo specified — search the user's few most-recently-active readable
	// repos at their default branch. Bounded to keep this cheap.
	rows, err := s.db.QueryContext(ctx,
		`SELECT rp.id, u.handle, rp.name, rp.default_branch
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE rp.owner_user_id = ? OR rp.visibility = 'public'
		 ORDER BY COALESCE(rp.pushed_at, rp.created_at) DESC
		 LIMIT ?`, u.ID, codeSearchMaxRepos)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "search failed")
		return
	}
	type cand struct {
		id            int64
		label, branch string
	}
	var cands []cand
	for rows.Next() {
		var c cand
		var owner, name string
		if rows.Scan(&c.id, &owner, &name, &c.branch) == nil {
			c.label = owner + "/" + name
			cands = append(cands, c)
		}
	}
	rows.Close()

	for _, c := range cands {
		if len(out) >= codeSearchMaxHits {
			break
		}
		sha, err := s.git.RevParse(c.id, c.branch)
		if err != nil {
			continue
		}
		s.grepInto(ctx, &out, c.id, c.label, sha, c.branch, q, pathspecs)
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/search/repos?q= — repos matching the query (reuses palette logic).
func (s *Server) handleSearchRepos(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	writeJSON(w, http.StatusOK, s.searchRepos(r, u.ID, q))
}

// GET /v1/search/users?q= — users matching handle or name.
func (s *Server) handleSearchUsers(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	like := "%" + q + "%"
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT handle, name FROM users
		 WHERE ? = '' OR handle LIKE ? OR name LIKE ?
		 ORDER BY handle LIMIT 10`, q, like, like)
	out := []map[string]any{}
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var handle, name string
			if rows.Scan(&handle, &name) == nil {
				out = append(out, map[string]any{
					"id": handle, "handle": handle, "name": name,
					"initials": initials(name, handle), "color": colorFor(handle),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// grepInto runs one repo's grep and appends shaped hits to out, respecting the
// overall hit cap.
func (s *Server) grepInto(ctx context.Context, out *[]map[string]any, repoID int64, label, sha, ref, q string, pathspecs []string) {
	remaining := codeSearchMaxHits - len(*out)
	if remaining <= 0 {
		return
	}
	hits, _ := s.git.Grep(ctx, repoID, sha, q, pathspecs, 20)
	for _, h := range hits {
		if len(*out) >= codeSearchMaxHits {
			return
		}
		*out = append(*out, map[string]any{
			"repo": label, "path": h.Path, "ref": ref, "line": h.Line, "text": h.Text,
			"route": map[string]any{"view": "repo", "repo": label, "file": h.Path},
		})
	}
}

// codeSearchReadable mirrors loadRepo's ACL for a repo resolved by query param
// (loadRepo reads chi URL params, which code search doesn't use).
func codeSearchReadable(row *repoRow, u *auth.User) bool {
	if row.Visibility == "public" {
		return true
	}
	return u != nil && row.OwnerUserID.Valid && row.OwnerUserID.Int64 == u.ID
}

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
