package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	assertionFailOut = "--- FAIL: TestF (0.00s)\n    a_test.go:7: F(1) = 0, want 2\nFAIL\nFAIL\tprobe\t0.297s\nFAIL\n"
	panicFailOut     = "--- FAIL: TestPanics (0.00s)\npanic: assignment to entry in nil map [recovered, repanicked]\n\ngoroutine 7 [running]:\nFAIL\tprobe\t0.352s\nFAIL\n"
	buildFailOut     = "# probe [probe.test]\n.\\a.go:3:32: syntax error: unexpected semicolon, expected expression\nFAIL\tprobe [build failed]\nFAIL\n"
	testFileFailOut  = "# probe [probe.test]\n.\\a_test.go:9:12: invalid operation: division by zero\nFAIL\tprobe [build failed]\nFAIL\n"
	vetFailOut       = "# probe\n# [probe]\n.\\vet.go:5:24: fmt.Printf format %d has arg \"x\" of wrong type string\nFAIL\tprobe [build failed]\nFAIL\n"
	depFailOut       = "# example.com/dep\nexample.com\\dep\\den.go:3:11: invalid operation: operator - not defined on \"a\" (untyped string constant)\nFAIL\texample.com/use [build failed]\nFAIL\n"
	toolFailOut      = "# probe [probe.test]\ncompile: out of memory\nFAIL\tprobe [build failed]\nFAIL\n"
	mixedFailOut     = "--- FAIL: TestF (0.00s)\n    a_test.go:7: F(1) = 0, want 2\nFAIL\tprobe [build failed]\nFAIL\n"
	failfastOut      = "=== RUN   TestF\n--- FAIL: TestF (5.04s)\n    a_test.go:7: bot search never started\nFAIL\tprobe\t35.162s\nFAIL\n"
	timedOutOut      = "panic: test timed out after 1s\n\trunning tests:\n\t\tTestSlow (1s)\nFAIL\tprobe\t1.375s\nFAIL\n"
	workerPanicOut   = "panic: runtime error: slice bounds out of range [:1] with capacity 0\n\ngoroutine 68 [running]:\nFAIL\tprobe\t1.289s\nFAIL\n"
	greenSuiteOut    = "ok  probe 0.321s\n"
)

type fakeRunner struct {
	failPkg map[string]bool
	failOut string
	calls   []string
	onCall  func(ctx context.Context)
}

func (r *fakeRunner) runTest(ctx context.Context, pkg string) (string, error) {
	r.calls = append(r.calls, pkg)
	if r.onCall != nil {
		r.onCall(ctx)
	}
	if r.failPkg[pkg] {
		return r.failOut, &exec.ExitError{}
	}
	return "", nil
}

type infraRunner struct {
	calls int
}

func (r *infraRunner) runTest(context.Context, string) (string, error) {
	r.calls++
	return "", errors.New("exec: sh: not found")
}

func TestClassify(t *testing.T) {
	green := map[string]bool{"probe": true, "example.com/use": true}
	for _, tc := range []struct {
		name     string
		err      error
		out      string
		pkg      string
		baseline map[string]bool
		want     string
	}{
		{"green suite", nil, greenSuiteOut, "probe", green, verdictSurvived},
		{"assertion failure", &exec.ExitError{}, assertionFailOut, "probe", green, verdictKilled},
		{"panicking test carries the framework marker", &exec.ExitError{}, panicFailOut, "probe", green, verdictKilled},
		{"package build failure under a green baseline", &exec.ExitError{}, buildFailOut, "probe", green, verdictKilled},
		{"package build failure without a green baseline", &exec.ExitError{}, buildFailOut, "probe", nil, verdictRunError},
		{"package build failure for a foreign package key", &exec.ExitError{}, buildFailOut, "example.com/use", green, verdictRunError},
		{"test file build failure under a green baseline", &exec.ExitError{}, testFileFailOut, "probe", green, verdictKilled},
		{"vet rejection under a green baseline", &exec.ExitError{}, vetFailOut, "probe", green, verdictKilled},
		{"dependency build failure", &exec.ExitError{}, depFailOut, "example.com/use", green, verdictRunError},
		{"toolchain failure behind the build marker", &exec.ExitError{}, toolFailOut, "probe", green, verdictRunError},
		{"test failure signature outranks the build marker", &exec.ExitError{}, mixedFailOut, "probe", green, verdictKilled},
		{"first failure under failfast with the tail dropped", &exec.ExitError{}, failfastOut, "probe", green, verdictKilled},
		{"test timeout panic", &exec.ExitError{}, timedOutOut, "probe", green, verdictRunError},
		{"runtime panic on a background goroutine under a green baseline", &exec.ExitError{}, workerPanicOut, "probe", green, verdictKilled},
		{"runtime panic without a green baseline", &exec.ExitError{}, workerPanicOut, "probe", nil, verdictRunError},
		{"exit with no output", &exec.ExitError{}, "", "probe", green, verdictRunError},
		{"deadline with no output", context.DeadlineExceeded, "", "probe", green, verdictRunError},
	} {
		if got := classify(tc.err, tc.out, tc.pkg, tc.baseline); got != tc.want {
			t.Errorf("%s: classify = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestFileStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.go")
	orig := []byte("package p\n")
	if err := os.WriteFile(path, orig, 0o644); err != nil {
		t.Fatalf("WriteFile err = %v", err)
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("second snapshot err = %v", err)
	}
	if err := os.WriteFile(path, []byte("package q // corrupted\n"), 0o644); err != nil {
		t.Fatalf("corrupt err = %v", err)
	}
	if err := store.restoreAll(); err != nil {
		t.Fatalf("restoreAll err = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile err = %v", err)
	}
	if !bytes.Equal(got, orig) {
		t.Errorf("restored %q, want %q", got, orig)
	}
	if err := store.restoreAll(); err != nil {
		t.Fatalf("second restoreAll err = %v", err)
	}
	if got, err = os.ReadFile(path); err != nil || !bytes.Equal(got, orig) {
		t.Errorf("second restoreAll changed file: %q, err %v", got, err)
	}
}

func TestFileStoreRestoreMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gone.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile err = %v", err)
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove err = %v", err)
	}
	if err := store.restoreAll(); err == nil {
		t.Error("restoreAll on missing file err = nil, want error")
	}
}

func TestFileStoreSnapshotMissing(t *testing.T) {
	store := newFileStore()
	if err := store.snapshot([]string{filepath.Join(t.TempDir(), "missing.go")}); err == nil {
		t.Error("snapshot(missing) err = nil, want error")
	}
}

func writePkgFile(t *testing.T, dir, name, pkg, src string) (string, []mutation) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) err = %v", path, err)
	}
	ms := collectSource(t, path, src)
	for i := range ms {
		ms[i].file = path
		ms[i].pkg = pkg
	}
	return path, ms
}

func TestExecuteMutantsClassifiesAndRestores(t *testing.T) {
	dir := t.TempDir()
	srcA := "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	srcB := "package pb\n\nfunc G(a int) int {\n\treturn a - 1\n}\n"
	pathA, msA := writePkgFile(t, dir, "a.go", "pa", srcA)
	pathB, msB := writePkgFile(t, dir, "b.go", "pb", srcB)
	ms := append(msA, msB...)
	store := newFileStore()
	if err := store.snapshot([]string{pathA, pathB}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{failPkg: map[string]bool{"pa": true}, failOut: assertionFailOut}
	var out strings.Builder
	res, _, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, nil, nil, false)
	if err != nil {
		t.Fatalf("executeMutants err = %v", err)
	}
	if res.total != len(ms) || res.run != len(ms) {
		t.Errorf("res total/run = %d/%d, want %d/%d", res.total, res.run, len(ms), len(ms))
	}
	if res.killed != len(msA) || res.survived != len(msB) {
		t.Errorf("killed/survived = %d/%d, want %d/%d", res.killed, res.survived, len(msA), len(msB))
	}
	if len(r.calls) != len(ms) {
		t.Errorf("runner calls = %d, want %d", len(r.calls), len(ms))
	}
	s := out.String()
	if !strings.Contains(s, "a.go:4:11 replace + with - KILLED\n") {
		t.Errorf("output missing killed line:\n%s", s)
	}
	if !strings.Contains(s, " SURVIVED\n") {
		t.Errorf("output missing survived verdict:\n%s", s)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{path: pathA, want: srcA},
		{path: pathB, want: srcB},
	} {
		got, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatalf("ReadFile(%s) err = %v", tc.path, err)
		}
		if string(got) != tc.want {
			t.Errorf("%s not restored byte-for-byte: %q", tc.path, got)
		}
	}
}

func TestExecuteMutantsInterruptedBeforeStart(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) == 0 {
		t.Fatal("no mutants")
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := &fakeRunner{}
	res, _, _, _, err := executeMutants(ctx, &strings.Builder{}, dir, ms, store, r, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Errorf("err = %v, want interrupted", err)
	}
	if res.run != 0 {
		t.Errorf("run = %d, want 0", res.run)
	}
	if len(r.calls) != 0 {
		t.Errorf("runner calls = %d, want 0", len(r.calls))
	}
	got, rerr := os.ReadFile(path)
	if rerr != nil || !strings.Contains(string(got), "package p") {
		t.Errorf("file mutated despite interrupt: %q, err %v", got, rerr)
	}
}

func TestLoadResumeLog(t *testing.T) {
	log := "" +
		"CGO_ENABLED=1 go run ./cmd/mutate -allow .mutate-allow\n" +
		"internal/engine/tt.go:118:10 replace 0 with -1 KILLED\n" +
		"internal/engine/tt.go:118:10 replace 0 with 1 SURVIVED\n" +
		"internal/engine/bitboard.go:63:8 replace < with <= ALLOWED # proof text\n" +
		"mutate: 1483/1483 run, 1410 killed, 4 survived, 69 allowed\n" +
		"garbage line without verdict\n"
	got, err := loadResumeLog(strings.NewReader(log))
	if err != nil {
		t.Fatalf("loadResumeLog err = %v", err)
	}
	want := map[string]bool{"internal/engine/tt.go:118:10 replace 0 with -1": true}
	if len(got) != len(want) {
		t.Fatalf("loadResumeLog = %v, want only the killed key %v", got, want)
	}
	for k := range want {
		if !got[k] {
			t.Errorf("killed key %q missing from resume set", k)
		}
	}
}

func TestExecuteMutantsResumesKilledOnly(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 2 {
		t.Fatalf("need >= 2 mutants, got %d", len(ms))
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{}
	var out strings.Builder
	res, _, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, nil, map[string]bool{ms[1].key(dir): true}, false)
	if err != nil {
		t.Fatalf("executeMutants err = %v", err)
	}
	if res.total != len(ms) || res.run != len(ms) {
		t.Errorf("total/run = %d/%d, want %d/%d", res.total, res.run, len(ms), len(ms))
	}
	if res.killed != 1 || res.survived != len(ms)-1 {
		t.Errorf("killed/survived = %d/%d, want 1/%d", res.killed, res.survived, len(ms)-1)
	}
	if len(r.calls) != len(ms)-1 {
		t.Errorf("runner calls = %d, want %d: resumed keys must not re-run", len(r.calls), len(ms)-1)
	}
	s := out.String()
	if !strings.Contains(s, ms[1].key(dir)+" KILLED\n") {
		t.Errorf("output missing replayed killed line:\n%s", s)
	}
}

func TestExecuteMutantsResumeKillConsumesAllowEntry(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 3 {
		t.Fatalf("need >= 3 mutants, got %d", len(ms))
	}
	replay := ms[0].key(dir)
	fresh := ms[1].key(dir)
	allows := allowlist{replay: "triaged equivalence proof", fresh: "second proof"}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{}
	var out strings.Builder
	res, consumed, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, allows, map[string]bool{replay: true}, false)
	if err != nil {
		t.Fatalf("executeMutants err = %v", err)
	}
	if _, ok := consumed[replay]; !ok {
		t.Errorf("replayed kill did not consume its allow entry, consumed = %v", consumed)
	}
	if _, ok := consumed[fresh]; !ok {
		t.Errorf("fresh allowance not consumed, consumed = %v", consumed)
	}
	if res.killed != 1 || res.allowed != 1 || res.survived != len(ms)-2 {
		t.Errorf("killed/allowed/survived = %d/%d/%d, want 1/1/%d", res.killed, res.allowed, res.survived, len(ms)-2)
	}
	if len(r.calls) != len(ms)-2 {
		t.Errorf("runner calls = %d, want %d: replayed and allowed mutants must not re-run", len(r.calls), len(ms)-2)
	}
	if !strings.Contains(out.String(), replay+" KILLED\n") {
		t.Errorf("output missing the replayed kill line:\n%s", out.String())
	}
}

func TestRunMutationResumeConsumesAndReportsOnlyStale(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	_, ms := writePkgFile(t, dir, "a.go", "probe", "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 3 {
		t.Fatalf("need >= 3 mutants, got %d", len(ms))
	}
	replay := ms[0].key(dir)
	fresh := ms[1].key(dir)
	stale := "a.go:99:9 replace 9 with 8"
	resume, lerr := loadResumeLog(strings.NewReader(replay + " KILLED\n" + fresh + " SURVIVED\n" + "garbage line\n"))
	if lerr != nil {
		t.Fatalf("loadResumeLog err = %v", lerr)
	}
	var out strings.Builder
	_, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), &fakeRunner{},
		allowlist{replay: "triaged proof", fresh: "fresh proof", stale: "stale proof"}, resume, false)
	if err == nil || !strings.Contains(err.Error(), "1 unused allow entries") || !strings.Contains(err.Error(), stale) {
		t.Fatalf("err = %v, want exactly the stale entry reported unused", err)
	}
	if strings.Contains(err.Error(), replay) || strings.Contains(err.Error(), fresh) {
		t.Fatalf("err = %v, replayed and freshly judged entries must not be reported unused", err)
	}
	s := out.String()
	if !strings.Contains(s, replay+" KILLED\n") {
		t.Errorf("output missing the replayed kill line:\n%s", s)
	}
	if !strings.Contains(s, fresh+" ALLOWED # fresh proof\n") {
		t.Errorf("output missing the fresh allowed line:\n%s", s)
	}
	out.Reset()
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), &fakeRunner{},
		allowlist{replay: "triaged proof", fresh: "fresh proof"}, resume, false)
	if err != nil {
		t.Fatalf("runMutation(consumed entries) err = %v, want clean pass\n%s", err, out.String())
	}
	if res.killed != 1 || res.allowed != 1 || res.survived != len(ms)-2 {
		t.Errorf("killed/allowed/survived = %d/%d/%d, want 1/1/%d", res.killed, res.allowed, res.survived, len(ms)-2)
	}
}

func TestExecuteMutantsChallengeResumeOverlapIsChallenged(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 2 {
		t.Fatalf("need >= 2 mutants, got %d", len(ms))
	}
	overlapped := ms[0].key(dir)
	allows := allowlist{overlapped: "proof under audit"}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	var out strings.Builder
	res, consumed, _, challenged, err := executeMutants(context.Background(), &out, dir, ms, store, &fakeRunner{}, nil, allows, map[string]bool{overlapped: true}, true)
	if err != nil {
		t.Fatalf("executeMutants err = %v", err)
	}
	if len(challenged) != 1 || challenged[0].key(dir) != overlapped {
		t.Errorf("challenged = %v, want exactly the overlapped key", challenged)
	}
	if len(consumed) != 0 {
		t.Errorf("consumed = %v, want none: a challenge audit must not consume the entry on replay", consumed)
	}
	if res.killed != 0 || res.run != len(ms) {
		t.Errorf("killed/run = %d/%d, want 0/%d", res.killed, res.run, len(ms))
	}
	if !strings.Contains(out.String(), overlapped+" CHALLENGED\n") {
		t.Errorf("output missing the challenged verdict for the replayed kill:\n%s", out.String())
	}
}

func TestRunMutationChallengeResumeOverlapDemotes(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	_, ms := writePkgFile(t, dir, "a.go", "probe", "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 2 {
		t.Fatalf("need >= 2 mutants, got %d", len(ms))
	}
	overlapped := ms[0].key(dir)
	resume, lerr := loadResumeLog(strings.NewReader(overlapped + " KILLED\n"))
	if lerr != nil {
		t.Fatalf("loadResumeLog err = %v", lerr)
	}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), &fakeRunner{},
		allowlist{overlapped: "proof a prior pass killed"}, resume, true)
	if err == nil || !strings.Contains(err.Error(), "no longer equivalent") {
		t.Fatalf("err = %v, want the demotion: a serial challenge must not trust the replayed kill\n%s", err, out.String())
	}
	if res.allowed != 0 {
		t.Errorf("allowed = %d, want 0", res.allowed)
	}
	if !strings.Contains(out.String(), overlapped+" CHALLENGED\n") {
		t.Errorf("output missing the challenged verdict:\n%s", out.String())
	}
}

func TestDisplayPath(t *testing.T) {
	dir := t.TempDir()
	if got := displayPath(dir, filepath.Join(dir, "a.go")); got != "a.go" {
		t.Errorf("displayPath rel = %q, want a.go", got)
	}
	if got := displayPath(dir, filepath.Join("testdata", "a.go")); got == "" {
		t.Error("displayPath fallback empty")
	}
}

func TestExecuteMutantsBadEditsFail(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	ms[0].edits = []edit{{start: 99, end: 100, repl: "x"}}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	res, _, _, _, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, &fakeRunner{}, nil, nil, nil, false)
	if err == nil {
		t.Fatal("executeMutants(bad edits) err = nil, want error")
	}
	if res.run != 0 {
		t.Errorf("run = %d, want 0 (mutant never reached the test runner)", res.run)
	}
}

func TestExecuteMutantsMutantWriteFails(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatalf("Remove err = %v", err)
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("Mkdir err = %v", err)
	}
	res, _, _, _, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, &fakeRunner{}, nil, nil, nil, false)
	if err == nil {
		t.Fatal("executeMutants(unwritable target) err = nil, want write error")
	}
	if res.run != 0 {
		t.Errorf("run = %d, want 0", res.run)
	}
}

func TestExecuteMutantsRestoreWriteFails(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{onCall: func(context.Context) {
		if err := os.Remove(path); err != nil {
			t.Errorf("Remove err = %v", err)
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Errorf("Mkdir err = %v", err)
		}
	}}
	res, _, _, _, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, r, nil, nil, nil, false)
	if err == nil {
		t.Fatal("executeMutants(restore unwritable) err = nil, want write error")
	}
	if res.run != 1 {
		t.Errorf("run = %d, want 1 (failure strikes between mutant write and restore)", res.run)
	}
}

func TestFileStoreRestoreWriteFails(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.go")
	if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("WriteFile err = %v", err)
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	if err := os.WriteFile(path, []byte("package q\n"), 0o644); err != nil {
		t.Fatalf("drift err = %v", err)
	}
	if err := os.Chmod(path, 0o444); err != nil {
		t.Fatalf("Chmod err = %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	if err := store.restoreAll(); err == nil {
		t.Error("restoreAll(read-only target) err = nil, want write error")
	}
}

func TestExecRunnerCommandShape(t *testing.T) {
	r := execRunner{dir: t.TempDir(), timeout: 5 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := r.runTest(ctx, "example.com/p"); err == nil {
		t.Error("runTest with canceled ctx err = nil, want error")
	}
}

func TestRunTestWiresSuiteTimeoutIntoArgv(t *testing.T) {
	outer := 5 * time.Minute
	argv := testArgv(outer, "example.com/p")
	want := []string{"go", "test", "-count=1", "-short", "-v", "-failfast", "-timeout", config.SuiteTimeout(outer).String(), "example.com/p"}
	if !slices.Equal(argv, want) || config.SuiteTimeout(outer) == outer {
		t.Fatalf("argv = %v, want %v with the suite timeout derived below the outer window", argv, want)
	}
}

func TestExecuteMutantsInterruptMidRun(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	path, ms := writePkgFile(t, dir, "a.go", "p", src)
	if len(ms) < 2 {
		t.Fatalf("need >= 2 mutants, got %d", len(ms))
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &fakeRunner{onCall: func(context.Context) { cancel() }}
	res, _, _, _, err := executeMutants(ctx, &strings.Builder{}, dir, ms, store, r, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "interrupted") {
		t.Errorf("err = %v, want interrupted", err)
	}
	if res.run != 1 {
		t.Errorf("run = %d, want 1 (first mutant consumed by the interrupt)", res.run)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || string(got) != src {
		t.Errorf("file not restored after mid-run interrupt: %q, err %v", got, rerr)
	}
}

func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) err = %v", name, err)
		}
	}
	return dir
}

func TestDiscoverErrors(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	if _, err := discover(context.Background(), dir, []string{"./nope"}, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(missing) err = %v, want go list error", err)
	}
	dir = writeModule(t, map[string]string{"go.mod": "module probe\nbroken {\n"})
	if _, err := discover(context.Background(), dir, []string{"."}, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(broken go.mod) err = %v, want go list error", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discover(ctx, dir, []string{"."}, t.TempDir(), t.TempDir()); err == nil || strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(canceled ctx) err = %v, want raw exec error", err)
	}
}

func TestDiscoverNoGoFiles(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	if _, err := discover(context.Background(), dir, []string{"."}, t.TempDir(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "no Go files") {
		t.Errorf("discover(no go files) err = %v, want no Go files error", err)
	}
}

func TestExecuteMutantsInfraErrorVoidsRun(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	path, ms := writePkgFile(t, dir, "a.go", "p", src)
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &infraRunner{}
	res, _, _, _, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, r, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "infra failure") {
		t.Errorf("err = %v, want infra failure", err)
	}
	if res.killed != 0 {
		t.Errorf("killed = %d, want 0: a runner that never ran a test must not score kills", res.killed)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || string(got) != src {
		t.Errorf("file not restored after infra failure: %q, err %v", got, rerr)
	}
}

func TestRunMutationPrintsPlanLineBeforeVerdicts(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), &fakeRunner{}, nil, nil, false)
	if err != nil {
		t.Fatalf("runMutation err = %v", err)
	}
	if res.total < 1 {
		t.Fatalf("total = %d, want at least one mutant", res.total)
	}
	want := fmt.Sprintf("mutate: plan %d mutants over 1 package(s)\n", res.total)
	s := out.String()
	if !strings.Contains(s, want) {
		t.Fatalf("stdout missing plan line %q:\n%s", want, s)
	}
	planIdx := strings.Index(s, want)
	for _, marker := range []string{" SURVIVED\n", " KILLED\n", "mutate: baseline green"} {
		if idx := strings.Index(s, marker); idx >= 0 && idx < planIdx {
			t.Errorf("plan line must precede %q:\n%s", marker, s)
		}
	}
}

func TestRunMutationBaselineGuards(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	r := &fakeRunner{failPkg: map[string]bool{"probe": true}, failOut: assertionFailOut}
	_, err := runMutation(context.Background(), &strings.Builder{}, dir, []string{"."}, t.TempDir(), t.TempDir(), r, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "baseline") {
		t.Fatalf("runMutation(red baseline) err = %v, want baseline failure", err)
	}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), &fakeRunner{}, nil, nil, false)
	if err != nil {
		t.Fatalf("runMutation(green baseline) err = %v", err)
	}
	if !strings.Contains(out.String(), "mutate: baseline green for 1 package(s)\n") {
		t.Errorf("stdout missing baseline line:\n%s", out.String())
	}
	if res.run != res.total || res.survived != res.total {
		t.Errorf("run/survived = %d/%d, want %d/%d", res.run, res.survived, res.total, res.total)
	}
}

func TestRunCLIRunMutationFailure(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	var out, errOut strings.Builder
	if code := runCLI([]string{"-pkgs", "./nope", "-timeout", "10s"}, &out, &errOut, dir); code != 1 {
		t.Errorf("runCLI(bad pkg) = %d, want 1", code)
	}
	if errOut.Len() == 0 {
		t.Error("stderr empty, want go list failure message")
	}
}

func TestRunCLINoMutantsFails(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod":  "module probe\n\ngo 1.27\n",
		"decl.go": "package probe\n\ntype T struct {\n\tA int\n\tB string\n}\n",
	})
	var out, errOut strings.Builder
	if code := runCLI([]string{"-pkgs", ".", "-timeout", "10s"}, &out, &errOut, dir); code != 1 {
		t.Errorf("runCLI(no mutants) = %d, want 1 (gate must not pass green on nothing to mutate)", code)
	}
	if !strings.Contains(out.String(), "no mutants found") {
		t.Errorf("stdout missing no-mutants message: %q", out.String())
	}
}

func TestGoEnvOverrides(t *testing.T) {
	cache := t.TempDir()
	tmp := t.TempDir()
	t.Setenv("CGO_ENABLED", "0")
	t.Setenv("GOCACHE", "stale-shared-cache")
	t.Setenv("GOTMPDIR", "stale-shared-tmp")
	env := goEnv(cache, tmp)
	var cgo, gocache, gotmp []string
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_ENABLED=") {
			cgo = append(cgo, e)
		}
		if strings.HasPrefix(e, "GOCACHE=") {
			gocache = append(gocache, e)
		}
		if strings.HasPrefix(e, "GOTMPDIR=") {
			gotmp = append(gotmp, e)
		}
	}
	if len(cgo) != 1 || cgo[0] != "CGO_ENABLED=1" {
		t.Errorf("CGO_ENABLED entries = %v, want exactly [CGO_ENABLED=1]", cgo)
	}
	if len(gocache) != 1 || gocache[0] != "GOCACHE="+cache {
		t.Errorf("GOCACHE entries = %v, want exactly [GOCACHE=%s]", gocache, cache)
	}
	if len(gotmp) != 1 || gotmp[0] != "GOTMPDIR="+tmp {
		t.Errorf("GOTMPDIR entries = %v, want exactly [GOTMPDIR=%s]", gotmp, tmp)
	}
}

type failingReader struct {
	data []byte
	err  error
	done bool
}

func (r *failingReader) Read(p []byte) (int, error) {
	if !r.done {
		r.done = true
		n := copy(p, r.data)
		return n, nil
	}
	return 0, r.err
}

func TestLoadResumeLogReadFailure(t *testing.T) {
	_, err := loadResumeLog(&failingReader{data: []byte("a.go:1:1 d KILLED\n"), err: errors.New("disk gone")})
	if err == nil || !strings.Contains(err.Error(), "disk gone") {
		t.Errorf("loadResumeLog(failing reader) err = %v, want the read failure", err)
	}
}

func TestExecuteMutantsDeferredRestoreDriftSurfaces(t *testing.T) {
	dir := t.TempDir()
	path, ms := writePkgFile(t, dir, "a.go", "p", "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	bystander := filepath.Join(dir, "b.txt")
	mustWrite(t, bystander, "not go source\n")
	store := newFileStore()
	if err := store.snapshot([]string{path, bystander}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	if err := os.Remove(bystander); err != nil {
		t.Fatal(err)
	}
	_, _, _, _, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, &fakeRunner{}, nil, nil, nil, false)
	if err == nil {
		t.Fatal("executeMutants(drifted snapshot) err = nil, want the deferred restore failure")
	}
}

type baselineThenInfraRunner struct {
	calls int
}

func (r *baselineThenInfraRunner) runTest(context.Context, string) (string, error) {
	r.calls++
	if r.calls == 1 {
		return "", nil
	}
	return "", errors.New("exec: sh: not found")
}

type baselineThenExitErrRunner struct {
	calls int
}

func (r *baselineThenExitErrRunner) runTest(context.Context, string) (string, error) {
	r.calls++
	if r.calls == 1 {
		return "", nil
	}
	return "", &exec.ExitError{}
}

type baselineThenBuildFailRunner struct {
	calls int
}

func (r *baselineThenBuildFailRunner) runTest(context.Context, string) (string, error) {
	r.calls++
	if r.calls == 1 {
		return "", nil
	}
	return buildFailOut, &exec.ExitError{}
}

func TestRunBaselineFailureCarriesSuiteOutput(t *testing.T) {
	r := &fakeRunner{failPkg: map[string]bool{"probe": true}, failOut: "zz-baseline-marker: toolchain cache corrupt\n"}
	_, err := runBaseline(context.Background(), &strings.Builder{}, []mutation{{pkg: "probe"}}, r)
	if err == nil {
		t.Fatal("runBaseline(failing suite) err = nil, want the baseline failure")
	}
	if !strings.Contains(err.Error(), "zz-baseline-marker: toolchain cache corrupt") {
		t.Fatalf("baseline error lost the suite output: %v", err)
	}
}

func TestRunMutationMutantPhaseRunErrorPropagates(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	r := &baselineThenExitErrRunner{}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), r, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "run error") {
		t.Fatalf("runMutation(run error mid-gate) err = %v, want the run error", err)
	}
	if !strings.Contains(out.String(), " RUN ERROR\n") {
		t.Errorf("stdout missing the run error verdict:\n%s", out.String())
	}
	if res.total == 0 || res.run != 1 || res.killed != 0 {
		t.Errorf("total/run/killed = %d/%d/%d, want the first mutant attempted with no kill scored", res.total, res.run, res.killed)
	}
}

func TestRunMutationStillbornBuildFailureKills(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	r := &baselineThenBuildFailRunner{}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), r, nil, nil, false)
	if err != nil {
		t.Fatalf("runMutation(stillborn mutants) err = %v, want every package build failure scored killed\n%s", err, out.String())
	}
	if res.total == 0 || res.run != res.total || res.killed != res.total || res.survived != 0 {
		t.Errorf("total/run/killed/survived = %d/%d/%d/%d, want %d/%d/%d/0\n%s", res.total, res.run, res.killed, res.survived, res.total, res.total, res.total, out.String())
	}
	if !strings.Contains(out.String(), " KILLED\n") || strings.Contains(out.String(), " RUN ERROR\n") {
		t.Errorf("output missing killed verdicts:\n%s", out.String())
	}
}

func TestRunMutationChallengeStillbornDemotesAllowance(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	_, ms := writePkgFile(t, dir, "a.go", "probe", "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n")
	if len(ms) < 2 {
		t.Fatalf("need >= 2 mutants, got %d", len(ms))
	}
	stillborn := ms[0].key(dir)
	r := &baselineThenBuildFailRunner{}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"."}, t.TempDir(), t.TempDir(), r,
		allowlist{stillborn: "interim compile-fail entry"}, nil, true)
	if err == nil || !strings.Contains(err.Error(), "no longer equivalent") {
		t.Fatalf("err = %v, want the stillborn entry demoted out of the allowlist\n%s", err, out.String())
	}
	if strings.Contains(out.String(), " RUN ERROR\n") {
		t.Errorf("stillborn challenge must not void the run:\n%s", out.String())
	}
	if !strings.Contains(out.String(), stillborn+" CHALLENGED\n") {
		t.Errorf("output missing the challenged stillborn verdict:\n%s", out.String())
	}
	if res.allowed != 0 {
		t.Errorf("allowed = %d, want 0\n%s", res.allowed, out.String())
	}
}

func TestExecuteMutantsRunErrorVoidsPhase(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	path, ms := writePkgFile(t, dir, "a.go", "p", src)
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{failPkg: map[string]bool{"p": true}, failOut: buildFailOut}
	var out strings.Builder
	res, _, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "run error") {
		t.Fatalf("executeMutants(run error) err = %v, want the run error", err)
	}
	if res.killed != 0 {
		t.Errorf("killed = %d, want 0: without an established green baseline a build failure must not score a kill", res.killed)
	}
	if res.run != 1 {
		t.Errorf("run = %d, want 1 (the mutant was attempted)", res.run)
	}
	s := out.String()
	if !strings.Contains(s, " RUN ERROR\n") {
		t.Errorf("output missing the run error verdict line:\n%s", s)
	}
	if strings.Contains(s, " KILLED\n") {
		t.Errorf("output must not score a kill:\n%s", s)
	}
	if got, rerr := os.ReadFile(path); rerr != nil || string(got) != src {
		t.Errorf("file not restored after the run error: %q, err %v", got, rerr)
	}
}

func TestExecuteMutantsStillbornBuildFailureScoresKilled(t *testing.T) {
	dir := t.TempDir()
	src := "package probe\n\nfunc Greet(name string) string {\n\treturn \"hi \" + name\n}\n"
	path, ms := writePkgFile(t, dir, "greet.go", "probe", src)
	if len(ms) != 1 || ms[0].desc != "replace + with -" {
		t.Fatalf("fixture yielded %v, want exactly the string concat mutant", ms)
	}
	store := newFileStore()
	if err := store.snapshot([]string{path}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{failPkg: map[string]bool{"probe": true}, failOut: buildFailOut}
	var out strings.Builder
	res, _, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, map[string]bool{"probe": true}, nil, nil, false)
	if err != nil {
		t.Fatalf("executeMutants(stillborn) err = %v, want the build failure scored killed", err)
	}
	if res.killed != 1 || res.survived != 0 || res.run != 1 {
		t.Errorf("killed/survived/run = %d/%d/%d, want 1/0/1\n%s", res.killed, res.survived, res.run, out.String())
	}
	if !strings.Contains(out.String(), "greet.go:4:15 replace + with - KILLED\n") {
		t.Errorf("output missing the stillborn kill line:\n%s", out.String())
	}
	if got, rerr := os.ReadFile(path); rerr != nil || string(got) != src {
		t.Errorf("file not restored after the stillborn kill: %q, err %v", got, rerr)
	}
}

func TestRunMutationMutantPhaseInfraErrorPropagates(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	r := &baselineThenInfraRunner{}
	res, err := runMutation(context.Background(), &strings.Builder{}, dir, []string{"."}, t.TempDir(), t.TempDir(), r, nil, nil, false)
	if err == nil || !strings.Contains(err.Error(), "infra failure") {
		t.Fatalf("runMutation(infra mid-gate) err = %v, want the infra failure", err)
	}
	if res.total == 0 || res.run != 1 {
		t.Errorf("total/run = %d/%d, want the first mutant attempted", res.total, res.run)
	}
}

func TestDiscoverDeduplicatesRepeatedPattern(t *testing.T) {
	dir := writeModule(t, map[string]string{
		"go.mod": "module probe\n\ngo 1.27\n",
		"a.go":   "package probe\n\nfunc F(a int) int {\n\treturn a + 1\n}\n",
	})
	targets, err := discover(context.Background(), dir, []string{"./...", "."}, t.TempDir(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(targets) != 1 {
		t.Fatalf("discover(overlapping patterns) = %d targets, want 1", len(targets))
	}
}

func TestRunCLIRemovesPrivateDirs(t *testing.T) {
	before := len(cacheDirs(t)) + len(tmpDirs(t))
	dir := writeModule(t, map[string]string{
		"go.mod":  "module probe\n\ngo 1.27\n",
		"decl.go": "package probe\n\ntype T struct {\n\tA int\n\tB string\n}\n",
	})
	var out, errOut strings.Builder
	runCLI([]string{"-pkgs", ".", "-timeout", "10s"}, &out, &errOut, dir)
	if after := len(cacheDirs(t)) + len(tmpDirs(t)); after != before {
		t.Errorf("private run dirs: before %d, after %d, want equal (run-private cache and tmp must not outlive the run)", before, after)
	}
}

func cacheDirs(t *testing.T) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(os.TempDir(), "caro-mutate-cache-*"))
	if err != nil {
		t.Fatalf("glob cache dirs: %v", err)
	}
	return dirs
}

func tmpDirs(t *testing.T) []string {
	t.Helper()
	dirs, err := filepath.Glob(filepath.Join(os.TempDir(), "caro-mutate-tmp-*"))
	if err != nil {
		t.Fatalf("glob tmp dirs: %v", err)
	}
	return dirs
}
