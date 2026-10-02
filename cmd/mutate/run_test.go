package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type fakeRunner struct {
	failPkg map[string]bool
	calls   []string
	onCall  func(ctx context.Context)
}

func (r *fakeRunner) runTest(ctx context.Context, pkg string) error {
	r.calls = append(r.calls, pkg)
	if r.onCall != nil {
		r.onCall(ctx)
	}
	if r.failPkg[pkg] {
		return errors.New("test failed")
	}
	return nil
}

func TestClassify(t *testing.T) {
	if got := classify(nil); got != verdictSurvived {
		t.Errorf("classify(nil) = %q, want %q", got, verdictSurvived)
	}
	if got := classify(errors.New("exit 1")); got != verdictKilled {
		t.Errorf("classify(fail) = %q, want %q", got, verdictKilled)
	}
	if got := classify(context.DeadlineExceeded); got != verdictKilled {
		t.Errorf("classify(timeout) = %q, want %q", got, verdictKilled)
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
	r := &fakeRunner{failPkg: map[string]bool{"pa": true}}
	var out strings.Builder
	res, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, nil)
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
	res, err := executeMutants(ctx, &strings.Builder{}, dir, ms, store, r, nil, nil)
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
	got := loadResumeLog(strings.NewReader(log))
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
	res, err := executeMutants(context.Background(), &out, dir, ms, store, r, nil, map[string]bool{ms[1].key(dir): true})
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
	res, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, &fakeRunner{}, nil, nil)
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
	res, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, &fakeRunner{}, nil, nil)
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
	res, err := executeMutants(context.Background(), &strings.Builder{}, dir, ms, store, r, nil, nil)
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
	if err := r.runTest(ctx, "example.com/p"); err == nil {
		t.Error("runTest with canceled ctx err = nil, want error")
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
	res, err := executeMutants(ctx, &strings.Builder{}, dir, ms, store, r, nil, nil)
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
	if _, err := discover(context.Background(), dir, []string{"./nope"}); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(missing) err = %v, want go list error", err)
	}
	dir = writeModule(t, map[string]string{"go.mod": "module probe\nbroken {\n"})
	if _, err := discover(context.Background(), dir, []string{"."}); err == nil || !strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(broken go.mod) err = %v, want go list error", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := discover(ctx, dir, []string{"."}); err == nil || strings.Contains(err.Error(), "go list") {
		t.Errorf("discover(canceled ctx) err = %v, want raw exec error", err)
	}
}

func TestDiscoverNoGoFiles(t *testing.T) {
	dir := writeModule(t, map[string]string{"go.mod": "module probe\n\ngo 1.27\n"})
	if _, err := discover(context.Background(), dir, []string{"."}); err == nil || !strings.Contains(err.Error(), "no Go files") {
		t.Errorf("discover(no go files) err = %v, want no Go files error", err)
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
	t.Setenv("CGO_ENABLED", "0")
	env := goEnv()
	var cgo []string
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_ENABLED=") {
			cgo = append(cgo, e)
		}
	}
	if len(cgo) != 1 || cgo[0] != "CGO_ENABLED=1" {
		t.Errorf("CGO_ENABLED entries = %v, want exactly [CGO_ENABLED=1]", cgo)
	}
}
