// Command orchis is the Orchis Foundry server: HTTP API + frontend + (later)
// embedded SSH git server and LLM scan worker.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/orchis-ai/foundry/internal/api"
	"github.com/orchis-ai/foundry/internal/auth"
	"github.com/orchis-ai/foundry/internal/config"
	"github.com/orchis-ai/foundry/internal/db"
	gitstore "github.com/orchis-ai/foundry/internal/git"
	"github.com/orchis-ai/foundry/internal/logging"
	"github.com/orchis-ai/foundry/internal/oidc"
	"github.com/orchis-ai/foundry/internal/scan"
	"github.com/orchis-ai/foundry/internal/sshd"
	"github.com/orchis-ai/foundry/web"
)

func main() {
	var (
		configPath = flag.String("config", "", "path to config.yaml")
		showVer    = flag.Bool("version", false, "print version and exit")
		mintToken  = flag.String("mint-token", "", "create-or-find a user with this handle, mint an all-scope PAT, print it, and exit (admin/test escape hatch)")
	)
	flag.Parse()

	if *showVer {
		os.Stdout.WriteString(version + "\n")
		return
	}

	log := logging.Setup(slog.LevelInfo)

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Error("config load failed", "err", err)
		os.Exit(1)
	}

	// Frontend asset tree: on-disk override (dev) or embedded (prod).
	var webFS fs.FS
	if cfg.WebDir != "" {
		webFS = os.DirFS(cfg.WebDir)
		log.Info("serving frontend from disk", "dir", cfg.WebDir)
	} else {
		webFS = web.FS
		log.Info("serving embedded frontend")
	}

	// Database + migrations.
	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Error("db open failed", "err", err)
		os.Exit(1)
	}
	defer database.Close()
	log.Info("database ready", "path", cfg.DBPath)

	sessions := auth.NewManager(database)

	// CLI escape hatch: mint an all-scope PAT for a handle, then exit.
	if *mintToken != "" {
		ctx := context.Background()
		uid, err := sessions.EnsureUserByHandle(ctx, *mintToken)
		if err != nil {
			log.Error("mint-token: ensure user failed", "err", err)
			os.Exit(1)
		}
		created, err := sessions.CreateToken(ctx, uid, "cli-minted",
			[]string{"repo:read", "repo:write", "repo:admin", "actions:read", "packages:write", "user:read"}, 365)
		if err != nil {
			log.Error("mint-token: create failed", "err", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "user=%s token=%s\n", *mintToken, created.Token)
		return
	}

	// Ensure the repos storage directory exists.
	if err := os.MkdirAll(cfg.ReposDir, 0o750); err != nil {
		log.Error("could not create repos dir", "err", err)
		os.Exit(1)
	}
	gitStore := gitstore.NewStore(cfg.ReposDir)

	// OIDC providers.
	var github *oidc.GitHub
	for _, p := range cfg.OIDC {
		if p.ID == "github" && p.ClientID != "" {
			github = oidc.NewGitHub(p.ClientID, p.ClientSecret, cfg.ExternalURL+"/v1/auth/oidc/callback")
			log.Info("github oidc configured")
		}
	}

	// LLM supply-chain scan worker (started after the signal context exists).
	scanWorker := scan.NewWorker(database, gitStore, log)

	srv := api.New(cfg, log, webFS, database, sessions, github, gitStore, scanWorker)
	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           srv.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go scanWorker.Run(ctx)

	go func() {
		log.Info("http listening", "addr", cfg.HTTPAddr, "external_url", cfg.ExternalURL)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Error("http server error", "err", err)
			stop()
		}
	}()

	// Embedded SSH git server.
	sshServer := sshd.New(cfg.SSHAddr, cfg.SSHHostKey, log, database, sessions, gitStore)
	go func() {
		if err := sshServer.ListenAndServe(); err != nil {
			log.Error("ssh server error", "err", err)
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
}
