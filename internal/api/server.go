// Package api wires the HTTP surface for Orchis Foundry.
package api

import (
	"database/sql"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/orchis-ai/foundry/internal/auth"
	"github.com/orchis-ai/foundry/internal/config"
	gitstore "github.com/orchis-ai/foundry/internal/git"
	"github.com/orchis-ai/foundry/internal/oidc"
	"github.com/orchis-ai/foundry/internal/realtime"
	"github.com/orchis-ai/foundry/internal/scan"
	"github.com/orchis-ai/foundry/internal/webhook"
)

// Server holds shared dependencies for the HTTP handlers.
type Server struct {
	cfg      *config.Config
	log      *slog.Logger
	web      fs.FS // the frontend asset tree (index.html at root)
	db       *sql.DB
	sessions *auth.Manager
	github   *oidc.GitHub
	generic  map[string]*oidc.Generic // generic OIDC providers (Entra/Google/…) by id
	git      *gitstore.Store
	scan     *scan.Worker
	webhooks *webhook.Worker
	rt       *realtime.Hub
}

// New constructs a Server. webFS is the embedded (or on-disk) frontend tree.
func New(cfg *config.Config, log *slog.Logger, webFS fs.FS, db *sql.DB, sessions *auth.Manager, github *oidc.GitHub, git *gitstore.Store, scanWorker *scan.Worker, webhookWorker *webhook.Worker, hub *realtime.Hub) *Server {
	s := &Server{cfg: cfg, log: log, web: webFS, db: db, sessions: sessions, github: github, git: git, scan: scanWorker, webhooks: webhookWorker, rt: hub}
	// Build generic OIDC providers (anything configured with an issuer that
	// isn't the bespoke GitHub OAuth2 path). They self-discover lazily.
	s.generic = map[string]*oidc.Generic{}
	for _, p := range cfg.OIDC {
		if p.ID == "" || p.ID == "github" || p.Issuer == "" || p.ClientID == "" {
			continue
		}
		s.generic[p.ID] = oidc.NewGeneric(p.ID, p.Issuer, p.ClientID, p.ClientSecret,
			cfg.ExternalURL+"/v1/auth/oidc/callback", p.Scopes)
	}
	return s
}

// Router builds the chi router with all routes mounted.
func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(s.requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Get("/healthz", s.handleHealthz)

	// Internal hook callback (guarded by shared secret).
	r.Post("/internal/hooks/push", s.handlePushHook)

	// LLM-agent discovery (public).
	r.Get("/llms.txt", s.handleLLMsTxt)

	// Smart HTTP git endpoints (clone/push). Registered at root so clone URLs
	// look like https://host/<owner>/<repo>.git
	s.registerGitHTTP(r)

	// /v1 API
	r.Route("/v1", func(r chi.Router) {
		r.Use(s.authClaims) // resolves session cookie or Bearer PAT into context

		r.Get("/ping", func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"pong": "ok"})
		})
		r.Get("/openapi.json", s.handleOpenAPI)

		// Auth & session
		r.Get("/auth/methods", s.handleAuthMethods)
		r.Post("/auth/login", s.handleLogin)
		r.Post("/auth/signup", s.handleSignup)
		r.Get("/auth/oidc/callback", s.handleOIDCCallback)
		r.Get("/auth/oidc/{provider}", s.handleOIDCStart)
		r.Post("/auth/logout", s.handleLogout)
		r.Get("/me", s.handleMe)
		r.Patch("/me", s.requireUser(s.handlePatchMe))
		r.Post("/me/avatar", s.requireUser(s.handleUploadAvatar))
		r.Delete("/me/avatar", s.requireUser(s.handleDeleteAvatar))
		r.Get("/me/preferences", s.requireUser(s.handleGetPreferences))
		r.Patch("/me/preferences", s.requireUser(s.handlePatchPreferences))
		r.Get("/me/sessions", s.requireUser(s.handleListSessions))
		r.Delete("/me/sessions/{id}", s.requireUser(s.handleRevokeSession))
		r.Post("/me/sessions/revoke-others", s.requireUser(s.handleRevokeOtherSessions))

		// Personal access tokens
		r.Get("/me/tokens", s.requireUser(s.handleListTokens))
		r.Post("/me/tokens", s.requireUser(s.handleCreateToken))
		r.Post("/me/tokens/{id}/rotate", s.requireUser(s.handleRotateToken))
		r.Delete("/me/tokens/{id}", s.requireUser(s.handleDeleteToken))

		// SSH keys
		r.Get("/me/ssh-keys", s.requireUser(s.handleListSSHKeys))
		r.Post("/me/ssh-keys", s.requireUser(s.handleAddSSHKey))
		r.Delete("/me/ssh-keys/{id}", s.requireUser(s.handleDeleteSSHKey))

		// Per-user scanner settings
		r.Get("/me/scanner", s.requireUser(s.handleGetScanner))
		r.Put("/me/scanner", s.requireUser(s.handlePutScanner))

		// Webhooks (flat index + per-repo CRUD)
		r.Get("/me/webhooks", s.requireUser(s.handleListMyWebhooks))
		r.Get("/repos/{org}/{name}/webhooks", s.requireScope("repo:admin", s.handleListRepoWebhooks))
		r.Post("/repos/{org}/{name}/webhooks", s.requireScope("repo:admin", s.handleCreateWebhook))
		r.Patch("/repos/{org}/{name}/webhooks/{id}", s.requireScope("repo:admin", s.handleUpdateWebhook))
		r.Delete("/repos/{org}/{name}/webhooks/{id}", s.requireScope("repo:admin", s.handleDeleteWebhook))
		r.Post("/repos/{org}/{name}/webhooks/{id}/test", s.requireScope("repo:admin", s.handleTestWebhook))

		// Instance administration (auth config + user lifecycle)
		r.Get("/admin/settings", s.requireInstanceAdmin(s.handleAdminGetSettings))
		r.Put("/admin/settings", s.requireInstanceAdmin(s.handleAdminPutSettings))
		r.Get("/admin/users", s.requireInstanceAdmin(s.handleAdminListUsers))
		r.Post("/admin/users", s.requireInstanceAdmin(s.handleAdminCreateUser))
		r.Patch("/admin/users/{id}", s.requireInstanceAdmin(s.handleAdminUpdateUser))
		r.Delete("/admin/users/{id}", s.requireInstanceAdmin(s.handleAdminDeleteUser))

		// Dashboard
		r.Get("/me/activity", s.requireUser(s.handleActivity))
		r.Get("/me/inbox", s.requireUser(s.handleInbox))

		// Public user profiles
		r.Get("/users/{handle}", s.handleUserProfile)
		r.Get("/users/{handle}/avatar", s.handleServeAvatar)

		// Search (command palette + code search)
		r.Get("/search/palette", s.requireUser(s.handlePaletteSearch))
		r.Get("/search/code", s.requireUser(s.handleCodeSearch))
		r.Get("/search/repos", s.requireUser(s.handleSearchRepos))
		r.Get("/search/users", s.requireUser(s.handleSearchUsers))

		// Realtime stream (SSE)
		r.Get("/stream", s.requireUser(s.handleStream))

		// Pins & stars
		r.Get("/me/pinned", s.requireUser(s.handleListPinned))
		r.Put("/me/pinned/{org}/{name}", s.requireUser(s.handlePinRepo))
		r.Delete("/me/pinned/{org}/{name}", s.requireUser(s.handleUnpinRepo))
		r.Put("/me/stars/{org}/{name}", s.requireUser(s.handleStarRepo))
		r.Delete("/me/stars/{org}/{name}", s.requireUser(s.handleUnstarRepo))

		// Repositories
		r.Get("/repos", s.requireUser(s.handleListRepos))
		r.Post("/repos", s.requireScope("repo:admin", s.handleCreateRepo))
		r.Get("/repos/{org}/{name}", s.handleGetRepo)
		r.Patch("/repos/{org}/{name}", s.requireScope("repo:admin", s.handleUpdateRepo))
		r.Delete("/repos/{org}/{name}", s.requireScope("repo:admin", s.handleDeleteRepo))
		r.Get("/repos/{org}/{name}/tags", s.handleRepoTags)
		r.Get("/repos/{org}/{name}/collaborators", s.handleListCollaborators)
		r.Put("/repos/{org}/{name}/collaborators/{handle}", s.requireScope("repo:admin", s.handleSetCollaborator))
		r.Delete("/repos/{org}/{name}/collaborators/{handle}", s.requireScope("repo:admin", s.handleRemoveCollaborator))
		r.Get("/repos/{org}/{name}/releases", s.handleListReleases)
		r.Post("/repos/{org}/{name}/releases", s.requireScope("repo:write", s.handleCreateRelease))
		r.Delete("/repos/{org}/{name}/releases/{tag}", s.requireScope("repo:write", s.handleDeleteRelease))
		r.Get("/repos/{org}/{name}/releases/{tag}/tarball", s.handleReleaseTarball)
		r.Get("/repos/{org}/{name}/tree", s.handleRepoTree)
		r.Get("/repos/{org}/{name}/blob", s.handleRepoBlob)
		r.Get("/repos/{org}/{name}/raw", s.handleRepoRaw)
		r.Get("/repos/{org}/{name}/readme", s.handleRepoReadme)
		r.Get("/repos/{org}/{name}/branches", s.handleRepoBranches)
		r.Post("/repos/{org}/{name}/branches", s.requireScope("repo:write", s.handleCreateBranch))
		r.Delete("/repos/{org}/{name}/branches/*", s.requireScope("repo:write", s.handleDeleteBranch))
		r.Get("/repos/{org}/{name}/commits", s.handleRepoCommits)
		r.Post("/repos/{org}/{name}/commits", s.requireScope("repo:write", s.handleBrowserCommit))
		r.Post("/repos/{org}/{name}/commits/draft-message", s.requireUser(s.handleDraftCommitMessage))
		r.Get("/repos/{org}/{name}/commits/{sha}", s.handleRepoCommitDetail)

		// AI chat sidebar (per-user, toggleable)
		r.Post("/repos/{org}/{name}/chat", s.requireUser(s.handleChat))
		r.Get("/repos/{org}/{name}/chats", s.requireUser(s.handleListChats))
		r.Get("/repos/{org}/{name}/chats/{id}", s.requireUser(s.handleGetChat))

		// AI change proposals (human-in-the-loop gate)
		r.Get("/repos/{org}/{name}/proposals", s.handleListProposals)
		r.Post("/repos/{org}/{name}/proposals", s.requireScope("repo:write", s.handleCreateProposal))
		r.Get("/repos/{org}/{name}/proposals/{id}", s.handleGetProposal)
		r.Post("/repos/{org}/{name}/proposals/{id}/accept", s.requireScope("repo:write", s.handleAcceptProposal))
		r.Post("/repos/{org}/{name}/proposals/{id}/reject", s.requireScope("repo:write", s.handleRejectProposal))
		r.Get("/repos/{org}/{name}/compare", s.handleCompare)
		r.Get("/repos/{org}/{name}/checks", s.handleRepoChecks)
		r.Get("/repos/{org}/{name}/activity", s.handleRepoActivity)
		r.Get("/repos/{org}/{name}/blame", s.handleRepoBlame)
		r.Get("/repos/{org}/{name}/outline", s.handleRepoOutline)
		r.Get("/repos/{org}/{name}/pack", s.handleRepoPack)

		// Issues
		r.Get("/repos/{org}/{name}/issues", s.handleListIssues)
		r.Post("/repos/{org}/{name}/issues", s.requireScope("repo:write", s.handleCreateIssue))
		r.Get("/repos/{org}/{name}/issues/{num}", s.handleGetIssue)
		r.Patch("/repos/{org}/{name}/issues/{num}", s.requireScope("repo:write", s.handleUpdateIssue))
		r.Get("/repos/{org}/{name}/issues/{num}/comments", s.handleListIssueComments)
		r.Post("/repos/{org}/{name}/issues/{num}/comments", s.requireScope("repo:write", s.handleCreateIssueComment))

		// Pull requests (read + create)
		r.Get("/pulls", s.requireUser(s.handleListPullsGlobal))
		r.Get("/repos/{org}/{name}/pulls", s.handleListRepoPulls)
		r.Post("/repos/{org}/{name}/pulls", s.requireScope("repo:write", s.handleCreatePull))
		r.Get("/repos/{org}/{name}/pulls/{num}", s.handleGetPull)
		r.Patch("/repos/{org}/{name}/pulls/{num}", s.requireScope("repo:write", s.handleUpdatePull))
		r.Get("/repos/{org}/{name}/pulls/{num}/pack", s.handlePRPack)
		r.Post("/repos/{org}/{name}/pulls/{num}/summarize", s.requireUser(s.handleSummarizePR))
		r.Post("/repos/{org}/{name}/pulls/{num}/explain", s.requireUser(s.handleExplainPR))
		r.Post("/repos/{org}/{name}/pulls/{num}/draft-description", s.requireUser(s.handleDraftDescription))
		r.Get("/repos/{org}/{name}/pulls/{num}/files", s.handlePullFiles)
		r.Get("/repos/{org}/{name}/pulls/{num}/commits", s.handlePullCommits)
		r.Get("/repos/{org}/{name}/pulls/{num}/comments", s.handlePullComments)
		r.Get("/repos/{org}/{name}/pulls/{num}/checks", s.handlePullChecks)
		r.Post("/repos/{org}/{name}/pulls/{num}/scan", s.requireScope("repo:write", s.handleRescan))
		r.Post("/repos/{org}/{name}/pulls/{num}/comments", s.requireScope("repo:write", s.handleCreateComment))
		r.Post("/repos/{org}/{name}/pulls/{num}/reviews", s.requireScope("repo:write", s.handleSubmitReview))
		r.Post("/repos/{org}/{name}/pulls/{num}/merge", s.requireScope("repo:write", s.handleMerge))
		r.Post("/repos/{org}/{name}/pulls/{num}/request-review", s.requireScope("repo:write", s.handleRequestReview))
		r.Delete("/repos/{org}/{name}/pulls/{num}/request-review/{handle}", s.requireScope("repo:write", s.handleRemoveReviewer))
	})

	// Everything else serves the frontend (SPA-style: unknown paths → index.html).
	r.Handle("/*", s.spaHandler())
	return r
}

func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

// spaHandler serves static files from the frontend tree, falling back to
// index.html for unknown non-asset paths so client routing works.
func (s *Server) spaHandler() http.HandlerFunc {
	fileServer := http.FileServer(http.FS(s.web))
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.web, p); err != nil {
			// Not a real file — serve index.html (SPA fallback), but never
			// for obvious asset requests (avoid masking 404s on missing JS).
			if !looksLikeAsset(p) {
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/index.html"
				fileServer.ServeHTTP(w, r2)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}
}

func looksLikeAsset(p string) bool {
	for _, ext := range []string{".js", ".jsx", ".css", ".map", ".png", ".svg", ".ico", ".woff", ".woff2"} {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		s.log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"dur_ms", time.Since(start).Milliseconds(),
			"req_id", middleware.GetReqID(r.Context()),
		)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		// The frontend is precompiled (self-hosted React + a single bundle), so
		// scripts are 'self' only — no remote origins, no inline/eval. Inline
		// styles remain allowed: the app + index.html use style attributes and a
		// <style> block, and Google Fonts is loaded via <link>.
		h.Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data: https:; "+ // https: lets externally-hosted avatars (e.g. GitHub) render; uploaded avatars are served from 'self'
				"connect-src 'self'; "+
				"frame-ancestors 'none'; base-uri 'self'")
		next.ServeHTTP(w, r)
	})
}
