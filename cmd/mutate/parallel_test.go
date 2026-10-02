package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// scriptedRunner reports a fixed error per call, so tests can make the
// parallel workers behave differently from the serial confirmation pass.
type scriptedRunner struct {
	err error
}

func (r *scriptedRunner) runTest(ctx context.Context, pkg string) error { return r.err }

// suiteNeverRunner fails the test the moment any suite actually runs.
type suiteNeverRunner struct {
	t *testing.T
}

func (r *suiteNeverRunner) runTest(ctx context.Context, pkg string) error {
	r.t.Error("suite ran on a fully resumed population")
	return nil
}

// parallelFixture builds n module copies holding the same single-file
// package and returns the dirs plus the mutant population of dirs[0].
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
			return &scriptedRunner{err: errors.New("suite failed alone")}
		}
		return &scriptedRunner{}
	}
	var out strings.Builder
	res, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, newRunner, allows, nil)
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
	res, err := runMutationParallel(context.Background(), &out, dirs, []string{"./..."}, newRunner, nil, resume)
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
	serialRes, serr := runMutation(context.Background(), &serialOut, serialDir, []string{"."},
		execRunner{dir: serialDir, timeout: 60 * time.Second}, nil, nil)
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
	parallelRes, perr := runMutationParallel(context.Background(), &parallelOut, dirs, []string{"."},
		func(dir string) runner { return execRunner{dir: dir, timeout: 60 * time.Second} }, nil, nil)
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
