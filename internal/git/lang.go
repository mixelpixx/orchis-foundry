package git

import (
	"path/filepath"
	"strings"
)

// langForPath maps a filename to the short lang token the frontend uses
// (see langColor in src/views/repo.jsx: rust, toml, yaml, markdown, text, ...).
func langForPath(name string) string {
	base := strings.ToLower(filepath.Base(name))
	switch base {
	case "dockerfile":
		return "docker"
	case "makefile":
		return "makefile"
	case "go.mod", "go.sum":
		return "text"
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".rs":
		return "rust"
	case ".go":
		return "go"
	case ".ts", ".tsx":
		return "typescript"
	case ".js", ".jsx", ".mjs", ".cjs":
		return "javascript"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".sh", ".bash", ".zsh":
		return "shell"
	case ".toml":
		return "toml"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md", ".markdown":
		return "markdown"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".hpp":
		return "cpp"
	case ".java":
		return "java"
	case ".css":
		return "css"
	case ".html", ".htm":
		return "html"
	case ".sql":
		return "sql"
	default:
		return "text"
	}
}

// langDisplay maps the short lang token to the display name used in the Repo
// "language" field (e.g. "Rust", "TypeScript").
func langDisplay(lang string) string {
	switch lang {
	case "rust":
		return "Rust"
	case "go":
		return "Go"
	case "typescript":
		return "TypeScript"
	case "javascript":
		return "JavaScript"
	case "python":
		return "Python"
	case "ruby":
		return "Ruby"
	case "shell":
		return "Shell"
	case "toml":
		return "TOML"
	case "yaml":
		return "YAML"
	case "json":
		return "JSON"
	case "markdown":
		return "Markdown"
	case "c":
		return "C"
	case "cpp":
		return "C++"
	case "java":
		return "Java"
	case "css":
		return "CSS"
	case "html":
		return "HTML"
	case "sql":
		return "SQL"
	default:
		return ""
	}
}

// LangColor returns the oklch color the frontend uses per language.
func LangColor(display string) string {
	switch display {
	case "Rust":
		return "oklch(60% 0.14 30)"
	case "Go":
		return "oklch(62% 0.12 200)"
	case "TypeScript":
		return "oklch(62% 0.12 240)"
	case "JavaScript":
		return "oklch(75% 0.14 90)"
	case "Python":
		return "oklch(62% 0.10 250)"
	case "Markdown":
		return "oklch(60% 0.06 250)"
	case "Shell":
		return "oklch(70% 0.10 110)"
	default:
		return "oklch(60% 0.04 250)"
	}
}
