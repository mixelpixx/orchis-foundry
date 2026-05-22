package scan

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	gitstore "github.com/orchis-ai/foundry/internal/git"
)

// Worker drains the scan_jobs queue and runs scans against each repo owner's
// configured scanner. One in-process worker; bounded concurrency via the queue.
type Worker struct {
	db   *sql.DB
	git  *gitstore.Store
	log  *slog.Logger
	wake chan struct{}
}

func NewWorker(db *sql.DB, git *gitstore.Store, log *slog.Logger) *Worker {
	return &Worker{db: db, git: git, log: log, wake: make(chan struct{}, 1)}
}

// Enqueue adds a scan job for a pull at a sha and nudges the worker.
func (w *Worker) Enqueue(pullID, repoID int64, sha string) {
	// Avoid duplicate queued jobs for the same (pull, sha).
	var n int
	w.db.QueryRow(`SELECT COUNT(*) FROM scan_jobs WHERE pull_id=? AND sha=? AND status IN ('queued','running')`, pullID, sha).Scan(&n)
	if n > 0 {
		return
	}
	if _, err := w.db.Exec(`INSERT INTO scan_jobs (pull_id, repo_id, sha) VALUES (?,?,?)`, pullID, repoID, sha); err != nil {
		w.log.Error("enqueue scan failed", "err", err)
		return
	}
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// Run loops until ctx is cancelled, processing jobs as they appear.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // safety poll
	defer ticker.Stop()
	for {
		w.drain(ctx)
		select {
		case <-ctx.Done():
			return
		case <-w.wake:
		case <-ticker.C:
		}
	}
}

func (w *Worker) drain(ctx context.Context) {
	for {
		var jobID, pullID, repoID int64
		var sha string
		err := w.db.QueryRow(
			`SELECT id, pull_id, repo_id, sha FROM scan_jobs WHERE status='queued' ORDER BY id LIMIT 1`).
			Scan(&jobID, &pullID, &repoID, &sha)
		if err == sql.ErrNoRows {
			return
		}
		if err != nil {
			w.log.Error("scan poll failed", "err", err)
			return
		}
		w.process(ctx, jobID, pullID, repoID, sha)
	}
}

func (w *Worker) process(ctx context.Context, jobID, pullID, repoID int64, sha string) {
	w.db.Exec(`UPDATE scan_jobs SET status='running', started_at=datetime('now') WHERE id=?`, jobID)
	w.upsertCheck(repoID, sha, "pending", "Supply-chain scan running…")

	fail := func(msg string) {
		w.db.Exec(`UPDATE scan_jobs SET status='error', error=?, finished_at=datetime('now') WHERE id=?`, msg, jobID)
		w.upsertCheck(repoID, sha, "cancelled", "Scan could not run: "+msg)
		w.log.Warn("scan job failed", "job", jobID, "err", msg)
	}

	// Resolve the repo owner's scanner settings.
	var ownerID int64
	if err := w.db.QueryRow(`SELECT COALESCE(owner_user_id,0) FROM repos WHERE id=?`, repoID).Scan(&ownerID); err != nil || ownerID == 0 {
		fail("repo owner not found")
		return
	}
	st, ok := w.settingsFor(ownerID)
	if !ok || !st.Enabled {
		// No scanner configured: mark the job done with a neutral check.
		w.db.Exec(`UPDATE scan_jobs SET status='done', finished_at=datetime('now') WHERE id=?`, jobID)
		w.upsertCheck(repoID, sha, "cancelled", "No scanner configured by the repo owner. Configure one in Developer settings → Scanner.")
		return
	}

	// Compute the PR diff (base_sha..head_sha).
	var baseSHA, headSHA string
	w.db.QueryRow(`SELECT base_sha, head_sha FROM pulls WHERE id=?`, pullID).Scan(&baseSHA, &headSHA)
	if headSHA == "" {
		headSHA = sha
	}
	diffs, _ := w.git.Diff(repoID, baseSHA, headSHA)
	var raw string
	for _, fd := range diffs {
		raw += "diff --git a/" + fd.Path + " b/" + fd.Path + "\n"
		for _, h := range fd.Hunks {
			raw += h.Header + "\n"
			for _, ln := range h.Lines {
				p := " "
				if ln.Type == "add" {
					p = "+"
				} else if ln.Type == "del" {
					p = "-"
				}
				raw += p + ln.Text + "\n"
			}
		}
	}
	if raw == "" {
		w.db.Exec(`UPDATE scan_jobs SET status='done', finished_at=datetime('now') WHERE id=?`, jobID)
		w.upsertCheck(repoID, sha, "ok", "No changes to scan.")
		return
	}

	cctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	res, err := Run(cctx, st, raw)
	if err != nil {
		fail(err.Error())
		return
	}

	// Persist findings.
	w.db.Exec(`DELETE FROM scan_findings WHERE job_id=?`, jobID)
	for _, f := range res.Findings {
		w.db.Exec(
			`INSERT INTO scan_findings (job_id, severity, category, file, line_start, line_end, summary, suggestion)
			 VALUES (?,?,?,?,?,?,?,?)`,
			jobID, f.Severity, f.Category, f.File, nullableInt(f.LineStart), nullableInt(f.LineEnd), f.Summary, f.Suggestion)
	}

	status := map[string]string{"clean": "ok", "review": "pending", "block": "fail"}[res.Overall]
	if status == "" {
		status = "pending"
	}
	w.upsertCheck(repoID, sha, status, summarize(res))
	w.db.Exec(`UPDATE scan_jobs SET status='done', finished_at=datetime('now') WHERE id=?`, jobID)

	// Feed event when the scan flags something.
	if res.Overall != "clean" && len(res.Findings) > 0 {
		var owner, name string
		var num int
		if err := w.db.QueryRow(
			`SELECT uo.handle, rp.name, p.number FROM pulls p
			 JOIN repos rp ON rp.id = p.repo_id JOIN users uo ON uo.id = rp.owner_user_id
			 WHERE p.id = ?`, pullID).Scan(&owner, &name, &num); err == nil {
			w.db.Exec(`INSERT INTO activity (actor_id, kind, repo_id, target, title) VALUES (NULL,'scan_flagged',?,?,?)`,
				repoID, fmt.Sprintf("%s/%s#%d", owner, name, num), fmt.Sprintf("%s — %d finding(s)", res.Overall, len(res.Findings)))
		}
	}
	w.log.Info("scan complete", "job", jobID, "overall", res.Overall, "findings", len(res.Findings))
}

func summarize(res *Result) string {
	if len(res.Findings) == 0 {
		return "orchis-scan: clean — no supply-chain risks flagged."
	}
	out := fmt.Sprintf("orchis-scan: %s — %d finding(s):\n", res.Overall, len(res.Findings))
	for _, f := range res.Findings {
		loc := f.File
		if f.LineStart != nil {
			loc = fmt.Sprintf("%s:%d", f.File, *f.LineStart)
		}
		out += fmt.Sprintf("• [%s] %s (%s) — %s\n", f.Severity, loc, f.Category, f.Summary)
	}
	return out
}

// upsertCheck writes/updates the orchis-scan check row for a sha.
func (w *Worker) upsertCheck(repoID int64, sha, status, detail string) {
	w.db.Exec(
		`INSERT INTO checks (repo_id, sha, name, status, detail, finished_at)
		 VALUES (?,?, 'orchis-scan', ?, ?, datetime('now'))
		 ON CONFLICT(repo_id, sha, name) DO UPDATE SET status=excluded.status, detail=excluded.detail, finished_at=datetime('now')`,
		repoID, sha, status, detail)
}

func (w *Worker) settingsFor(userID int64) (Settings, bool) {
	var s Settings
	var enabled int
	err := w.db.QueryRow(
		`SELECT enabled, provider, base_url, model, api_key FROM scanner_settings WHERE user_id=?`, userID).
		Scan(&enabled, &s.Provider, &s.BaseURL, &s.Model, &s.APIKey)
	if err != nil {
		return Settings{}, false
	}
	s.Enabled = enabled == 1
	return s, true
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}
