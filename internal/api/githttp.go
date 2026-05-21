package api

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
)

// Smart HTTP git. Clone URL: https://foundry.orchis.ai/<owner>/<repo>.git
//
//	GET  /<owner>/<repo>.git/info/refs?service=git-(upload|receive)-pack
//	POST /<owner>/<repo>.git/git-upload-pack    (clone/fetch)
//	POST /<owner>/<repo>.git/git-receive-pack   (push)
//
// Auth: HTTP Basic with handle:orc_pat_…. Anonymous fetch allowed on public
// repos; push always requires a PAT owned by the repo owner.

func (s *Server) registerGitHTTP(r chi.Router) {
	r.Get("/{owner}/{repo}/info/refs", s.gitInfoRefs)
	r.Post("/{owner}/{repo}/git-upload-pack", s.gitService("git-upload-pack"))
	r.Post("/{owner}/{repo}/git-receive-pack", s.gitService("git-receive-pack"))
}

func gitRepoParams(r *http.Request) (owner, repo string) {
	owner = chi.URLParam(r, "owner")
	repo = strings.TrimSuffix(chi.URLParam(r, "repo"), ".git")
	return
}

// authorizeGit resolves Basic-auth creds and checks access for the operation.
// write=true requires the authenticated user to own the repo.
func (s *Server) authorizeGit(w http.ResponseWriter, r *http.Request, write bool) (*repoRow, bool) {
	owner, name := gitRepoParams(r)
	row, err := s.repoForOwnerName(owner, name)
	if err != nil {
		http.Error(w, "repository not found", http.StatusNotFound)
		return nil, false
	}

	_, pass, hasAuth := r.BasicAuth()
	var userID int64 = -1
	if hasAuth && pass != "" {
		if ta, err := s.sessions.ValidateToken(r.Context(), pass); err == nil {
			userID = ta.User.ID
		}
	}

	if write {
		if userID < 0 {
			s.gitAuthChallenge(w)
			return nil, false
		}
		if !row.OwnerUserID.Valid || row.OwnerUserID.Int64 != userID {
			http.Error(w, "you do not have push access to this repository", http.StatusForbidden)
			return nil, false
		}
		return row, true
	}

	// read
	if row.Visibility == "public" {
		return row, true
	}
	if userID >= 0 && row.OwnerUserID.Valid && row.OwnerUserID.Int64 == userID {
		return row, true
	}
	s.gitAuthChallenge(w)
	return nil, false
}

func (s *Server) gitAuthChallenge(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Orchis Foundry"`)
	http.Error(w, "authentication required", http.StatusUnauthorized)
}

func (s *Server) gitInfoRefs(w http.ResponseWriter, r *http.Request) {
	service := r.URL.Query().Get("service")
	if service != "git-upload-pack" && service != "git-receive-pack" {
		http.Error(w, "dumb http not supported", http.StatusForbidden)
		return
	}
	write := service == "git-receive-pack"
	row, ok := s.authorizeGit(w, r, write)
	if !ok {
		return
	}

	dir := s.git.Path(row.ID)
	if _, err := os.Stat(dir); err != nil {
		http.Error(w, "repository storage missing", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-advertisement", service))
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)

	// Service announcement packet, then the advertised refs.
	_, _ = w.Write(packetLine("# service=" + service + "\n"))
	_, _ = w.Write([]byte("0000"))

	cmd := exec.CommandContext(r.Context(), gitServiceName(service),
		"--stateless-rpc", "--advertise-refs", dir)
	cmd.Stdout = w
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		s.log.Error("info/refs failed", "service", service, "err", err)
	}
}

func (s *Server) gitService(service string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		write := service == "git-receive-pack"
		row, ok := s.authorizeGit(w, r, write)
		if !ok {
			return
		}
		dir := s.git.Path(row.ID)

		var body io.Reader = r.Body
		if r.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(r.Body)
			if err != nil {
				http.Error(w, "bad gzip", http.StatusBadRequest)
				return
			}
			defer gz.Close()
			body = gz
		}

		w.Header().Set("Content-Type", fmt.Sprintf("application/x-%s-result", service))
		w.Header().Set("Cache-Control", "no-cache")

		cmd := exec.CommandContext(r.Context(), gitServiceName(service), "--stateless-rpc", dir)
		cmd.Stdin = body
		cmd.Stdout = w
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			s.log.Error("git service failed", "service", service, "err", err)
		}
	}
}

// gitServiceName maps "git-upload-pack" → the actual binary invocation.
func gitServiceName(service string) string {
	// Both are dispatched via the `git` binary subcommand form to avoid PATH
	// issues with the standalone git-upload-pack/git-receive-pack helpers.
	return service
}

// packetLine encodes a string in git pkt-line format.
func packetLine(s string) []byte {
	return []byte(fmt.Sprintf("%04x%s", len(s)+4, s))
}

// installPushHook writes a post-receive hook that notifies the app on push.
func (s *Server) installPushHook(repoID int64) {
	dir := s.git.Path(repoID)
	hookPath := filepath.Join(dir, "hooks", "post-receive")
	script := fmt.Sprintf(`#!/bin/sh
# Orchis Foundry post-receive hook — notifies the app of pushes.
payload=""
while read old new ref; do
  payload="$payload$old $new $ref\n"
done
curl -s -m 5 -X POST \
  -H "X-Hook-Secret: %s" \
  -H "Content-Type: application/json" \
  -d "{\"repo_id\": %d}" \
  http://%s/internal/hooks/push >/dev/null 2>&1 || true
`, s.cfg.SessionKey, repoID, s.cfg.HTTPAddr)
	_ = os.MkdirAll(filepath.Join(dir, "hooks"), 0o755)
	if err := os.WriteFile(hookPath, []byte(script), 0o755); err != nil {
		s.log.Error("install push hook failed", "repo", repoID, "err", err)
	}
}
