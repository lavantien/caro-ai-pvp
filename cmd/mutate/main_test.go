package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSplitPkgs(t *testing.T) {
	got := splitPkgs(" a , internal/rules,,./internal/engine ")
	want := "./a|./internal/rules|./internal/engine"
	parts := make([]string, 0, len(got))
	parts = append(parts, got...)
	if strings.Join(parts, "|") != want {
		t.Errorf("splitPkgs = %v, want %s", got, want)
	}
	if got := splitPkgs(" , ,"); len(got) != 0 {
		t.Errorf("splitPkgs(empty) = %v, want none", got)
	}
}

func TestExitCode(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		res  result
		want int
	}{
		{name: "all killed", res: result{killed: 4, survived: 0, run: 4, total: 4}, want: 0},
		{name: "allowed survivor", res: result{killed: 3, allowed: 1, run: 4, total: 4}, want: 0},
		{name: "survivor", res: result{killed: 3, survived: 1, run: 4, total: 4}, want: 1},
		{name: "run error", err: errors.New("boom"), want: 1},
		{name: "no mutants", res: result{}, want: 1},
		{name: "aborted", res: result{killed: 2, run: 2, total: 4}, want: 1},
	} {
		if got := exitCode(tc.err, tc.res); got != tc.want {
			t.Errorf("%s: exitCode = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestRunCLIUsage(t *testing.T) {
	for _, args := range [][]string{
		{"stray"},
		{"-timeout", "1s"},
		{"-pkgs", " , "},
		{"-badflag"},
	} {
		if code := runCLI(args, &strings.Builder{}, &strings.Builder{}, "."); code != 2 {
			t.Errorf("runCLI(%v) = %d, want 2", args, code)
		}
	}
}

func TestRunCLIResumeLogMissingFailsClosed(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	var out, errOut strings.Builder
	if code := runCLI([]string{"-resume", filepath.Join(dir, "missing.log")}, &out, &errOut, dir); code != 2 {
		t.Fatalf("runCLI(missing resume log) = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "missing.log") {
		t.Errorf("stderr %q does not name the missing resume log", errOut.String())
	}
	if out.Len() != 0 {
		t.Errorf("stdout %q written before the resume failure", out.String())
	}
}

func TestRunCLIResumeAndAllowScopeReports(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":  "module probe\n\ngo 1.27\n",
		"decl.go": "package probe\n\ntype T struct{ A int }\n",
	})
	resume := filepath.Join(dir, "prior.log")
	if err := os.WriteFile(resume, []byte("decl.go:3:15 replace 1 with 2 KILLED\nmutate: 1/1 run, 1 killed, 0 survived, 0 allowed\n"), 0o644); err != nil {
		t.Fatalf("stage resume log: %v", err)
	}
	allow := filepath.Join(dir, "allow.txt")
	if err := os.WriteFile(allow, []byte("decl.go:3:15 replace 1 with 2 # probe proof\n"), 0o644); err != nil {
		t.Fatalf("stage allowlist: %v", err)
	}
	var out, errOut strings.Builder
	code := runCLI([]string{"-pkgs", ".", "-timeout", "10s", "-resume", resume, "-allow", allow, "-allow-scope", "internal/engine"}, &out, &errOut, dir)
	if code != 1 {
		t.Fatalf("runCLI(no mutants over resume) = %d, want 1", code)
	}
	s := out.String()
	if !strings.Contains(s, "mutate: resuming, 1 prior kills replayed\n") {
		t.Errorf("stdout missing the resume report:\n%s", s)
	}
	if !strings.Contains(s, "mutate: allowlist scoped to 0 entries\n") {
		t.Errorf("stdout missing the scoped allowlist count:\n%s", s)
	}
	if !strings.Contains(s, "no mutants found") {
		t.Errorf("stdout missing the no-mutants verdict:\n%s", s)
	}
}

func TestRunCLIPrivateCacheCreationFailsClosed(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":  "module probe\n\ngo 1.27\n",
		"decl.go": "package probe\n\ntype T struct{ A int }\n",
	})
	missing := filepath.Join(dir, "no-temp")
	t.Setenv("TMP", missing)
	t.Setenv("TEMP", missing)
	t.Setenv("TMPDIR", missing)
	var out, errOut strings.Builder
	if code := runCLI([]string{"-pkgs", ".", "-timeout", "10s"}, &out, &errOut, dir); code != 2 {
		t.Fatalf("runCLI(no temp root for the private cache) = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "no-temp") {
		t.Errorf("stderr %q does not name the unusable temp root", errOut.String())
	}
}

func TestRunCLIParallelNoMutantsStillGates(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":  "module probe\n\ngo 1.27\n",
		"decl.go": "package probe\n\ntype T struct{ A int }\n",
	})
	var out, errOut strings.Builder
	if code := runCLI([]string{"-pkgs", ".", "-timeout", "10s", "-parallel", "2"}, &out, &errOut, dir); code != 1 {
		t.Fatalf("runCLI(parallel over no mutants) = %d, want 1: an empty population must not pass the gate", code)
	}
	if !strings.Contains(out.String(), "no mutants found") {
		t.Errorf("stdout missing the no-mutants verdict:\n%s", out.String())
	}
}
