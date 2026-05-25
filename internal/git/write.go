package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// FileChange is one file edit applied by CommitFiles. Content is the full new
// file body; when Delete is true the path is removed and Content is ignored.
type FileChange struct {
	Path    string `json:"path"`
	Content []byte `json:"-"`
	Delete  bool   `json:"delete"`
}

// validRepoPath rejects paths that could escape the tree or confuse git.
func validRepoPath(p string) bool {
	if p == "" || strings.ContainsRune(p, 0) {
		return false
	}
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") || strings.HasSuffix(p, "/..") || p == ".." {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// gitEnv runs a git plumbing command in the bare repo with a custom index file
// and author/committer identity, optionally feeding stdin. Returns trimmed stdout.
// workTree is an empty scratch dir so index ops like `update-index --force-remove`
// pass git's "must be run in a work tree" guard; its contents are never read
// because we only use object-store / cacheinfo operations.
func (s *Store) gitEnv(repoID int64, indexFile, workTree, name, email string, stdin []byte, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", s.Path(repoID)}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_INDEX_FILE="+indexFile,
		"GIT_WORK_TREE="+workTree,
		"GIT_AUTHOR_NAME="+name, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+name, "GIT_COMMITTER_EMAIL="+email,
	)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %v: %s", strings.Join(args, " "), err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

// ApplyPatchCommit applies a unified-diff patch onto branch (off baseSHA/HEAD)
// and commits the result, returning the new commit SHA. Like CommitFiles it
// works directly in the bare repo with a temp index: read-tree the base, then
// `git apply --cached` the patch (which writes blobs + updates the index),
// then write-tree → commit-tree → update-ref. Returns an error if the patch
// does not apply cleanly so the caller can surface it to the human reviewer.
func (s *Store) ApplyPatchCommit(repoID int64, branch, baseSHA, patch, message, authorName, authorEmail string) (newSHA, oldSHA string, err error) {
	if !ValidRefName(branch) {
		return "", "", fmt.Errorf("invalid branch name")
	}
	if strings.TrimSpace(patch) == "" {
		return "", "", fmt.Errorf("empty patch")
	}
	if message == "" {
		return "", "", fmt.Errorf("empty commit message")
	}
	if authorName == "" {
		authorName = "Orchis"
	}
	if authorEmail == "" {
		authorEmail = "noreply@orchis.local"
	}

	var parent string
	if s.BranchExists(repoID, branch) {
		parent, _ = s.RevParse(repoID, "refs/heads/"+branch)
		oldSHA = parent
	} else if baseSHA != "" {
		parent, _ = s.RevParse(repoID, baseSHA)
	} else if !s.IsEmpty(repoID) {
		parent, _ = s.RevParse(repoID, "HEAD")
	}

	idx, err := os.CreateTemp("", "orchis-index-*")
	if err != nil {
		return "", "", err
	}
	idxPath := idx.Name()
	idx.Close()
	os.Remove(idxPath)
	defer os.Remove(idxPath)
	workTree, err := os.MkdirTemp("", "orchis-wt-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(workTree)

	if parent != "" {
		if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, "read-tree", parent); err != nil {
			return "", "", err
		}
	}
	// Apply to the index only. --cached writes the new blobs to the object DB
	// and stages them without needing a populated work tree.
	if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, []byte(patch),
		"apply", "--cached", "--whitespace=nowarn", "-"); err != nil {
		return "", "", fmt.Errorf("patch did not apply cleanly: %w", err)
	}

	tree, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, "write-tree")
	if err != nil {
		return "", "", err
	}
	ctArgs := []string{"commit-tree", tree}
	if parent != "" {
		ctArgs = append(ctArgs, "-p", parent)
	}
	newSHA, err = s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, []byte(message), ctArgs...)
	if err != nil {
		return "", "", err
	}
	updArgs := []string{"update-ref", "refs/heads/" + branch, newSHA}
	if oldSHA != "" {
		updArgs = append(updArgs, oldSHA)
	}
	if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, updArgs...); err != nil {
		return "", "", err
	}
	return newSHA, oldSHA, nil
}

// CommitFiles applies file changes onto branch and returns the new commit SHA.
// It writes directly into the bare repo via git plumbing (no working checkout):
// hash-object → temp index → write-tree → commit-tree → update-ref.
//
// If the branch already exists its tip is the parent; otherwise the branch is
// created starting from baseSHA (or, when baseSHA is empty and the repo has
// commits, from the repo's current HEAD; an empty repo produces a root commit).
// The caller is responsible for ACL checks and for running onRefUpdated after.
func (s *Store) CommitFiles(repoID int64, branch, baseSHA, message, authorName, authorEmail string, changes []FileChange) (newSHA, oldSHA string, err error) {
	if !ValidRefName(branch) {
		return "", "", fmt.Errorf("invalid branch name")
	}
	if len(changes) == 0 {
		return "", "", fmt.Errorf("no changes")
	}
	for _, c := range changes {
		if !validRepoPath(c.Path) {
			return "", "", fmt.Errorf("invalid path: %s", c.Path)
		}
	}
	if message == "" {
		return "", "", fmt.Errorf("empty commit message")
	}
	if authorName == "" {
		authorName = "Orchis"
	}
	if authorEmail == "" {
		authorEmail = "noreply@orchis.local"
	}

	// Resolve the parent commit. Branch tip wins; else baseSHA; else HEAD.
	var parent string
	if s.BranchExists(repoID, branch) {
		parent, _ = s.RevParse(repoID, "refs/heads/"+branch)
		oldSHA = parent
	} else if baseSHA != "" {
		parent, _ = s.RevParse(repoID, baseSHA)
	} else if !s.IsEmpty(repoID) {
		parent, _ = s.RevParse(repoID, "HEAD")
	}

	// Temp index + scratch work-tree dir so we never touch the repo's real
	// index and so index ops that demand a work tree are satisfied.
	idx, err := os.CreateTemp("", "orchis-index-*")
	if err != nil {
		return "", "", err
	}
	idxPath := idx.Name()
	idx.Close()
	os.Remove(idxPath) // git creates it fresh; we only want a unique path
	defer os.Remove(idxPath)
	workTree, err := os.MkdirTemp("", "orchis-wt-*")
	if err != nil {
		return "", "", err
	}
	defer os.RemoveAll(workTree)

	// Seed the index from the parent tree (if any).
	if parent != "" {
		if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, "read-tree", parent); err != nil {
			return "", "", err
		}
	}

	// Apply each change into the index.
	for _, c := range changes {
		if c.Delete {
			if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, "update-index", "--force-remove", c.Path); err != nil {
				return "", "", err
			}
			continue
		}
		blob, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, c.Content, "hash-object", "-w", "--stdin")
		if err != nil {
			return "", "", err
		}
		if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil,
			"update-index", "--add", "--cacheinfo", "100644,"+blob+","+c.Path); err != nil {
			return "", "", err
		}
	}

	// Build the tree and commit.
	tree, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, "write-tree")
	if err != nil {
		return "", "", err
	}
	ctArgs := []string{"commit-tree", tree}
	if parent != "" {
		ctArgs = append(ctArgs, "-p", parent)
	}
	newSHA, err = s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, []byte(message), ctArgs...)
	if err != nil {
		return "", "", err
	}

	// Move the branch ref. Pass the expected old value (CAS) when updating an
	// existing branch so concurrent writes can't silently clobber each other.
	updArgs := []string{"update-ref", "refs/heads/" + branch, newSHA}
	if oldSHA != "" {
		updArgs = append(updArgs, oldSHA)
	}
	if _, err := s.gitEnv(repoID, idxPath, workTree, authorName, authorEmail, nil, updArgs...); err != nil {
		return "", "", err
	}
	return newSHA, oldSHA, nil
}
