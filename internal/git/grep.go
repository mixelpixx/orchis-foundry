package git

import (
	"context"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// GrepHit is one code-search match.
type GrepHit struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

const grepSnippetMax = 400

// Grep runs a literal (fixed-string) content search against a single commit of
// a bare repo, using `git grep`. It is safe against injection by construction:
//   - everything is passed as separate argv elements (no shell),
//   - the query is bound to `-e` so it can never be read as a flag,
//   - sha must be a resolved 40-hex object (resolve via RevParse first); a
//     non-hex sha is rejected here as a defense-in-depth check,
//   - pathspecs come after `--`, and callers pass only server-built globs.
//
// The caller supplies ctx (use context.WithTimeout to bound runaway searches).
// maxPerFile caps matches per file; the overall result count is bounded by the
// caller. Binary files are skipped (-I).
func (s *Store) Grep(ctx context.Context, repoID int64, sha, query string, pathspecs []string, maxPerFile int) ([]GrepHit, error) {
	if !shaRe.MatchString(sha) {
		return nil, errBadSHA
	}
	if maxPerFile <= 0 || maxPerFile > 50 {
		maxPerFile = 20
	}

	args := []string{
		"-C", s.Path(repoID), "grep",
		"--no-color", "-n", "-I", "-F",
		"--max-count=" + strconv.Itoa(maxPerFile),
		"-e", query,
		sha,
	}
	if len(pathspecs) > 0 {
		args = append(args, "--")
		args = append(args, pathspecs...)
	}

	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		// `git grep` exits non-zero (1) when there are simply no matches; that
		// is not an error for us. Any real failure also yields empty output.
		return []GrepHit{}, nil
	}

	prefix := sha + ":"
	hits := make([]GrepHit, 0, 32)
	for _, ln := range strings.Split(string(out), "\n") {
		if ln == "" || !strings.HasPrefix(ln, prefix) {
			continue
		}
		rest := strings.TrimPrefix(ln, prefix)
		m := grepLineRe.FindStringSubmatch(rest)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		text := m[3]
		if len(text) > grepSnippetMax {
			text = text[:grepSnippetMax]
		}
		hits = append(hits, GrepHit{Path: m[1], Line: n, Text: text})
	}
	return hits, nil
}

var (
	shaRe      = regexp.MustCompile(`^[0-9a-fA-F]{7,64}$`)
	grepLineRe = regexp.MustCompile(`^(.*?):(\d+):(.*)$`)
	errBadSHA  = &grepError{"grep: sha must be a resolved hex object id"}
)

type grepError struct{ msg string }

func (e *grepError) Error() string { return e.msg }
