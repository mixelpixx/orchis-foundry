package git

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// FileDiff matches PR_DIFF_FILES in src/data.jsx.
type FileDiff struct {
	Path      string     `json:"path"`
	Additions int        `json:"additions"`
	Deletions int        `json:"deletions"`
	Hunks     []Hunk     `json:"hunks"`
	Comments  []struct{} `json:"comments"` // populated by the API layer from DB
}

type Hunk struct {
	Header string     `json:"header"`
	Lines  []DiffLine `json:"lines"`
}

type DiffLine struct {
	Type string    `json:"type"` // ctx | add | del
	Num  [2]string `json:"num"`  // [leftNum, rightNum]; " " when N/A
	Text string    `json:"text"`
}

// MergeBase returns the common ancestor of two refs (for PR base).
func (s *Store) MergeBase(repoID int64, a, b string) (string, error) {
	out, err := exec.Command("git", "-C", s.Path(repoID), "merge-base", a, b).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParse resolves a ref to a full SHA.
func (s *Store) RevParse(repoID int64, ref string) (string, error) {
	out, err := exec.Command("git", "-C", s.Path(repoID), "rev-parse", ref).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

var hunkHeaderRe = regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

// Diff computes the unified diff between base and head and parses it into the
// FileDiff shape the frontend expects.
func (s *Store) Diff(repoID int64, base, head string) ([]FileDiff, error) {
	out, err := exec.Command("git", "-C", s.Path(repoID), "diff", "--unified=3", "--no-color", base+".."+head).Output()
	if err != nil {
		return []FileDiff{}, nil
	}
	return parseDiff(string(out)), nil
}

func parseDiff(diff string) []FileDiff {
	var files []FileDiff
	var cur *FileDiff
	var leftNum, rightNum int

	lines := strings.Split(diff, "\n")
	for i := 0; i < len(lines); i++ {
		ln := lines[i]
		switch {
		case strings.HasPrefix(ln, "diff --git "):
			if cur != nil {
				files = append(files, *cur)
			}
			cur = &FileDiff{Hunks: []Hunk{}, Comments: []struct{}{}}
		case strings.HasPrefix(ln, "+++ "):
			if cur != nil {
				p := strings.TrimPrefix(ln, "+++ ")
				p = strings.TrimPrefix(p, "b/")
				if p != "/dev/null" {
					cur.Path = p
				}
			}
		case strings.HasPrefix(ln, "--- "):
			if cur != nil && cur.Path == "" {
				p := strings.TrimPrefix(ln, "--- ")
				p = strings.TrimPrefix(p, "a/")
				if p != "/dev/null" {
					cur.Path = p
				}
			}
		case strings.HasPrefix(ln, "@@"):
			if cur == nil {
				continue
			}
			m := hunkHeaderRe.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			leftNum, _ = strconv.Atoi(m[1])
			rightNum, _ = strconv.Atoi(m[3])
			cur.Hunks = append(cur.Hunks, Hunk{Header: ln, Lines: []DiffLine{}})
		case cur != nil && len(cur.Hunks) > 0 && (strings.HasPrefix(ln, " ") || strings.HasPrefix(ln, "+") || strings.HasPrefix(ln, "-")):
			// Skip the file-mode/index metadata lines that also start with +/- only
			// inside a hunk; here we're guaranteed to be in a hunk.
			h := &cur.Hunks[len(cur.Hunks)-1]
			text := ln[1:]
			switch ln[0] {
			case ' ':
				h.Lines = append(h.Lines, DiffLine{Type: "ctx", Num: [2]string{strconv.Itoa(leftNum), strconv.Itoa(rightNum)}, Text: text})
				leftNum++
				rightNum++
			case '+':
				h.Lines = append(h.Lines, DiffLine{Type: "add", Num: [2]string{" ", strconv.Itoa(rightNum)}, Text: text})
				rightNum++
				cur.Additions++
			case '-':
				h.Lines = append(h.Lines, DiffLine{Type: "del", Num: [2]string{strconv.Itoa(leftNum), " "}, Text: text})
				leftNum++
				cur.Deletions++
			}
		}
	}
	if cur != nil {
		files = append(files, *cur)
	}
	return files
}

// DiffStat returns total additions, deletions, files changed between two refs.
func (s *Store) DiffStat(repoID int64, base, head string) (adds, dels, files int) {
	out, err := exec.Command("git", "-C", s.Path(repoID), "diff", "--numstat", base+".."+head).Output()
	if err != nil {
		return 0, 0, 0
	}
	for _, ln := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if ln == "" {
			continue
		}
		parts := strings.Fields(ln)
		if len(parts) < 3 {
			continue
		}
		a, _ := strconv.Atoi(parts[0])
		d, _ := strconv.Atoi(parts[1])
		adds += a
		dels += d
		files++
	}
	return
}

// CountCommits returns how many commits head is ahead of base.
func (s *Store) CountCommits(repoID int64, base, head string) int {
	out, err := exec.Command("git", "-C", s.Path(repoID), "rev-list", "--count", base+".."+head).Output()
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

// RangeCommits lists commits in base..head (newest first).
func (s *Store) RangeCommits(repoID int64, base, head string, limit int) ([]Commit, error) {
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	out, err := exec.Command("git", "-C", s.Path(repoID), "log",
		"--pretty=format:%H%x1f%an%x1f%ae%x1f%aI%x1f%s", "-n", strconv.Itoa(limit), base+".."+head).Output()
	if err != nil {
		return []Commit{}, nil
	}
	var commits []Commit
	for _, ln := range strings.Split(string(out), "\n") {
		if ln == "" {
			continue
		}
		f := strings.Split(ln, "\x1f")
		if len(f) < 5 {
			continue
		}
		commits = append(commits, Commit{
			SHA: f[0], Short: f[0][:7], Author: f[1], Email: f[2], Date: f[3], Message: f[4],
		})
	}
	return commits, nil
}
