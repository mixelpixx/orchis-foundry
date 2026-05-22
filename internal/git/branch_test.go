package git

import (
	"testing"
)

func TestValidRefName(t *testing.T) {
	good := []string{"main", "feature/x", "release-1.2", "a_b.c", "dev/jana/fix"}
	for _, n := range good {
		if !ValidRefName(n) {
			t.Errorf("expected %q valid", n)
		}
	}
	// Hostile / malformed names must be rejected (defense-in-depth so they
	// never reach git as flags or escape ref parsing).
	bad := []string{
		"", "-D", "--force", "-rf", // flag-like
		"foo..bar",   // range
		"a b",        // space
		"feat;rm -rf", "$(x)", "`id`", // shell metachars
		"trailing/", "x.lock", "/abs",
		"héllo", // non-ascii outside allowlist
	}
	for _, n := range bad {
		if ValidRefName(n) {
			t.Errorf("expected %q rejected", n)
		}
	}
}

func TestCreateAndDeleteBranch(t *testing.T) {
	st, _ := setupGrepRepo(t) // bare repo id=1 on main with content

	if err := st.CreateBranch(1, "feature/test", ""); err != nil {
		t.Fatalf("create from HEAD: %v", err)
	}
	if !st.BranchExists(1, "feature/test") {
		t.Fatal("branch should exist")
	}
	// Duplicate rejected.
	if err := st.CreateBranch(1, "feature/test", ""); err == nil {
		t.Error("expected duplicate-branch error")
	}
	// Hostile name rejected before reaching git.
	if err := st.CreateBranch(1, "--force", ""); err == nil {
		t.Error("expected invalid-name error for --force")
	}
	// Bad start point rejected.
	if err := st.CreateBranch(1, "from-bogus", "no-such-ref"); err == nil {
		t.Error("expected start-point error")
	}
	// Delete works.
	if err := st.DeleteBranch(1, "feature/test"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if st.BranchExists(1, "feature/test") {
		t.Fatal("branch should be gone")
	}
}
