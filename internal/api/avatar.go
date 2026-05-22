package api

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

const maxAvatarBytes = 512 * 1024

// avatarTypes are the image content-types we accept (sniffed, not trusted from
// the client).
var avatarTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true,
}

// avatarsDir is a writable directory beside the repos dir (covered by the
// systemd ReadWritePaths).
func (s *Server) avatarsDir() string {
	return filepath.Join(filepath.Dir(s.cfg.ReposDir), "avatars")
}

// POST /v1/me/avatar — multipart upload (field "avatar"). Stored on disk and
// served from a 'self' URL (CSP-friendly).
func (s *Server) handleUploadAvatar(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	r.Body = http.MaxBytesReader(w, r.Body, maxAvatarBytes+4096)
	if err := r.ParseMultipartForm(maxAvatarBytes + 4096); err != nil {
		writeError(w, http.StatusBadRequest, "image too large (max 512 KB) or invalid form")
		return
	}
	file, _, err := r.FormFile("avatar")
	if err != nil {
		writeError(w, http.StatusBadRequest, "no avatar file")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxAvatarBytes+1))
	if err != nil || len(data) == 0 {
		writeError(w, http.StatusBadRequest, "could not read image")
		return
	}
	if len(data) > maxAvatarBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "image too large (max 512 KB)")
		return
	}
	if !avatarTypes[http.DetectContentType(data)] {
		writeError(w, http.StatusBadRequest, "unsupported image type (png, jpeg, gif, webp)")
		return
	}

	dir := s.avatarsDir()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		writeError(w, http.StatusInternalServerError, "storage unavailable")
		return
	}
	if err := os.WriteFile(filepath.Join(dir, strconv.FormatInt(u.ID, 10)), data, 0o640); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save avatar")
		return
	}
	// Cache-bust so the new image shows immediately.
	url := "/v1/users/" + u.Handle + "/avatar?t=" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if err := s.sessions.UpdateAvatar(r.Context(), u.ID, url); err != nil {
		writeError(w, http.StatusInternalServerError, "could not update profile")
		return
	}
	u.AvatarURL = url
	writeJSON(w, http.StatusOK, meResponse(u))
}

// DELETE /v1/me/avatar — remove the uploaded avatar (revert to initials).
func (s *Server) handleDeleteAvatar(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	_ = os.Remove(filepath.Join(s.avatarsDir(), strconv.FormatInt(u.ID, 10)))
	_ = s.sessions.UpdateAvatar(r.Context(), u.ID, "")
	u.AvatarURL = ""
	writeJSON(w, http.StatusOK, meResponse(u))
}

// GET /v1/users/{handle}/avatar — serve a user's uploaded avatar bytes.
func (s *Server) handleServeAvatar(w http.ResponseWriter, r *http.Request) {
	handle := chi.URLParam(r, "handle")
	var id int64
	if s.db.QueryRowContext(r.Context(), `SELECT id FROM users WHERE handle = ?`, handle).Scan(&id) != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(filepath.Join(s.avatarsDir(), strconv.FormatInt(id, 10)))
	if err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", http.DetectContentType(data))
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, _ = w.Write(data)
}
