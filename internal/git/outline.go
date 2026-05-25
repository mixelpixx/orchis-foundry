package git

import (
	"regexp"
	"strings"
)

// Symbol is one structural entry in a file outline. CGO-free and approximate —
// it's an LLM affordance (read the names, then fetch only the bodies you need),
// not a compiler. Tree-sitter could drop in behind this same shape later.
type Symbol struct {
	Kind      string `json:"kind"` // func | method | class | struct | enum | interface | type | import
	Name      string `json:"name"`
	LineStart int    `json:"lineStart"`
	LineEnd   int    `json:"lineEnd"`
	Signature string `json:"signature"`
}

type outlineRule struct {
	kind string
	re   *regexp.Regexp
}

// Per-language declaration patterns. Capture group 1 is the symbol name.
var outlineRules = map[string][]outlineRule{
	"go": {
		{"func", regexp.MustCompile(`^func\s+(?:\([^)]*\)\s+)?([A-Za-z_]\w*)\s*\(`)},
		{"type", regexp.MustCompile(`^type\s+([A-Za-z_]\w*)\s+`)},
	},
	"rust": {
		{"func", regexp.MustCompile(`^\s*(?:pub\s+)?(?:async\s+)?fn\s+([A-Za-z_]\w*)`)},
		{"struct", regexp.MustCompile(`^\s*(?:pub\s+)?struct\s+([A-Za-z_]\w*)`)},
		{"enum", regexp.MustCompile(`^\s*(?:pub\s+)?enum\s+([A-Za-z_]\w*)`)},
		{"interface", regexp.MustCompile(`^\s*(?:pub\s+)?trait\s+([A-Za-z_]\w*)`)},
	},
	"javascript": {
		{"func", regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?(?:async\s+)?function\s+([A-Za-z_$][\w$]*)`)},
		{"class", regexp.MustCompile(`^\s*(?:export\s+)?(?:default\s+)?class\s+([A-Za-z_$][\w$]*)`)},
		{"func", regexp.MustCompile(`^\s*(?:export\s+)?const\s+([A-Za-z_$][\w$]*)\s*=\s*(?:async\s+)?\(`)},
	},
	"python": {
		{"func", regexp.MustCompile(`^\s*(?:async\s+)?def\s+([A-Za-z_]\w*)`)},
		{"class", regexp.MustCompile(`^\s*class\s+([A-Za-z_]\w*)`)},
	},
	"java": {
		{"class", regexp.MustCompile(`^\s*(?:public\s+|private\s+|protected\s+|final\s+|abstract\s+|static\s+)*class\s+([A-Za-z_]\w*)`)},
		{"interface", regexp.MustCompile(`^\s*(?:public\s+|private\s+)?interface\s+([A-Za-z_]\w*)`)},
	},
}

// outlineLang maps the shared langForPath token to an outline ruleset key
// ("" = unsupported). TypeScript reuses the JavaScript rules.
func outlineLang(path string) string {
	switch langForPath(path) {
	case "go":
		return "go"
	case "rust":
		return "rust"
	case "javascript", "typescript":
		return "javascript"
	case "python":
		return "python"
	case "java":
		return "java"
	default:
		return ""
	}
}

// Outline extracts top-level symbols from a file at ref ("" = HEAD). Returns an
// empty slice for unsupported languages or unreadable files.
func (s *Store) Outline(repoID int64, ref, path string) ([]Symbol, error) {
	lang := outlineLang(path)
	if lang == "" {
		return []Symbol{}, nil
	}
	blob, err := s.FileBlob(repoID, ref, path)
	if err != nil {
		return nil, err
	}
	rules := outlineRules[lang]
	lines := strings.Split(blob.Content, "\n")
	out := []Symbol{}
	for i, ln := range lines {
		for _, rule := range rules {
			m := rule.re.FindStringSubmatch(ln)
			if m == nil {
				continue
			}
			start := i + 1
			end := blockEnd(lines, i, lang)
			out = append(out, Symbol{
				Kind: rule.kind, Name: m[1], LineStart: start, LineEnd: end,
				Signature: strings.TrimSpace(ln),
			})
			break // first matching rule wins for this line
		}
	}
	return out, nil
}

// blockEnd returns the 1-based last line of the block beginning at line index i.
// Brace languages: balance { }. Python: until the indentation returns to the
// declaration's level or less. Best-effort; falls back to the start line.
func blockEnd(lines []string, i int, lang string) int {
	if lang == "python" {
		baseIndent := indentOf(lines[i])
		for j := i + 1; j < len(lines); j++ {
			if strings.TrimSpace(lines[j]) == "" {
				continue
			}
			if indentOf(lines[j]) <= baseIndent {
				return j // line j is 1-based index of the line *after* the block's last
			}
		}
		return len(lines)
	}
	// Brace-balanced languages.
	depth := 0
	seen := false
	for j := i; j < len(lines); j++ {
		for _, c := range lines[j] {
			if c == '{' {
				depth++
				seen = true
			} else if c == '}' {
				depth--
			}
		}
		if seen && depth <= 0 {
			return j + 1
		}
		if !seen && j > i+40 {
			break // no opening brace nearby (e.g. a one-line type/decl)
		}
	}
	return i + 1
}

func indentOf(s string) int {
	n := 0
	for _, c := range s {
		if c == ' ' {
			n++
		} else if c == '\t' {
			n += 4
		} else {
			break
		}
	}
	return n
}
