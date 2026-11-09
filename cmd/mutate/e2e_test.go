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
	code := runCLI([]string{"-pkgs", ".", "-timeout", "3m"}, &out, &errOut, dir)
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

func TestEndToEndStillbornBuildFailureKills(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":      "module stillborn\n\ngo 1.27\n",
		"greet.go":    "package stillborn\n\nfunc Greet(name string) string {\n\treturn \"hi \" + name\n}\n",
		"add.go":      "package stillborn\n\nfunc Add(a, b int) int {\n\treturn a + b\n}\n",
		"add_test.go": "package stillborn\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) {\n\tif Add(2, 3) != 5 {\n\t\tt.Fatal(\"Add(2, 3) differs from 5\")\n\t}\n}\n",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) err = %v", name, err)
		}
	}
	var out, errOut strings.Builder
	code := runCLI([]string{"-pkgs", ".", "-timeout", "3m"}, &out, &errOut, dir)
	s := out.String()
	if code != 0 {
		t.Fatalf("runCLI code = %d, want 0 (the stillborn and the real kill both score killed, no allow entry needed)\nstdout:\n%s\nstderr:\n%s", code, s, errOut.String())
	}
	if !strings.Contains(s, "greet.go:4:15 replace + with - KILLED\n") {
		t.Errorf("output missing the stillborn kill line:\n%s", s)
	}
	if !strings.Contains(s, "add.go:4:11 replace + with - KILLED\n") {
		t.Errorf("output missing the real kill line:\n%s", s)
	}
	if strings.Contains(s, " RUN ERROR\n") || strings.Contains(s, " ALLOWED ") {
		t.Errorf("a fresh run needs no allowance and must not void:\n%s", s)
	}
	if got := mustRead(t, filepath.Join(dir, "greet.go")); got != files["greet.go"] {
		t.Errorf("greet.go not restored byte-for-byte:\n%s", got)
	}

	allowPath := filepath.Join(t.TempDir(), "mutate-allow")
	entry := "greet.go:4:15 replace + with - # interim compile-fail entry"
	if err := os.WriteFile(allowPath, []byte(entry+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(allow) err = %v", err)
	}
	out.Reset()
	errOut.Reset()
	code = runCLI([]string{"-pkgs", ".", "-timeout", "3m", "-allow", allowPath, "-challenge"}, &out, &errOut, dir)
	s = out.String()
	if code != 1 {
		t.Fatalf("runCLI(challenge) code = %d, want 1 (the killed entry demotes)\nstdout:\n%s\nstderr:\n%s", code, s, errOut.String())
	}
	if !strings.Contains(s, "greet.go:4:15 replace + with - CHALLENGED\n") {
		t.Errorf("output missing the challenged stillborn verdict:\n%s", s)
	}
	if !strings.Contains(errOut.String(), "no longer equivalent") {
		t.Errorf("stderr missing the demotion failure: %q", errOut.String())
	}
	if strings.Contains(s, " RUN ERROR\n") {
		t.Errorf("the challenge run must score the stillborn killed instead of erroring:\n%s", s)
	}
	if got := mustRead(t, filepath.Join(dir, "greet.go")); got != files["greet.go"] {
		t.Errorf("greet.go not restored byte-for-byte after the challenge:\n%s", got)
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
	code := runCLI([]string{"-pkgs", ".", "-timeout", "3m", "-allow", allowPath}, &out, &errOut, dir)
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
	code = runCLI([]string{"-pkgs", ".", "-timeout", "3m", "-allow", stale}, &out, &errOut, dir)
	if code != 1 {
		t.Errorf("runCLI(stale allowlist) = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "unused allow entries") {
		t.Errorf("stderr missing unused-entry failure: %q", errOut.String())
	}
}
