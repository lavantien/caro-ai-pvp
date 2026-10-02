package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeAllowFile(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mutate-allow")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) err = %v", path, err)
	}
	return path
}

func TestLoadAllowlistNone(t *testing.T) {
	for name, path := range map[string]string{
		"empty flag":   "",
		"missing file": filepath.Join(t.TempDir(), "missing"),
		"empty file":   writeAllowFile(t, ""),
	} {
		a, err := loadAllowlist(path)
		if err != nil {
			t.Errorf("%s: loadAllowlist err = %v, want nil", name, err)
		}
		if len(a) != 0 {
			t.Errorf("%s: %d allowances, want 0", name, len(a))
		}
	}
}

func TestLoadAllowlistEntries(t *testing.T) {
	path := writeAllowFile(t, "# equivalence proofs, one per line\n\n"+
		"internal/rules/win.go:42:15 replace 0 with 1 # proof one\n"+
		"  internal/rules/naive.go:129:13 replace 1 with 0  #  proof two  \n")
	a, err := loadAllowlist(path)
	if err != nil {
		t.Fatalf("loadAllowlist err = %v", err)
	}
	if len(a) != 2 {
		t.Fatalf("%d entries, want 2: %v", len(a), a)
	}
	for key, want := range map[string]string{
		"internal/rules/win.go:42:15 replace 0 with 1":    "proof one",
		"internal/rules/naive.go:129:13 replace 1 with 0": "proof two",
	} {
		if got := a[key]; got != want {
			t.Errorf("entry %q reason = %q, want %q", key, got, want)
		}
	}
}

func TestLoadAllowlistRejectsMalformed(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"missing separator", "a.go:1:1 replace 1 with 2\n"},
		{"empty reason", "a.go:1:1 replace 1 with 2 #   \n"},
		{"duplicate entry", "a.go:1:1 replace 1 with 2 # one\na.go:1:1 replace 1 with 2 # two\n"},
	} {
		if _, err := loadAllowlist(writeAllowFile(t, tc.body)); err == nil {
			t.Errorf("%s: loadAllowlist err = nil, want error", tc.name)
		}
	}
}

func TestLoadAllowlistUnreadable(t *testing.T) {
	if _, err := loadAllowlist(t.TempDir()); err == nil {
		t.Error("loadAllowlist(directory) err = nil, want read error")
	}
}

func TestMutationKeyMatchesReportLineAndAllowEntry(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	path, ms := writePkgFile(t, dir, "a.go", "p", src)
	m := findMutant(t, ms, path, 4, "replace + with -")
	key := m.key(dir)
	if key != "a.go:4:11 replace + with -" {
		t.Fatalf("key = %q, want a.go:4:11 replace + with -", key)
	}
	a, err := loadAllowlist(writeAllowFile(t, key+" # proof\n"))
	if err != nil {
		t.Fatalf("loadAllowlist err = %v", err)
	}
	if _, ok := a[key]; !ok {
		t.Errorf("allow entry written from the report line does not match mutant key %q", key)
	}
}

func TestExecuteMutantsAllowSurvivor(t *testing.T) {
	dir := t.TempDir()
	srcA := "package pa\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	srcB := "package pb\n\nfunc G(a int) int {\n\treturn a - 1\n}\n"
	pathA, msA := writePkgFile(t, dir, "a.go", "pa", srcA)
	pathB, msB := writePkgFile(t, dir, "b.go", "pb", srcB)
	ms := append(msA, msB...)
	allows := allowlist{}
	for _, m := range msB {
		allows[m.key(dir)] = "equivalent: " + m.desc
	}
	store := newFileStore()
	if err := store.snapshot([]string{pathA, pathB}); err != nil {
		t.Fatalf("snapshot err = %v", err)
	}
	r := &fakeRunner{failPkg: map[string]bool{"pa": true}}
	var out strings.Builder
	res, _, _, err := executeMutants(context.Background(), &out, dir, ms, store, r, allows, nil)
	if err != nil {
		t.Fatalf("executeMutants err = %v", err)
	}
	if res.killed != len(msA) || res.allowed != len(msB) || res.survived != 0 {
		t.Errorf("killed/allowed/survived = %d/%d/%d, want %d/%d/0", res.killed, res.allowed, res.survived, len(msA), len(msB))
	}
	s := out.String()
	if strings.Contains(s, " SURVIVED\n") {
		t.Errorf("allowed run must print no survivors:\n%s", s)
	}
	if !strings.Contains(s, "b.go:4:11 replace - with + ALLOWED # equivalent: replace - with +\n") {
		t.Errorf("output missing allowed line with reason:\n%s", s)
	}
	for _, tc := range []struct {
		path string
		want string
	}{
		{pathA, srcA},
		{pathB, srcB},
	} {
		if got, rerr := os.ReadFile(tc.path); rerr != nil || string(got) != tc.want {
			t.Errorf("%s not restored: %q, err %v", tc.path, got, rerr)
		}
	}
}

func TestRunMutationUnusedAllowEntryFails(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(go.mod) err = %v", err)
	}
	src := "package p\n\nfunc F(a int) int {\n\treturn a + 1\n}\n"
	_, ms := writePkgFile(t, dir, "a.go", "p", src)
	if len(ms) == 0 {
		t.Fatal("no mutants collected")
	}
	allows := allowlist{"a.go:99:9 replace 9 with 8": "stale proof"}
	var out strings.Builder
	res, err := runMutation(context.Background(), &out, dir, []string{"./..."}, &fakeRunner{failPkg: map[string]bool{"fixture": true}}, allows, nil)
	if err == nil || !strings.Contains(err.Error(), "unused allow entries") {
		t.Fatalf("err = %v, want unused allow entries failure", err)
	}
	if res.survived != 0 || res.allowed != 0 {
		t.Errorf("survived/allowed = %d/%d, want 0/0 (entry never matched)", res.survived, res.allowed)
	}
	if res.killed != res.total {
		t.Errorf("killed/total = %d/%d, want every mutant killed by the failing suite", res.killed, res.total)
	}
}

func TestRunCLIBadAllowlistExits2(t *testing.T) {
	path := writeAllowFile(t, "a.go:1:1 replace 1 with 2\n")
	var out, errOut strings.Builder
	code := runCLI([]string{"-pkgs", ".", "-timeout", "10s", "-allow", path}, &out, &errOut, t.TempDir())
	if code != 2 {
		t.Errorf("runCLI(bad allowlist) = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "want entry") {
		t.Errorf("stderr missing parse diagnostic: %q", errOut.String())
	}
}
