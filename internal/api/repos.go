package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

var repoNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)

// repoRow is the DB view of a repo.
type repoRow struct {
	ID            int64
	OwnerUserID   sql.NullInt64
	OwnerHandle   string
	Name          string
	Description   string
	Visibility    string
	DefaultBranch string
	Language      string
	CreatedAt     string
	PushedAt      sql.NullString
	Pinned        bool
	Stars         int
	Starred       bool
}

// repoJSON shapes a repo for the frontend (matches REPOS in src/data.jsx).
func repoJSON(r *repoRow) map[string]any {
	updated := r.CreatedAt
	if r.PushedAt.Valid && r.PushedAt.String != "" {
		updated = r.PushedAt.String
	}
	return map[string]any{
		"id":            r.OwnerHandle + "/" + r.Name,
		"name":          r.Name,
		"org":           r.OwnerHandle,
		"description":   r.Description,
		"language":      r.Language,
		"languageColor": gitstore.LangColor(r.Language),
		"stars":         r.Stars,
		"starred":       r.Starred,
		"forks":         0,
		"watchers":      0,
		"visibility":    r.Visibility,
		"defaultBranch": r.DefaultBranch,
		"updated":       relativeTime(updated),
		"pinned":        r.Pinned,
	}
}

// loadRepo resolves :org/:name to a repo row and checks read access for the
// current user. Returns (row, true) if accessible.
func (s *Server) loadRepo(r *http.Request) (*repoRow, bool) {
	owner := chi.URLParam(r, "org")
	name := chi.URLParam(r, "name")
	row := &repoRow{}
	var pushedAt sql.NullString
	err := s.db.QueryRowContext(r.Context(),
		`SELECT rp.id, rp.owner_user_id, u.handle, rp.name, rp.description, rp.visibility,
		        rp.default_branch, rp.language, rp.created_at, rp.pushed_at
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE u.handle = ? AND rp.name = ?`,
		owner, name).Scan(&row.ID, &row.OwnerUserID, &row.OwnerHandle, &row.Name,
		&row.Description, &row.Visibility, &row.DefaultBranch, &row.Language, &row.CreatedAt, &pushedAt)
	if err != nil {
		return nil, false
	}
	row.PushedAt = pushedAt

	// ACL: public repos readable by anyone (incl. unauthenticated); others
	// require the owner. (Org membership ACL is a later milestone.)
	if row.Visibility == "public" {
		return row, true
	}
	u := userFrom(r)
	if u != nil && row.OwnerUserID.Valid && row.OwnerUserID.Int64 == u.ID {
		return row, true
	}
	return nil, false
}

func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	rows, err := s.db.QueryContext(r.Context(),
		`SELECT rp.id, rp.owner_user_id, u.handle, rp.name, rp.description, rp.visibility,
		        rp.default_branch, rp.language, rp.created_at, rp.pushed_at,
		        EXISTS(SELECT 1 FROM user_repo_pins p WHERE p.repo_id = rp.id AND p.user_id = ?) AS pinned,
		        (SELECT COUNT(*) FROM user_repo_stars s WHERE s.repo_id = rp.id) AS stars,
		        EXISTS(SELECT 1 FROM user_repo_stars s WHERE s.repo_id = rp.id AND s.user_id = ?) AS starred
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE rp.owner_user_id = ? OR rp.visibility = 'public'
		 ORDER BY COALESCE(rp.pushed_at, rp.created_at) DESC`,
		u.ID, u.ID, u.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not list repos")
		return
	}
	defer rows.Close()
	out := []map[string]any{}
	for rows.Next() {
		row := &repoRow{}
		var pushedAt sql.NullString
		var pinned, starred int
		if err := rows.Scan(&row.ID, &row.OwnerUserID, &row.OwnerHandle, &row.Name, &row.Description,
			&row.Visibility, &row.DefaultBranch, &row.Language, &row.CreatedAt, &pushedAt, &pinned, &row.Stars, &starred); err != nil {
			continue
		}
		row.PushedAt = pushedAt
		row.Pinned = pinned == 1
		row.Starred = starred == 1
		out = append(out, repoJSON(row))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateRepo(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	var in struct {
		Org         string `json:"org"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	// v1: repos are owned by the creating user. The org field, if present,
	// must match the user's own handle.
	if in.Org != "" && in.Org != u.Handle {
		writeError(w, http.StatusBadRequest, "v1 only supports repos under your own handle ("+u.Handle+")")
		return
	}
	if !repoNameRe.MatchString(in.Name) {
		writeError(w, http.StatusBadRequest, "invalid repo name")
		return
	}
	if in.Visibility != "public" && in.Visibility != "private" && in.Visibility != "internal" {
		in.Visibility = "private"
	}

	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO repos (owner_user_id, name, description, visibility, default_branch)
		 VALUES (?,?,?,?, 'main')`,
		u.ID, in.Name, in.Description, in.Visibility)
	if err != nil {
		writeError(w, http.StatusConflict, "a repo with that name already exists")
		return
	}
	id, _ := res.LastInsertId()
	if err := s.git.InitBare(id, "main"); err != nil {
		_, _ = s.db.ExecContext(r.Context(), `DELETE FROM repos WHERE id = ?`, id)
		s.log.Error("init bare failed", "err", err)
		writeError(w, http.StatusInternalServerError, "could not initialize repository storage")
		return
	}
	s.installPushHook(id)
	s.logActivity(r.Context(), u.ID, "repo_created", id, u.Handle+"/"+in.Name, in.Name)

	row := &repoRow{
		ID: id, OwnerUserID: sql.NullInt64{Int64: u.ID, Valid: true}, OwnerHandle: u.Handle,
		Name: in.Name, Description: in.Description, Visibility: in.Visibility,
		DefaultBranch: "main", CreatedAt: time.Now().UTC().Format("2006-01-02 15:04:05"),
	}
	writeJSON(w, http.StatusOK, repoJSON(row))
}

func (s *Server) handleGetRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	// pinned / starred flags for the current user + total star count
	_ = s.db.QueryRowContext(r.Context(),
		`SELECT COUNT(*) FROM user_repo_stars WHERE repo_id = ?`, row.ID).Scan(&row.Stars)
	if u := userFrom(r); u != nil {
		var pinned, starred int
		_ = s.db.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM user_repo_pins WHERE repo_id = ? AND user_id = ?`, row.ID, u.ID).Scan(&pinned)
		_ = s.db.QueryRowContext(r.Context(),
			`SELECT COUNT(*) FROM user_repo_stars WHERE repo_id = ? AND user_id = ?`, row.ID, u.ID).Scan(&starred)
		row.Pinned = pinned > 0
		row.Starred = starred > 0
	}
	writeJSON(w, http.StatusOK, repoJSON(row))
}

// PATCH /v1/repos/{org}/{name} { name?, description?, visibility?, defaultBranch? }
func (s *Server) handleUpdateRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	if !row.OwnerUserID.Valid || row.OwnerUserID.Int64 != u.ID {
		writeError(w, http.StatusForbidden, "only the repo owner can change settings")
		return
	}
	var in struct {
		Name          *string `json:"name"`
		Description   *string `json:"description"`
		Visibility    *string `json:"visibility"`
		DefaultBranch *string `json:"defaultBranch"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	if in.Name != nil {
		if !repoNameRe.MatchString(*in.Name) {
			writeError(w, http.StatusBadRequest, "invalid repo name")
			return
		}
		if _, err := s.db.ExecContext(r.Context(), `UPDATE repos SET name = ? WHERE id = ?`, *in.Name, row.ID); err != nil {
			writeError(w, http.StatusConflict, "a repo with that name already exists")
			return
		}
		row.Name = *in.Name
	}
	if in.Description != nil {
		s.db.ExecContext(r.Context(), `UPDATE repos SET description = ? WHERE id = ?`, *in.Description, row.ID)
	}
	if in.Visibility != nil {
		if *in.Visibility != "public" && *in.Visibility != "private" && *in.Visibility != "internal" {
			writeError(w, http.StatusBadRequest, "invalid visibility")
			return
		}
		s.db.ExecContext(r.Context(), `UPDATE repos SET visibility = ? WHERE id = ?`, *in.Visibility, row.ID)
	}
	if in.DefaultBranch != nil && *in.DefaultBranch != "" {
		if err := s.git.SetDefaultBranch(row.ID, *in.DefaultBranch); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		s.db.ExecContext(r.Context(), `UPDATE repos SET default_branch = ? WHERE id = ?`, *in.DefaultBranch, row.ID)
	}
	// Return the fresh repo (full row).
	fresh := &repoRow{}
	var pushedAt sql.NullString
	err := s.db.QueryRowContext(r.Context(),
		`SELECT rp.id, rp.owner_user_id, u.handle, rp.name, rp.description, rp.visibility,
		        rp.default_branch, rp.language, rp.created_at, rp.pushed_at
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id WHERE rp.id = ?`, row.ID).
		Scan(&fresh.ID, &fresh.OwnerUserID, &fresh.OwnerHandle, &fresh.Name, &fresh.Description,
			&fresh.Visibility, &fresh.DefaultBranch, &fresh.Language, &fresh.CreatedAt, &pushedAt)
	if err != nil {
		writeJSON(w, http.StatusOK, repoJSON(row))
		return
	}
	fresh.PushedAt = pushedAt
	writeJSON(w, http.StatusOK, repoJSON(fresh))
}

// GET /v1/repos/{org}/{name}/tags
func (s *Server) handleRepoTags(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	tags, _ := s.git.Tags(row.ID)
	writeJSON(w, http.StatusOK, tags)
}

func (s *Server) handleDeleteRepo(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	if !row.OwnerUserID.Valid || row.OwnerUserID.Int64 != u.ID {
		writeError(w, http.StatusForbidden, "not your repo")
		return
	}
	_, _ = s.db.ExecContext(r.Context(), `DELETE FROM repos WHERE id = ?`, row.ID)
	_ = s.git.Remove(row.ID)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRepoTree(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	nodes, err := s.git.Tree(row.ID, r.URL.Query().Get("ref"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "tree read failed")
		return
	}
	writeJSON(w, http.StatusOK, nodes)
}

func (s *Server) handleRepoBlob(w http.ResponseWriter, r *http.Request) {
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
	blob, err := s.git.FileBlob(row.ID, r.URL.Query().Get("ref"), path)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	writeJSON(w, http.StatusOK, blob)
}

func (s *Server) handleRepoRaw(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	b, err := s.git.RawBytes(row.ID, r.URL.Query().Get("ref"), r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	_, _ = w.Write(b)
}

func (s *Server) handleRepoReadme(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	content, format, found := s.git.Readme(row.ID, r.URL.Query().Get("ref"))
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"content": "", "format": "markdown"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": content, "format": format})
}

func (s *Server) handleRepoBranches(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	branches, _ := s.git.Branches(row.ID, row.DefaultBranch)
	writeJSON(w, http.StatusOK, branches)
}

func (s *Server) handleRepoCommits(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	commits, _ := s.git.Commits(row.ID, r.URL.Query().Get("ref"), limit)
	writeJSON(w, http.StatusOK, commits)
}

// repoByID loads a repo row by numeric ID (used by git-http + hooks).
func (s *Server) repoByID(id int64) (*repoRow, error) {
	row := &repoRow{}
	var pushedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT rp.id, rp.owner_user_id, u.handle, rp.name, rp.visibility, rp.default_branch
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id WHERE rp.id = ?`, id).
		Scan(&row.ID, &row.OwnerUserID, &row.OwnerHandle, &row.Name, &row.Visibility, &row.DefaultBranch)
	if err != nil {
		return nil, err
	}
	row.PushedAt = pushedAt
	return row, nil
}

// repoForOwnerName resolves owner-handle + name to a repo row (no ACL).
func (s *Server) repoForOwnerName(owner, name string) (*repoRow, error) {
	row := &repoRow{}
	err := s.db.QueryRow(
		`SELECT rp.id, rp.owner_user_id, u.handle, rp.name, rp.visibility, rp.default_branch
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE u.handle = ? AND rp.name = ?`, owner, name).
		Scan(&row.ID, &row.OwnerUserID, &row.OwnerHandle, &row.Name, &row.Visibility, &row.DefaultBranch)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	return row, err
}

func relativeTime(ts string) string {
	t, err := time.Parse("2006-01-02 15:04:05", ts)
	if err != nil {
		if t, err = time.Parse(time.RFC3339, ts); err != nil {
			return ts
		}
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return strconv.Itoa(int(d.Minutes())) + " min ago"
	case d < 24*time.Hour:
		return strconv.Itoa(int(d.Hours())) + " h ago"
	case d < 48*time.Hour:
		return "yesterday"
	case d < 7*24*time.Hour:
		return strconv.Itoa(int(d.Hours()/24)) + " d ago"
	default:
		return strconv.Itoa(int(d.Hours()/(24*7))) + " w ago"
	}
}
