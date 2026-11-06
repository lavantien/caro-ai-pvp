package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func writeMsgFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestHookMainUsageErrors(t *testing.T) {
	cases := []struct {
		name  string
		argv0 string
		args  []string
		want  int
	}{
		{"no command", "hook.exe", nil, 2},
		{"unknown command", "hook.exe", []string{"bogus"}, 2},
		{"commit-msg without file", "hook.exe", []string{"commit-msg"}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var out, errOut bytes.Buffer
			if got := hookMain(tc.argv0, tc.args, &out, &errOut); got != tc.want {
				t.Fatalf("hookMain(%q, %v) = %d, want %d, stderr: %s", tc.argv0, tc.args, got, tc.want, errOut.String())
			}
			if errOut.Len() == 0 {
				t.Fatal("usage errors must print to stderr")
			}
		})
	}
}

func TestHookMainCommitMsg(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	good := writeMsgFile(t, "feat(hook): dispatch by argv0 basename\n\nbody line\n")
	if got := hookMain("hook.exe", []string{"commit-msg", good}, &out, &errOut); got != 0 {
		t.Fatalf("valid message exited %d, stderr: %s", got, errOut.String())
	}
	bad := writeMsgFile(t, "not a conventional subject\n")
	if got := hookMain("hook.exe", []string{"commit-msg", bad}, &out, &errOut); got != 1 {
		t.Fatalf("nonconforming message exited %d, want 1", got)
	}
	if got := hookMain("hook.exe", []string{"commit-msg", filepath.Join(t.TempDir(), "absent")}, &out, &errOut); got != 1 {
		t.Fatalf("unreadable message file exited %d, want 1", got)
	}
}

func TestHookMainPreCommitPassesOnEmptyDir(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if got := hookMain("pre-commit.exe", nil, &out, &errOut); got != 0 {
		t.Fatalf("pre-commit on empty dir exited %d, want 0, output: %s stderr: %s", got, out.String(), errOut.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("No recognized project type found.")) {
		t.Fatalf("empty dir must report no recognized project, got: %s", out.String())
	}
}

func TestHookMainInstallFailsOutsideModule(t *testing.T) {
	t.Chdir(t.TempDir())
	var out, errOut bytes.Buffer
	if got := hookMain("hook.exe", []string{"install"}, &out, &errOut); got != 1 {
		t.Fatalf("install outside a go module exited %d, want 1, stderr: %s", got, errOut.String())
	}
	if !bytes.Contains(errOut.Bytes(), []byte("hook: install:")) {
		t.Fatalf("install failure must name the failing phase, stderr: %s", errOut.String())
	}
}
