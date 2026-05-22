package git

import (
	"encoding/json"
	"testing"
)

// TestFileDiffGoldenShape locks the JSON shape of a PR file diff to the
// frontend contract in src/data.jsx (PR_DIFF_FILES). The frontend renders
// f.path / f.additions / f.deletions / f.hunks[].header / .lines[].type /
// .num[0..1] / .text, and the API attaches f.comments. If this golden changes
// you are changing the frontend contract — update both deliberately.
func TestFileDiffGoldenShape(t *testing.T) {
	// Built the way diff.go builds it (Comments initialized to an empty slice).
	fd := FileDiff{
		Path:      "crates/atlas-core/src/router.rs",
		Additions: 18,
		Deletions: 22,
		Comments:  []struct{}{},
		Hunks: []Hunk{
			{
				Header: "@@ -12,7 +12,7 @@ pub struct Router {",
				Lines: []DiffLine{
					{Type: "ctx", Num: [2]string{"14", "14"}, Text: "    sinks: BatchSink,"},
					{Type: "del", Num: [2]string{"15", " "}, Text: "    fallback: Sender,"},
					{Type: "add", Num: [2]string{" ", "15"}, Text: "    fallback: BatchSink,"},
				},
			},
		},
	}

	got, err := json.Marshal(fd)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	const want = `{"path":"crates/atlas-core/src/router.rs","additions":18,"deletions":22,` +
		`"hunks":[{"header":"@@ -12,7 +12,7 @@ pub struct Router {","lines":[` +
		`{"type":"ctx","num":["14","14"],"text":"    sinks: BatchSink,"},` +
		`{"type":"del","num":["15"," "],"text":"    fallback: Sender,"},` +
		`{"type":"add","num":[" ","15"],"text":"    fallback: BatchSink,"}` +
		`]}],"comments":[]}`

	if string(got) != want {
		t.Errorf("FileDiff JSON shape drifted from the frontend contract.\n got: %s\nwant: %s", got, want)
	}
}

// TestDiffLineNumIsPair guards the [leftNum,rightNum] array shape the diff
// gutter depends on (ln.num[0] / ln.num[1]).
func TestDiffLineNumIsPair(t *testing.T) {
	b, _ := json.Marshal(DiffLine{Type: "add", Num: [2]string{" ", "42"}, Text: "x"})
	const want = `{"type":"add","num":[" ","42"],"text":"x"}`
	if string(b) != want {
		t.Errorf("DiffLine shape drifted.\n got: %s\nwant: %s", b, want)
	}
}
