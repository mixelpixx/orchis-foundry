package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// releaseJSON shapes a release row for the frontend, enriched with the tag's
// current target sha.
func (s *Server) releaseJSON(r *http.Request, repoID int64, id int64) map[string]any {
	var (
		tag, name, body, created string
		authorID                 sql.NullInt64
		prerelease               int
	)
	err := s.db.QueryRowContext(r.Context(),
		`SELECT tag, name, body, author_id, prerelease, created_at FROM releases WHERE id = ?`, id).
		Scan(&tag, &name, &body, &authorID, &prerelease, &created)
	if err != nil {
		return nil
	}
	sha, _ := s.git.RevParse(repoID, "refs/tags/"+tag)
	short := sha
	if len(short) > 7 {
		short = short[:7]
	}
	var author any
	if authorID.Valid {
		author = s.userBrief(authorID.Int64)
	}
	if name == "" {
		name = tag
	}
	return map[string]any{
		"tag": tag, "name": name, "body": body, "author": author,
		"prerelease": prerelease == 1, "created": relativeTime(created),
		"sha": short, "tarball": "/v1/repos/" + chi.URLParam(r, "org") + "/" + chi.URLParam(r, "name") + "/releases/" + tag + "/tarball",
	}
}

// GET /v1/repos/{org}/{name}/releases
func (s *Server) handleListReleases(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	rows, _ := s.db.QueryContext(r.Context(), `SELECT id FROM releases WHERE repo_id = ? ORDER BY id DESC`, row.ID)
	var ids []int64
	if rows != nil {
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
	}
	out := []map[string]any{}
	for _, id := range ids {
		if rj := s.releaseJSON(r, row.ID, id); rj != nil {
			out = append(out, rj)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// POST /v1/repos/{org}/{name}/releases { tag, name?, body?, target?, prerelease? }
func (s *Server) handleCreateRelease(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	u := userFrom(r)
	if !s.requireWrite(w, row) {
		return
	}
	var in struct {
		Tag        string `json:"tag"`
		Name       string `json:"name"`
		Body       string `json:"body"`
		Target     string `json:"target"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || strings.TrimSpace(in.Tag) == "" {
		writeError(w, http.StatusBadRequest, "tag required")
		return
	}
	in.Tag = strings.TrimSpace(in.Tag)

	// Create the git tag unless it already exists (then we just attach metadata).
	if !s.git.TagExists(row.ID, in.Tag) {
		target := in.Target
		if target == "" {
			target = row.DefaultBranch
		}
		if _, err := s.git.CreateTag(row.ID, in.Tag, target); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	pre := 0
	if in.Prerelease {
		pre = 1
	}
	res, err := s.db.ExecContext(r.Context(),
		`INSERT INTO releases (repo_id, tag, name, body, author_id, prerelease) VALUES (?,?,?,?,?,?)`,
		row.ID, in.Tag, in.Name, in.Body, u.ID, pre)
	if err != nil {
		writeError(w, http.StatusConflict, "a release for that tag already exists")
		return
	}
	s.logActivity(r.Context(), u.ID, "release_published", row.ID, row.OwnerHandle+"/"+row.Name, in.Tag)
	if s.webhooks != nil {
		s.webhooks.Fire(r.Context(), row.ID, "release", map[string]any{
			"event": "release", "action": "published", "repo": row.OwnerHandle + "/" + row.Name, "tag": in.Tag,
		})
	}
	id, _ := res.LastInsertId()
	writeJSON(w, http.StatusOK, s.releaseJSON(r, row.ID, id))
}

// DELETE /v1/repos/{org}/{name}/releases/{tag}  (removes release metadata + the git tag)
func (s *Server) handleDeleteRelease(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	if !s.requireWrite(w, row) {
		return
	}
	tag := chi.URLParam(r, "tag")
	s.db.ExecContext(r.Context(), `DELETE FROM releases WHERE repo_id = ? AND tag = ?`, row.ID, tag)
	_ = s.git.DeleteTag(row.ID, tag)
	w.WriteHeader(http.StatusNoContent)
}

// GET /v1/repos/{org}/{name}/releases/{tag}/tarball — gzipped source archive.
func (s *Server) handleReleaseTarball(w http.ResponseWriter, r *http.Request) {
	row, ok := s.loadRepo(r)
	if !ok {
		writeError(w, http.StatusNotFound, "repo not found")
		return
	}
	tag := chi.URLParam(r, "tag")
	if !s.git.TagExists(row.ID, tag) {
		writeError(w, http.StatusNotFound, "tag not found")
		return
	}
	data, err := s.git.Archive(row.ID, "refs/tags/"+tag)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not build archive")
		return
	}
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+row.Name+"-"+sanitizeFilename(tag)+".tar.gz\"")
	_, _ = w.Write(data)
}

// sanitizeFilename keeps a tag safe inside a Content-Disposition filename.
func sanitizeFilename(s string) string {
	return strings.NewReplacer("/", "-", "\"", "", "\\", "", "\n", "", "\r", "").Replace(s)
}
