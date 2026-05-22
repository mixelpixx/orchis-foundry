// Package git provides repository storage + read access.
//
// Reads use go-git (in-process). Writes / pack protocol (A5/A6) shell out to
// the system `git` binary, which is more reliable for the pack negotiation.
package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Store manages bare repos under a root directory.
type Store struct {
	root string
}

func NewStore(root string) *Store { return &Store{root: root} }

// Path returns the on-disk bare repo dir for a repo ID.
func (s *Store) Path(repoID int64) string {
	return filepath.Join(s.root, fmt.Sprintf("%d.git", repoID))
}

// InitBare creates a new bare repo with HEAD pointing at defaultBranch.
func (s *Store) InitBare(repoID int64, defaultBranch string) error {
	path := s.Path(repoID)
	if defaultBranch == "" {
		defaultBranch = "main"
	}
	// Use the system git for init so hooks/config match what receive-pack expects.
	cmd := exec.Command("git", "init", "--bare", "--initial-branch="+defaultBranch, path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git init --bare: %v: %s", err, out)
	}
	return nil
}

// Remove deletes a repo's on-disk storage.
func (s *Store) Remove(repoID int64) error {
	return os.RemoveAll(s.Path(repoID))
}

func (s *Store) open(repoID int64) (*gogit.Repository, error) {
	return gogit.PlainOpen(s.Path(repoID))
}

// Node is a file-tree entry matching FILE_TREE in src/data.jsx.
type Node struct {
	Type     string  `json:"type"` // "dir" | "file"
	Name     string  `json:"name"`
	Path     string  `json:"path"`
	Lang     string  `json:"lang,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

// Branch matches the branches endpoint.
type Branch struct {
	Name      string `json:"name"`
	SHA       string `json:"sha"`
	IsDefault bool   `json:"isDefault"`
}

// Commit matches the commits endpoint.
type Commit struct {
	SHA     string `json:"sha"`
	Short   string `json:"short"`
	Message string `json:"message"`
	Author  string `json:"author"`
	Email   string `json:"email"`
	Date    string `json:"date"`
}

// Blob matches the blob endpoint.
type Blob struct {
	Content string `json:"content"`
	Lang    string `json:"lang"`
	Size    int64  `json:"size"`
	Lines   int    `json:"lines"`
}

// IsEmpty reports whether the repo has no commits yet (fresh / never pushed).
func (s *Store) IsEmpty(repoID int64) bool {
	r, err := s.open(repoID)
	if err != nil {
		return true
	}
	_, err = r.Head()
	return err != nil
}

// resolveRef resolves a ref name (branch, tag, sha, or "" = HEAD) to a commit.
func (s *Store) resolveRef(r *gogit.Repository, ref string) (*object.Commit, error) {
	var hash plumbing.Hash
	if ref == "" {
		h, err := r.Head()
		if err != nil {
			return nil, err
		}
		hash = h.Hash()
	} else if h, err := r.ResolveRevision(plumbing.Revision(ref)); err == nil {
		hash = *h
	} else if bref, err := r.Reference(plumbing.NewBranchReferenceName(ref), true); err == nil {
		hash = bref.Hash()
	} else {
		return nil, fmt.Errorf("ref not found: %s", ref)
	}
	return r.CommitObject(hash)
}

// Tree returns the full nested file tree at ref (matching how the frontend
// consumes FILE_TREE). Empty repos return an empty slice.
func (s *Store) Tree(repoID int64, ref string) ([]*Node, error) {
	r, err := s.open(repoID)
	if err != nil {
		return []*Node{}, nil
	}
	commit, err := s.resolveRef(r, ref)
	if err != nil {
		return []*Node{}, nil // empty repo
	}
	tree, err := commit.Tree()
	if err != nil {
		return nil, err
	}
	return buildTree(tree, ""), nil
}

func buildTree(tree *object.Tree, prefix string) []*Node {
	var nodes []*Node
	for _, e := range tree.Entries {
		full := e.Name
		if prefix != "" {
			full = prefix + "/" + e.Name
		}
		if e.Mode.IsFile() {
			nodes = append(nodes, &Node{Type: "file", Name: e.Name, Path: full, Lang: langForPath(e.Name)})
		} else {
			sub, err := tree.Tree(e.Name)
			if err != nil {
				continue
			}
			nodes = append(nodes, &Node{Type: "dir", Name: e.Name, Path: full, Children: buildTree(sub, full)})
		}
	}
	// Dirs first, then files; alphabetical within each (matches typical UI).
	sort.SliceStable(nodes, func(i, j int) bool {
		if nodes[i].Type != nodes[j].Type {
			return nodes[i].Type == "dir"
		}
		return nodes[i].Name < nodes[j].Name
	})
	return nodes
}

// FileBlob returns a file's content at ref.
func (s *Store) FileBlob(repoID int64, ref, path string) (*Blob, error) {
	r, err := s.open(repoID)
	if err != nil {
		return nil, err
	}
	commit, err := s.resolveRef(r, ref)
	if err != nil {
		return nil, err
	}
	f, err := commit.File(path)
	if err != nil {
		return nil, err
	}
	content, err := f.Contents()
	if err != nil {
		return nil, err
	}
	return &Blob{
		Content: content,
		Lang:    langForPath(path),
		Size:    f.Size,
		Lines:   strings.Count(content, "\n") + 1,
	}, nil
}

// RawBytes returns raw file bytes at ref.
func (s *Store) RawBytes(repoID int64, ref, path string) ([]byte, error) {
	b, err := s.FileBlob(repoID, ref, path)
	if err != nil {
		return nil, err
	}
	return []byte(b.Content), nil
}

// Readme finds a README.* at the repo root.
func (s *Store) Readme(repoID int64, ref string) (string, string, bool) {
	for _, name := range []string{"README.md", "README.MD", "Readme.md", "README", "readme.md"} {
		if b, err := s.FileBlob(repoID, ref, name); err == nil {
			format := "markdown"
			if !strings.Contains(strings.ToLower(name), ".md") {
				format = "text"
			}
			return b.Content, format, true
		}
	}
	return "", "", false
}

// Branches lists branches with the default flagged.
func (s *Store) Branches(repoID int64, defaultBranch string) ([]Branch, error) {
	r, err := s.open(repoID)
	if err != nil {
		return []Branch{}, nil
	}
	iter, err := r.Branches()
	if err != nil {
		return []Branch{}, nil
	}
	var out []Branch
	_ = iter.ForEach(func(ref *plumbing.Reference) error {
		name := ref.Name().Short()
		out = append(out, Branch{Name: name, SHA: ref.Hash().String(), IsDefault: name == defaultBranch})
		return nil
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Commits lists commits from ref, newest first, up to limit.
func (s *Store) Commits(repoID int64, ref string, limit int) ([]Commit, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	r, err := s.open(repoID)
	if err != nil {
		return []Commit{}, nil
	}
	commit, err := s.resolveRef(r, ref)
	if err != nil {
		return []Commit{}, nil
	}
	iter, err := r.Log(&gogit.LogOptions{From: commit.Hash})
	if err != nil {
		return nil, err
	}
	var out []Commit
	_ = iter.ForEach(func(c *object.Commit) error {
		if len(out) >= limit {
			return fmt.Errorf("stop")
		}
		msg := strings.TrimRight(c.Message, "\n")
		out = append(out, Commit{
			SHA:     c.Hash.String(),
			Short:   c.Hash.String()[:7],
			Message: msg,
			Author:  c.Author.Name,
			Email:   c.Author.Email,
			Date:    c.Author.When.UTC().Format("2006-01-02T15:04:05Z"),
		})
		return nil
	})
	return out, nil
}

// Tags lists tags (name + target sha), newest-ish first by name desc.
func (s *Store) Tags(repoID int64) ([]Branch, error) {
	out, err := exec.Command("git", "-C", s.Path(repoID),
		"for-each-ref", "--format=%(refname:short) %(objectname)", "refs/tags").Output()
	if err != nil {
		return []Branch{}, nil
	}
	var tags []Branch
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if ln == "" {
			continue
		}
		parts := strings.Fields(ln)
		if len(parts) >= 2 {
			tags = append(tags, Branch{Name: parts[0], SHA: parts[1]})
		}
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Name > tags[j].Name })
	return tags, nil
}

// BranchExists reports whether a branch ref exists.
func (s *Store) BranchExists(repoID int64, branch string) bool {
	r, err := s.open(repoID)
	if err != nil {
		return false
	}
	_, err = r.Reference(plumbing.NewBranchReferenceName(branch), true)
	return err == nil
}

// SetDefaultBranch points HEAD at the given branch (must exist).
func (s *Store) SetDefaultBranch(repoID int64, branch string) error {
	if !s.BranchExists(repoID, branch) {
		return fmt.Errorf("branch does not exist: %s", branch)
	}
	cmd := exec.Command("git", "-C", s.Path(repoID), "symbolic-ref", "HEAD", "refs/heads/"+branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("set HEAD: %v: %s", err, out)
	}
	return nil
}

// branchNameRe is a conservative allow-list for branch / tag names. It rejects
// anything that could be read as a flag (leading '-') or escape git's ref
// parsing. git itself does the final validation; this is defense-in-depth so
// hostile names never reach the command line as options.
var branchNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._/-]{0,199}$`)

// ValidRefName reports whether name is a safe branch/tag name to pass to git.
func ValidRefName(name string) bool {
	if !branchNameRe.MatchString(name) {
		return false
	}
	if strings.Contains(name, "..") || strings.HasSuffix(name, "/") || strings.HasSuffix(name, ".lock") {
		return false
	}
	return true
}

// CreateBranch creates a new branch pointing at startPoint (a ref or sha; ""
// means the repo's current HEAD). The name is validated; startPoint is resolved
// to a sha first so it can't be smuggled as a flag.
func (s *Store) CreateBranch(repoID int64, name, startPoint string) error {
	if !ValidRefName(name) {
		return fmt.Errorf("invalid branch name")
	}
	if s.BranchExists(repoID, name) {
		return fmt.Errorf("branch already exists: %s", name)
	}
	if startPoint == "" {
		startPoint = "HEAD"
	}
	sha, err := s.RevParse(repoID, startPoint)
	if err != nil {
		return fmt.Errorf("start point not found: %s", startPoint)
	}
	cmd := exec.Command("git", "-C", s.Path(repoID), "branch", name, sha)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create branch: %v: %s", err, out)
	}
	return nil
}

// DeleteBranch force-deletes a branch. The caller must prevent deleting the
// default branch (git also refuses to delete the branch HEAD points at).
func (s *Store) DeleteBranch(repoID int64, name string) error {
	if !ValidRefName(name) {
		return fmt.Errorf("invalid branch name")
	}
	if !s.BranchExists(repoID, name) {
		return fmt.Errorf("branch does not exist: %s", name)
	}
	cmd := exec.Command("git", "-C", s.Path(repoID), "branch", "-D", name)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("delete branch: %v: %s", err, out)
	}
	return nil
}

// DetectLanguage returns the dominant language by file extension at HEAD.
func (s *Store) DetectLanguage(repoID int64) string {
	nodes, _ := s.Tree(repoID, "")
	counts := map[string]int{}
	var walk func([]*Node)
	walk = func(ns []*Node) {
		for _, n := range ns {
			if n.Type == "file" && n.Lang != "" && n.Lang != "text" {
				counts[langDisplay(n.Lang)]++
			}
			walk(n.Children)
		}
	}
	walk(nodes)
	best, bestN := "", 0
	for k, v := range counts {
		if v > bestN {
			best, bestN = k, v
		}
	}
	return best
}
