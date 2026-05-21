package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

const (
	packDefaultMax = 512 * 1024
	packHardMax    = 4 * 1024 * 1024
	packPerFileMax = 200 * 1024
)

// flattenFiles walks the nested tree into a flat list of file paths.
func flattenFiles(nodes []*gitstore.Node, out *[]string) {
	for _, n := range nodes {
		if n.Type == "file" {
			*out = append(*out, n.Path)
		} else {
			flattenFiles(n.Children, out)
		}
	}
}

func isBinary(content string) bool {
	// Heuristic: a NUL byte in the first 8KB means binary.
	n := len(content)
	if n > 8192 {
		n = 8192
	}
	return strings.IndexByte(content[:n], 0) >= 0
}

// GET /v1/repos/{org}/{name}/pack?ref=&maxBytes=
// Flattens a repo into one LLM-ingestible markdown document.
func (s *Server) handleRepoPack(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	ref := r.URL.Query().Get("ref")
	maxBytes := packDefaultMax
	if v, err := strconv.Atoi(r.URL.Query().Get("maxBytes")); err == nil && v > 0 {
		maxBytes = v
	}
	if maxBytes > packHardMax {
		maxBytes = packHardMax
	}

	nodes, _ := s.git.Tree(row.ID, ref)
	var files []string
	flattenFiles(nodes, &files)

	var b strings.Builder
	b.WriteString("# Repository: " + row.OwnerHandle + "/" + row.Name + "\n\n")
	if ref != "" {
		b.WriteString("Ref: `" + ref + "`\n\n")
	}
	b.WriteString("## File tree\n\n```\n")
	for _, f := range files {
		b.WriteString(f + "\n")
	}
	b.WriteString("```\n\n## Files\n\n")

	var skipped []string
	truncated := false
	for _, path := range files {
		if b.Len() >= maxBytes {
			truncated = true
			break
		}
		blob, err := s.git.FileBlob(row.ID, ref, path)
		if err != nil {
			continue
		}
		if blob.Size > packPerFileMax || isBinary(blob.Content) {
			skipped = append(skipped, path)
			continue
		}
		fence := "```" + blob.Lang + "\n"
		b.WriteString("### " + path + "\n\n" + fence + blob.Content)
		if !strings.HasSuffix(blob.Content, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("```\n\n")
	}
	if len(skipped) > 0 {
		b.WriteString("## Skipped (binary or oversized)\n\n```\n" + strings.Join(skipped, "\n") + "\n```\n")
	}
	if truncated {
		b.WriteString("\n_[pack truncated at " + strconv.Itoa(maxBytes) + " bytes — raise maxBytes for more]_\n")
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(b.String()))
}

// GET /v1/repos/{org}/{name}/pulls/{num}/pack
// One document describing a PR: meta + commits + diff + comments.
func (s *Server) handlePRPack(w http.ResponseWriter, r *http.Request) {
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

	var title, body, head, base, baseSHA, headSHA, state string
	s.db.QueryRowContext(r.Context(),
		`SELECT title, body, head_branch, base_branch, base_sha, head_sha, state FROM pulls WHERE id = ?`, pullID).
		Scan(&title, &body, &head, &base, &baseSHA, &headSHA, &state)

	var b strings.Builder
	b.WriteString("# PR #" + strconv.Itoa(num) + ": " + title + "\n\n")
	b.WriteString("- Repo: `" + row.OwnerHandle + "/" + row.Name + "`\n")
	b.WriteString("- State: " + state + "\n")
	b.WriteString("- Branch: `" + head + "` → `" + base + "`\n\n")
	if body != "" {
		b.WriteString("## Description\n\n" + body + "\n\n")
	}

	commits, _ := s.git.RangeCommits(row.ID, baseSHA, headSHA, 100)
	if len(commits) > 0 {
		b.WriteString("## Commits\n\n")
		for _, c := range commits {
			b.WriteString("- `" + c.Short + "` " + strings.SplitN(c.Message, "\n", 2)[0] + " — " + c.Author + "\n")
		}
		b.WriteString("\n")
	}

	diffs, _ := s.git.Diff(row.ID, baseSHA, headSHA)
	b.WriteString("## Diff\n\n```diff\n")
	b.WriteString(renderDiff(diffs))
	b.WriteString("```\n\n")

	// Existing comments.
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT pc.body, COALESCE(pc.path,''), pc.line, u.handle FROM pull_comments pc JOIN users u ON u.id = pc.author_id WHERE pc.pull_id = ? ORDER BY pc.id`, pullID)
	if rows != nil {
		defer rows.Close()
		wrote := false
		for rows.Next() {
			var cbody, path, handle string
			var line interface{}
			if rows.Scan(&cbody, &path, &line, &handle) == nil {
				if !wrote {
					b.WriteString("## Comments\n\n")
					wrote = true
				}
				loc := ""
				if path != "" {
					loc = " (" + path + ")"
				}
				b.WriteString("- **@" + handle + "**" + loc + ": " + cbody + "\n")
			}
		}
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(b.String()))
}
