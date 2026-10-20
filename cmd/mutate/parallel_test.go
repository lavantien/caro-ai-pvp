package main

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedRunner struct {
	err error
}

func (r *scriptedRunner) runTest(ctx context.Context, pkg string) error { return r.err }

type greenThenFailRunner struct {
	calls int
}

func (r *greenThenFailRunner) runTest(context.Context, string) error {
	r.calls++
	if r.calls == 1 {
		return nil
	}
	return &exec.ExitError{}
}

type suiteNeverRunner struct {
	t *testing.T
}

func (r *suiteNeverRunner) runTest(ctx context.Context, pkg string) error {
	r.t.Error("suite ran on a fully resumed population")
	return nil
}

func parallelFixture(t *testing.T, n int, src string) ([]string, []mutation) {
	t.Helper()
	dirs := make([]string, n)
	for i := range dirs {
		dirs[i] = t.TempDir()
		if werr := os.WriteFile(filepath.Join(dirs[i], "go.mod"), []byte("module fixture\n\ngo 1.27\n"), 0o644); werr != nil {
			t.Fatalf("WriteFile(go.mod) err = %v", werr)
		}
		if werr := os.WriteFile(filepath.Join(dirs[i], "a.go"), []byte(src), 0o644); werr != nil {
			t.Fatalf("WriteFile(a.go) err = %v", werr)
		}
	}
	path, ms := writePkgFile(t, dirs[0], "a.go", "pa", src)
	if len(ms) == 0 {
		t.Fatal("no mutants collected")
	}
	_ = path
	return dirs, ms
}

func TestRewriteMutantsPreservesKeys(t *testing.T) {
	dirs, ms := parallelFixture(t, 2, "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	rewritten := rewriteMutants(ms, dirs[0], dirs[1])
	for i := range ms {
		if got, want := rewritten[i].key(dirs[1]), ms[i].key(dirs[0]); got != want {
			t.Errorf("rewritten key %q != original %q", got, want)
		}
		if rewritten[i].file == ms[i].file {
			t.Errorf("file path not rewritten: %s", ms[i].file)
		}
	}
}

func TestParallelSerialConfirmFlipsLoadLostKills(t *testing.T) {
	src := "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	dirs, ms := parallelFixture(t, 2, src)
	if len(ms) < 3 {
		t.Fatalf("fixture yielded %d mutants, want at least 3", len(ms))
	}
	var mu sync.Mutex
	invocations := map[string]int{}
	allows := allowlist{ms[0].key(dirs[0]): "probe fixture: scripted equivalence"}
	newRunner := func(dir string) runner {
		mu.Lock()
		invocations[dir]++
		n := invocations[dir]
		mu.Unlock()
		if dir == dirs[0] && n > 1 {
			return &scriptedRunner{err: &exec.ExitError{}}
		}
		return &scriptedRunner{}
	}
	var out strings.Builder
	res, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, allows, nil, false)
	if err != nil {
		t.Fatalf("runMutationParallel err = %v\noutput:\n%s", err, out.String())
	}
	if res.total != len(ms) || res.run != len(ms) {
		t.Errorf("total/run = %d/%d, want %d/%d", res.total, res.run, len(ms), len(ms))
	}
	if res.survived != 0 {
		t.Errorf("survived = %d, want 0: the serial pass must flip every load-lost kill, output:\n%s", res.survived, out.String())
	}
	if res.killed != len(ms)-1 {
		t.Errorf("killed = %d, want %d", res.killed, len(ms)-1)
	}
	if res.allowed != 1 {
		t.Errorf("allowed = %d, want 1", res.allowed)
	}
}

func TestParallelResumeReplaysWithoutRunning(t *testing.T) {
	dirs, ms := parallelFixture(t, 2, "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	resume := map[string]bool{}
	for _, m := range ms {
		resume[m.key(dirs[0])] = true
	}
	newRunner := func(dir string) runner {
		return &suiteNeverRunner{t: t}
	}
	var out strings.Builder
	res, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, nil, resume, false)
	if err != nil {
		t.Fatalf("runMutationParallel err = %v", err)
	}
	if res.killed != len(ms) || res.survived != 0 || res.run != len(ms) {
		t.Errorf("res = %+v, want all %d replayed as killed", res, len(ms))
	}
}

func TestParallelEndToEndMatchesSerial(t *testing.T) {
	files := map[string]string{
		"go.mod":        "module fixture\n\ngo 1.27\n",
		"arith.go":      mustRead(t, filepath.Join("testdata", "arith.go")),
		"arith_test.go": mustRead(t, filepath.Join("testdata", "arith_test.go")),
	}
	writeAll := func(dir string) {
		t.Helper()
		for name, content := range files {
			if werr := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); werr != nil {
				t.Fatalf("WriteFile(%s) err = %v", name, werr)
			}
		}
	}
	serialDir := t.TempDir()
	writeAll(serialDir)
	var serialOut strings.Builder
	serialRes, serr := runMutation(context.Background(), &serialOut, serialDir, []string{"."}, t.TempDir(),
		execRunner{dir: serialDir, cacheDir: t.TempDir(), timeout: 60 * time.Second}, nil, nil, false)
	if serr != nil {
		t.Fatalf("serial err = %v", serr)
	}
	if serialRes.total == 0 {
		t.Fatal("fixture produced no mutants")
	}
	dirs := []string{t.TempDir(), t.TempDir()}
	for _, d := range dirs {
		writeAll(d)
	}
	var parallelOut strings.Builder
	parallelRes, perr := runMutationParallel(context.Background(), &parallelOut, dirs, []string{"."}, t.TempDir(),
		func(dir string) runner { return execRunner{dir: dir, cacheDir: t.TempDir(), timeout: 60 * time.Second} }, nil, nil, false)
	if perr != nil {
		t.Fatalf("parallel err = %v\n%s", perr, parallelOut.String())
	}
	if serialRes != parallelRes {
		t.Errorf("serial %+v != parallel %+v\nserial:\n%s\nparallel:\n%s", serialRes, parallelRes, serialOut.String(), parallelOut.String())
	}
	if got := mustRead(t, filepath.Join(dirs[0], "arith.go")); got != files["arith.go"] {
		t.Error("parallel run left arith.go mutated")
	}
}

func TestChallengeDemotesKilledAllowance(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) err = %v", err)
	}
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	_, ms := writePkgFile(t, dir, "a.go", "p", src)
	allows := allowlist{ms[0].key(dir): "proof that no longer holds"}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"./..."}, t.TempDir(),
		&greenThenFailRunner{}, allows, nil, true)
	if err == nil || !strings.Contains(err.Error(), "no longer equivalent") {
		t.Fatalf("err = %v, want a demotion failure\n%s", err, out.String())
	}
	if strings.Contains(out.String(), " ALLOWED ") {
		t.Errorf("demoted entry must not be allowed:\n%s", out.String())
	}
	if res.allowed != 0 {
		t.Errorf("allowed = %d, want 0", res.allowed)
	}
}

func TestChallengeSurvivesConsumesAllowance(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) err = %v", err)
	}
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	_, ms := writePkgFile(t, dir, "a.go", "p", src)
	allows := allowlist{ms[0].key(dir): "sound proof"}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"./..."}, t.TempDir(), &fakeRunner{}, allows, nil, true)
	if err != nil {
		t.Fatalf("err = %v\n%s", err, out.String())
	}
	if res.allowed != 1 || res.survived != len(ms)-1 {
		t.Errorf("allowed/survived = %d/%d, want 1/%d", res.allowed, res.survived, len(ms)-1)
	}
	if !strings.Contains(out.String(), " ALLOWED # sound proof\n") {
		t.Errorf("missing allowed line:\n%s", out.String())
	}
}

func TestParallelChallengeSerialResolution(t *testing.T) {
	src := "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	for _, tc := range []struct {
		name       string
		resolveErr error
		wantErr    string
	}{
		{"serial survive consumes the allowance", nil, ""},
		{"serial kill demotes the allowance", &exec.ExitError{}, "no longer equivalent"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dirs, ms := parallelFixture(t, 2, src)
			allows := allowlist{ms[0].key(dirs[0]): "proof under test"}
			var mu sync.Mutex
			invocations := map[string]int{}
			newRunner := func(dir string) runner {
				mu.Lock()
				invocations[dir]++
				n := invocations[dir]
				mu.Unlock()
				if dir == dirs[0] && n == 1 {
					return &scriptedRunner{}
				}
				if dir == dirs[0] && n >= 3 {
					return &scriptedRunner{err: tc.resolveErr}
				}
				return &scriptedRunner{err: &exec.ExitError{}}
			}
			var out strings.Builder
			res, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, allows, nil, true)
			if !strings.Contains(out.String(), " CHALLENGED\n") {
				t.Fatalf("missing challenged verdict:\n%s", out.String())
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("err = %v, want clean resolution\n%s", err, out.String())
				}
				if res.allowed != 1 {
					t.Errorf("allowed = %d, want 1\n%s", res.allowed, out.String())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want demotion after serial kill\n%s", err, out.String())
			}
		})
	}
}

func TestConfirmInterruptIsNotAKill(t *testing.T) {
	src := "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	dirs, _ := parallelFixture(t, 2, src)
	ctx, cancel := context.WithCancel(context.Background())
	var mu sync.Mutex
	invocations := map[string]int{}
	newRunner := func(dir string) runner {
		mu.Lock()
		invocations[dir]++
		n := invocations[dir]
		mu.Unlock()
		if dir == dirs[0] && n >= 2 {
			cancel()
			return &scriptedRunner{err: context.Canceled}
		}
		return &scriptedRunner{}
	}
	var out strings.Builder
	res, err := runMutationParallel(ctx, &out, dirs, []string{"./..."}, t.TempDir(), newRunner, nil, nil, false)
	if err == nil {
		t.Fatalf("interrupted confirm must surface an error, got clean result %+v\n%s", res, out.String())
	}
	if strings.Contains(out.String(), " KILLED\n") {
		t.Errorf("canceled confirm suite classified as killed:\n%s", out.String())
	}
}

func TestRewriteMutantsLeavesAlienPaths(t *testing.T) {
	ms := []mutation{{file: filepath.Join("rel", "a.go"), line: 1, col: 2, desc: "d"}}
	got := rewriteMutants(ms, t.TempDir(), t.TempDir())
	if got[0].file != ms[0].file {
		t.Errorf("rewrite moved an alien path: %q, want %q", got[0].file, ms[0].file)
	}
}

func TestParallelDiscoverFailurePropagates(t *testing.T) {
	dirs := []string{writeModule(t, map[string]string{"go.mod": "module probe\nbroken {\n"}), t.TempDir()}
	_, err := runMutationParallel(context.Background(), &strings.Builder{}, dirs, []string{"./..."}, t.TempDir(),
		func(string) runner { return &scriptedRunner{} }, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "go list") {
		t.Fatalf("runMutationParallel(broken module) err = %v, want the go list failure", err)
	}
}

func TestParallelBaselineFailurePropagates(t *testing.T) {
	dirs, _ := parallelFixture(t, 2, "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	_, err := runMutationParallel(context.Background(), &strings.Builder{}, dirs, []string{"./..."}, t.TempDir(),
		func(string) runner { return &scriptedRunner{err: &exec.ExitError{}} }, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("runMutationParallel(red baseline) err = %v, want the baseline failure", err)
	}
}

func TestParallelWorkerSnapshotFailureAborts(t *testing.T) {
	dirs, _ := parallelFixture(t, 2, "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	dirs[1] = t.TempDir()
	var out strings.Builder
	_, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(),
		func(string) runner { return &scriptedRunner{} }, nil, nil, false)
	if err == nil {
		t.Fatalf("runMutationParallel(missing worker copy) err = nil, want the aborted run:\n%s", out.String())
	}
	if !strings.Contains(err.Error(), "worker 1") && !strings.Contains(err.Error(), "interrupted") {
		t.Fatalf("runMutationParallel(missing worker copy) err = %v, want the worker snapshot failure or its cancel:\n%s", err, out.String())
	}
}

func TestParallelConfirmPassInfraErrorFails(t *testing.T) {
	dirs, _ := parallelFixture(t, 2, "package pa\n\nfunc F() bool {\n\treturn true\n}\n")
	var mu sync.Mutex
	invocations := map[string]int{}
	newRunner := func(dir string) runner {
		mu.Lock()
		invocations[dir]++
		n := invocations[dir]
		mu.Unlock()
		if dir == dirs[0] && n >= 3 {
			return &infraRunner{}
		}
		return &scriptedRunner{}
	}
	var out strings.Builder
	_, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "infra failure reverifying") {
		t.Fatalf("runMutationParallel(confirm infra) err = %v, want the reverify failure:\n%s", err, out.String())
	}
}

func TestParallelUnusedAllowFails(t *testing.T) {
	dirs, _ := parallelFixture(t, 2, "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	allows := allowlist{"nope.go:1:1 replace 1 with 2": "stale proof for a mutant that no longer exists"}
	var mu sync.Mutex
	invocations := map[string]int{}
	newRunner := func(dir string) runner {
		mu.Lock()
		invocations[dir]++
		n := invocations[dir]
		mu.Unlock()
		if dir == dirs[0] && n == 1 {
			return &scriptedRunner{}
		}
		return &scriptedRunner{err: &exec.ExitError{}}
	}
	var out strings.Builder
	_, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, allows, nil, false)
	if err == nil || !strings.Contains(err.Error(), "nope.go") {
		t.Fatalf("runMutationParallel(unused allowance) err = %v, want the unused-allow failure:\n%s", err, out.String())
	}
}

func TestParallelChallengeResolveInfraErrorFails(t *testing.T) {
	src := "package pa\n\nfunc F() bool {\n\treturn true\n}\n"
	dirs, ms := parallelFixture(t, 2, src)
	if len(ms) != 1 {
		t.Fatalf("fixture yielded %d mutants, want exactly 1", len(ms))
	}
	allows := allowlist{ms[0].key(dirs[0]): "proof under test"}
	var mu sync.Mutex
	invocations := map[string]int{}
	newRunner := func(dir string) runner {
		mu.Lock()
		invocations[dir]++
		n := invocations[dir]
		mu.Unlock()
		if dir == dirs[0] && n == 1 {
			return &scriptedRunner{}
		}
		if dir == dirs[0] && n >= 3 {
			return &infraRunner{}
		}
		return &scriptedRunner{err: &exec.ExitError{}}
	}
	var out strings.Builder
	_, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, t.TempDir(), newRunner, allows, nil, true)
	if !strings.Contains(out.String(), " CHALLENGED\n") {
		t.Fatalf("missing challenged verdict:\n%s", out.String())
	}
	if err == nil || !strings.Contains(err.Error(), "infra failure reverifying") {
		t.Fatalf("runMutationParallel(resolve infra) err = %v, want the reverify failure:\n%s", err, out.String())
	}
}

func TestReverifySeriallySnapshotFailure(t *testing.T) {
	dir := t.TempDir()
	ms := []mutation{{file: filepath.Join(dir, "missing.go"), pkg: "p", line: 1, col: 1, desc: "d", edits: []edit{{start: 0, end: 1, repl: "x"}}}}
	if err := reverifySerially(context.Background(), dir, ms, &scriptedRunner{}, nil); err == nil {
		t.Error("reverifySerially(missing file) err = nil, want the snapshot failure")
	}
}

func TestReverifySeriallyInterruptedBeforeLoop(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := reverifySerially(ctx, dir, ms, &scriptedRunner{}, nil)
	if err == nil || !errors.Is(err, errInterrupted) {
		t.Errorf("reverifySerially(dead ctx) err = %v, want interrupted", err)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || !strings.Contains(string(got), "package p") {
		t.Errorf("file mutated despite the interrupt: %q, err %v", got, rerr)
	}
}

func TestReverifySeriallyBadEdits(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	ms[0].edits = []edit{{start: 99, end: 100, repl: "x"}}
	err := reverifySerially(context.Background(), dir, ms, &scriptedRunner{}, nil)
	if err == nil || !strings.Contains(err.Error(), "bad or overlapping edit") {
		t.Errorf("reverifySerially(bad edits) err = %v, want the edit failure", err)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || !strings.Contains(string(got), "package p") {
		t.Errorf("file mutated despite the edit failure: %q, err %v", got, rerr)
	}
}

func TestReverifySeriallyMutantWriteFails(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if err := reverifySerially(context.Background(), dir, ms, &scriptedRunner{}, nil); err == nil {
		t.Error("reverifySerially(read-only target) err = nil, want the mutant write failure")
	}
}

func TestReverifySeriallyRestoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	r := &fakeRunner{onCall: func(context.Context) {
		if err := os.Remove(path); err != nil {
			t.Errorf("Remove err = %v", err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Errorf("Mkdir err = %v", err)
		}
	}}
	if err := reverifySerially(context.Background(), dir, ms, r, nil); err == nil {
		t.Error("reverifySerially(restore unwritable) err = nil, want the write failure")
	}
}

func TestReverifySeriallyInterruptAfterTest(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	ctx, cancel := context.WithCancel(context.Background())
	r := &fakeRunner{onCall: func(context.Context) { cancel() }}
	err := reverifySerially(ctx, dir, ms, r, nil)
	if err == nil || !errors.Is(err, errInterrupted) {
		t.Errorf("reverifySerially(cancel after test) err = %v, want interrupted", err)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || !strings.Contains(string(got), "package p") {
		t.Errorf("file not restored after the interrupt: %q, err %v", got, rerr)
	}
}

func TestReverifySeriallyInfraErrorVoidsVerdict(t *testing.T) {
	dir := t.TempDir()
	_, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	err := reverifySerially(context.Background(), dir, ms, &infraRunner{}, nil)
	if err == nil || !strings.Contains(err.Error(), "infra failure reverifying") {
		t.Errorf("reverifySerially(infra runner) err = %v, want the infra failure", err)
	}
}
