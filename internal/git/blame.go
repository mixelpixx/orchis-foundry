package git

import (
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// BlameLine is one line's provenance, matching what the frontend blame gutter
// renders. Date is RFC3339 (the API layer turns it into a relative string).
type BlameLine struct {
	SHA     string `json:"sha"`
	Short   string `json:"short"`
	Author  string `json:"author"`
	Date    string `json:"date"`
	Summary string `json:"summary"`
	Line    int    `json:"line"`
	Text    string `json:"text"`
}

// Blame returns per-line authorship for a file at ref ("" = HEAD), parsed from
// `git blame --line-porcelain`. Commit metadata is emitted once per sha by git
// and cached here so each content line can reference it.
func (s *Store) Blame(repoID int64, ref, path string) ([]BlameLine, error) {
	if ref == "" {
		ref = "HEAD"
	}
	// `--` separates the rev from the pathspec so a file named like a flag or a
	// ref can't be misparsed.
	out, err := exec.Command("git", "-C", s.Path(repoID),
		"blame", "--line-porcelain", ref, "--", path).Output()
	if err != nil {
		return nil, err
	}

	type meta struct{ author, date, summary string }
	cache := map[string]*meta{}
	var cur string
	var lines []BlameLine

	for _, ln := range strings.Split(string(out), "\n") {
		switch {
		case ln == "":
			continue
		case ln[0] == '\t':
			// Content line for the current commit + final line number.
			m := cache[cur]
			if m == nil {
				m = &meta{}
			}
			short := cur
			if len(short) > 7 {
				short = short[:7]
			}
			lines = append(lines, BlameLine{
				SHA: cur, Short: short, Author: m.author, Date: m.date,
				Summary: m.summary, Line: len(lines) + 1, Text: ln[1:],
			})
		default:
			fields := strings.SplitN(ln, " ", 2)
			key := fields[0]
			// A header line begins with a 40-hex sha followed by line numbers.
			if len(key) == 40 && isHex(key) {
				cur = key
				if cache[cur] == nil {
					cache[cur] = &meta{}
				}
				continue
			}
			m := cache[cur]
			if m == nil {
				continue
			}
			val := ""
			if len(fields) > 1 {
				val = fields[1]
			}
			switch key {
			case "author":
				m.author = val
			case "summary":
				m.summary = val
			case "author-time":
				if secs, e := strconv.ParseInt(val, 10, 64); e == nil {
					m.date = time.Unix(secs, 0).UTC().Format(time.RFC3339)
				}
			}
		}
	}
	return lines, nil
}

func isHex(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}
