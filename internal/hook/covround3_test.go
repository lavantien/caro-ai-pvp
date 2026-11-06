package hook

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func directCtx(t *testing.T, dir string) *ctx {
	t.Helper()
	return &ctx{root: dir, out: &strings.Builder{}, env: os.Environ()}
}

func TestPrintHeadTruncatesLongOutput(t *testing.T) {
	h := directCtx(t, t.TempDir())
	h.printHead("a\nb\nc\nd\n", 2)
	got := h.out.(*strings.Builder).String()
	if strings.Contains(got, "c") || !strings.Contains(got, "b") {
		t.Fatalf("printHead kept %q, want the first 2 lines only", got)
	}
}

func TestTryToolReportsMissingTool(t *testing.T) {
	h := directCtx(t, t.TempDir())
	if _, _, ok := h.tryTool("no-such-tool-anywhere", nil, h.root); ok {
		t.Fatal("tryTool on an absent tool must report not found")
	}
}

func TestGitAddSkipsEmptyFileList(t *testing.T) {
	h := directCtx(t, newRepo(t))
	h.gitAdd()
}

func TestToolPathRejectsMissingAbsoluteAndEmptyPathEntries(t *testing.T) {
	if _, ok := ToolPath(os.Environ(), filepath.Join(t.TempDir(), "absent.exe")); ok {
		t.Fatal("absolute path to a missing file must not resolve")
	}
	env := []string{"PATH=" + string(os.PathListSeparator) + string(os.PathListSeparator) + filepath.Join(t.TempDir(), "nowhere")}
	if _, ok := ToolPath(env, "git"); ok {
		t.Fatal("empty PATH entries must be skipped without a match")
	}
}

func TestStagedReturnsNilOutsideARepo(t *testing.T) {
	h := directCtx(t, t.TempDir())
	if got := h.staged(`\.go$`); got != nil {
		t.Fatalf("staged outside a repo = %v, want nil", got)
	}
	if _, err := StagedFiles(os.Environ(), h.root, `\.go$`); err == nil {
		t.Fatal("StagedFiles outside a repo must error")
	}
}

func TestCheckersReturnWithoutStagedMatches(t *testing.T) {
	h := directCtx(t, newRepo(t))
	h.pythonChecks()
	h.csharpChecks()
	h.cChecks()
	h.goChecks()
	h.phpChecks()
	if h.failed {
		t.Fatal("no staged files must not fail any checker")
	}
}

func TestScalaChecksReturnWithoutBuildFile(t *testing.T) {
	h := directCtx(t, newRepo(t))
	h.scalaChecks()
}

func TestScalaFormatSkipsWithoutTool(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "a.scala", "object A\n")
	h := directCtx(t, dir)
	h.scalaFormatChecks()
	if h.failed {
		t.Fatal("missing scalafmt must not fail the check")
	}
}

func TestPhpPintSkipsWithoutTool(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "a.php", "<?php\n")
	h := directCtx(t, dir)
	h.phpPintChecks()
	if h.failed {
		t.Fatal("missing pint must not fail the check")
	}
}

func TestSpellCheckSkipsWhenNothingStaged(t *testing.T) {
	dir := newRepo(t)
	fake := t.TempDir()
	buildFakeTool(t, fake, "typos", 0)
	out, err := runHook(t, dir, envWith(t, fake))
	if err != nil {
		t.Fatalf("hook with typos and nothing staged failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "Spell check") {
		t.Fatalf("spell check must skip without staged files:\n%s", out)
	}
}

func TestSpellCheckReportsUnexpectedToolExit(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "a.txt", "words\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "typos", 3)
	out, err := runHook(t, dir, envWith(t, fake))
	if err == nil {
		t.Fatal("typos exiting 3 must fail the hook")
	}
	if !strings.Contains(out, "typos exited 3") {
		t.Fatalf("output missing the tool exit report:\n%s", out)
	}
}

func TestGoChecksReportsVetIssues(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "go.mod", "module tmp\n\ngo 1.27\n")
	stageFile(t, dir, "a.go", "package x\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "go", 1)
	out, err := runHook(t, dir, minimalPathEnv(t, fake))
	if err != nil {
		t.Fatalf("vet findings must report without failing the hook: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Go vet found issues.") {
		t.Fatalf("output missing the vet report:\n%s", out)
	}
}

func TestPythonFallsBackToBlackAndIsort(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "pyproject.toml", "[project]\n")
	stageFile(t, dir, "a.py", "x = 1\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "black", 1)
	buildFakeTool(t, fake, "isort", 1)
	out, err := runHook(t, dir, minimalPathEnv(t, fake))
	if err != nil {
		t.Fatalf("black and isort reformatting must not fail the hook: %v\n%s", err, out)
	}
	for _, want := range []string{"Running black...", "Running isort..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestJavaChecksApplySpotlessAndReportCheckstyle(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "pom.xml", "<project><dependency>spotless</dependency></project>\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "mvn", 1)
	out, err := runHook(t, dir, envWith(t, fake))
	if err != nil {
		t.Fatalf("checkstyle warnings must report without failing the hook: %v\n%s", err, out)
	}
	for _, want := range []string{"Running spotless check...", "Checkstyle warnings."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestScalaChecksReportCompileWarnings(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "build.sbt", "name := \"x\"\n")
	stageFile(t, dir, "a.scala", "object A\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "sbt", 1)
	out, err := runHook(t, dir, envWith(t, fake))
	if err != nil {
		t.Fatalf("sbt warnings must report without failing the hook: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Compilation warnings.") {
		t.Fatalf("output missing the sbt report:\n%s", out)
	}
}

func TestCsharpWalksUpToNearestCsproj(t *testing.T) {
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "svc", "deep"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.Join("svc", "App.csproj"), "<Project/>\n")
	stageFile(t, dir, filepath.Join("svc", "deep", "a.cs"), "class A {}\n")
	stageFile(t, dir, filepath.Join("svc", "b.cs"), "class B {}\n")
	stageFile(t, dir, "loose.cs", "class L {}\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "dotnet", 0)
	h := &ctx{root: dir, out: &strings.Builder{}, env: envWith(t, fake)}
	h.csharpChecks()
	got := h.out.(*strings.Builder).String()
	if n := strings.Count(got, "check+build"); n != 1 {
		t.Fatalf("csproj walked %d times, want 1 deduplicated run:\n%s", n, got)
	}
	if strings.Contains(got, "loose") {
		t.Fatalf("a staged cs without any csproj must be skipped:\n%s", got)
	}
}

func TestRustDeduplicatesCargoRoots(t *testing.T) {
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageFile(t, dir, "Cargo.toml", "[package]\nname = \"x\"\n")
	stageFile(t, dir, "a.rs", "fn a() {}\n")
	stageFile(t, dir, filepath.Join("sub", "b.rs"), "fn b() {}\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "cargo", 0)
	h := &ctx{root: dir, out: &strings.Builder{}, env: envWith(t, fake)}
	h.rustChecks()
	got := h.out.(*strings.Builder).String()
	if n := strings.Count(got, "Running cargo clippy"); n != 1 {
		t.Fatalf("cargo ran %d times, want 1 deduplicated root:\n%s", n, got)
	}
}

func TestRunUsesFrontendDirForNodeProject(t *testing.T) {
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "frontend"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.Join("frontend", "package.json"), "{\"scripts\": {\"check\": \"tsc\"}}\n")
	stageFile(t, dir, filepath.Join("frontend", "a.ts"), "export const x = 1\n")
	fake := t.TempDir()
	buildFakeTool(t, fake, "npx", 0)
	buildFakeTool(t, fake, "npm", 0)
	out, err := runHook(t, dir, envWith(t, fake))
	if err != nil {
		t.Fatalf("frontend node checks failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running Prettier...") {
		t.Fatalf("output missing frontend prettier run:\n%s", out)
	}
}

func TestCommitMsgRejectsEmptySubject(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "msg")
	if err := os.WriteFile(p, []byte("\nbody\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := CommitMsg(p, &b); err == nil {
		t.Fatal("empty subject must be rejected")
	}
	if !strings.Contains(b.String(), "Commit message cannot be empty.") {
		t.Fatalf("output missing the empty-subject report:\n%s", b.String())
	}
}

func TestCommitMsgPassesMergeSubjects(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "msg")
	if err := os.WriteFile(p, []byte("Merge branch 'feature'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := CommitMsg(p, &strings.Builder{}); err != nil {
		t.Fatalf("merge subject must pass: %v", err)
	}
}

func TestCommitMsgReportsStripRewriteFailure(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "msg")
	if err := os.WriteFile(p, []byte("feat: subject\n\nCo-Authored-By: Claude <noreply>\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o444); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	var b strings.Builder
	if err := CommitMsg(p, &b); err != nil {
		t.Fatalf("strip rewrite failure must not reject the message: %v", err)
	}
	if !strings.Contains(b.String(), "rewrite") {
		t.Fatalf("output missing the rewrite failure report:\n%s", b.String())
	}
}

func TestInstallFailsWhenRootIsAFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "notadir")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Install(f); err == nil || !strings.Contains(err.Error(), "install") {
		t.Fatalf("Install on a file root = %v, want install error", err)
	}
}

func TestInstallFailsWhenCopyRunsBeforeRealBuild(t *testing.T) {
	fake := t.TempDir()
	buildFakeTool(t, fake, "go", 0)
	t.Setenv("PATH", fake)
	err := Install(t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "copy commit-msg") {
		t.Fatalf("Install with a fake go that writes no binary = %v, want copy failure", err)
	}
}
