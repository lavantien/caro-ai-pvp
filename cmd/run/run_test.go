package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func testRunner() *runner {
	return &runner{target: "test", root: `C:\repo`, goExe: ".exe", base: map[string]string{"CGO_ENABLED": "1", "GOTMPDIR": `C:\repo\.gotmp`}}
}

func specs(t *testing.T, target string, v vars) []step {
	t.Helper()
	steps, err := buildSteps(target, testRunner(), v)
	if err != nil {
		t.Fatalf("buildSteps(%s): %v", target, err)
	}
	return steps
}

func TestBuildTarget(t *testing.T) {
	steps := specs(t, "build", vars{})
	if len(steps) != 1 {
		t.Fatalf("build steps = %d, want 1", len(steps))
	}
	s := steps[0]
	if s.path != "go" || !reflect.DeepEqual(s.args, []string{"build", "-o", "bin/caro.exe", "./cmd/caro"}) {
		t.Fatalf("build argv = %v %v", s.path, s.args)
	}
	if s.env["CGO_ENABLED"] != "1" {
		t.Fatalf("build env = %v", s.env)
	}
}

func TestCoverTarget(t *testing.T) {
	steps := specs(t, "cover", vars{"ARGS": "-run TestCover -count=1", "PKG": "./internal/server"})
	want := []string{"test", "-race", "-coverprofile=coverage.out", "-covermode=atomic", "-timeout", goTestTimeout, "-run", "TestCover", "-count=1", "./internal/server"}
	if !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("cover argv = %v, want %v", steps[0].args, want)
	}
	if steps[1].args[1] != "cover" || steps[1].args[len(steps[1].args)-1] != "-func=coverage.out" {
		t.Fatalf("cover-func argv = %v", steps[1].args)
	}
	if !reflect.DeepEqual(steps[2].args, []string{"run", "./cmd/covergate", "coverage.out"}) {
		t.Fatalf("covergate argv = %v", steps[2].args)
	}
}

func TestTestPkgTarget(t *testing.T) {
	steps := specs(t, "test-pkg", vars{"ARGS": "-run TestX", "PKG": "./internal/tourney"})
	want := []string{"test", "-race", "-timeout", goTestTimeout, "-run", "TestX", "./internal/tourney"}
	if !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("test-pkg argv = %v, want %v", steps[0].args, want)
	}
}

func TestHexNoiseTarget(t *testing.T) {
	steps := specs(t, "hex-noise", vars{})
	arena := steps[1]
	want := []string{"-geometry", "hex", "-floor", "hex-noise-tc1", "-floorSeat", "master-nobook", "-tc", "1", "-games", "20", "-floorDirs", "5", "-floorBlockDirs", "5", "-floorMaxBlocks", "3", "-floorVerdict", "hex-noise"}
	if arena.path != "bin/arena.exe" || !reflect.DeepEqual(arena.args, want) {
		t.Fatalf("hex-noise argv = %v %v", arena.path, arena.args)
	}
	if arena.env["GOMEMLIMIT"] != "12GiB" {
		t.Fatalf("hex-noise env = %v", arena.env)
	}
	steps = specs(t, "hex-noise", vars{"TC": "2", "GAMES": "40"})
	if got := steps[1].args[3]; got != "hex-noise-tc2" {
		t.Fatalf("hex-noise floor = %s, want hex-noise-tc2", got)
	}
	if got := steps[1].args[9]; got != "40" {
		t.Fatalf("hex-noise games = %s, want 40", got)
	}
}

func TestHexPassgatesDefaults(t *testing.T) {
	steps := specs(t, "hex-passgates", vars{})
	want := []string{"-geometry", "hex", "-passgate", "hard", "-tc", "1", "-games", "20"}
	if !reflect.DeepEqual(steps[1].args, want) {
		t.Fatalf("hex-passgates argv = %v, want %v", steps[1].args, want)
	}
}

func TestHexLadderDefaults(t *testing.T) {
	steps := specs(t, "hex-ladder", vars{})
	want := []string{"-geometry", "hex", "-ladder", "-tc", "1", "-games", "40"}
	if !reflect.DeepEqual(steps[1].args, want) {
		t.Fatalf("hex-ladder argv = %v, want %v", steps[1].args, want)
	}
}

func TestTourneyTargets(t *testing.T) {
	steps := specs(t, "tourney-smoke-32", vars{"ARGS": "--db scratch.db --parallel 1"})
	want := []string{"run", "./cmd/caro", "tourney", "smoke32", "--db", "scratch.db", "--parallel", "1"}
	if !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("smoke32 argv = %v, want %v", steps[0].args, want)
	}
	if steps[0].env["GOMEMLIMIT"] != "12GiB" {
		t.Fatalf("smoke32 env = %v", steps[0].env)
	}
	steps = specs(t, "tourney-close", vars{})
	if _, has := steps[0].env["GOMEMLIMIT"]; has {
		t.Fatalf("tourney-close carries GOMEMLIMIT: %v", steps[0].env)
	}
	steps = specs(t, "tourney-floor-gm32", vars{})
	if steps[0].args[3] != "floor-gm32" {
		t.Fatalf("floor-gm32 argv = %v", steps[0].args)
	}
}

func TestMutateStepArgs(t *testing.T) {
	got := mutateStepArgs(vars{"CHALLENGE": "1", "PARALLEL": "8", "ARGS": "-pkgs internal/clock"}, "4", []string{"-resume", "L"})
	want := []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", "8", "-challenge", "-pkgs", "internal/clock", "-resume", "L"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutateStepArgs = %v, want %v", got, want)
	}
	got = mutateStepArgs(vars{}, "1", nil)
	want = []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutateStepArgs defaults = %v, want %v", got, want)
	}
}

func TestMutateEnv(t *testing.T) {
	r := testRunner()
	env := r.mutateEnv(vars{})
	want := `C:\repo\.scratch\tmp`
	for _, k := range []string{"TMP", "TEMP", "GOTMPDIR"} {
		if env[k] != want {
			t.Fatalf("mutateEnv %s = %s, want %s", k, env[k], want)
		}
	}
	env = r.mutateEnv(vars{"MUTATE_TMP": `D:\elsewhere`})
	if env["TMP"] != `D:\elsewhere` || env["CGO_ENABLED"] != "1" {
		t.Fatalf("mutateEnv override = %v", env)
	}
}

func TestMutateSmokeEnv(t *testing.T) {
	steps := specs(t, "mutate-smoke", vars{})
	want := []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-allow-scope", "internal/rules", "-parallel", "2", "-pkgs", "internal/rules"}
	if !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("mutate-smoke argv = %v, want %v", steps[0].args, want)
	}
	if steps[0].env["GOTMPDIR"] != filepath.Join(testRunner().root, ".scratch", "tmp") {
		t.Fatalf("mutate-smoke env = %v", steps[0].env)
	}
}

func TestHasMutateVerdict(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "m.log")
	if err := os.WriteFile(p, []byte("mutate: bootstrap\nsome noise\nmutate: 497/497 run 1596s\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !hasMutateVerdict(p) {
		t.Fatal("verdict line present but not detected")
	}
	if err := os.WriteFile(p, []byte("noise xmutate: 1/1 run\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasMutateVerdict(p) {
		t.Fatal("mid-line match detected")
	}
	if err := os.WriteFile(p, []byte("mutate: bootstrap only\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if hasMutateVerdict(p) {
		t.Fatal("bootstrap line detected as verdict")
	}
}

func TestTailLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "t.log")
	if err := os.WriteFile(p, []byte("a\nb\nc\nd\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := tailLines(p, 2); !reflect.DeepEqual(got, []string{"c", "d"}) {
		t.Fatalf("tailLines = %v", got)
	}
	if got := tailLines(p, 9); len(got) != 4 {
		t.Fatalf("tailLines over-ask = %v", got)
	}
}

func TestGoVersionScore(t *testing.T) {
	cases := map[string]struct {
		want int
		ok   bool
	}{
		"1.27.2": {1027002, true},
		"1.27.1": {1027001, true},
		"1.27":   {1027000, true},
		"1.26.9": {1026009, true},
		"2.0":    {2000000, true},
		"x":      {0, false},
		"1.x.3":  {0, false},
	}
	for in, c := range cases {
		got, ok := goVersionScore(in)
		if ok != c.ok || got != c.want {
			t.Fatalf("goVersionScore(%q) = %d,%v want %d,%v", in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseVarsAndSplitArgs(t *testing.T) {
	v, err := parseVars([]string{"TC=2", "ARGS=-run X -count=1", "CHALLENGE=1"})
	if err != nil {
		t.Fatal(err)
	}
	if v["TC"] != "2" || v["CHALLENGE"] != "1" {
		t.Fatalf("vars = %v", v)
	}
	if got := splitArgs(v); !reflect.DeepEqual(got, []string{"-run", "X", "-count=1"}) {
		t.Fatalf("splitArgs = %v", got)
	}
	if _, err := parseVars([]string{"TC"}); err == nil {
		t.Fatal("bare token accepted")
	}
}

func TestScriptTargets(t *testing.T) {
	steps := specs(t, "bookval-short", vars{"ARGS": "-Games 12"})
	want := []string{"-NoProfile", "-File", "scripts/bookval-short.ps1", "-Games", "12"}
	if steps[0].path != "pwsh.exe" || !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("bookval-short = %v %v", steps[0].path, steps[0].args)
	}
	steps = specs(t, "evaladopt-arms", vars{})
	if steps[0].args[2] != "scripts/evaladopt-arms.ps1" {
		t.Fatalf("evaladopt-arms = %v", steps[0].args)
	}
}

func TestNestedModuleBinTargets(t *testing.T) {
	steps := specs(t, "logstats", vars{"ARGS": "--dir logs/tourny/run --out report.md"})
	if steps[0].dir != "playground/logstats" || !reflect.DeepEqual(steps[0].args, []string{"build", "-o", "../../bin/logstats.exe", "."}) {
		t.Fatalf("logstats build = %v dir=%s", steps[0].args, steps[0].dir)
	}
	if steps[1].path != "bin/logstats.exe" || !reflect.DeepEqual(steps[1].args, []string{"--dir", "logs/tourny/run", "--out", "report.md"}) {
		t.Fatalf("logstats run = %v %v", steps[1].path, steps[1].args)
	}
	steps = specs(t, "db-checkpoint", vars{})
	if steps[1].path != "bin/walckpt.exe" || !reflect.DeepEqual(steps[1].args, []string{"db"}) {
		t.Fatalf("db-checkpoint = %v %v", steps[1].path, steps[1].args)
	}
	steps = specs(t, "clocktune", vars{"ARGS": "-tc 3"})
	if steps[0].args[len(steps[0].args)-1] != "clocktune.exe" && steps[0].args[len(steps[0].args)-1] != "." {
		t.Fatalf("clocktune build = %v", steps[0].args)
	}
	if steps[1].path != "bin/clocktune.exe" || !reflect.DeepEqual(steps[1].args, []string{"-tc", "3"}) {
		t.Fatalf("clocktune run = %v %v", steps[1].path, steps[1].args)
	}
}

func TestBookgenTargets(t *testing.T) {
	steps := specs(t, "bookgen", vars{"ARGS": "-phase corpus"})
	if steps[0].env["GOMEMLIMIT"] != "12GiB" {
		t.Fatalf("bookgen-build env = %v", steps[0].env)
	}
	if steps[1].path != "bin/bookgen.exe" || !reflect.DeepEqual(steps[1].args, []string{"-phase", "corpus"}) {
		t.Fatalf("bookgen run = %v %v", steps[1].path, steps[1].args)
	}
	if steps[1].env["GOMEMLIMIT"] != "12GiB" {
		t.Fatalf("bookgen env = %v", steps[1].env)
	}
	steps = specs(t, "booktree", vars{"ARGS": "-tier master playground/bookgen/out/book-corpus.txt"})
	want := []string{"-phase", "tree", "-tier", "master", "playground/bookgen/out/book-corpus.txt"}
	if !reflect.DeepEqual(steps[1].args, want) {
		t.Fatalf("booktree argv = %v", steps[1].args)
	}
	steps = specs(t, "bookgen-test", vars{})
	if steps[1].env["GOMEMLIMIT"] != "2GiB" || steps[1].args[0] != "test" {
		t.Fatalf("bookgen-test = %v %v", steps[1].args, steps[1].env)
	}
}

func TestRegateArmsAndChain(t *testing.T) {
	if !reflect.DeepEqual(regateArms, []string{"regate21-r1", "regate21-r2", "regate21-r3", "regate32-r1"}) {
		t.Fatalf("regateArms = %v", regateArms)
	}
	root := t.TempDir()
	t.Chdir(root)
	logs := filepath.Join("logs", "tourny")
	old := filepath.Join(logs, "1000-regate21-r1")
	fresh := filepath.Join(logs, "2000-regate21-r1")
	for _, d := range []string{old, fresh} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(fresh, "summary.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	os.Chtimes(fresh, time.Now(), time.Now())
	if got := newestRunDir("regate21-r1"); got != fresh {
		t.Fatalf("newestRunDir = %s, want %s", got, fresh)
	}
	if got := newestRunDir("regate32-r1"); got != "" {
		t.Fatalf("newestRunDir missing arm = %s", got)
	}
}

func TestDiagramNeedsRender(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.MkdirAll(diagramsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mmd := filepath.Join(diagramsDir, "a.mmd")
	cfg := filepath.Join(diagramsDir, "mermaid-config.json")
	pup := filepath.Join(diagramsDir, "puppeteer.json")
	for _, f := range []string{mmd, cfg, pup} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	png := filepath.Join(diagramsDir, "a.png")
	if !diagramNeedsRender(png, mmd) {
		t.Fatal("missing png must render")
	}
	if err := os.WriteFile(png, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if diagramNeedsRender(png, mmd) {
		t.Fatal("fresh png must not render")
	}
	os.Chtimes(mmd, time.Now(), time.Now())
	if !diagramNeedsRender(png, mmd) {
		t.Fatal("newer mmd must render")
	}
	past := time.Now().Add(-time.Hour)
	os.Chtimes(png, past, past)
	os.Chtimes(cfg, time.Now(), time.Now())
	if !diagramNeedsRender(png, mmd) {
		t.Fatal("newer config must render")
	}
}

func TestChildEnvOverrides(t *testing.T) {
	r := testRunner()
	env := r.childEnv(map[string]string{"GOMEMLIMIT": "12GiB"})
	sawCgo, sawMem, sawTmp := 0, 0, 0
	for _, kv := range env {
		switch {
		case strings.HasPrefix(kv, "CGO_ENABLED="):
			sawCgo++
			if kv != "CGO_ENABLED=1" {
				t.Fatalf("CGO_ENABLED = %s", kv)
			}
		case strings.HasPrefix(kv, "GOMEMLIMIT="):
			sawMem++
		case strings.HasPrefix(kv, "GOTMPDIR="):
			sawTmp++
			if kv != "GOTMPDIR="+`C:\repo\.gotmp` {
				t.Fatalf("GOTMPDIR = %s", kv)
			}
		}
	}
	if sawCgo != 1 || sawMem != 1 || sawTmp != 1 {
		t.Fatalf("dup or missing env keys: cgo=%d mem=%d tmp=%d", sawCgo, sawMem, sawTmp)
	}
}

func TestTargetInventoryMatchesMakefile(t *testing.T) {
	want := []string{
		"all", "doctor", "fmt", "fmt-check", "lint", "vet", "build", "test", "test-race", "test-pkg",
		"cover", "cover-pcts", "bench", "fuzz", "mutate", "mutate-resume", "mutate-smoke", "mutate-full",
		"run", "serve", "migrate", "shots", "tourney-smoke-32", "tourney-smoke-10", "tourney-smoke-105",
		"tourney-ponderprobe", "tourney-floor-hard21", "tourney-floor-hard32", "tourney-floor-gm21",
		"tourney-floor-gm32", "tourney-regate21-r1", "tourney-regate21-r2", "tourney-regate21-r3",
		"tourney-regate32-r1", "tourney-regate-chain", "tourney-full", "tourney-resume", "tourney-close",
		"firewall", "tidy", "diagrams", "contrast", "icons", "ci", "logstats", "crossarm", "db-checkpoint",
		"arena", "arena-test", "hex-noise", "hex-passgates", "hex-ladder", "bookgen", "bookgen-build",
		"booktree", "bookgen-test", "clocktune", "playground-modules", "evalfit-test", "gpu-spike",
		"bookval-short", "bookval-confirm", "bookval-cross", "booktree-run", "evaladopt-arms",
	}
	for _, w := range want {
		_, inReg := registry[w]
		_, inFlow := flows[w]
		if !inReg && !inFlow {
			t.Errorf("make target %q missing from the runner", w)
		}
	}
}

func TestPlaygroundModulesSteps(t *testing.T) {
	t.Chdir("../..")
	steps, err := buildSteps("playground-modules", testRunner(), vars{})
	if err != nil {
		t.Fatal(err)
	}
	if len(steps) == 0 {
		t.Fatal("no playground module steps")
	}
	for _, s := range steps {
		if s.path != "go" || !reflect.DeepEqual(s.args, []string{"-C", s.dir[strings.LastIndex(s.dir, string(os.PathSeparator))+1:], "build", "-o", "../../bin/", "."}) {
			t.Fatalf("module step = %v %v", s.path, s.args)
		}
		if !strings.HasPrefix(s.dir, filepath.Join("playground")) {
			t.Fatalf("module dir = %s", s.dir)
		}
	}
}
