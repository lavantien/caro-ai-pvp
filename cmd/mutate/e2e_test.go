package main

import (
	"fmt"
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
	if !strings.Contains(s, "mutate: ") || !strings.Contains(s, " run, ") || !strings.Contains(s, " survived, ") {
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

func TestEndToEndAllowlistGreensGate(t *testing.T) {
	dir := t.TempDir()
	src := "package probe\n\nfunc Clip(a, b int) int {\n\tif a < b {\n\t\treturn a + 1\n\t}\n\treturn b - 1\n}\n"
	files := map[string]string{
		"go.mod":       "module probe\n\ngo 1.27\n",
		"clip.go":      src,
		"clip_test.go": "package probe\n\nimport \"testing\"\n\nfunc TestNothing(t *testing.T) {}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) err = %v", name, err)
		}
	}
	ms := collectSource(t, filepath.Join(dir, "clip.go"), src)
	if len(ms) == 0 {
		t.Fatal("no mutants collected")
	}
	var body strings.Builder
	for _, m := range ms {
		fmt.Fprintf(&body, "%s # probe fixture: nop tests cannot observe anything\n", m.key(dir))
	}
	allowPath := filepath.Join(t.TempDir(), "mutate-allow")
	if err := os.WriteFile(allowPath, []byte(body.String()), 0o644); err != nil {
		t.Fatalf("WriteFile(allow) err = %v", err)
	}
	var out, errOut strings.Builder
	code := runCLI([]string{"-pkgs", ".", "-timeout", "60s", "-allow", allowPath}, &out, &errOut, dir)
	s := out.String()
	if code != 0 {
		t.Fatalf("runCLI code = %d, want 0 (every survivor allowed)\nstdout:\n%s\nstderr:\n%s", code, s, errOut.String())
	}
	if strings.Contains(s, " KILLED\n") || strings.Contains(s, " SURVIVED\n") {
		t.Errorf("nop-test fixture must yield only allowances:\n%s", s)
	}
	if !strings.Contains(s, " ALLOWED # probe fixture: nop tests cannot observe anything\n") {
		t.Errorf("output missing allowed verdicts with reasons:\n%s", s)
	}
	if !strings.Contains(s, " 0 survived, ") || !strings.Contains(s, fmt.Sprintf(" %d allowed\n", len(ms))) {
		t.Errorf("output missing zero-survivor summary:\n%s", s)
	}
	if got := mustRead(t, filepath.Join(dir, "clip.go")); got != src {
		t.Errorf("clip.go not restored byte-for-byte:\n%s", got)
	}

	stale := filepath.Join(t.TempDir(), "mutate-allow")
	if err := os.WriteFile(stale, []byte("clip.go:99:9 replace 9 with 8 # stale entry\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(stale) err = %v", err)
	}
	out.Reset()
	errOut.Reset()
	code = runCLI([]string{"-pkgs", ".", "-timeout", "60s", "-allow", stale}, &out, &errOut, dir)
	if code != 1 {
		t.Errorf("runCLI(stale allowlist) = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "unused allow entries") {
		t.Errorf("stderr missing unused-entry failure: %q", errOut.String())
	}
}
