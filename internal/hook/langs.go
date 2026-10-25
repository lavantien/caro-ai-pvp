package hook

import (
	"os"
	"path/filepath"
	"strings"
)

func (h *ctx) hasRootFile(dir, name string) bool {
	_, err := os.Stat(filepath.Join(h.root, filepath.FromSlash(dir), name))
	return err == nil
}

func (h *ctx) rustChecks() {
	files := h.staged(`\.rs$`)
	if len(files) == 0 {
		return
	}
	h.section("Rust")
	var roots []string
	for _, f := range files {
		d := filepath.ToSlash(filepath.Dir(f))
		for d != "." && !h.hasRootFile(d, "Cargo.toml") {
			d = filepath.ToSlash(filepath.Dir(d))
		}
		if !h.hasRootFile(d, "Cargo.toml") {
			continue
		}
		dup := false
		for _, r := range roots {
			if r == d {
				dup = true
			}
		}
		if !dup {
			roots = append(roots, d)
		}
	}
	if len(roots) == 0 {
		h.printf("no Cargo.toml above staged rust files, skipping cargo gates")
		return
	}
	for _, d := range roots {
		work := filepath.Join(h.root, filepath.FromSlash(d))
		h.printf("Running cargo fmt in %s...", d)
		if _, code, ok := h.tryTool("cargo", []string{"fmt", "--check"}, work); ok && code != 0 {
			h.tryTool("cargo", []string{"fmt"}, work)
			h.gitAdd(files...)
			h.printf("Files formatted. Please review.")
		}
		h.printf("Running cargo clippy in %s...", d)
		if _, code, _ := h.tryTool("cargo", []string{"clippy", "--quiet", "--", "-D", "warnings"}, work); code != 0 {
			h.printf("Clippy warnings found.")
		}
		h.printf("Running cargo check in %s...", d)
		if _, code, _ := h.tryTool("cargo", []string{"check"}, work); code != 0 {
			h.printf("Cargo check failed (%s).", d)
			h.failed = true
		}
	}
}

func (h *ctx) cChecks() {
	h.section("C/C++")
	files := h.staged(`\.(c|h|cpp|hpp|cc|cxx)$`)
	if len(files) == 0 {
		return
	}
	if _, ok := ToolPath(h.env, "clang-format"); ok {
		h.printf("Running clang-format...")
		for _, f := range files {
			if _, err := os.Stat(filepath.Join(h.root, filepath.FromSlash(f))); err != nil {
				continue
			}
			if _, code, ok := h.tryTool("clang-format", []string{"-i", f}, h.root); ok && code == 0 {
				h.gitAdd(f)
			}
		}
	}
	if _, ok := ToolPath(h.env, "clang-tidy"); ok {
		if _, err := os.Stat(filepath.Join(h.root, "compile_commands.json")); err == nil {
			h.printf("Running clang-tidy...")
			for _, f := range files {
				if _, err := os.Stat(filepath.Join(h.root, filepath.FromSlash(f))); err != nil {
					continue
				}
				h.tryTool("clang-tidy", []string{f}, h.root)
			}
		}
	}
	if _, ok := ToolPath(h.env, "cppcheck"); ok {
		h.printf("Running cppcheck...")
		if _, code, _ := h.tryTool("cppcheck", append([]string{"--enable=all", "--error-exitcode=1"}, files...), h.root); code != 0 {
			h.printf("Cppcheck warnings.")
		}
	}
}

func (h *ctx) hasAny(dir string, paths ...string) bool {
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(p))); err == nil {
			return true
		}
	}
	return false
}

func (h *ctx) gitAddAll(dir string) {
	if path, ok := ToolPath(h.env, "git"); ok {
		_, _, _ = capture(path, []string{"add", "."}, h.env, dir)
	}
}

func (h *ctx) nodeChecks(dir string) {
	h.section("JavaScript/TypeScript")
	if _, code, ok := h.tryTool("npx", []string{"--no-install", "prettier", "--version"}, dir); ok && code == 0 {
		h.printf("Running Prettier...")
		args := []string{"--no-install", "prettier", "--check", "."}
		if h.hasAny(dir, "node_modules/prettier-plugin-svelte/index.js", "node_modules/prettier-plugin-svelte/dist/index.js") {
			args = append(args, "--plugin=prettier-plugin-svelte")
		}
		if _, code, _ := h.tryTool("npx", args, dir); code != 0 {
			write := []string{"--no-install", "prettier", "--write", "."}
			if len(args) > 4 {
				write = append(write, args[4])
			}
			out, _, _ := h.tryTool("npx", write, dir)
			h.printHead(out, 5)
			h.gitAddAll(dir)
			h.printf("Files formatted with Prettier.")
		}
	}
	if _, code, ok := h.tryTool("npx", []string{"--no-install", "eslint", "--version"}, dir); ok && code == 0 {
		h.printf("Running ESLint...")
		out, _, _ := h.tryTool("npx", []string{"--no-install", "eslint", "."}, dir)
		h.printHead(out, 20)
	}
	pkg := filepath.Join(dir, "package.json")
	if b, err := os.ReadFile(pkg); err == nil && strings.Contains(string(b), `"check"`) {
		h.printf("Running type check...")
		if _, code, _ := h.tryTool("npm", []string{"run", "check", "--silent"}, dir); code != 0 {
			h.printf("Type check failed.")
			h.failed = true
		}
	} else if _, ok := ToolPath(h.env, "tsc"); ok {
		if _, err := os.Stat(filepath.Join(dir, "tsconfig.json")); err == nil {
			h.printf("Running tsc...")
			if _, code, _ := h.tryTool("tsc", []string{"--noEmit"}, dir); code != 0 {
				h.printf("TypeScript errors found.")
			}
		}
	}
}

func (h *ctx) pythonChecks() {
	h.section("Python")
	files := h.staged(`\.py$|pyproject\.toml`)
	if len(files) == 0 {
		return
	}
	if _, ok := ToolPath(h.env, "ruff"); ok {
		h.printf("Running ruff format...")
		if _, code, _ := h.tryTool("ruff", append([]string{"format", "--check"}, files...), h.root); code != 0 {
			h.tryTool("ruff", append([]string{"format"}, files...), h.root)
			h.gitAdd(files...)
		}
		h.printf("Running ruff check...")
		if _, code, _ := h.tryTool("ruff", append([]string{"check"}, files...), h.root); code != 0 {
			if _, code, _ := h.tryTool("ruff", append([]string{"check", "--fix"}, files...), h.root); code != 0 {
				h.failed = true
			}
		}
	} else {
		if _, ok := ToolPath(h.env, "black"); ok {
			h.printf("Running black...")
			if _, code, _ := h.tryTool("black", append([]string{"--check"}, files...), h.root); code != 0 {
				h.tryTool("black", files, h.root)
				h.gitAdd(files...)
			}
		}
		if _, ok := ToolPath(h.env, "isort"); ok {
			h.printf("Running isort...")
			if _, code, _ := h.tryTool("isort", append([]string{"--check-only"}, files...), h.root); code != 0 {
				h.tryTool("isort", files, h.root)
				h.gitAdd(files...)
			}
		}
	}
	if _, ok := ToolPath(h.env, "mypy"); ok {
		h.printf("Running mypy...")
		if _, code, _ := h.tryTool("mypy", files, h.root); code != 0 {
			h.printf("Type check warnings.")
		}
	}
}

func (h *ctx) hasCsproj(dir string) bool {
	matches, _ := filepath.Glob(filepath.Join(h.root, filepath.FromSlash(dir), "*.csproj"))
	return len(matches) > 0
}

func (h *ctx) csharpChecks() {
	h.section("C#")
	files := h.staged(`\.(cs|csproj)$`)
	if len(files) == 0 {
		return
	}
	if _, ok := ToolPath(h.env, "dotnet"); !ok {
		return
	}
	seen := map[string]bool{}
	for _, f := range files {
		d := filepath.ToSlash(filepath.Dir(f))
		for d != "." && !h.hasCsproj(d) {
			d = filepath.ToSlash(filepath.Dir(d))
		}
		if d == "." && !h.hasCsproj(d) {
			continue
		}
		if seen[d] {
			continue
		}
		seen[d] = true
		matches, _ := filepath.Glob(filepath.Join(h.root, filepath.FromSlash(d), "*.csproj"))
		for _, p := range matches {
			label := filepath.ToSlash(filepath.Join(d, filepath.Base(p)))
			h.printf("check+build %s", label)
			if _, code, _ := h.tryTool("dotnet", append([]string{"format", p, "--verify-no-changes", "--include"}, files...), h.root); code != 0 {
				h.printf("Format drift in staged files (%s).", label)
				h.failed = true
			}
			if _, code, _ := h.tryTool("dotnet", []string{"build", p, "-v", "q", "--nologo"}, h.root); code != 0 {
				h.printf("Build failed: %s", label)
				h.failed = true
			}
		}
	}
}

func (h *ctx) javaChecks() {
	h.section("Java")
	pom := filepath.Join(h.root, "pom.xml")
	if b, err := os.ReadFile(pom); err == nil && strings.Contains(string(b), "spotless") {
		h.printf("Running spotless check...")
		if _, code, _ := h.tryTool("mvn", []string{"spotless:check"}, h.root); code != 0 {
			h.tryTool("mvn", []string{"spotless:apply"}, h.root)
			h.gitAddAll(h.root)
		}
	}
	if _, ok := ToolPath(h.env, "google-java-format"); ok {
		h.printf("Running google-java-format...")
		h.tryTool("google-java-format", []string{"--replace"}, h.root)
	}
	if _, err := os.Stat(pom); err == nil {
		if _, ok := ToolPath(h.env, "mvn"); ok {
			h.printf("Running checkstyle...")
			if _, code, _ := h.tryTool("mvn", []string{"checkstyle:check"}, h.root); code != 0 {
				h.printf("Checkstyle warnings.")
			}
		}
	}
}

func (h *ctx) scalaChecks() {
	h.section("Scala")
	if _, err := os.Stat(filepath.Join(h.root, "build.sbt")); err != nil {
		return
	}
	if _, ok := ToolPath(h.env, "sbt"); !ok {
		return
	}
	h.printf("Running sbt compile...")
	if _, code, _ := h.tryTool("sbt", []string{"compile"}, h.root); code != 0 {
		h.printf("Compilation warnings.")
	}
}

func (h *ctx) runnable(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir() && filepath.Ext(path) != ""
}

func (h *ctx) phpChecks() {
	files := h.staged(`\.php$`)
	if len(files) == 0 {
		return
	}
	if p := filepath.Join(h.root, "vendor", "bin", "pint.bat"); h.runnable(p) {
		h.printf("Running Laravel Pint...")
		if _, code, _ := h.tryTool(p, []string{"--test"}, h.root); code != 0 {
			h.tryTool(p, nil, h.root)
			h.gitAdd(files...)
		}
	}
	if p := filepath.Join(h.root, "vendor", "bin", "php-cs-fixer.bat"); h.runnable(p) {
		h.printf("Running php-cs-fixer...")
		if _, code, _ := h.tryTool(p, []string{"fix", "--dry-run"}, h.root); code != 0 {
			h.tryTool(p, []string{"fix"}, h.root)
			h.gitAdd(files...)
		}
	}
	if p := filepath.Join(h.root, "vendor", "bin", "phpstan.bat"); h.runnable(p) {
		h.printf("Running PHPStan...")
		if _, code, _ := h.tryTool(p, []string{"analyse", "--memory-limit=1G"}, h.root); code != 0 {
			h.printf("PHPStan warnings.")
		}
	}
	if p := filepath.Join(h.root, "vendor", "bin", "psalm.bat"); h.runnable(p) {
		h.printf("Running Psalm...")
		if _, code, _ := h.tryTool(p, []string{"--show-info=false"}, h.root); code != 0 {
			h.printf("Psalm warnings.")
		}
	}
}

func (h *ctx) scalaFormatChecks() {
	files := h.staged(`\.scala$`)
	if len(files) == 0 {
		return
	}
	if _, ok := ToolPath(h.env, "scalafmt"); !ok {
		return
	}
	h.section("Scala format")
	h.printf("Running scalafmt...")
	if _, code, _ := h.tryTool("scalafmt", []string{"--non-interactive"}, h.root); code == 0 {
		h.gitAdd(files...)
		return
	}
	h.printf("scalafmt failed, check .scalafmt.conf.")
}

func (h *ctx) phpPintChecks() {
	files := h.staged(`\.php$`)
	if len(files) == 0 {
		return
	}
	if h.runnable(filepath.Join(h.root, "vendor", "bin", "pint.bat")) {
		return
	}
	path, ok := ToolPath(h.env, "pint")
	if !ok {
		return
	}
	h.section("PHP")
	h.printf("Running pint...")
	for _, f := range files {
		if _, code, _ := h.tryTool(path, []string{f}, h.root); code == 0 {
			h.gitAdd(f)
		}
	}
}

func (h *ctx) shellChecks() {
	files := h.staged(`\.(sh|bash)$`)
	if len(files) == 0 {
		return
	}
	h.section("Shell")
	if _, ok := ToolPath(h.env, "shfmt"); ok {
		h.printf("Running shfmt...")
		for _, f := range files {
			if _, code, ok := h.tryTool("shfmt", []string{"-w", f}, h.root); ok && code == 0 {
				h.gitAdd(f)
			}
		}
	}
	if _, ok := ToolPath(h.env, "shellcheck"); ok {
		h.printf("Running shellcheck...")
		if out, code, _ := h.tryTool("shellcheck", append([]string{"--severity=error"}, files...), h.root); code != 0 {
			h.printHead(out, 40)
			h.printf("shellcheck found errors in staged shell files.")
			h.failed = true
		}
	}
}

func (h *ctx) luaChecks() {
	files := h.staged(`\.lua$`)
	if len(files) == 0 {
		return
	}
	h.section("Lua")
	if _, ok := ToolPath(h.env, "stylua"); ok {
		h.printf("Running stylua...")
		for _, f := range files {
			if _, code, ok := h.tryTool("stylua", []string{f}, h.root); ok && code == 0 {
				h.gitAdd(f)
			}
		}
	}
	if _, ok := ToolPath(h.env, "selene"); ok {
		h.printf("Running selene...")
		out, _, _ := h.tryTool("selene", files, h.root)
		h.printHead(out, 30)
	}
}
