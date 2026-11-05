package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
}

func tempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, a := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "run@test"},
		{"config", "user.name", "runtest"},
	} {
		c := exec.Command("git", a...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", a, err, out)
		}
	}
	return dir
}

func commitFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", name)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add %s: %v: %s", name, err, out)
	}
	c = exec.Command("git", "commit", "-q", "--no-verify", "-m", "add "+name)
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git commit %s: %v: %s", name, err, out)
	}
}

func TestExecCaptureReportsOutputAndExit(t *testing.T) {
	out, code, err := execCapture("go", []string{"version"}, os.Environ(), "")
	if err != nil || code != 0 || !strings.HasPrefix(out, "go version") {
		t.Fatalf("execCapture go version = %q %d %v", out, code, err)
	}
	_, code, err = execCapture("go", []string{"tool", "definitely-missing-one"}, os.Environ(), "")
	if err != nil || code == 0 {
		t.Fatalf("execCapture failing go = code %d err %v, want nonzero code nil err", code, err)
	}
	_, _, err = execCapture("definitely-missing-tool-xyz", nil, os.Environ(), "")
	if err == nil {
		t.Fatal("execCapture missing tool returned nil error")
	}
}

func TestExitCoderErrorAndUnwrap(t *testing.T) {
	inner := errors.New("boom")
	ec := &exitCoder{code: 7, err: inner}
	if ec.Error() != "boom" {
		t.Fatalf("Error = %q", ec.Error())
	}
	if !errors.Is(ec, inner) {
		t.Fatal("Unwrap lost the inner error")
	}
}

func TestGoEnvGOEXE(t *testing.T) {
	got, err := goEnvGOEXE()
	if err != nil {
		t.Fatalf("goEnvGOEXE: %v", err)
	}
	if runtime.GOOS == "windows" && got != ".exe" {
		t.Fatalf("GOEXE on windows = %q, want .exe", got)
	}
}

func TestExitCodeProcess(t *testing.T) {
	if os.Getenv("GO_RUN_EXIT_HELPER") == "" {
		return
	}
	switch os.Getenv("GO_RUN_EXIT_CASE") {
	case "coder":
		exitCode(&exitCoder{code: 7, err: errors.New("seven")})
	case "plain":
		exitCode(errors.New("plain"))
	}
}

func TestExitCodeMapsErrorsToCodes(t *testing.T) {
	for _, tc := range []struct {
		Case string
		Want int
	}{{"coder", 7}, {"plain", 1}} {
		cmd := exec.Command(os.Args[0], "-test.run=TestExitCodeProcess")
		cmd.Env = append(os.Environ(), "GO_RUN_EXIT_HELPER=1", "GO_RUN_EXIT_CASE="+tc.Case)
		err := cmd.Run()
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("%s: process did not exit: %v", tc.Case, err)
		}
		if ee.ExitCode() != tc.Want {
			t.Fatalf("%s: exit = %d, want %d", tc.Case, ee.ExitCode(), tc.Want)
		}
	}
}

func TestRunRegisteredPrefersFlowAndRejectsUnknown(t *testing.T) {
	registerFlow("flow-test-probe", func(r *runner, v vars) error {
		return fmt.Errorf("probe fired")
	})
	t.Cleanup(func() { delete(flows, "flow-test-probe") })
	if err := runRegistered(nil, nil, "flow-test-probe"); err == nil || err.Error() != "probe fired" {
		t.Fatalf("runRegistered flow = %v, want probe fired", err)
	}
	if err := runRegistered(testRunner(), vars{}, "no-such-target-anywhere"); err == nil {
		t.Fatal("runRegistered accepted an unknown target")
	}
}

func TestDoctorFlowChecksToolchain(t *testing.T) {
	root := repoRoot(t)
	r := &runner{target: "doctor", root: root, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1"}}
	if err := runRegistered(r, vars{}, "doctor"); err != nil {
		t.Fatalf("doctor failed on a healthy toolchain: %v", err)
	}
}

func TestGofmtPathResolvesTool(t *testing.T) {
	got := gofmtPath()
	if got == "gofmt" {
		return
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("gofmtPath = %q, stat: %v", got, err)
	}
}

func TestTrackedGoFilesListsGoOnly(t *testing.T) {
	dir := tempGitRepo(t)
	commitFile(t, dir, "a.go", "package tmp\n")
	commitFile(t, dir, "b.txt", "note\n")
	t.Chdir(dir)
	got, err := trackedGoFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "a.go" {
		t.Fatalf("trackedGoFiles = %v, want [a.go]", got)
	}
	empty := tempGitRepo(t)
	t.Chdir(empty)
	got, err = trackedGoFiles()
	if err != nil || got != nil {
		t.Fatalf("trackedGoFiles on empty repo = %v, %v, want nil nil", got, err)
	}
}

func isolateGitDiscovery(t *testing.T, dir string) {
	t.Helper()

	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
}

func TestGitHeadShaReadsRepo(t *testing.T) {
	dir := tempGitRepo(t)
	commitFile(t, dir, "a.txt", "x\n")
	t.Chdir(dir)
	sha, err := gitHeadSha()
	if err != nil {
		t.Fatal(err)
	}
	if len(sha) != 40 {
		t.Fatalf("sha %q is not 40 chars", sha)
	}
	plain := t.TempDir()
	t.Chdir(plain)
	isolateGitDiscovery(t, plain)
	_, err = gitHeadSha()
	if err == nil {
		t.Fatal("gitHeadSha succeeded outside a repo")
	}
}

func TestNewRunnerRequiresRepoRootAndHonorsGotmpdir(t *testing.T) {
	plain := t.TempDir()
	t.Chdir(plain)
	if _, err := newRunner("build", vars{}); err == nil || !strings.Contains(err.Error(), "go.mod") {
		t.Fatalf("newRunner outside repo = %v, want go.mod error", err)
	}
	withMod := t.TempDir()
	if err := os.WriteFile(filepath.Join(withMod, "go.mod"), []byte("module tmp\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(withMod)
	t.Setenv("GOTMPDIR", filepath.Join(withMod, "custom-tmp"))
	r, err := newRunner("build", vars{})
	if err != nil {
		t.Fatal(err)
	}
	if r.base["GOTMPDIR"] != filepath.Join(withMod, "custom-tmp") {
		t.Fatalf("GOTMPDIR override = %q", r.base["GOTMPDIR"])
	}
}

func TestRunnerRunAllStopsAtFailure(t *testing.T) {
	var ran []string
	stubExecStep(t, func(s step, env []string) (int, error) {
		ran = append(ran, s.name)
		if s.name == "second" {
			return 2, nil
		}
		return 0, nil
	})
	r := mutateFlowRunner(t)
	err := r.runAll([]step{{name: "first", path: "go"}, {name: "second", path: "go"}, {name: "third", path: "go"}})
	if err == nil {
		t.Fatal("runAll passed a failing step")
	}
	var ec *exitCoder
	if !errors.As(err, &ec) || ec.code != 2 {
		t.Fatalf("runAll error = %v, want exitCoder 2", err)
	}
	if strings.Join(ran, ",") != "first,second" {
		t.Fatalf("steps ran = %v", ran)
	}
}

func TestMutateLogFailureIgnoresMissingLog(t *testing.T) {
	if err := mutateLogFailure("mutate-resume", filepath.Join(t.TempDir(), "absent.log")); err != nil {
		t.Fatalf("missing log reported failure: %v", err)
	}
}

func TestFuzzFlowBuildsEveryGroupStep(t *testing.T) {
	t.Chdir(repoRoot(t))
	var steps []step
	stubExecStep(t, func(s step, env []string) (int, error) {
		steps = append(steps, s)
		return 0, nil
	})
	r := &runner{target: "fuzz", root: repoRoot(t), goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(t.TempDir(), "gotmp")}}
	if err := runRegistered(r, vars{}, "fuzz"); err != nil {
		t.Fatalf("fuzz flow failed: %v", err)
	}
	if len(steps) != 11 {
		t.Fatalf("fuzz steps = %d, want 11", len(steps))
	}
	var fuzztime bool
	for _, s := range steps {
		for _, a := range s.args {
			if strings.HasPrefix(a, "-fuzztime=") {
				fuzztime = true
			}
		}
	}
	if !fuzztime {
		t.Fatal("fuzz steps carry no -fuzztime flag")
	}
}

func TestDiagramsFlowRendersStaleSources(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "docs", "diagrams"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "docs", "diagrams", "a.puml"), []byte("@startuml\n@enduml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PLANTUML_JAR", filepath.Join(dir, "plantuml.jar"))
	var steps []step
	stubExecStep(t, func(s step, env []string) (int, error) {
		steps = append(steps, s)
		return 0, nil
	})
	r := &runner{target: "diagrams", root: dir, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(dir, ".gotmp")}}
	if err := runRegistered(r, vars{}, "diagrams"); err != nil {
		t.Fatalf("diagrams flow failed: %v", err)
	}
	if len(steps) != 1 || steps[0].path != "java" {
		t.Fatalf("diagram steps = %+v", steps)
	}
}

func TestChainFlowCompletesArmsWithSummaries(t *testing.T) {
	dir := tempGitRepo(t)
	for _, arm := range regateArms {
		runDir := filepath.Join(dir, "logs", "tourny", "x-"+arm)
		if err := os.MkdirAll(runDir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runDir, "summary.txt"), []byte("done\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, nil })
	r := &runner{target: "tourney-regate-chain", root: dir, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(dir, ".gotmp")}}
	if err := runRegistered(r, vars{}, "tourney-regate-chain"); err != nil {
		t.Fatalf("chain flow failed: %v", err)
	}
}

func TestCiFlowRunsChainAndRegeneratesBadges(t *testing.T) {
	dir := tempGitRepo(t)
	commitFile(t, dir, "a.go", "package tmp\n")
	for _, sub := range []string{filepath.Join("playground", "bookgen"), filepath.Join("playground", "evalfit")} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	var names []string
	stubExecStep(t, func(s step, env []string) (int, error) {
		names = append(names, s.name)
		return 0, nil
	})
	r := &runner{target: "ci", root: dir, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(dir, ".gotmp")}}
	if err := runRegistered(r, vars{}, "ci"); err != nil {
		t.Fatalf("ci flow failed: %v", err)
	}
	joined := "," + strings.Join(names, ",") + ","
	for _, want := range []string{",build,", ",cover,", ",cover-badges,"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("ci steps missed %s: %v", want, names)
		}
	}
}

func TestMainExitsPerArgumentShape(t *testing.T) {
	root := repoRoot(t)
	exe := filepath.Join(t.TempDir(), "run-helper.exe")
	build := exec.Command("go", "build", "-o", exe, "./cmd/run")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build cmd/run: %v: %s", err, out)
	}
	cases := []struct {
		name string
		args []string
		cwd  string
		want int
		out  string
	}{
		{"no args", nil, root, 2, "usage"},
		{"unknown target", []string{"no-such-target"}, root, 2, "unknown target"},
		{"failing flow", []string{"fmt-check"}, "", 1, "fmt-check"},
	}
	for _, tc := range cases {
		cwd := tc.cwd
		if cwd == "" {
			cwd = tempGitRepo(t)
			commitFile(t, cwd, "go.mod", "module tmp\n\ngo 1.27\n")
			commitFile(t, cwd, "bad.go", "package tmp\n\nfunc  f( ) {\n}\n")
		}
		cmd := exec.Command(exe, tc.args...)
		cmd.Dir = cwd
		out, err := cmd.CombinedOutput()
		var ee *exec.ExitError
		if !errors.As(err, &ee) {
			t.Fatalf("%s: process did not exit: %v: %s", tc.name, err, out)
		}
		if ee.ExitCode() != tc.want {
			t.Fatalf("%s: exit = %d, want %d: %s", tc.name, ee.ExitCode(), tc.want, out)
		}
		if !strings.Contains(string(out), tc.out) {
			t.Fatalf("%s: output missing %q: %s", tc.name, tc.out, out)
		}
	}
}
