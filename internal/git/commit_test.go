package git

import (
	"os"
	"os/exec"
	"testing"
)

// setupCommitRepo builds a bare repo (id 1) with two commits and returns the
// store + both shas (root first, then second).
func setupCommitRepo(t *testing.T) (*Store, string, string) {
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
	writeFile(t, work, "a.txt", "one\n")
	run("add", "-A")
	run("commit", "-q", "-m", "root")
	writeFile(t, work, "a.txt", "one\ntwo\n")
	writeFile(t, work, "b.txt", "new file\n")
	run("add", "-A")
	run("commit", "-q", "-m", "second")
	run("push", "-q", st.Path(1), "main")

	revAt := func(n string) string {
		cmd := exec.Command("git", "-C", st.Path(1), "rev-parse", n)
		out, err := cmd.Output()
		if err != nil {
			t.Fatalf("rev-parse %s: %v", n, err)
		}
		return string(out[:40])
	}
	return st, revAt("main~1"), revAt("main")
}

func TestCommitDetailSecondCommit(t *testing.T) {
	st, _, second := setupCommitRepo(t)
	meta, files, err := st.CommitDetail(1, second)
	if err != nil {
		t.Fatalf("commit detail: %v", err)
	}
	if meta.Message != "second" || meta.SHA != second {
		t.Errorf("meta wrong: %+v", meta)
	}
	// Second commit modifies a.txt and adds b.txt → 2 files.
	if len(files) != 2 {
		t.Fatalf("expected 2 files in diff, got %d: %+v", len(files), files)
	}
}

func TestCommitDetailRootCommit(t *testing.T) {
	st, root, _ := setupCommitRepo(t)
	// Root commit has no parent — must diff against the empty tree, not error.
	meta, files, err := st.CommitDetail(1, root)
	if err != nil {
		t.Fatalf("root commit detail: %v", err)
	}
	if meta.Message != "root" {
		t.Errorf("root meta wrong: %+v", meta)
	}
	if len(files) != 1 || files[0].Path != "a.txt" {
		t.Fatalf("root diff should add a.txt, got %+v", files)
	}
}

func TestCommitDetailBogusRef(t *testing.T) {
	st, _, _ := setupCommitRepo(t)
	if _, _, err := st.CommitDetail(1, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Error("expected error for nonexistent commit")
	}
}
