package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeShim(t *testing.T, dir, name string, lines ...string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		body := "@echo off\r\n" + strings.Join(lines, "\r\n") + "\r\n"
		if err := os.WriteFile(filepath.Join(dir, name+".bat"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	body := "#!/bin/sh\n" + strings.Join(lines, "\n") + "\n"
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func tempRootRunner(t *testing.T) *runner {
	t.Helper()
	dir := t.TempDir()
	return &runner{target: "test", root: dir, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": filepath.Join(dir, ".gotmp")}}
}

func TestDoctorFlowFailsWithoutCToolchain(t *testing.T) {
	shim := filepath.Join(t.TempDir(), "bin")
	writeShim(t, shim, "go", "echo go version go1.27 fake")
	t.Setenv("PATH", shim)
	err := flows["doctor"](tempRootRunner(t), vars{})
	if err == nil || !strings.Contains(err.Error(), "no C toolchain") {
		t.Fatalf("doctor without gcc or clang = %v, want no C toolchain error", err)
	}
}

func TestDoctorFlowFailsWhenCCVersionFails(t *testing.T) {
	shim := filepath.Join(t.TempDir(), "bin")
	writeShim(t, shim, "go", "echo go version go1.27 fake")
	writeShim(t, shim, "gcc", "exit 1")
	t.Setenv("PATH", shim)
	err := flows["doctor"](tempRootRunner(t), vars{})
	if err == nil || !strings.Contains(err.Error(), "--version failed") {
		t.Fatalf("doctor with a failing cc = %v, want --version failed error", err)
	}
}

func TestDoctorFlowRejectsUnknownGoVersion(t *testing.T) {
	shim := filepath.Join(t.TempDir(), "bin")
	writeShim(t, shim, "go", "echo not a go version string")
	writeShim(t, shim, "gcc", "echo gcc fake 13.0")
	t.Setenv("PATH", shim)
	err := flows["doctor"](tempRootRunner(t), vars{})
	if err == nil || !strings.Contains(err.Error(), "unparsable go version") {
		t.Fatalf("doctor with garbage GOVERSION = %v, want unparsable error", err)
	}
}

func TestDoctorFlowPassesOnRealToolchain(t *testing.T) {
	_, gccErr := exec.LookPath("gcc")
	_, clangErr := exec.LookPath("clang")
	err := flows["doctor"](tempRootRunner(t), vars{})
	if gccErr != nil && clangErr != nil {
		if err == nil || !strings.Contains(err.Error(), "no C toolchain") {
			t.Fatalf("doctor without any cc = %v, want no C toolchain error", err)
		}
		return
	}
	if err != nil {
		t.Fatalf("doctor on a real toolchain = %v, want nil", err)
	}
}

func TestGofmtPathFallsBackWhenGorootBinMissing(t *testing.T) {
	shim := filepath.Join(t.TempDir(), "bin")
	writeShim(t, shim, "go", "echo not-a-root")
	t.Setenv("PATH", shim)
	if got := gofmtPath(); got != "gofmt" {
		t.Fatalf("gofmtPath with unreachable GOROOT bin = %q, want gofmt", got)
	}
}

func TestParseTasklistCSVRows(t *testing.T) {
	if got := parseTasklistCSV("\"img,\"unclosed\n"); got != nil {
		t.Fatalf("malformed csv = %v, want nil", got)
	}
	raw := "image,pid\ncolumnonly\n,500\nproc.exe,\nproc.exe,500\n"
	got := parseTasklistCSV(raw)
	if len(got) != 2 || got[0].Image != "image" || got[1].Image != "proc.exe" || got[1].PID != "500" {
		t.Fatalf("parseTasklistCSV kept %v, want the two-field rows only", got)
	}
}

func TestDiagramsFlowRendersThroughStub(t *testing.T) {
	dir := t.TempDir()
	diagrams := filepath.Join(dir, "docs", "diagrams")
	if err := os.MkdirAll(diagrams, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(diagrams, "a.puml"), []byte("@startuml\nx -> y\n@enduml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLANTUML_JAR", filepath.Join(dir, "plantuml.jar"))
	t.Chdir(dir)
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, nil })
	r := tempRootRunner(t)
	if err := flows["diagrams"](r, vars{}); err != nil {
		t.Fatalf("diagrams with stubbed java = %v, want nil", err)
	}
	stubExecStep(t, func(s step, env []string) (int, error) { return 0, errors.New("java spawn failed") })
	if err := flows["diagrams"](r, vars{}); err == nil {
		t.Fatal("diagrams with failing java must error")
	}
}

func TestDiagramsFlowSkipsFreshPng(t *testing.T) {
	dir := t.TempDir()
	diagrams := filepath.Join(dir, "docs", "diagrams")
	if err := os.MkdirAll(diagrams, 0o755); err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(diagrams, "a.puml")
	if err := os.WriteFile(src, []byte("@startuml\nx -> y\n@enduml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(strings.TrimSuffix(src, ".puml")+".png", []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLANTUML_JAR", filepath.Join(dir, "plantuml.jar"))
	t.Chdir(dir)
	called := false
	stubExecStep(t, func(s step, env []string) (int, error) { called = true; return 0, nil })
	if err := flows["diagrams"](tempRootRunner(t), vars{}); err != nil {
		t.Fatalf("diagrams with fresh png = %v, want nil", err)
	}
	if called {
		t.Fatal("fresh png must not re-render")
	}
}

func TestFmtCheckFlowReportsUnformattedFile(t *testing.T) {
	dir := tempGitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "clean.go"), []byte("package tmp\n\nfunc f() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty := "package  tmp\n"
	if err := os.WriteFile(filepath.Join(dir, "dirty.go"), []byte(dirty), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"clean.go", "dirty.go"} {
		c := exec.Command("git", "add", name)
		c.Dir = dir
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git add %s: %v: %s", name, err, out)
		}
	}
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(dir))
	t.Chdir(dir)
	if err := flows["fmt-check"](tempRootRunner(t), vars{}); err == nil {
		t.Fatal("fmt-check with a dirty file must fail")
	}
}

func TestPlaygroundModulesBuildsOnlyRootBinaries(t *testing.T) {
	dir := t.TempDir()
	for mod, files := range map[string]map[string]string{
		"a": {"go.mod": "module a\n\ngo 1.27\n", "main.go": "package main\n\nfunc main() {}\n"},
		"b": {"plain.txt": "x"},
		"c": {"go.mod": "module c\n\ngo 1.27\n", "lib.go": "package lib\n"},
	} {
		sub := filepath.Join(dir, "playground", mod)
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(sub, name), []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "playground", "loose.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	steps, err := buildSteps("playground-modules", tempRootRunner(t), vars{})
	if err != nil {
		t.Fatalf("playground-modules over fake tree: %v", err)
	}
	if len(steps) != 1 || steps[0].name != "build a" {
		t.Fatalf("playground-modules steps = %v, want only build a", steps)
	}
}

func TestPlaygroundModulesRequiresPlaygroundDir(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := buildSteps("playground-modules", tempRootRunner(t), vars{}); err == nil {
		t.Fatal("playground-modules without a playground dir must error")
	}
}
