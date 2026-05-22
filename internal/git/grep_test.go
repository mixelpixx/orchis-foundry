package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// setupGrepRepo builds a bare repo (id 1) under a temp Store with known content
// and returns the store + resolved HEAD sha. Skips if git isn't available.
func setupGrepRepo(t *testing.T) (*Store, string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	st := NewStore(root)
	if err := st.InitBare(1, "main"); err != nil {
		t.Fatalf("init bare: %v", err)
	}
	work := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = work
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.io",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.io")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	run("init", "-q")
	run("checkout", "-q", "-B", "main")
	writeFile(t, work, "main.go", "package main\n\nfunc main() {\n\tprintln(\"hello needle world\")\n}\n")
	writeFile(t, work, "router/sink.go", "package router\n\n// needle lives here too\nvar X = 1\n")
	writeFile(t, work, "README.md", "# title\n\nno match here\n")
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	run("push", "-q", st.Path(1), "main")

	sha, err := st.RevParse(1, "main")
	if err != nil || len(sha) < 7 {
		t.Fatalf("rev-parse: %v (%q)", err, sha)
	}
	return st, sha
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGrepFindsMatches(t *testing.T) {
	st, sha := setupGrepRepo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	hits, err := st.Grep(ctx, 1, sha, "needle", nil, 20)
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("want 2 hits, got %d: %+v", len(hits), hits)
	}
	// Paths + line numbers must parse correctly (path contains a slash).
	byPath := map[string]GrepHit{}
	for _, h := range hits {
		byPath[h.Path] = h
	}
	if h, ok := byPath["main.go"]; !ok || h.Line != 4 {
		t.Errorf("main.go hit wrong: %+v", h)
	}
	if h, ok := byPath["router/sink.go"]; !ok || h.Line != 3 {
		t.Errorf("router/sink.go hit wrong: %+v", h)
	}
}

func TestGrepLangPathspec(t *testing.T) {
	st, sha := setupGrepRepo(t)
	ctx := context.Background()
	// Restrict to *.md — "needle" only lives in .go files, so zero hits.
	hits, _ := st.Grep(ctx, 1, sha, "needle", []string{"*.md"}, 20)
	if len(hits) != 0 {
		t.Fatalf("expected 0 md hits, got %d: %+v", len(hits), hits)
	}
	// "title" lives in README.md.
	hits, _ = st.Grep(ctx, 1, sha, "title", []string{"*.md"}, 20)
	if len(hits) != 1 || hits[0].Path != "README.md" {
		t.Fatalf("expected README.md title hit, got %+v", hits)
	}
}

// TestGrepAdversarialInputs ensures hostile queries are treated as literal
// search text (or no match) — never as flags, shell, or commands.
func TestGrepAdversarialInputs(t *testing.T) {
	st, sha := setupGrepRepo(t)
	ctx := context.Background()

	cases := []string{
		"-n",                       // looks like a flag
		"--max-count=999",          // looks like a flag
		"; rm -rf /",               // shell metachars
		"$(whoami)",                // command substitution
		"`id`",                     // backticks
		"needle && echo pwned",     // chaining
		"../../../../etc/passwd",   // path traversal text
	}
	for _, q := range cases {
		hits, err := st.Grep(ctx, 1, sha, q, nil, 20)
		if err != nil {
			t.Errorf("query %q errored (should be literal/no-match): %v", q, err)
		}
		// None of these literals exist in the fixture, so expect zero hits and
		// — critically — no panic/side effect.
		if len(hits) != 0 {
			t.Errorf("query %q unexpectedly matched: %+v", q, hits)
		}
	}
}

// TestGrepRejectsNonHexSHA is defense-in-depth: a sha that isn't a hex object
// id (e.g. an attempt to smuggle a flag/path via the ref slot) is rejected
// before reaching git.
func TestGrepRejectsNonHexSHA(t *testing.T) {
	st, _ := setupGrepRepo(t)
	for _, bad := range []string{"--output=/tmp/x", "main", "HEAD", "-n", "../../etc", ""} {
		if _, err := st.Grep(context.Background(), 1, bad, "needle", nil, 20); err == nil {
			t.Errorf("expected rejection for non-hex sha %q", bad)
		}
	}
}
