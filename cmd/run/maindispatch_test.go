package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func chdirModuleRoot(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tmp\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
}

func TestRunMainArgErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no target", nil, 2},
		{"bad var", []string{"build", "NOEQUALS"}, 2},
		{"unknown target", []string{"not-a-target"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			chdirModuleRoot(t)
			if got := runMain(tc.args); got != tc.want {
				t.Fatalf("runMain(%v) = %d, want %d", tc.args, got, tc.want)
			}
		})
	}
}

func TestRunMainRequiresModuleRoot(t *testing.T) {
	t.Chdir(t.TempDir())
	if got := runMain([]string{"build"}); got != 2 {
		t.Fatalf("runMain outside a module = %d, want 2", got)
	}
}

func TestRunMainFlowExitCode(t *testing.T) {
	dir := tempGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module tmp\n\ngo 1.27\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package tmp\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := exec.Command("git", "add", "x.go")
	c.Dir = dir
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git add x.go: %v: %s", err, out)
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, nil })
	if got := runMain([]string{"fmt"}); got != 0 {
		t.Fatalf("fmt flow with green steps = %d, want 0", got)
	}
	stubExecStep(t, func(s step, env []string) (int, error) { return 7, nil })
	if got := runMain([]string{"fmt"}); got != 7 {
		t.Fatalf("fmt flow with exit 7 = %d, want 7", got)
	}
}

func TestRunMainBuildTarget(t *testing.T) {
	chdirModuleRoot(t)
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, nil })
	if got := runMain([]string{"build"}); got != 0 {
		t.Fatalf("build target with green steps = %d, want 0", got)
	}
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, errors.New("spawn failed") })
	if got := runMain([]string{"build"}); got != 1 {
		t.Fatalf("build target with spawn error = %d, want 1", got)
	}
}

func TestExitCodeValue(t *testing.T) {
	if got := exitCodeValue(&exitCoder{code: 9, err: errors.New("x")}); got != 9 {
		t.Fatalf("exitCoder code = %d, want 9", got)
	}
	if got := exitCodeValue(syscall.Errno(5)); got != 5 {
		t.Fatalf("errno = %d, want 5", got)
	}
	if got := exitCodeValue(errors.New("plain")); got != 1 {
		t.Fatalf("plain error = %d, want 1", got)
	}
}
