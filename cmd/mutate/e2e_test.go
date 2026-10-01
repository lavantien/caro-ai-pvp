package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) err = %v", path, err)
	}
	return string(b)
}

func TestEndToEndFixtureModule(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":        "module fixture\n\ngo 1.27\n",
		"arith.go":      mustRead(t, filepath.Join("testdata", "arith.go")),
		"logic.go":      mustRead(t, filepath.Join("testdata", "logic.go")),
		"loops.go":      mustRead(t, filepath.Join("testdata", "loops.go")),
		"arith_test.go": mustRead(t, filepath.Join("testdata", "arith_test.go")),
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) err = %v", name, err)
		}
	}
	var out, errOut strings.Builder
	code := runCLI([]string{"-pkgs", ".", "-timeout", "60s"}, &out, &errOut, dir)
	if code != 1 {
		t.Fatalf("runCLI code = %d, want 1 (weak tests leave survivors)\nstdout:\n%s\nstderr:\n%s", code, out.String(), errOut.String())
	}
	s := out.String()
	if !strings.Contains(s, "arith.go:4:11 replace + with - KILLED\n") {
		t.Errorf("output missing the Add kill line:\n%s", s)
	}
	if !strings.Contains(s, " SURVIVED\n") {
		t.Errorf("output missing survived verdicts:\n%s", s)
	}
	if !strings.Contains(s, "mutate: ") || !strings.Contains(s, " run, ") || !strings.Contains(s, " survived\n") {
		t.Errorf("output missing summary:\n%s", s)
	}
	if errOut.Len() > 0 {
		t.Errorf("unexpected stderr: %s", errOut.String())
	}
	for _, name := range []string{"arith.go", "logic.go", "loops.go", "arith_test.go"} {
		if got := mustRead(t, filepath.Join(dir, name)); got != files[name] {
			t.Errorf("%s not restored byte-for-byte after run:\n%s", name, got)
		}
	}
}
