package hook

import (
	"bytes"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

func newRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, a := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "hook@test"},
		{"config", "user.name", "hooktest"},
		{"commit", "--allow-empty", "--no-verify", "-q", "-m", "init"},
	} {
		c := exec.Command("git", a...)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", a, err, out)
		}
	}
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func gitIn(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return string(out)
}

func buildFakeTool(t *testing.T, dir, name string, code int, prints ...string) string {
	t.Helper()
	src := filepath.Join(dir, name+".go")
	if err := os.WriteFile(src, []byte(fmtExit(code, prints...)), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, src)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake %s: %v: %s", name, err, out)
	}
	return exe
}

func fmtExit(code int, prints ...string) string {
	src := "package main\n\nimport \"os\"\n\nfunc main() {\n"
	if len(prints) > 0 {
		src = "package main\n\nimport (\n\t\"fmt\"\n\t\"os\"\n)\n\nfunc main() {\n"
		for _, p := range prints {
			src += "\tfmt.Println(" + strconv.Quote(p) + ")\n"
		}
	}
	digits := "0123456789"
	s := ""
	n := code
	if n == 0 {
		s = "0"
	}
	for n > 0 {
		s = string(digits[n%10]) + s
		n /= 10
	}
	return src + "\tos.Exit(" + s + ")\n}\n"
}

func writeVendorTool(t *testing.T, dir, tool string, code int) {
	t.Helper()
	name := tool + ".bat"
	content := "@echo off\r\nexit /b " + strconv.Itoa(code) + "\r\n"
	mode := 0o644
	if runtime.GOOS != "windows" {
		name = tool
		content = "#!/bin/sh\nexit " + strconv.Itoa(code) + "\n"
		mode = 0o755
	}
	bin := filepath.Join(dir, "vendor", "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, name), []byte(content), fs.FileMode(mode)); err != nil {
		t.Fatal(err)
	}
}

func envWith(t *testing.T, extraDir string) []string {
	t.Helper()
	env := os.Environ()
	for i, kv := range env {
		k, v, ok := strings.Cut(kv, "=")
		if ok && strings.EqualFold(k, "PATH") {
			env[i] = k + "=" + extraDir + string(os.PathListSeparator) + v
			return env
		}
	}
	return append(env, "PATH="+extraDir)
}

func runHook(t *testing.T, dir string, env []string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := Run(dir, &buf, env)
	return buf.String(), err
}

func TestStagedFilesFilterACMRAndExtension(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "a.go", "package x\n")
	writeFile(t, dir, "b.go", "package y\n")
	writeFile(t, dir, "c.txt", "hello\n")
	gitIn(t, dir, "add", ".")
	gitIn(t, dir, "commit", "--no-verify", "-q", "-m", "base")
	writeFile(t, dir, "a.go", "package x\nvar z = 1\n")
	rm := exec.Command("git", "rm", "-q", "b.go")
	rm.Dir = dir
	if out, err := rm.CombinedOutput(); err != nil {
		t.Fatalf("git rm: %v: %s", err, out)
	}
	gitIn(t, dir, "add", "a.go")
	files, err := StagedFiles(os.Environ(), dir, `\.go$`)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "a.go" {
		t.Fatalf("staged go files = %v, want [a.go]", files)
	}
	gitIn(t, dir, "commit", "--no-verify", "-q", "-m", "step")
	gitIn(t, dir, "mv", "a.go", "z.go")
	files, err = StagedFiles(os.Environ(), dir, `\.go$`)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0] != "z.go" {
		t.Fatalf("staged renamed go files = %v, want [z.go]", files)
	}
}

func TestSpellGateBlocksOnTyposFailure(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "typos", 2)
	writeFile(t, dir, "clean.txt", "hello world\n")
	gitIn(t, dir, "add", "clean.txt")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with failing typos, out:\n%s", out)
	}
	if !strings.Contains(out, "Spelling errors found in staged files.") {
		t.Fatalf("output missing spell failure line:\n%s", out)
	}
}

func TestPassesWithoutProjectTypes(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "note.txt", "the quick brown fox\n")
	gitIn(t, dir, "add", "note.txt")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "All checks passed!") {
		t.Fatalf("output missing pass line:\n%s", out)
	}
	if strings.Contains(out, "=== Go ===") {
		t.Fatalf("unexpected go section without go.mod:\n%s", out)
	}
}

func TestGoFormatterRestagesStagedFiles(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "go.mod", "module tmp\n\ngo 1.27\n")
	gitIn(t, dir, "add", "go.mod")
	gitIn(t, dir, "commit", "--no-verify", "-q", "-m", "mod")
	writeFile(t, dir, "a.go", "package tmp\n\nfunc  f( ) {\n}\n")
	gitIn(t, dir, "add", "a.go")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "=== Go ===") {
		t.Fatalf("output missing go section:\n%s", out)
	}
	staged := gitIn(t, dir, "show", ":a.go")
	if strings.Contains(staged, "func  f( )") {
		t.Fatalf("staged file still unformatted:\n%s", staged)
	}
	if !strings.Contains(staged, "func f()") {
		t.Fatalf("staged file not reformatted:\n%s", staged)
	}
}

func stageGoFile(t *testing.T, dir string) {
	t.Helper()
	writeFile(t, dir, "go.mod", "module tmp\n\ngo 1.27\n")
	gitIn(t, dir, "add", "go.mod")
	gitIn(t, dir, "commit", "--no-verify", "-q", "-m", "mod")
	writeFile(t, dir, "a.go", "package tmp\n")
	gitIn(t, dir, "add", "a.go")
}

func TestGoLintCleanRunIsSilent(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "golangci-lint", 0)
	stageGoFile(t, dir)
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean lint: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running golangci-lint...") {
		t.Fatalf("golangci-lint step did not run:\n%s", out)
	}
	if strings.Contains(out, "Lint warnings found.") {
		t.Fatalf("clean lint run printed a warning:\n%s", out)
	}
}

func TestGoLintFindingsAreVisibleAndAdvertised(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "golangci-lint", 1, "a.go:1:1: boom (errcheck)")
	stageGoFile(t, dir)
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on lint findings: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Lint warnings found.") {
		t.Fatalf("output missing lint warning line:\n%s", out)
	}
	if !strings.Contains(out, "a.go:1:1: boom (errcheck)") {
		t.Fatalf("lint findings hidden from output:\n%s", out)
	}
}

func TestGoLintToolFailureIsNotAWarning(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "golangci-lint", 3)
	stageGoFile(t, dir)
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on lint tool failure: %v\n%s", err, out)
	}
	if strings.Contains(out, "Lint warnings found.") {
		t.Fatalf("tool failure reported as warnings:\n%s", out)
	}
	if !strings.Contains(out, "golangci-lint exited 3.") {
		t.Fatalf("output missing honest exit line:\n%s", out)
	}
}

func TestShellcheckBlocksStagedShellFiles(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "shellcheck", 1)
	writeFile(t, dir, "bad.sh", "echo nope\n")
	gitIn(t, dir, "add", "bad.sh")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with failing shellcheck, out:\n%s", out)
	}
	if !strings.Contains(out, "shellcheck found errors in staged shell files.") {
		t.Fatalf("output missing shellcheck failure line:\n%s", out)
	}
}

func TestPowershellBranchSkipsWithoutModule(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "broken.ps1", "function {\n")
	gitIn(t, dir, "add", "broken.ps1")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed on staged ps1: %v\n%s", err, out)
	}
}

func TestInstallSetsLocalHooksPath(t *testing.T) {
	_, thisFile, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(thisFile), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root not found at %s: %v", root, err)
	}
	if err := Install(root); err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(gitIn(t, root, "config", "--local", "--get", "core.hooksPath"))
	want := filepath.Join(root, ".githooks")
	if got != want {
		t.Fatalf("core.hooksPath = %q, want %q", got, want)
	}
	for _, name := range []string{"pre-commit.exe", "commit-msg.exe"} {
		if _, err := os.Stat(filepath.Join(root, ".githooks", name)); err != nil {
			t.Fatalf("missing hook exe %s: %v", name, err)
		}
	}
}

func writeMsgFile(t *testing.T, msg string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "msg")
	if err := os.WriteFile(p, []byte(msg), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCommitMsgAcceptsConventionalSubject(t *testing.T) {
	p := writeMsgFile(t, "feat(api): add the thing\n\nbody line\n")
	var buf bytes.Buffer
	if err := CommitMsg(p, &buf); err != nil {
		t.Fatalf("valid message rejected: %v\n%s", err, buf.String())
	}
}

func TestCommitMsgRejectsBadSubject(t *testing.T) {
	p := writeMsgFile(t, "add the thing\n")
	var buf bytes.Buffer
	err := CommitMsg(p, &buf)
	if err == nil {
		t.Fatal("bad subject accepted")
	}
	if !strings.Contains(buf.String(), "does not follow Conventional Commits format") {
		t.Fatalf("output missing format error:\n%s", buf.String())
	}
}

func TestCommitMsgSkipsMergeAndRevert(t *testing.T) {
	for _, subject := range []string{"Merge branch x", "Revert \"feat: x\""} {
		p := writeMsgFile(t, subject+"\n")
		var buf bytes.Buffer
		if err := CommitMsg(p, &buf); err != nil {
			t.Fatalf("%q rejected: %v", subject, err)
		}
	}
}

func TestCommitMsgStripsAttribution(t *testing.T) {
	p := writeMsgFile(t, "feat: native hooks\n\nCo-Authored-By: Claude Code <noreply@anthropic.com>\n")
	var buf bytes.Buffer
	if err := CommitMsg(p, &buf); err != nil {
		t.Fatalf("rejected: %v\n%s", err, buf.String())
	}
	b, _ := os.ReadFile(p)
	if strings.Contains(string(b), "Co-Authored-By") {
		t.Fatalf("attribution survived:\n%s", b)
	}
	if !strings.Contains(buf.String(), "Stripped AI attribution trailer") {
		t.Fatalf("output missing strip notice:\n%s", buf.String())
	}
}

func TestCommitMsgRejectsLongSubject(t *testing.T) {
	p := writeMsgFile(t, "feat: "+strings.Repeat("x", 100)+"\n")
	var buf bytes.Buffer
	if err := CommitMsg(p, &buf); err == nil {
		t.Fatal("long subject accepted")
	}
	if !strings.Contains(buf.String(), "Subject exceeds 100 characters") {
		t.Fatalf("output missing length error:\n%s", buf.String())
	}
}

func TestCommitMsgRejectsMissingBlankLine(t *testing.T) {
	p := writeMsgFile(t, "feat: subject\nbody right after\n")
	var buf bytes.Buffer
	if err := CommitMsg(p, &buf); err == nil {
		t.Fatal("missing blank line accepted")
	}
	if !strings.Contains(buf.String(), "blank line between subject and body") {
		t.Fatalf("output missing blank line error:\n%s", buf.String())
	}
}
