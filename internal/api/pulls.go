package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

// userBrief returns the {id,name,handle,color,initials} shape the frontend uses
// for author/reviewer objects.
func (s *Server) userBrief(id int64) map[string]any {
	var handle, name string
	if err := s.db.QueryRow(`SELECT handle, name FROM users WHERE id = ?`, id).Scan(&handle, &name); err != nil {
		return map[string]any{"id": "?", "name": "unknown", "handle": "unknown", "color": "var(--fg-3)", "initials": "?"}
	}
	return map[string]any{
		"id": handle, "name": name, "handle": handle,
		"color": colorFor(handle), "initials": initials(name, handle),
	}
}

// pullSummary builds the PullSummary shape (matches PRS in src/data.jsx).
func (s *Server) pullSummary(r *http.Request, pullID int64) map[string]any {
	var (
		id, repoID, authorID                          int64
		number, additions, deletions, files, commits  int
		title, body, head, base, state, updated       string
		ownerHandle, repoName                          string
	)
	err := s.db.QueryRowContext(r.Context(),
		`SELECT p.id, p.repo_id, p.number, p.title, p.body, p.author_id, p.head_branch, p.base_branch,
		        p.state, p.additions, p.deletions, p.files_count, p.commits_count, p.updated_at,
		        u.handle, rp.name
		 FROM pulls p JOIN repos rp ON rp.id = p.repo_id JOIN users u ON u.id = rp.owner_user_id
		 WHERE p.id = ?`, pullID).
		Scan(&id, &repoID, &number, &title, &body, &authorID, &head, &base, &state,
			&additions, &deletions, &files, &commits, &updated, &ownerHandle, &repoName)
	if err != nil {
		return nil
	}

	// reviewers
	reviewers := []map[string]any{}
	rrows, _ := s.db.QueryContext(r.Context(), `SELECT user_id FROM pull_reviewers WHERE pull_id = ?`, id)
	if rrows != nil {
		for rrows.Next() {
			var uid int64
			if rrows.Scan(&uid) == nil {
				reviewers = append(reviewers, s.userBrief(uid))
			}
		}
		rrows.Close()
	}

	// comment count + check aggregate
	var commentCount int
	s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM pull_comments WHERE pull_id = ?`, id).Scan(&commentCount)

	var headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT head_sha FROM pulls WHERE id = ?`, id).Scan(&headSHA)
	passed, failed, pending := 0, 0, 0
	crows, _ := s.db.QueryContext(r.Context(), `SELECT status FROM checks WHERE repo_id = ? AND sha = ?`, repoID, headSHA)
	if crows != nil {
		for crows.Next() {
			var st string
			crows.Scan(&st)
			switch st {
			case "ok":
				passed++
			case "fail":
				failed++
			default:
				pending++
			}
		}
		crows.Close()
	}

	return map[string]any{
		"id":          number,
		"title":       title,
		"repo":        ownerHandle + "/" + repoName,
		"author":      s.userBrief(authorID),
		"branch":      head,
		"base":        base,
		"status":      state,
		"reviewers":   reviewers,
		"checks":      map[string]int{"passed": passed, "failed": failed, "pending": pending},
		"additions":   additions,
		"deletions":   deletions,
		"commits":     commits,
		"files":       files,
		"updated":     relativeTime(updated),
		"description": body,
		"labels":      []any{},
		"comments":    commentCount,
	}
}

func (s *Server) pullIDByNumber(repoID int64, number int) (int64, bool) {
	var id int64
	err := s.db.QueryRow(`SELECT id FROM pulls WHERE repo_id = ? AND number = ?`, repoID, number).Scan(&id)
	return id, err == nil
}

// GET /v1/pulls?filter=review-requested|yours|mentioned|all
func (s *Server) handleListPullsGlobal(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	filter := r.URL.Query().Get("filter")
	var rows *sql.Rows
	var err error
	switch filter {
	case "yours":
		rows, err = s.db.QueryContext(r.Context(), `SELECT id FROM pulls WHERE author_id = ? ORDER BY updated_at DESC`, u.ID)
	case "review-requested":
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT p.id FROM pulls p JOIN pull_reviewers pr ON pr.pull_id = p.id WHERE pr.user_id = ? ORDER BY p.updated_at DESC`, u.ID)
	default: // all / mentioned → repos I can see
		rows, err = s.db.QueryContext(r.Context(),
			`SELECT p.id FROM pulls p JOIN repos rp ON rp.id = p.repo_id
			 WHERE rp.owner_user_id = ? OR rp.visibility = 'public' ORDER BY p.updated_at DESC`, u.ID)
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list pulls")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			if ps := s.pullSummary(r, id); ps != nil {
				out = append(out, ps)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/:org/:name/pulls?state=open|closed|all
func (s *Server) handleListRepoPulls(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	state := r.URL.Query().Get("state")
	q := `SELECT id FROM pulls WHERE repo_id = ?`
	args := []any{row.ID}
	if state == "open" || state == "closed" || state == "merged" {
		q += ` AND state = ?`
		args = append(args, state)
	}
	q += ` ORDER BY number DESC`
	rows, err := s.db.QueryContext(r.Context(), q, args...)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list pulls")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			if ps := s.pullSummary(r, id); ps != nil {
				out = append(out, ps)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// GET /v1/repos/:org/:name/compare?base=&head= — preview a PR between two
// branches (diff + commits) without creating one. Read-only, ACL via loadRepo.
func (s *Server) handleCompare(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	base := r.URL.Query().Get("base")
	if base == "" {
		base = row.DefaultBranch
	}
	head := r.URL.Query().Get("head")
	if head == "" {
		writeError(w, http.StatusBadRequest, "head branch required")
		return
	}
	headSHA, err := s.git.RevParse(row.ID, head)
	if err != nil {
		writeError(w, http.StatusBadRequest, "head not found: "+head)
		return
	}
	baseSHA, err := s.git.MergeBase(row.ID, base, head)
	if err != nil || baseSHA == "" {
		if baseSHA, err = s.git.RevParse(row.ID, base); err != nil {
			writeError(w, http.StatusBadRequest, "base not found: "+base)
			return
		}
	}
	adds, dels, files := s.git.DiffStat(row.ID, baseSHA, headSHA)
	commits, _ := s.git.RangeCommits(row.ID, baseSHA, headSHA, 200)
	diffs, _ := s.git.Diff(row.ID, baseSHA, headSHA)
	writeJSON(w, http.StatusOK, map[string]any{
		"base": base, "head": head,
		"aheadBy": len(commits), "additions": adds, "deletions": dels, "filesCount": files,
		"commits": commits, "files": diffs,
	})
}

// POST /v1/repos/:org/:name/pulls { title, head, base, body }
func (s *Server) handleCreatePull(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	var in struct {
		Title, Head, Base, Body string
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Title == "" || in.Head == "" {
		writeError(w, http.StatusBadRequest, "title and head branch are required")
		return
	}
	if in.Base == "" {
		in.Base = row.DefaultBranch
	}

	headSHA, err := s.git.RevParse(row.ID, in.Head)
	if err != nil {
		writeError(w, http.StatusBadRequest, "head branch not found: "+in.Head)
		return
	}
	baseSHA, err := s.git.MergeBase(row.ID, in.Base, in.Head)
	if err != nil || baseSHA == "" {
		baseSHA, _ = s.git.RevParse(row.ID, in.Base)
	}

	adds, dels, files := s.git.DiffStat(row.ID, baseSHA, headSHA)
	commits := s.git.CountCommits(row.ID, baseSHA, headSHA)

	var number int
	s.db.QueryRowContext(r.Context(), `SELECT COALESCE(MAX(number),0)+1 FROM pulls WHERE repo_id = ?`, row.ID).Scan(&number)

	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO pulls (repo_id, number, title, body, author_id, head_branch, base_branch, head_sha, base_sha, additions, deletions, files_count, commits_count)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		row.ID, number, in.Title, in.Body, u.ID, in.Head, in.Base, headSHA, baseSHA, adds, dels, files, commits)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not create pull request")
		return
	}
	id, _ := res.LastInsertId()
	if s.scan != nil {
		s.scan.Enqueue(id, row.ID, headSHA)
	}
	s.logActivity(r.Context(), u.ID, "pr_opened", row.ID, row.OwnerHandle+"/"+row.Name+"#"+strconv.Itoa(number), in.Title)
	if s.webhooks != nil {
		s.webhooks.Fire(r.Context(), row.ID, "pull_request", map[string]any{
			"event": "pull_request", "action": "opened", "repo": row.OwnerHandle + "/" + row.Name,
			"number": number, "title": in.Title, "head": in.Head, "base": in.Base, "author": u.Handle,
		})
	}
	writeJSON(w, http.StatusOK, s.pullSummary(r, id))
}

func (s *Server) handleGetPull(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	writeJSON(w, http.StatusOK, s.pullSummary(r, id))
}

// GET .../pulls/:num/files → [FileDiff] with comments attached
func (s *Server) handlePullFiles(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var baseSHA, headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT base_sha, head_sha FROM pulls WHERE id = ?`, id).Scan(&baseSHA, &headSHA)

	diffs, _ := s.git.Diff(row.ID, baseSHA, headSHA)

	// Attach line comments per file.
	out := make([]map[string]any, 0, len(diffs))
	for _, fd := range diffs {
		comments := []map[string]any{}
		crows, _ := s.db.QueryContext(r.Context(),
			`SELECT author_id, body, line, side, created_at FROM pull_comments WHERE pull_id = ? AND path = ? AND line IS NOT NULL ORDER BY id`,
			id, fd.Path)
		if crows != nil {
			for crows.Next() {
				var aid int64
				var body, side, created string
				var line int
				if crows.Scan(&aid, &body, &line, &side, &created) == nil {
					comments = append(comments, map[string]any{
						"line": line, "side": side, "author": s.userBrief(aid),
						"when": relativeTime(created), "body": body, "suggestion": nil,
					})
				}
			}
			crows.Close()
		}
		out = append(out, map[string]any{
			"path": fd.Path, "additions": fd.Additions, "deletions": fd.Deletions,
			"hunks": fd.Hunks, "comments": comments,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePullCommits(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var baseSHA, headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT base_sha, head_sha FROM pulls WHERE id = ?`, id).Scan(&baseSHA, &headSHA)
	commits, _ := s.git.RangeCommits(row.ID, baseSHA, headSHA, 100)
	writeJSON(w, http.StatusOK, commits)
}

func (s *Server) handlePullComments(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT author_id, body, COALESCE(path,''), line, COALESCE(side,''), created_at FROM pull_comments WHERE pull_id = ? ORDER BY id`, id)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var aid int64
			var body, path, side, created string
			var line sql.NullInt64
			if rows.Scan(&aid, &body, &path, &line, &side, &created) == nil {
				c := map[string]any{"author": s.userBrief(aid), "body": body, "when": relativeTime(created), "path": path, "side": side}
				if line.Valid {
					c["line"] = line.Int64
				}
				out = append(out, c)
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handlePullChecks(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	num, _ := strconv.Atoi(chi.URLParam(r, "num"))
	id, found := s.pullIDByNumber(row.ID, num)
	if !found {
		writeError(w, http.StatusNotFound, "pull request not found")
		return
	}
	var headSHA string
	s.db.QueryRowContext(r.Context(), `SELECT head_sha FROM pulls WHERE id = ?`, id).Scan(&headSHA)
	rows, _ := s.db.QueryContext(r.Context(),
		`SELECT name, status, detail, COALESCE(external_url,'') FROM checks WHERE repo_id = ? AND sha = ? ORDER BY name`, row.ID, headSHA)
	out := []map[string]any{}
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var name, status, detail, url string
			if rows.Scan(&name, &status, &detail, &url) == nil {
				out = append(out, map[string]any{"name": name, "status": status, "detail": detail, "externalUrl": url})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}
