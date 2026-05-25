package git

import "testing"

func TestOutlineGo(t *testing.T) {
	st, head := setupGrepRepo(t) // main.go + router/sink.go committed on main
	// Add a richer Go file to outline.
	src := "package main\n\nimport \"fmt\"\n\ntype Server struct {\n\tName string\n}\n\nfunc (s *Server) Run() error {\n\tfmt.Println(s.Name)\n\treturn nil\n}\n\nfunc helper(x int) int {\n\treturn x + 1\n}\n"
	if _, _, err := st.CommitFiles(1, "main", head, "add server.go", "t", "t@t.io",
		[]FileChange{{Path: "server.go", Content: []byte(src)}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	syms, err := st.Outline(1, "main", "server.go")
	if err != nil {
		t.Fatalf("outline: %v", err)
	}
	got := map[string]string{}
	for _, s := range syms {
		got[s.Name] = s.Kind
	}
	if got["Server"] != "type" {
		t.Errorf("expected type Server, got %v", got)
	}
	if got["Run"] != "func" {
		t.Errorf("expected func Run, got %v", got)
	}
	if got["helper"] != "func" {
		t.Errorf("expected func helper, got %v", got)
	}
	// Run's block should span more than one line (brace matching worked).
	for _, s := range syms {
		if s.Name == "Run" && s.LineEnd <= s.LineStart {
			t.Errorf("Run lineEnd %d should exceed lineStart %d", s.LineEnd, s.LineStart)
		}
	}
}

func TestOutlinePython(t *testing.T) {
	st, head := setupGrepRepo(t)
	src := "import os\n\nclass Widget:\n    def __init__(self):\n        self.x = 1\n\n    def render(self):\n        return self.x\n\ndef top_level():\n    return 42\n"
	if _, _, err := st.CommitFiles(1, "main", head, "add w.py", "t", "t@t.io",
		[]FileChange{{Path: "w.py", Content: []byte(src)}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	syms, _ := st.Outline(1, "main", "w.py")
	names := map[string]bool{}
	for _, s := range syms {
		names[s.Name] = true
	}
	for _, want := range []string{"Widget", "__init__", "render", "top_level"} {
		if !names[want] {
			t.Errorf("expected symbol %q in %v", want, names)
		}
	}
}

func TestOutlineUnsupported(t *testing.T) {
	st, head := setupGrepRepo(t)
	if _, _, err := st.CommitFiles(1, "main", head, "add data", "t", "t@t.io",
		[]FileChange{{Path: "data.bin", Content: []byte("\x00\x01\x02")}}); err != nil {
		t.Fatalf("commit: %v", err)
	}
	syms, err := st.Outline(1, "main", "data.bin")
	if err != nil || len(syms) != 0 {
		t.Errorf("expected empty outline for unsupported file, got %v (err %v)", syms, err)
	}
}
