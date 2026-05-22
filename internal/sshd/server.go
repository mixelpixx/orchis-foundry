// Package sshd is the embedded SSH server that speaks the git wire protocol.
// Auth is by registered public key (fingerprint → user). Push/fetch shell out
// to git-upload-pack / git-receive-pack against the repo's bare dir.
package sshd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"

	glssh "github.com/gliderlabs/ssh"
	"golang.org/x/crypto/ssh"

	"github.com/orchis-ai/foundry/internal/auth"
	gitstore "github.com/orchis-ai/foundry/internal/git"
)

type Server struct {
	addr     string
	hostKey  string
	log      *slog.Logger
	db       *sql.DB
	sessions *auth.Manager
	git      *gitstore.Store
}

func New(addr, hostKeyPath string, log *slog.Logger, db *sql.DB, sessions *auth.Manager, git *gitstore.Store) *Server {
	return &Server{addr: addr, hostKey: hostKeyPath, log: log, db: db, sessions: sessions, git: git}
}

// ListenAndServe blocks serving SSH. Run in a goroutine.
func (s *Server) ListenAndServe() error {
	if err := s.ensureHostKey(); err != nil {
		return fmt.Errorf("host key: %w", err)
	}
	signer, err := loadSigner(s.hostKey)
	if err != nil {
		return err
	}

	srv := &glssh.Server{
		Addr: s.addr,
		// Accept the connection if the key maps to a known user; stash the user.
		PublicKeyHandler: func(ctx glssh.Context, key glssh.PublicKey) bool {
			u, err := s.sessions.UserBySSHKey(ctx, key)
			if err != nil || u == nil {
				return false
			}
			ctx.SetValue("user", u)
			return true
		},
		Handler: s.handleSession,
	}
	srv.AddHostKey(signer)
	s.log.Info("ssh listening", "addr", s.addr)
	return srv.ListenAndServe()
}

func (s *Server) handleSession(sess glssh.Session) {
	u, _ := sess.Context().Value("user").(*auth.User)
	if u == nil {
		io.WriteString(sess.Stderr(), "orchis: not authenticated\n")
		sess.Exit(1)
		return
	}

	cmd := sess.Command() // already shlex-split by gliderlabs/ssh
	if len(cmd) < 2 {
		io.WriteString(sess.Stderr(), "orchis: this server only serves git. Use git clone/push.\n")
		sess.Exit(1)
		return
	}
	service := cmd[0] // git-upload-pack | git-receive-pack
	if service != "git-upload-pack" && service != "git-receive-pack" {
		io.WriteString(sess.Stderr(), "orchis: unsupported command\n")
		sess.Exit(1)
		return
	}

	owner, name, ok := parseRepoArg(cmd[1])
	if !ok {
		io.WriteString(sess.Stderr(), "orchis: bad repo path\n")
		sess.Exit(1)
		return
	}

	repoID, vis, ownerID, err := s.resolveRepo(sess.Context(), owner, name)
	if err != nil {
		io.WriteString(sess.Stderr(), "orchis: repository not found\n")
		sess.Exit(1)
		return
	}

	// Determine the user's role: owner, or a collaborator role (read/write/admin).
	role := ""
	if ownerID == u.ID {
		role = "owner"
	} else {
		var cr string
		if s.db.QueryRowContext(sess.Context(),
			`SELECT role FROM repo_collaborators WHERE repo_id = ? AND user_id = ?`, repoID, u.ID).Scan(&cr) == nil {
			role = cr
		}
	}

	write := service == "git-receive-pack"
	if write {
		// Push requires owner or an admin/write collaborator.
		if role != "owner" && role != "admin" && role != "write" {
			io.WriteString(sess.Stderr(), "orchis: you do not have push access to this repository\n")
			sess.Exit(1)
			return
		}
	} else {
		// read: public repos OK for any authed user; otherwise owner/collaborator.
		if vis != "public" && role == "" {
			io.WriteString(sess.Stderr(), "orchis: you do not have read access to this repository\n")
			sess.Exit(1)
			return
		}
	}

	dir := s.git.Path(repoID)
	if _, statErr := os.Stat(dir); statErr != nil {
		io.WriteString(sess.Stderr(), "orchis: repository storage missing\n")
		sess.Exit(1)
		return
	}

	c := exec.CommandContext(sess.Context(), service, dir)
	c.Stdin = sess
	c.Stdout = sess
	c.Stderr = sess.Stderr()
	if err := c.Run(); err != nil {
		s.log.Error("git ssh service failed", "service", service, "err", err)
		sess.Exit(1)
		return
	}
	sess.Exit(0)
}

// resolveRepo returns (repoID, visibility, ownerUserID).
func (s *Server) resolveRepo(ctx context.Context, owner, name string) (int64, string, int64, error) {
	var repoID, ownerID int64
	var vis string
	err := s.db.QueryRowContext(ctx,
		`SELECT rp.id, rp.visibility, COALESCE(rp.owner_user_id,0)
		 FROM repos rp JOIN users u ON u.id = rp.owner_user_id
		 WHERE u.handle = ? AND rp.name = ?`, owner, name).Scan(&repoID, &vis, &ownerID)
	return repoID, vis, ownerID, err
}

// parseRepoArg turns "'owner/name.git'" / "/owner/name.git" into (owner, name).
func parseRepoArg(arg string) (string, string, bool) {
	arg = strings.Trim(arg, "'\"")
	arg = strings.TrimPrefix(arg, "/")
	arg = strings.TrimSuffix(arg, ".git")
	parts := strings.Split(arg, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// ensureHostKey generates an ed25519 host key if none exists.
func (s *Server) ensureHostKey() error {
	if _, err := os.Stat(s.hostKey); err == nil {
		return nil
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	pemBlock, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		return err
	}
	return os.WriteFile(s.hostKey, pem.EncodeToMemory(pemBlock), 0o600)
}

func loadSigner(path string) (ssh.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(b)
}
