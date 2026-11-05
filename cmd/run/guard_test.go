package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildOutputImageVariants(t *testing.T) {
	cases := []struct {
		name  string
		step  step
		goExe string
		want  string
		ok    bool
	}{
		{"all target", step{name: "build", path: "go", args: []string{"build", "-o", "bin/caro.exe", "./cmd/caro"}}, ".exe", "caro.exe", true},
		{"all target linux", step{name: "build", path: "go", args: []string{"build", "-o", "bin/caro", "./cmd/caro"}}, "", "caro", true},
		{"playground module dir output", step{name: "build arena", path: "go", args: []string{"-C", "playground/arena", "build", "-o", "../../bin/", "."}}, ".exe", "arena.exe", true},
		{"dir output with step dir", step{name: "build arena", path: "go", args: []string{"build", "-o", "../../bin/", "."}, dir: "playground/arena"}, ".exe", "arena.exe", true},
		{"named output", step{name: "arena-build", path: "go", args: []string{"build", "-o", "../../bin/arena.exe", "."}, dir: "playground/arena"}, ".exe", "arena.exe", true},
		{"named output without ext", step{name: "logstats-build", path: "go", args: []string{"build", "-o", "../../bin/logstats", "."}}, ".exe", "logstats.exe", true},
		{"nvcc shared", step{name: "nvcc", path: "nvcc", args: []string{"-shared", "-o", "bin/spike.dll", "playground/gpuoffload/spike.cu"}}, ".exe", "spike.dll", true},
		{"go test has no output", step{name: "test", path: "go", args: []string{"test", "-race", "./..."}}, ".exe", "", false},
		{"go run has no output", step{name: "run", path: "go", args: []string{"run", "./cmd/caro"}}, ".exe", "", false},
		{"non go path", step{name: "contrast", path: "go", args: []string{"run", "./contrast"}, dir: "playground"}, ".exe", "", false},
		{"plain exe path", step{name: "arena", path: "bin/arena.exe", args: []string{"-ladder"}}, ".exe", "", false},
	}
	for _, tc := range cases {
		got, ok := buildOutputImage(tc.step, tc.goExe)
		if got != tc.want || ok != tc.ok {
			t.Errorf("%s: buildOutputImage = (%q,%v), want (%q,%v)", tc.name, got, ok, tc.want, tc.ok)
		}
	}
}

func TestParseTasklistCSV(t *testing.T) {
	raw := "\"arena.exe\",\"31144\",\"Console\",\"1\",\"2,058,240 K\"\r\n" +
		"\"pwsh.exe\",\"28300\",\"Console\",\"1\",\"31,004 K\"\r\n" +
		"\"arena.exe~\",\"15020\",\"Console\",\"1\",\"1,165,000 K\"\r\n"
	got := parseTasklistCSV(raw)
	want := []runningProcess{{"arena.exe", "31144"}, {"pwsh.exe", "28300"}, {"arena.exe~", "15020"}}
	if len(got) != len(want) {
		t.Fatalf("parseTasklistCSV returned %d rows, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRunningPIDsForOwnImage(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	image := filepath.Base(exe)
	pids, err := runningPIDsFor(image)
	if err != nil {
		t.Fatalf("runningPIDsFor(%q): %v", image, err)
	}
	self := itoa(os.Getpid())
	found := false
	for _, p := range pids {
		if p == self {
			found = true
		}
	}
	if !found {
		t.Errorf("runningPIDsFor(%q) = %v, want own pid %s present", image, pids, self)
	}
}

func TestRunningPIDsForForeignImage(t *testing.T) {
	image := "definitely-not-running-9f3a1c.exe"
	pids, err := runningPIDsFor(image)
	if err != nil {
		t.Fatalf("runningPIDsFor(%q): %v", image, err)
	}
	if len(pids) != 0 {
		t.Errorf("runningPIDsFor(%q) = %v, want none", image, pids)
	}
}

func TestGuardRefusesBuildOfRunningImage(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	image := filepath.Base(exe)
	goExe := ""
	if strings.EqualFold(filepath.Ext(image), ".exe") {
		goExe = ".exe"
	}
	r := &runner{target: "test-target", root: ".", goExe: goExe, base: map[string]string{"GOTMPDIR": t.TempDir()}}
	s := step{name: "build", path: "go", args: []string{"build", "-o", "bin/" + image, "."}}
	_, err = r.runCode(s)
	if err == nil {
		t.Fatalf("runCode accepted a build over the running image %q", image)
	}
	msg := err.Error()
	if !strings.Contains(msg, image) {
		t.Errorf("guard error lacks image name %q: %s", image, msg)
	}
	if !strings.Contains(msg, "running") {
		t.Errorf("guard error lacks the running wording: %s", msg)
	}
	if !strings.Contains(msg, "defer") && !strings.Contains(msg, "stop") {
		t.Errorf("guard error lacks the instruction wording: %s", msg)
	}
}
