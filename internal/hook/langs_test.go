package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func stageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	writeFile(t, dir, name, content)
	gitIn(t, dir, "add", name)
}

func TestRustChecksRunCargoGates(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "cargo", 0)
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"x\"\n")
	stageFile(t, dir, "a.rs", "fn main() {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean cargo: %v\n%s", err, out)
	}
	for _, want := range []string{"=== Rust ===", "Running cargo fmt in .", "Running cargo clippy in .", "Running cargo check in ."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRustChecksFailOnCargoCheckError(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "cargo", 1)
	writeFile(t, dir, "Cargo.toml", "[package]\nname = \"x\"\n")
	stageFile(t, dir, "a.rs", "fn main() {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with failing cargo check:\n%s", out)
	}
	for _, want := range []string{"Files formatted. Please review.", "Clippy warnings found.", "Cargo check failed (.)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestRustChecksSkipWithoutCargoManifest(t *testing.T) {
	dir := newRepo(t)
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	stageFile(t, dir, filepath.FromSlash("sub/a.rs"), "fn main() {}\n")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed without Cargo.toml: %v\n%s", err, out)
	}
	if !strings.Contains(out, "no Cargo.toml above staged rust files, skipping cargo gates") {
		t.Fatalf("output missing skip line:\n%s", out)
	}
}

func TestRustChecksUseNearestCargoRoot(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "cargo", 0)
	if err := os.MkdirAll(filepath.Join(dir, "lib"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.FromSlash("lib/Cargo.toml"), "[package]\nname = \"lib\"\n")
	stageFile(t, dir, filepath.FromSlash("lib/a.rs"), "fn f() {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on nested crate: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running cargo fmt in lib") {
		t.Fatalf("output missing nested root line:\n%s", out)
	}
}

func TestCChecksRunFormattersAndLints(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "typos", 0)
	buildFakeTool(t, fakeDir, "clang-format", 0)
	buildFakeTool(t, fakeDir, "clang-tidy", 0)
	buildFakeTool(t, fakeDir, "cppcheck", 1)
	writeFile(t, dir, "Makefile", "all:\n")
	writeFile(t, dir, "compile_commands.json", "[]\n")
	stageFile(t, dir, "a.c", "int main(){}\n")
	writeFile(t, dir, "ghost.c", "int g(){}\n")
	gitIn(t, dir, "add", "ghost.c")
	if err := os.Remove(filepath.Join(dir, "ghost.c")); err != nil {
		t.Fatal(err)
	}
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on c checks: %v\n%s", err, out)
	}
	for _, want := range []string{"=== C/C++ ===", "Running clang-format...", "Running clang-tidy...", "Running cppcheck...", "Cppcheck warnings."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNodeChecksRunPrettierEslintAndScriptCheck(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "npx", 0)
	buildFakeTool(t, fakeDir, "npm", 0)
	writeFile(t, dir, "package.json", "{\"scripts\": {\"check\": \"tsc\"}}\n")
	stageFile(t, dir, "a.ts", "export const x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean node checks: %v\n%s", err, out)
	}
	for _, want := range []string{"=== JavaScript/TypeScript ===", "Running Prettier...", "Running ESLint...", "Running type check..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNodeChecksTscFallbackWithoutScript(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "npx", 0)
	buildFakeTool(t, fakeDir, "tsc", 1)
	writeFile(t, dir, "package.json", "{\"name\": \"x\"}\n")
	writeFile(t, dir, "tsconfig.json", "{}\n")
	stageFile(t, dir, "a.ts", "export const x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on tsc warnings: %v\n%s", err, out)
	}
	for _, want := range []string{"Running tsc...", "TypeScript errors found."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPythonChecksRuffAndMypy(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"x\"\n")
	gitIn(t, dir, "add", "pyproject.toml")
	buildFakeTool(t, fakeDir, "ruff", 0)
	buildFakeTool(t, fakeDir, "mypy", 0)
	stageFile(t, dir, "a.py", "x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean python checks: %v\n%s", err, out)
	}
	for _, want := range []string{"=== Python ===", "Running ruff format...", "Running ruff check...", "Running mypy..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func minimalPathEnv(t *testing.T, fakeDir string) []string {
	t.Helper()
	gitPath, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	path := fakeDir + string(os.PathListSeparator) + filepath.Dir(gitPath)
	if root := os.Getenv("SystemRoot"); root != "" {
		path += string(os.PathListSeparator) + filepath.Join(root, "System32")
	}
	return []string{"PATH=" + path}
}

func TestPythonChecksBlackIsortFallback(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"x\"\n")
	gitIn(t, dir, "add", "pyproject.toml")
	buildFakeTool(t, fakeDir, "black", 0)
	buildFakeTool(t, fakeDir, "isort", 0)
	stageFile(t, dir, "a.py", "x = 1\n")
	out, err := runHook(t, dir, minimalPathEnv(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on black fallback: %v\n%s", err, out)
	}
	for _, want := range []string{"Running black...", "Running isort..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPythonChecksRuffFixFailureFails(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	writeFile(t, dir, "pyproject.toml", "[project]\nname = \"x\"\n")
	gitIn(t, dir, "add", "pyproject.toml")
	buildFakeTool(t, fakeDir, "ruff", 1)
	stageFile(t, dir, "a.py", "x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with unfixed ruff findings:\n%s", out)
	}
	if !strings.Contains(out, "=== Python ===") {
		t.Fatalf("output missing python section:\n%s", out)
	}
}

func TestCsharpChecksFormatAndBuild(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "dotnet", 0)
	writeFile(t, dir, "app.csproj", "<Project Sdk=\"Microsoft.NET.Sdk\" />\n")
	stageFile(t, dir, "a.cs", "class A {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean dotnet checks: %v\n%s", err, out)
	}
	if !strings.Contains(out, "check+build app.csproj") {
		t.Fatalf("output missing csproj line:\n%s", out)
	}
}

func TestCsharpChecksBuildFailureFails(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "dotnet", 1)
	writeFile(t, dir, "app.csproj", "<Project Sdk=\"Microsoft.NET.Sdk\" />\n")
	stageFile(t, dir, "a.cs", "class A {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with failing dotnet build:\n%s", out)
	}
	for _, want := range []string{"Format drift in staged files (app.csproj).", "Build failed: app.csproj"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestJavaChecksSpotlessFormatAndCheckstyle(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "mvn", 0)
	buildFakeTool(t, fakeDir, "google-java-format", 0)
	writeFile(t, dir, "pom.xml", "<project><build><plugins><plugin><artifactId>spotless</artifactId></plugin></plugins></build></project>\n")
	stageFile(t, dir, "A.java", "class A {}\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean java checks: %v\n%s", err, out)
	}
	for _, want := range []string{"=== Java ===", "Running spotless check...", "Running google-java-format...", "Running checkstyle..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestScalaChecksCompile(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "sbt", 0)
	writeFile(t, dir, "build.sbt", "scalaVersion := \"3\"\n")
	stageFile(t, dir, "note.txt", "scala project\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean sbt: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running sbt compile...") {
		t.Fatalf("output missing sbt line:\n%s", out)
	}
}

func TestScalaChecksSkipWithoutSbt(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "build.sbt", "scalaVersion := \"3\"\n")
	stageFile(t, dir, "note.txt", "scala project\n")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed without sbt: %v\n%s", err, out)
	}
	if strings.Contains(out, "Running sbt compile...") {
		t.Fatalf("sbt ran without the tool:\n%s", out)
	}
}

func TestPhpChecksVendorBins(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "composer.json", "{}\n")
	for _, tool := range []string{"pint", "php-cs-fixer", "phpstan", "psalm"} {
		writeVendorTool(t, dir, tool, 1)
	}
	stageFile(t, dir, "a.php", "<?php echo 1;\n")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed on vendor bin checks: %v\n%s", err, out)
	}
	for _, want := range []string{"Running Laravel Pint...", "Running php-cs-fixer...", "Running PHPStan...", "Running Psalm..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestPhpPintChecksGlobalBinary(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "pint", 0)
	stageFile(t, dir, "a.php", "<?php echo 1;\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on global pint: %v\n%s", err, out)
	}
	for _, want := range []string{"=== PHP ===", "Running pint..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestScalaFormatChecksRunScalafmt(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "scalafmt", 0)
	stageFile(t, dir, "a.scala", "object A\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on clean scalafmt: %v\n%s", err, out)
	}
	if !strings.Contains(out, "=== Scala format ===") {
		t.Fatalf("output missing scala format section:\n%s", out)
	}
}

func TestScalaFormatChecksFailureIsReported(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "scalafmt", 1)
	stageFile(t, dir, "a.scala", "object A\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on scalafmt config error: %v\n%s", err, out)
	}
	if !strings.Contains(out, "scalafmt failed, check .scalafmt.conf.") {
		t.Fatalf("output missing scalafmt failure line:\n%s", out)
	}
}

func TestLuaChecksStyluaAndSelene(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "stylua", 0)
	buildFakeTool(t, fakeDir, "selene", 0, "a.lua:1: warning dummy")
	stageFile(t, dir, "a.lua", "x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on lua checks: %v\n%s", err, out)
	}
	for _, want := range []string{"=== Lua ===", "Running stylua...", "Running selene...", "a.lua:1: warning dummy"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestShellChecksShfmtRestages(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "shfmt", 0)
	buildFakeTool(t, fakeDir, "shellcheck", 0)
	stageFile(t, dir, "run.sh", "echo hi\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed on shfmt: %v\n%s", err, out)
	}
	if !strings.Contains(out, "=== Shell ===") {
		t.Fatalf("output missing shell section:\n%s", out)
	}
}

func TestPowershellChecksRunAnalyzerOnWindows(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "a.ps1", "Write-Host hi\n")
	if runtime.GOOS != "windows" {
		out, err := runHook(t, dir, os.Environ())
		if err != nil {
			t.Fatalf("hook failed on non-windows ps1: %v\n%s", err, out)
		}
		if strings.Contains(out, "=== PowerShell ===") {
			t.Fatalf("powershell section ran on non-windows:\n%s", out)
		}
		return
	}
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "pwsh", 0)
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed with fake analyzer: %v\n%s", err, out)
	}
	for _, want := range []string{"=== PowerShell ===", "Running PSScriptAnalyzer..."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestDetectProjectsAllMarkers(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		want  []string
	}{
		{"empty", nil, nil},
		{"go", map[string]string{"go.mod": "module x\n"}, []string{"go"}},
		{"node", map[string]string{"package.json": "{}\n"}, []string{"node"}},
		{"node frontend", map[string]string{"package.json": "{}\n", filepath.Join("frontend", "package.json"): "{}\n"}, []string{"node", "node-frontend"}},
		{"python pyproject", map[string]string{"pyproject.toml": ""}, []string{"python"}},
		{"python lock", map[string]string{"poetry.lock": ""}, []string{"python"}},
		{"csharp shallow", map[string]string{"a.cs": ""}, []string{"csharp"}},
		{"java pom", map[string]string{"pom.xml": ""}, []string{"java"}},
		{"java gradle", map[string]string{"build.gradle": ""}, []string{"java"}},
		{"scala", map[string]string{"build.sbt": ""}, []string{"scala"}},
		{"php", map[string]string{"composer.json": ""}, []string{"php"}},
		{"c makefile", map[string]string{"Makefile": "", "a.c": ""}, []string{"c"}},
		{"cpp cmake", map[string]string{"CMakeLists.txt": ""}, []string{"cpp"}},
		{"csharp backend", map[string]string{filepath.Join("backend", "a.cs"): ""}, []string{"csharp", "csharp-backend"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for path, content := range tc.files {
				full := filepath.Join(dir, path)
				if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			got := detectProjects(dir)
			if len(got) != len(tc.want) {
				t.Fatalf("detectProjects = %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("detectProjects = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestToolPathResolvesCandidatesAndMisses(t *testing.T) {
	fakeDir := t.TempDir()
	exe := buildFakeTool(t, fakeDir, "toolx", 0)
	env := []string{"PATH=" + fakeDir}
	got, ok := ToolPath(env, "toolx")
	if !ok || got != exe {
		t.Fatalf("ToolPath = %q, %v, want %q", got, ok, exe)
	}
	if _, ok := ToolPath([]string{"PATH="}, "toolx"); ok {
		t.Fatal("ToolPath found a tool on an empty PATH")
	}
	if _, ok := ToolPath(nil, "toolx"); ok {
		t.Fatal("ToolPath found a tool without PATH")
	}
}

func TestStagedFilesRejectsInvalidPattern(t *testing.T) {
	dir := newRepo(t)
	stageFile(t, dir, "a.go", "package x\n")
	if _, err := StagedFiles(os.Environ(), dir, `[`); err == nil {
		t.Fatal("invalid pattern accepted")
	}
	env := []string{"PATH=" + t.TempDir()}
	if _, err := StagedFiles(env, dir, `\.go$`); err == nil {
		t.Fatal("missing git accepted")
	}
}

func TestTryToolReportsStartFailure(t *testing.T) {
	fakeDir := t.TempDir()
	name := "broken"
	payload := []byte("not an executable payload")
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	if err := os.WriteFile(filepath.Join(fakeDir, name), payload, 0o644); err != nil {
		t.Fatal(err)
	}
	h := &ctx{root: t.TempDir(), env: []string{"PATH=" + fakeDir}}
	_, code, ok := h.tryTool("broken", nil, h.root)
	if !ok || code != -1 {
		t.Fatalf("tryTool on unstartable binary = code %d ok %v, want -1 true", code, ok)
	}
}
