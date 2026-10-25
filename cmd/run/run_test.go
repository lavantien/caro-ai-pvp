package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
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

func TestCoverBadgesTarget(t *testing.T) {
	steps := specs(t, "cover-badges", vars{"SHA": "abc123", "RUN_ID": "42", "VERDICT": "failed", "OUT": "badges"})
	want := []string{"run", "./cmd/covergate", "-badges", "coverage.out", "badges", "abc123", "42", "failed"}
	if steps[0].path != "go" || !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("cover-badges = %v %v", steps[0].path, steps[0].args)
	}
	steps = specs(t, "cover-badges", vars{"SHA": "abc123", "RUN_ID": "42", "VERDICT": "passed"})
	if !reflect.DeepEqual(steps[0].args, []string{"run", "./cmd/covergate", "-badges", "coverage.out", badgesDir, "abc123", "42", "passed"}) {
		t.Fatalf("cover-badges default out = %v", steps[0].args)
	}
	for _, missing := range []string{"RUN_ID", "SHA", "VERDICT"} {
		v := vars{"SHA": "abc123", "RUN_ID": "42", "VERDICT": "passed"}
		delete(v, missing)
		if _, err := buildSteps("cover-badges", testRunner(), v); err == nil {
			t.Fatalf("cover-badges built without %s", missing)
		}
	}
}

func TestCoverVerdictTarget(t *testing.T) {
	steps := specs(t, "cover-verdict", vars{"VERDICT": "failed", "SHA": "abc123", "RUN_ID": "42", "OUT": "b"})
	want := []string{"run", "./cmd/covergate", "-verdict", "failed", "b", "abc123", "42"}
	if steps[0].path != "go" || !reflect.DeepEqual(steps[0].args, want) {
		t.Fatalf("cover-verdict = %v %v", steps[0].path, steps[0].args)
	}
	steps = specs(t, "cover-verdict", vars{"VERDICT": "passed", "SHA": "abc123", "RUN_ID": "42"})
	if !reflect.DeepEqual(steps[0].args, []string{"run", "./cmd/covergate", "-verdict", "passed", badgesDir, "abc123", "42"}) {
		t.Fatalf("cover-verdict default out = %v", steps[0].args)
	}
	for _, missing := range []string{"RUN_ID", "SHA", "VERDICT"} {
		v := vars{"SHA": "abc123", "RUN_ID": "42", "VERDICT": "passed"}
		delete(v, missing)
		if _, err := buildSteps("cover-verdict", testRunner(), v); err == nil {
			t.Fatalf("cover-verdict built without %s", missing)
		}
	}
}

func TestBadgeRunID(t *testing.T) {
	got := badgeRunID(time.Date(2026, 10, 25, 12, 0, 0, 0, time.UTC))
	if got != "20261025-120000" {
		t.Fatalf("badgeRunID = %q, want 20261025-120000", got)
	}
}

func TestBadgeRegenTarget(t *testing.T) {
	target, verdict := badgeRegenTarget(nil)
	if target != "cover-badges" || verdict != config.VerdictPassed {
		t.Fatalf("badgeRegenTarget(nil) = %q %q, want cover-badges passed", target, verdict)
	}
	target, verdict = badgeRegenTarget(fmt.Errorf("chain failed"))
	if target != "cover-verdict" || verdict != config.VerdictFailed {
		t.Fatalf("badgeRegenTarget(err) = %q %q, want cover-verdict failed", target, verdict)
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
	got, err := mutateStepArgs(vars{"CHALLENGE": "1", "PARALLEL": "8", "ARGS": "-pkgs internal/clock"}, "4", []string{"-resume", "L"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", "8", "-challenge", "-pkgs", "internal/clock", "-resume", "L"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutateStepArgs = %v, want %v", got, want)
	}
	got, err = mutateStepArgs(vars{}, "1", nil)
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", "1"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mutateStepArgs defaults = %v, want %v", got, want)
	}
	if _, err := mutateStepArgs(vars{"ARGS": `--db "a`}, "1", nil); err == nil {
		t.Fatal("malformed ARGS accepted")
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
	got, err := splitArgs(v)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"-run", "X", "-count=1"}) {
		t.Fatalf("splitArgs = %v", got)
	}
	if _, err := parseVars([]string{"TC"}); err == nil {
		t.Fatal("bare token accepted")
	}
}

func TestParseArgTokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
		err  string
	}{
		{in: "-run X -count=1", want: []string{"-run", "X", "-count=1"}},
		{in: `--out "a b.md" -tc 3`, want: []string{"--out", "a b.md", "-tc", "3"}},
		{in: `--dir logs/tourny/run --out report.md`, want: []string{"--dir", "logs/tourny/run", "--out", "report.md"}},
		{in: `""`, want: []string{""}},
		{in: `x y  z`, want: []string{"x", "y", "z"}},
		{in: `--db "C:\a`, err: "unterminated"},
		{in: `a"b`, err: "unquoted"},
		{in: `"a"b`, err: "closing"},
		{in: `"a""b"`, err: "unquoted"},
	}
	for _, c := range cases {
		got, err := parseArgTokens(c.in)
		if c.err != "" {
			if err == nil || !strings.Contains(err.Error(), c.err) {
				t.Errorf("parseArgTokens(%q) err = %v, want containing %q", c.in, err, c.err)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseArgTokens(%q) unexpected err %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseArgTokens(%q) = %v, want %v", c.in, got, c.want)
		}
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
	if steps[0].dir != "" || !reflect.DeepEqual(steps[0].args, []string{"-C", "playground/bookgen", "build", "-o", "../../bin/bookgen.exe", "."}) {
		t.Fatalf("bookgen-build = %v dir=%q", steps[0].args, steps[0].dir)
	}
	if steps[0].env["GOMEMLIMIT"] != "12GiB" || steps[0].env["CGO_ENABLED"] != "1" {
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
	if err := os.Chtimes(fresh, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
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
	src := filepath.Join(diagramsDir, "a.puml")
	common := filepath.Join(diagramsDir, "common.iuml")
	for _, f := range []string{src, common} {
		if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	png := filepath.Join(diagramsDir, "a.png")
	if !diagramNeedsRender(png, src) {
		t.Fatal("missing png must render")
	}
	if err := os.WriteFile(png, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	if diagramNeedsRender(png, src) {
		t.Fatal("fresh png must not render")
	}
	if err := os.Chtimes(src, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if !diagramNeedsRender(png, src) {
		t.Fatal("newer source must render")
	}
	past := time.Now().Add(-time.Hour)
	if err := os.Chtimes(png, past, past); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(common, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if !diagramNeedsRender(png, src) {
		t.Fatal("newer common style must render")
	}
	if err := os.Chtimes(png, time.Now(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(common); err != nil {
		t.Fatal(err)
	}
	if diagramNeedsRender(png, src) {
		t.Fatal("absent common style must not force a render")
	}
}

func TestFindPlantumlJar(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PLANTUML_JAR", filepath.Join(dir, "custom.jar"))
	jar, err := findPlantumlJar()
	if err != nil || jar != filepath.Join(dir, "custom.jar") {
		t.Fatalf("env override = %s, %v", jar, err)
	}
	home := t.TempDir()
	old := filepath.Join(home, "scoop", "apps", "plantuml", "1.2026.8")
	newest := filepath.Join(home, "scoop", "apps", "plantuml", "1.2026.9")
	for _, v := range []string{old, newest} {
		if err := os.MkdirAll(v, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(v, "plantuml.jar"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	os.Unsetenv("PLANTUML_JAR")
	t.Setenv("USERPROFILE", home)
	t.Setenv("HOME", home)
	jar, err = findPlantumlJar()
	if err != nil || jar != filepath.Join(newest, "plantuml.jar") {
		t.Fatalf("scoop discovery = %s, %v", jar, err)
	}
	empty := t.TempDir()
	t.Setenv("USERPROFILE", empty)
	t.Setenv("HOME", empty)
	if jar, err := findPlantumlJar(); err == nil {
		t.Fatalf("missing jar resolved to %s", jar)
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

func TestRunnerTargetSurface(t *testing.T) {
	want := map[string]bool{
		"all": true, "build": true, "lint": true, "vet": true, "test": true,
		"test-race": true, "test-pkg": true, "cover": true, "cover-pcts": true, "cover-badges": true, "cover-verdict": true, "bench": true,
		"tidy": true, "run": true, "migrate": true, "serve": true, "firewall": true,
		"contrast": true, "icons": true, "shots": true, "playground-modules": true, "evalfit-test": true,
		"arena-test": true, "bookgen-build": true, "bookgen": true, "booktree": true, "bookgen-test": true,
		"clocktune": true, "logstats": true, "crossarm": true, "db-checkpoint": true, "gpu-spike": true,
		"arena": true, "hex-noise": true, "hex-passgates": true, "hex-ladder": true,
		"bookval-short": true, "bookval-confirm": true, "bookval-cross": true, "booktree-run": true, "evaladopt-arms": true,
		"mutate": true, "mutate-smoke": true,
		"tourney-smoke-32": true, "tourney-smoke-10": true, "tourney-smoke-105": true,
		"tourney-ponderprobe": true, "tourney-floor-hard21": true, "tourney-floor-hard32": true,
		"tourney-floor-gm21": true, "tourney-floor-gm32": true,
		"tourney-regate21-r1": true, "tourney-regate21-r2": true, "tourney-regate21-r3": true,
		"tourney-regate32-r1": true, "tourney-full": true, "tourney-resume": true, "tourney-close": true,
		"doctor": true, "fmt": true, "fmt-check": true, "fuzz": true,
		"mutate-resume": true, "mutate-full": true, "tourney-regate-chain": true, "diagrams": true, "ci": true,
	}
	if len(want) < 67 {
		t.Fatalf("frozen surface holds only %d targets, the literal was truncated", len(want))
	}
	runnerNames := map[string]bool{}
	for n := range registry {
		runnerNames[n] = true
	}
	for n := range flows {
		runnerNames[n] = true
	}
	for n := range want {
		if !runnerNames[n] {
			t.Errorf("frozen target %q missing from the runner", n)
		}
	}
	for n := range runnerNames {
		if !want[n] {
			t.Errorf("runner target %q absent from the frozen surface", n)
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
		if s.path != "go" || !strings.HasPrefix(strings.Join(s.args, " "), "-C playground"+string(os.PathSeparator)) {
			t.Fatalf("module step = %v %v", s.path, s.args)
		}
		if !reflect.DeepEqual(s.args[2:], []string{"build", "-o", "../../bin/", "."}) || s.dir != "" {
			t.Fatalf("module argv = %v dir=%q", s.args, s.dir)
		}
	}
}
