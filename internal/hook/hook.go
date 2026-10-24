package hook

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type ctx struct {
	root   string
	out    io.Writer
	env    []string
	failed bool
}

func (h *ctx) printf(format string, a ...any) {
	fmt.Fprintf(h.out, format+"\n", a...)
}

func (h *ctx) section(name string) {
	h.printf("\n=== %s ===", name)
}

func (h *ctx) printHead(s string, n int) {
	lines := strings.Split(s, "\n")
	if n > 0 && len(lines) > n {
		lines = lines[:n]
	}
	for _, l := range lines {
		if l != "" {
			h.printf("%s", l)
		}
	}
}

func capture(path string, args []string, env []string, dir string) (string, int, error) {
	cmd := exec.Command(path, args...)
	cmd.Env = env
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return strings.TrimSpace(string(out)), ee.ExitCode(), nil
		}
		return "", 0, err
	}
	return strings.TrimSpace(string(out)), 0, nil
}

func (h *ctx) tryTool(name string, args []string, dir string) (string, int, bool) {
	path, found := ToolPath(h.env, name)
	if !found {
		return "", 0, false
	}
	out, code, err := capture(path, args, h.env, dir)
	if err != nil && code == 0 {
		return out, -1, true
	}
	return out, code, true
}

func (h *ctx) gitAdd(files ...string) {
	if len(files) == 0 {
		return
	}
	if path, ok := ToolPath(h.env, "git"); ok {
		capture(path, append([]string{"add", "--"}, files...), h.env, h.root)
	}
}

func pathEnv(env []string) (string, []string) {
	path := ""
	exts := []string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		switch strings.ToUpper(k) {
		case "PATH":
			path = v
		case "PATHEXT":
			for _, e := range strings.Split(v, ";") {
				if e != "" {
					exts = append(exts, e)
				}
			}
		}
	}
	return path, exts
}

func ToolPath(env []string, name string) (string, bool) {
	pathList, exts := pathEnv(env)
	if pathList == "" {
		return "", false
	}
	candidates := []string{name}
	if runtime.GOOS == "windows" && filepath.Ext(name) == "" {
		for _, e := range append([]string{".exe", ".cmd", ".bat", ".com"}, exts...) {
			candidates = append(candidates, name+e)
		}
	}
	for _, dir := range filepath.SplitList(pathList) {
		if dir == "" {
			continue
		}
		for _, c := range candidates {
			full := filepath.Join(dir, c)
			if st, err := os.Stat(full); err == nil && !st.IsDir() {
				return full, true
			}
		}
	}
	return "", false
}

func StagedFiles(env []string, dir, pattern string) ([]string, error) {
	gitPath, ok := ToolPath(env, "git")
	if !ok {
		return nil, errors.New("hook: git not found")
	}
	out, code, err := capture(gitPath, []string{"diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR"}, env, dir)
	if err != nil || code != 0 {
		return nil, fmt.Errorf("hook: git diff --cached: %v (exit %d)", err, code)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, f := range strings.Split(out, "\x00") {
		if f != "" && re.MatchString(f) {
			files = append(files, f)
		}
	}
	return files, nil
}

func (h *ctx) staged(pattern string) []string {
	files, err := StagedFiles(h.env, h.root, pattern)
	if err != nil {
		return nil
	}
	return files
}

func findDepth(root string, depth int, pattern string) string {
	var found string
	filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return filepath.SkipAll
		}
		if p == root {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.Count(rel, string(filepath.Separator)) >= depth {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			if ok, _ := filepath.Match(pattern, d.Name()); ok {
				found = p
			}
		}
		return nil
	})
	return found
}

func detectProjects(root string) []string {
	var projects []string
	has := func(rel string) bool {
		_, err := os.Stat(filepath.Join(root, rel))
		return err == nil
	}
	if has("go.mod") {
		projects = append(projects, "go")
	}
	if has("package.json") {
		projects = append(projects, "node")
	}
	for _, m := range []string{"pyproject.toml", "requirements.txt", "setup.py", "poetry.lock"} {
		if has(m) {
			projects = append(projects, "python")
			break
		}
	}
	if findDepth(root, 3, "*.cs") != "" {
		projects = append(projects, "csharp")
	}
	for _, m := range []string{"pom.xml", "build.gradle", "build.gradle.kts"} {
		if has(m) {
			projects = append(projects, "java")
			break
		}
	}
	if has("build.sbt") {
		projects = append(projects, "scala")
	}
	if has("composer.json") {
		projects = append(projects, "php")
	}
	if has("Makefile") && findDepth(root, 1, "*.c") != "" {
		projects = append(projects, "c")
	}
	if has("CMakeLists.txt") || has("meson.build") {
		projects = append(projects, "cpp")
	}
	if has(filepath.Join("frontend", "package.json")) {
		projects = append(projects, "node-frontend")
	}
	if has("backend") && findDepth(filepath.Join(root, "backend"), 3, "*.cs") != "" {
		projects = append(projects, "csharp-backend")
	}
	return projects
}

func (h *ctx) spellCheck() {
	if _, ok := ToolPath(h.env, "typos"); !ok {
		return
	}
	files := h.staged(".")
	if len(files) == 0 {
		return
	}
	h.section("Spell check")
	out, code, ok := h.tryTool("typos", append([]string{"--force-exclude"}, files...), h.root)
	if ok && code == 0 {
		return
	}
	h.printHead(out, 30)
	h.failed = true
	if code == 2 {
		h.printf("Spelling errors found in staged files.")
		return
	}
	h.printf("typos exited %d (64 = unreadable file, 1 = usage/config error).", code)
}

func (h *ctx) goChecks() {
	files := h.staged(`\.go$`)
	if len(files) == 0 {
		return
	}
	h.section("Go")
	if _, ok := ToolPath(h.env, "goimports"); ok {
		h.printf("Running goimports...")
		h.tryTool("goimports", append([]string{"-w"}, files...), h.root)
	} else if _, ok := ToolPath(h.env, "gofmt"); ok {
		h.printf("Running go fmt...")
		if out, code, ok := h.tryTool("gofmt", append([]string{"-l"}, files...), h.root); ok && code == 0 {
			var unformatted []string
			for _, l := range strings.Split(out, "\n") {
				if l != "" {
					unformatted = append(unformatted, l)
				}
			}
			if len(unformatted) > 0 {
				h.tryTool("gofmt", append([]string{"-w"}, unformatted...), h.root)
			}
		}
	}
	h.gitAdd(files...)
	if _, ok := ToolPath(h.env, "golangci-lint"); ok {
		h.printf("Running golangci-lint...")
		if _, code, _ := h.tryTool("golangci-lint", []string{"run", "--no-config", "./..."}, h.root); code != 0 {
			h.printf("Lint warnings found.")
		}
	}
	h.printf("Running go vet...")
	if _, code, _ := h.tryTool("go", []string{"vet", "./..."}, h.root); code != 0 {
		h.printf("Go vet found issues.")
	}
}

func (h *ctx) powershellChecks() {
	files := h.staged(`\.ps1$`)
	if len(files) == 0 || runtime.GOOS != "windows" {
		return
	}
	if _, ok := ToolPath(h.env, "pwsh"); !ok {
		return
	}
	if _, code, _ := h.tryTool("pwsh", []string{"-NoProfile", "-Command", "exit [int](-not (Get-Module -ListAvailable PSScriptAnalyzer))"}, h.root); code != 0 {
		return
	}
	h.section("PowerShell")
	h.printf("Running PSScriptAnalyzer...")
	out, _, _ := h.tryTool("pwsh", append([]string{"-NoProfile", "-Command", `@(Invoke-ScriptAnalyzer -Path $args -Severity Error) | ForEach-Object { "{0}:{1}: {2}" -f $_.ScriptName, $_.Line, $_.Message }`}, files...), h.root)
	if out != "" {
		h.printf("%s", out)
		h.printf("PSScriptAnalyzer issues in staged PowerShell files.")
	}
}

func Run(root string, out io.Writer, env []string) error {
	started := time.Now()
	h := &ctx{root: root, out: out, env: env}
	fmt.Fprintf(out, "hook: pre-commit start %s\n", root)
	h.printf("Pre-commit checks...")
	h.spellCheck()
	projects := detectProjects(root)
	if len(projects) == 0 {
		h.printf("No recognized project type found.")
	}
	for _, project := range projects {
		switch project {
		case "go":
			h.goChecks()
		case "c", "cpp":
			h.cChecks()
		case "node", "node-frontend":
			dir := root
			if _, err := os.Stat(filepath.Join(root, "frontend")); err == nil {
				dir = filepath.Join(root, "frontend")
			}
			h.nodeChecks(dir)
		case "python":
			h.pythonChecks()
		case "csharp", "csharp-backend":
			h.csharpChecks()
		case "java":
			h.javaChecks()
		case "scala":
			h.scalaChecks()
		case "php":
			h.phpChecks()
		}
	}
	h.rustChecks()
	h.shellChecks()
	h.luaChecks()
	h.scalaFormatChecks()
	h.phpPintChecks()
	h.powershellChecks()
	if h.failed {
		h.printf("\nPre-commit checks failed. Fix issues or use --no-verify to bypass.")
		fmt.Fprintf(out, "hook: pre-commit failed %s (%s)\n", root, time.Since(started).Round(time.Millisecond))
		return fmt.Errorf("hook: pre-commit failed for %s", root)
	}
	h.printf("All checks passed!")
	fmt.Fprintf(out, "hook: pre-commit exit 0 %s (%s)\n", root, time.Since(started).Round(time.Millisecond))
	return nil
}

func Install(root string) error {
	hooksDir := filepath.Join(root, ".githooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("hook: install: %w", err)
	}
	goPath, ok := ToolPath(os.Environ(), "go")
	if !ok {
		return fmt.Errorf("hook: install: go not found")
	}
	preCommit := filepath.Join(hooksDir, "pre-commit.exe")
	if _, code, err := capture(goPath, []string{"build", "-o", preCommit, "./cmd/hook"}, os.Environ(), root); err != nil || code != 0 {
		return fmt.Errorf("hook: install: build pre-commit.exe: %v (exit %d)", err, code)
	}
	commitMsg := filepath.Join(hooksDir, "commit-msg.exe")
	if err := copyFile(preCommit, commitMsg); err != nil {
		return fmt.Errorf("hook: install: copy commit-msg.exe: %w", err)
	}
	gitPath, ok := ToolPath(os.Environ(), "git")
	if !ok {
		return fmt.Errorf("hook: install: git not found")
	}
	if _, code, err := capture(gitPath, []string{"config", "--local", "core.hooksPath", hooksDir}, os.Environ(), root); err != nil || code != 0 {
		return fmt.Errorf("hook: install: git config core.hooksPath: %v (exit %d)", err, code)
	}
	fmt.Printf("hook: install: core.hooksPath=%s, pre-commit.exe and commit-msg.exe run natively\n", hooksDir)
	return nil
}

func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o755)
}
