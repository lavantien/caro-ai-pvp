package hook

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildToolSource(t *testing.T, dir, name, src string) string {
	t.Helper()
	srcPath := filepath.Join(dir, name+".go")
	if err := os.WriteFile(srcPath, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, name)
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, srcPath)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake %s: %v: %s", name, err, out)
	}
	return exe
}

func TestNodeChecksPrettierWritesAndSveltePlugin(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildToolSource(t, fakeDir, "npx", `package main

import (
	"fmt"
	"os"
)

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" {
			fmt.Println("3.3.3")
			return
		}
		fmt.Print(a + " ")
	}
	fmt.Println()
	os.Exit(1)
}
`)
	writeFile(t, dir, "package.json", "{\"name\": \"x\"}\n")
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "prettier-plugin-svelte"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.FromSlash("node_modules/prettier-plugin-svelte/index.js"), "export default {}\n")
	stageFile(t, dir, "a.ts", "export const x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed after prettier write: %v\n%s", err, out)
	}
	for _, want := range []string{"Running Prettier...", "Files formatted with Prettier.", "--plugin=prettier-plugin-svelte"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestNodeChecksTypeScriptFallbackWithoutCheckScript(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "npx", 1)
	buildFakeTool(t, fakeDir, "tsc", 1)
	writeFile(t, dir, "package.json", "{\"name\": \"x\"}\n")
	writeFile(t, dir, "tsconfig.json", "{}\n")
	stageFile(t, dir, "a.ts", "export const x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err != nil {
		t.Fatalf("hook failed with tsc findings: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Running tsc...") || !strings.Contains(out, "TypeScript errors found.") {
		t.Fatalf("output missing tsc fallback lines:\n%s", out)
	}
}

func TestNodeChecksScriptCheckFailureFailsHook(t *testing.T) {
	dir := newRepo(t)
	fakeDir := t.TempDir()
	buildFakeTool(t, fakeDir, "npx", 1)
	buildFakeTool(t, fakeDir, "npm", 1)
	writeFile(t, dir, "package.json", "{\"scripts\": {\"check\": \"tsc\"}}\n")
	stageFile(t, dir, "a.ts", "export const x = 1\n")
	out, err := runHook(t, dir, envWith(t, fakeDir))
	if err == nil {
		t.Fatalf("hook passed with failing check script:\n%s", out)
	}
	if !strings.Contains(out, "Type check failed.") {
		t.Fatalf("output missing type check failure line:\n%s", out)
	}
}

func TestPhpChecksRunVendorTools(t *testing.T) {
	dir := newRepo(t)
	writeFile(t, dir, "composer.json", "{}\n")
	vendor := filepath.Join(dir, "vendor", "bin")
	if err := os.MkdirAll(vendor, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.FromSlash("vendor/bin/pint.bat"), "@echo off\nexit /b 1\n")
	writeFile(t, dir, filepath.FromSlash("vendor/bin/php-cs-fixer.bat"), "@echo off\nexit /b 0\n")
	writeFile(t, dir, filepath.FromSlash("vendor/bin/phpstan.bat"), "@echo off\nexit /b 1\n")
	writeFile(t, dir, filepath.FromSlash("vendor/bin/psalm.bat"), "@echo off\nexit /b 1\n")
	stageFile(t, dir, "a.php", "<?php\n")
	out, err := runHook(t, dir, os.Environ())
	if err != nil {
		t.Fatalf("hook failed on php vendor tools: %v\n%s", err, out)
	}
	for _, want := range []string{"Running Laravel Pint...", "Running php-cs-fixer...", "Running PHPStan...", "Running Psalm...", "Psalm warnings."} {
		if !strings.Contains(out, want) {
			t.Fatalf("output missing %q:\n%s", want, out)
		}
	}
}

func TestInstallFailsWithoutGoInPath(t *testing.T) {
	dir := newRepo(t)
	t.Setenv("PATH", "")
	if err := Install(dir); err == nil || !strings.Contains(err.Error(), "go not found") {
		t.Fatalf("Install without go = %v, want go not found error", err)
	}
}

func TestInstallFailsOutsideAModule(t *testing.T) {
	dir := newRepo(t)
	err := Install(dir)
	if err == nil || !strings.Contains(err.Error(), "build pre-commit.exe") {
		t.Fatalf("Install outside a module = %v, want build failure error", err)
	}
}
