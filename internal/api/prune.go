package api

import (
	"path"
	"strings"
)

// Context-pruning engine: keep boilerplate, dependencies, build output and
// lockfiles out of LLM context so small/local models aren't drowned in noise.
// Applied by the repo pack, the chat context assembler, and outline.

// prunedDirs are directory names that, anywhere in a path, drop the file.
var prunedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "dist": true, "build": true,
	".git": true, ".next": true, ".nuxt": true, "target": true, // rust/java
	"__pycache__": true, ".venv": true, "venv": true, ".idea": true,
	".vscode": true, "coverage": true, "out": true, "bin": true, "obj": true,
}

// prunedFiles are exact basenames (lockfiles + noise) that drop the file.
var prunedFiles = map[string]bool{
	"package-lock.json": true, "pnpm-lock.yaml": true, "yarn.lock": true,
	"cargo.lock": true, "go.sum": true, "poetry.lock": true,
	"composer.lock": true, "gemfile.lock": true, "bun.lockb": true,
	".ds_store": true,
}

// prunedExts are suffixes treated as build/minified/binary noise.
var prunedExts = []string{
	".min.js", ".min.css", ".map", ".lock",
	".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico", ".svg",
	".woff", ".woff2", ".ttf", ".eot", ".pdf", ".zip", ".gz", ".wasm",
}

// shouldPrune reports whether a repo-relative path should be excluded from LLM
// context. extraGlobs are user-configured patterns (simple `*` globs matched
// against the basename or the full path) layered on top of the defaults.
func shouldPrune(p string, extraGlobs []string) bool {
	lp := strings.ToLower(strings.TrimPrefix(p, "/"))
	base := path.Base(lp)
	for _, seg := range strings.Split(lp, "/") {
		if prunedDirs[seg] {
			return true
		}
	}
	if prunedFiles[base] {
		return true
	}
	for _, ext := range prunedExts {
		if strings.HasSuffix(base, ext) {
			return true
		}
	}
	for _, g := range extraGlobs {
		g = strings.ToLower(strings.TrimSpace(g))
		if g == "" {
			continue
		}
		if ok, _ := path.Match(g, base); ok {
			return true
		}
		if ok, _ := path.Match(g, lp); ok {
			return true
		}
		// Bare directory/word match anywhere in the path.
		if !strings.ContainsAny(g, "*?[") {
			for _, seg := range strings.Split(lp, "/") {
				if seg == g {
					return true
				}
			}
		}
	}
	return false
}

// parsePruneGlobs splits the stored prune_globs field (newline or comma
// separated) into a clean slice.
func parsePruneGlobs(s string) []string {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == '\n' || r == ',' || r == '\r' })
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, f)
		}
	}
	return out
}
