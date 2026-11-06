package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func sweepVars(t *testing.T) vars {
	t.Helper()
	return vars{
		"ARGS":       `flag "quoted arg" plain`,
		"TC":         "2",
		"GAMES":      "6",
		"OPP":        "medium",
		"PKG":        "./internal/geom",
		"SHA":        "deadbeef",
		"RUN_ID":     "20261106-000000",
		"VERDICT":    "passed",
		"OUT":        t.TempDir(),
		"LOG":        filepath.Join(t.TempDir(), "mutate.log"),
		"LABEL":      "sweep",
		"CHALLENGE":  "1",
		"PARALLEL":   "1",
		"MUTATE_TMP": t.TempDir(),
	}
}

func TestBuildStepsBuildsEveryRegisteredTarget(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)
	tmp := t.TempDir()
	r := &runner{target: "sweep", root: root, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": tmp}}
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		steps, err := buildSteps(name, r, sweepVars(t))
		if err != nil {
			t.Fatalf("buildSteps(%s): %v", name, err)
		}
		if len(steps) == 0 {
			t.Fatalf("buildSteps(%s) produced no steps", name)
		}
		for _, s := range steps {
			if s.path == "" || s.name == "" {
				t.Fatalf("buildSteps(%s) produced an empty step %+v", name, s)
			}
		}
	}
}

func TestBuildStepsProvenanceErrors(t *testing.T) {
	for _, name := range []string{"cover-badges", "cover-verdict"} {
		if _, err := buildSteps(name, testRunner(), vars{}); err == nil || !strings.Contains(err.Error(), "provenance") {
			t.Fatalf("buildSteps(%s) without provenance = %v, want provenance error", name, err)
		}
	}
	if _, err := buildSteps("no-such-target", testRunner(), vars{}); err == nil || !strings.Contains(err.Error(), "unknown target") {
		t.Fatalf("buildSteps(unknown) = %v, want unknown target error", err)
	}
}

func TestParseArgTokensShapes(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		err  string
	}{
		{"", nil, ""},
		{"a", []string{"a"}, ""},
		{"a b\tc", []string{"a", "b", "c"}, ""},
		{`"x y" z`, []string{"x y", "z"}, ""},
		{`"a"`, []string{"a"}, ""},
		{`""`, []string{""}, ""},
		{`a"b`, nil, "quote inside an unquoted ARGS token"},
		{`"a"x`, nil, "characters after a closing ARGS quote"},
		{`"a`, nil, "unterminated ARGS quote"},
	}
	for _, tc := range cases {
		got, err := parseArgTokens(tc.in)
		if tc.err != "" {
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("parseArgTokens(%q) = %v, want error %q", tc.in, err, tc.err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("parseArgTokens(%q): %v", tc.in, err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("parseArgTokens(%q) = %v, want %v", tc.in, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("parseArgTokens(%q) = %v, want %v", tc.in, got, tc.want)
			}
		}
	}
}

func TestFmtFlowRewritesTrackedGoFiles(t *testing.T) {
	dir := tempGitRepo(t)
	commitFile(t, dir, "a.go", "package tmp\n")
	t.Chdir(dir)
	var names []string
	stubExecStep(t, func(s step, env []string) (int, error) {
		names = append(names, s.name)
		return 0, nil
	})
	r := &runner{target: "fmt", root: dir, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(dir, ".gotmp")}}
	if err := runRegistered(r, vars{}, "fmt"); err != nil {
		t.Fatalf("fmt flow failed: %v", err)
	}
	if strings.Join(names, ",") != "fmt" {
		t.Fatalf("fmt flow steps = %v", names)
	}
	empty := tempGitRepo(t)
	commitFile(t, empty, "note.txt", "x\n")
	t.Chdir(empty)
	names = nil
	if err := runRegistered(r, vars{}, "fmt"); err != nil {
		t.Fatalf("fmt flow without go files failed: %v", err)
	}
	if len(names) != 0 {
		t.Fatalf("fmt flow ran steps without go files: %v", names)
	}
}

func TestMutateResumeRequiresLog(t *testing.T) {
	if err := runRegistered(mutateFlowRunner(t), vars{}, "mutate-resume"); err == nil || !strings.Contains(err.Error(), "LOG=") {
		t.Fatalf("mutate-resume without LOG = %v, want LOG requirement error", err)
	}
}

func TestMutateResumeSucceedsOnCleanRun(t *testing.T) {
	dir := t.TempDir()
	log := filepath.Join(dir, "m.log")
	calls := 0
	stubExecStep(t, func(s step, env []string) (int, error) {
		calls++
		return 0, nil
	})
	v := vars{"LOG": log, "CHALLENGE": "1", "MUTATE_TMP": t.TempDir()}
	if err := runRegistered(mutateFlowRunner(t), v, "mutate-resume"); err != nil {
		t.Fatalf("mutate-resume clean run failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("mutate-resume ran %d steps, want 1", calls)
	}
}

func TestMutateFullExhaustsRetriesWithoutVerdict(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	calls := 0
	stubExecStep(t, func(s step, env []string) (int, error) {
		calls++
		return 1, nil
	})
	v := vars{"LABEL": "sweep", "MUTATE_TMP": t.TempDir()}
	err := runRegistered(mutateFlowRunner(t), v, "mutate-full")
	if err == nil || !strings.Contains(err.Error(), "no definitive verdict after 5 attempts") {
		t.Fatalf("mutate-full without verdict = %v, want retry exhaustion error", err)
	}
	if calls != 5 {
		t.Fatalf("mutate-full ran %d attempts, want 5", calls)
	}
	if _, statErr := os.Stat(filepath.Join("logs", "archive", "mutate-sweep.log")); statErr != nil {
		t.Fatalf("mutate-full archive log: %v", statErr)
	}
}
