package api

import "testing"

func TestShouldPruneDefaults(t *testing.T) {
	prune := []string{
		"node_modules/react/index.js",
		"frontend/node_modules/x/y.js",
		"vendor/github.com/foo/bar.go",
		"dist/app.js",
		"build/out.o",
		"package-lock.json",
		"web/pnpm-lock.yaml",
		"Cargo.lock",
		"go.sum",
		"app.min.js",
		"styles.min.css",
		"logo.png",
		"bundle.js.map",
	}
	for _, p := range prune {
		if !shouldPrune(p, nil) {
			t.Errorf("expected %q pruned", p)
		}
	}
	keep := []string{
		"src/main.go", "internal/api/server.go", "README.md",
		"cmd/orchis/main.go", "web/src/app.jsx", "go.mod",
	}
	for _, p := range keep {
		if shouldPrune(p, nil) {
			t.Errorf("expected %q kept", p)
		}
	}
}

func TestShouldPruneExtraGlobs(t *testing.T) {
	globs := parsePruneGlobs("*.snap\ngenerated\n  *.pb.go ,  ")
	if len(globs) != 3 {
		t.Fatalf("expected 3 globs, got %d: %v", len(globs), globs)
	}
	cases := map[string]bool{
		"components/Button.snap":   true,  // *.snap basename
		"generated/api.ts":         true,  // bare dir match
		"rpc/service.pb.go":        true,  // *.pb.go
		"src/Button.tsx":           false, // unrelated
		"docs/generated-guide.md":  false, // 'generated' only matches a full segment
	}
	for p, want := range cases {
		if got := shouldPrune(p, globs); got != want {
			t.Errorf("shouldPrune(%q) = %v, want %v", p, got, want)
		}
	}
}
