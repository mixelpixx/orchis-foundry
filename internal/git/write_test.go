package git

import (
	"testing"
)

func TestCommitFilesNewBranch(t *testing.T) {
	st, headSHA := setupGrepRepo(t) // bare repo id=1 on main

	// Add a new file + modify an existing one on a fresh branch off HEAD.
	changes := []FileChange{
		{Path: "docs/NOTES.md", Content: []byte("# Notes\nhello\n")},
		{Path: "main.go", Content: []byte("package main\n\nfunc main() {}\n")},
	}
	newSHA, oldSHA, err := st.CommitFiles(1, "ai/edit", headSHA, "feat: add notes", "Alice", "alice@x.io", changes)
	if err != nil {
		t.Fatalf("CommitFiles: %v", err)
	}
	if oldSHA != "" {
		t.Errorf("new branch should have empty oldSHA, got %q", oldSHA)
	}
	if len(newSHA) < 7 {
		t.Fatalf("bad newSHA %q", newSHA)
	}
	if !st.BranchExists(1, "ai/edit") {
		t.Fatal("branch ai/edit should exist")
	}
	// New file is present on the new branch with expected content.
	blob, err := st.FileBlob(1, "ai/edit", "docs/NOTES.md")
	if err != nil {
		t.Fatalf("read new file: %v", err)
	}
	if blob.Content != "# Notes\nhello\n" {
		t.Errorf("unexpected content: %q", blob.Content)
	}
	// main.go was modified on the branch.
	mb, _ := st.FileBlob(1, "ai/edit", "main.go")
	if mb == nil || mb.Content != "package main\n\nfunc main() {}\n" {
		t.Errorf("main.go not updated on branch")
	}
	// The default branch is untouched.
	mainBlob, _ := st.FileBlob(1, "main", "main.go")
	if mainBlob == nil || mainBlob.Content == "package main\n\nfunc main() {}\n" {
		t.Error("main branch should be unchanged")
	}
}

func TestCommitFilesSecondCommitAndDelete(t *testing.T) {
	st, headSHA := setupGrepRepo(t)

	// First commit on a new branch.
	first, _, err := st.CommitFiles(1, "wip", headSHA, "add a", "T", "t@t.io",
		[]FileChange{{Path: "a.txt", Content: []byte("A\n")}})
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	// Second commit on the same branch: its parent must be the first commit
	// (CAS via oldSHA), and it deletes README.md inherited from HEAD.
	second, oldSHA, err := st.CommitFiles(1, "wip", "", "rm readme", "T", "t@t.io",
		[]FileChange{{Path: "README.md", Delete: true}})
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if oldSHA != first {
		t.Errorf("expected oldSHA=%s, got %s", first, oldSHA)
	}
	if second == first {
		t.Error("second commit should differ from first")
	}
	if _, err := st.FileBlob(1, "wip", "README.md"); err == nil {
		t.Error("README.md should be deleted on wip branch")
	}
	if _, err := st.FileBlob(1, "wip", "a.txt"); err != nil {
		t.Error("a.txt should still exist on wip branch")
	}
}

func TestCommitFilesRejectsBadInput(t *testing.T) {
	st, headSHA := setupGrepRepo(t)
	bad := []FileChange{{Path: "../escape", Content: []byte("x")}}
	if _, _, err := st.CommitFiles(1, "b1", headSHA, "msg", "T", "t@t.io", bad); err == nil {
		t.Error("expected path-escape rejection")
	}
	if _, _, err := st.CommitFiles(1, "--force", headSHA, "msg", "T", "t@t.io",
		[]FileChange{{Path: "ok.txt", Content: []byte("x")}}); err == nil {
		t.Error("expected invalid branch rejection")
	}
	if _, _, err := st.CommitFiles(1, "b2", headSHA, "", "T", "t@t.io",
		[]FileChange{{Path: "ok.txt", Content: []byte("x")}}); err == nil {
		t.Error("expected empty-message rejection")
	}
}
