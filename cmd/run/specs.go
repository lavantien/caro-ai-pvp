package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type specBuilder func(r *runner, v vars) ([]step, error)

var registry = map[string]specBuilder{}

func register(name string, b specBuilder) {
	registry[name] = b
}

func splitArgs(v vars) ([]string, error) {
	if !v.has("ARGS") {
		return nil, nil
	}
	return parseArgTokens(v["ARGS"])
}

func parseArgTokens(s string) ([]string, error) {
	var out []string
	var b strings.Builder
	inQuote := false
	closed := false
	started := false
	appendTok := func() {
		if started {
			out = append(out, b.String())
			b.Reset()
		}
		started = false
		closed = false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '"':
			if inQuote {
				inQuote = false
				closed = true
				continue
			}
			if closed || started {
				return nil, fmt.Errorf("run: quote inside an unquoted ARGS token: %q", s)
			}
			inQuote = true
			started = true
		case (c == ' ' || c == '\t') && !inQuote:
			appendTok()
		default:
			if closed {
				return nil, fmt.Errorf("run: characters after a closing ARGS quote: %q", s)
			}
			b.WriteByte(c)
			started = true
		}
	}
	if inQuote {
		return nil, fmt.Errorf("run: unterminated ARGS quote: %q", s)
	}
	appendTok()
	return out, nil
}

func goStep(name string, args ...string) step {
	return step{name: name, path: "go", args: args, env: map[string]string{"CGO_ENABLED": "1"}}
}

func goDirStep(name, dir string, args ...string) step {
	s := goStep(name, args...)
	s.dir = dir
	return s
}

func tourneyEnv(r *runner, v vars) map[string]string {
	return map[string]string{"CGO_ENABLED": "1", "GOMEMLIMIT": v.get("TOURNEY_MEMLIMIT", "12GiB")}
}

func registerGoTargets() {
	register("all", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("build", "build", "-o", "bin/caro"+r.goExe, "./cmd/caro")}, nil
	})
	register("build", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("build", "build", "-o", "bin/caro"+r.goExe, "./cmd/caro")}, nil
	})
	register("lint", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("lint", "vet", "./...")}, nil
	})
	register("vet", registry["lint"])
	register("test", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("test", "test", "-timeout", goTestTimeout, "./...")}, nil
	})
	register("test-race", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("test-race", "test", "-race", "-timeout", goTestTimeout, "./...")}, nil
	})
	register("test-pkg", func(r *runner, v vars) ([]step, error) {
		extra, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		args := append([]string{"test", "-race", "-timeout", goTestTimeout}, extra...)
		args = append(args, v.get("PKG", "./..."))
		return []step{{name: "test-pkg", path: "go", args: args, env: map[string]string{"CGO_ENABLED": "1"}}}, nil
	})
	register("cover", func(r *runner, v vars) ([]step, error) {
		extra, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		args := append([]string{"test", "-race", "-coverprofile=coverage.out", "-covermode=atomic", "-timeout", goTestTimeout}, extra...)
		args = append(args, v.get("PKG", "./..."))
		return []step{
			{name: "cover", path: "go", args: args, env: map[string]string{"CGO_ENABLED": "1"}},
			goStep("cover-func", "tool", "cover", "-func=coverage.out"),
			goStep("covergate", "run", "./cmd/covergate", "coverage.out"),
		}, nil
	})
	register("cover-pcts", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("cover-pcts", "run", "./cmd/covergate", "-pcts", "coverage.out")}, nil
	})
	register("cover-badges", func(r *runner, v vars) ([]step, error) {
		if !v.has("SHA") || !v.has("RUN_ID") || !v.has("VERDICT") {
			return nil, fmt.Errorf("run: cover-badges needs SHA=, RUN_ID= and VERDICT= provenance vars")
		}
		return []step{goStep("cover-badges", "run", "./cmd/covergate", "-badges", "coverage.out", v.get("OUT", "."), v["SHA"], v["RUN_ID"], v["VERDICT"])}, nil
	})
	register("cover-verdict", func(r *runner, v vars) ([]step, error) {
		if !v.has("SHA") || !v.has("RUN_ID") || !v.has("VERDICT") {
			return nil, fmt.Errorf("run: cover-verdict needs SHA=, RUN_ID= and VERDICT= provenance vars")
		}
		return []step{goStep("cover-verdict", "run", "./cmd/covergate", "-verdict", v["VERDICT"], v.get("OUT", "."), v["SHA"], v["RUN_ID"])}, nil
	})
	register("bench", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("bench", "test", "-run=^$", "-bench=.", "-benchmem", "./...")}, nil
	})
	register("tidy", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("tidy", "mod", "tidy")}, nil
	})
	register("run", func(r *runner, v vars) ([]step, error) {
		args, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		if len(args) == 0 {
			args = []string{"ports"}
		}
		return []step{{name: "run", path: "go", args: append([]string{"run", "./cmd/caro"}, args...), env: map[string]string{"CGO_ENABLED": "1"}}}, nil
	})
	register("migrate", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("migrate", "run", "./cmd/caro", "migrate")}, nil
	})
	register("serve", func(r *runner, v vars) ([]step, error) {
		extra, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{{name: "serve", path: "go", args: append([]string{"run", "./cmd/caro", "serve"}, extra...), env: map[string]string{"CGO_ENABLED": "1"}}}, nil
	})
	register("firewall", func(r *runner, v vars) ([]step, error) {
		return []step{goStep("firewall", "run", "./cmd/caro", "firewall")}, nil
	})
}

func registerPlaygroundTargets() {
	register("contrast", func(r *runner, v vars) ([]step, error) {
		return []step{goDirStep("contrast", "playground", "run", "./contrast")}, nil
	})
	register("icons", func(r *runner, v vars) ([]step, error) {
		return []step{goDirStep("icons", "playground", "run", "./icongen", "-out", "../internal/server/web/static/icons")}, nil
	})
	register("shots", func(r *runner, v vars) ([]step, error) {
		extra, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{{name: "shots", path: "go", args: append([]string{"-C", "playground/shots", "run", "."}, extra...), env: map[string]string{"CGO_ENABLED": "1"}}}, nil
	})
	register("playground-modules", func(r *runner, v vars) ([]step, error) {
		entries, err := os.ReadDir("playground")
		if err != nil {
			return nil, err
		}
		var steps []step
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			if _, err := os.Stat(filepath.Join("playground", e.Name(), "go.mod")); err != nil {
				continue
			}
			steps = append(steps, step{
				name: "build " + e.Name(),
				path: "go",
				args: []string{"-C", filepath.Join("playground", e.Name()), "build", "-o", "../../bin/", "."},
				env:  map[string]string{"CGO_ENABLED": "1"},
			})
		}
		return steps, nil
	})
	register("evalfit-test", func(r *runner, v vars) ([]step, error) {
		return []step{
			goDirStep("evalfit-vet", "playground/evalfit", "vet", "./..."),
			{name: "evalfit-test", path: "go", args: []string{"test", "-race", "-parallel", "2", "-timeout", goTestTimeout, "./..."}, dir: "playground/evalfit", env: map[string]string{"CGO_ENABLED": "1", "GOMEMLIMIT": "2GiB"}},
		}, nil
	})
	register("arena-test", func(r *runner, v vars) ([]step, error) {
		return []step{
			goDirStep("arena-vet", "playground/arena", "vet", "./..."),
			goDirStep("arena-test", "playground/arena", "test", "./..."),
		}, nil
	})
	register("bookgen-build", func(r *runner, v vars) ([]step, error) {
		return []step{{name: "bookgen-build", path: "go", args: []string{"-C", "playground/bookgen", "build", "-o", "../../bin/bookgen" + r.goExe, "."}, env: tourneyEnv(r, v)}}, nil
	})
	register("bookgen", func(r *runner, v vars) ([]step, error) {
		build, err := registry["bookgen-build"](r, v)
		if err != nil {
			return nil, err
		}
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return append(build, step{
			name: "bookgen",
			path: "bin/bookgen" + r.goExe,
			args: runArgs,
			env:  map[string]string{"GOMEMLIMIT": v.get("BOOKGEN_MEMLIMIT", v.get("TOURNEY_MEMLIMIT", "12GiB"))},
		}), nil
	})
	register("booktree", func(r *runner, v vars) ([]step, error) {
		build, err := registry["bookgen-build"](r, v)
		if err != nil {
			return nil, err
		}
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return append(build, step{
			name: "booktree",
			path: "bin/bookgen" + r.goExe,
			args: append([]string{"-phase", "tree"}, runArgs...),
			env:  map[string]string{"GOMEMLIMIT": v.get("BOOKGEN_MEMLIMIT", v.get("TOURNEY_MEMLIMIT", "12GiB"))},
		}), nil
	})
	register("bookgen-test", func(r *runner, v vars) ([]step, error) {
		return []step{
			goDirStep("bookgen-vet", "playground/bookgen", "vet", "./..."),
			{name: "bookgen-test", path: "go", args: []string{"test", "-race", "-parallel", "2", "-timeout", goTestTimeout, "./..."}, dir: "playground/bookgen", env: map[string]string{"CGO_ENABLED": "1", "GOMEMLIMIT": "2GiB"}},
		}, nil
	})
	register("clocktune", func(r *runner, v vars) ([]step, error) {
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{
			goDirStep("clocktune-build", "playground/clocktune", "build", "-o", "../../bin/clocktune.exe", "."),
			{name: "clocktune", path: "bin/clocktune.exe", args: runArgs},
		}, nil
	})
	register("logstats", func(r *runner, v vars) ([]step, error) {
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{
			goDirStep("logstats-build", "playground/logstats", "build", "-o", "../../bin/logstats"+r.goExe, "."),
			{name: "logstats", path: "bin/logstats" + r.goExe, args: runArgs},
		}, nil
	})
	register("crossarm", func(r *runner, v vars) ([]step, error) {
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{
			goDirStep("crossarm-build", "playground/crossarm", "build", "-o", "../../bin/crossarm"+r.goExe, "."),
			{name: "crossarm", path: "bin/crossarm" + r.goExe, args: runArgs},
		}, nil
	})
	register("db-checkpoint", func(r *runner, v vars) ([]step, error) {
		return []step{
			goDirStep("walckpt-build", "playground/walckpt", "build", "-o", "../../bin/walckpt.exe", "."),
			{name: "db-checkpoint", path: "bin/walckpt.exe", args: []string{"db"}},
		}, nil
	})
	register("gpu-spike", func(r *runner, v vars) ([]step, error) {
		if err := os.MkdirAll("bin", 0o755); err != nil {
			return nil, err
		}
		return []step{
			{name: "nvcc", path: "nvcc", args: []string{"-shared", "-o", "bin/spike.dll", "playground/gpuoffload/spike.cu"}},
			goDirStep("gpuoffload-build", "playground/gpuoffload", "build", "-o", "../../bin/gpuoffload.exe", "."),
		}, nil
	})
}

func registerArenaTargets() {
	arenaBuild := func() step {
		return goDirStep("arena-build", "playground/arena", "build", "-o", "../../bin/arena.exe", ".")
	}
	register("arena", func(r *runner, v vars) ([]step, error) {
		runArgs, err := splitArgs(v)
		if err != nil {
			return nil, err
		}
		return []step{arenaBuild(), {name: "arena", path: "bin/arena.exe", args: runArgs, env: map[string]string{"CGO_ENABLED": "1"}}}, nil
	})
	register("hex-noise", func(r *runner, v vars) ([]step, error) {
		tc := v.get("TC", "1")
		games := v.get("GAMES", "20")
		return []step{arenaBuild(), {
			name: "hex-noise",
			path: "bin/arena.exe",
			args: []string{"-geometry", "hex", "-floor", "hex-noise-tc" + tc, "-floorSeat", "master-nobook", "-tc", tc, "-games", games, "-floorDirs", "5", "-floorBlockDirs", "5", "-floorMaxBlocks", "3", "-floorVerdict", "hex-noise"},
			env:  tourneyEnv(r, v),
		}}, nil
	})
	register("hex-passgates", func(r *runner, v vars) ([]step, error) {
		return []step{arenaBuild(), {
			name: "hex-passgates",
			path: "bin/arena.exe",
			args: []string{"-geometry", "hex", "-passgate", v.get("OPP", "hard"), "-tc", v.get("TC", "1"), "-games", v.get("GAMES", "20")},
			env:  tourneyEnv(r, v),
		}}, nil
	})
	register("hex-ladder", func(r *runner, v vars) ([]step, error) {
		return []step{arenaBuild(), {
			name: "hex-ladder",
			path: "bin/arena.exe",
			args: []string{"-geometry", "hex", "-ladder", "-tc", v.get("TC", "1"), "-games", v.get("GAMES", "40")},
			env:  tourneyEnv(r, v),
		}}, nil
	})
}

func registerTourneyTargets() {
	tourney := func(sub string, mem bool) specBuilder {
		return func(r *runner, v vars) ([]step, error) {
			extra, err := splitArgs(v)
			if err != nil {
				return nil, err
			}
			args := append([]string{"run", "./cmd/caro", "tourney", sub}, extra...)
			env := map[string]string{"CGO_ENABLED": "1"}
			if mem {
				env["GOMEMLIMIT"] = v.get("TOURNEY_MEMLIMIT", "12GiB")
			}
			return []step{{name: sub, path: "go", args: args, env: env}}, nil
		}
	}
	for _, t := range []struct {
		name string
		sub  string
		mem  bool
	}{
		{"tourney-smoke-32", "smoke32", true},
		{"tourney-smoke-10", "smoke10", true},
		{"tourney-smoke-105", "smoke105", true},
		{"tourney-ponderprobe", "ponderprobe", true},
		{"tourney-floor-hard21", "floor-hard21", true},
		{"tourney-floor-hard32", "floor-hard32", true},
		{"tourney-floor-gm21", "floor-gm21", true},
		{"tourney-floor-gm32", "floor-gm32", true},
		{"tourney-regate21-r1", "regate21-r1", true},
		{"tourney-regate21-r2", "regate21-r2", true},
		{"tourney-regate21-r3", "regate21-r3", true},
		{"tourney-regate32-r1", "regate32-r1", true},
		{"tourney-full", "full", true},
		{"tourney-resume", "resume", true},
		{"tourney-close", "close", false},
	} {
		register(t.name, tourney(t.sub, t.mem))
	}
}

func registerScriptTargets() {
	script := func(name, file string) specBuilder {
		return func(r *runner, v vars) ([]step, error) {
			extra, err := splitArgs(v)
			if err != nil {
				return nil, err
			}
			return []step{{name: name, path: "pwsh.exe", args: append([]string{"-NoProfile", "-File", file}, extra...)}}, nil
		}
	}
	register("bookval-short", script("bookval-short", "scripts/bookval-short.ps1"))
	register("bookval-confirm", script("bookval-confirm", "scripts/bookval-confirm.ps1"))
	register("bookval-cross", script("bookval-cross", "scripts/bookval-cross.ps1"))
	register("booktree-run", script("booktree-run", "scripts/booktree-run.ps1"))
	register("evaladopt-arms", script("evaladopt-arms", "scripts/evaladopt-arms.ps1"))
}

func registerMutateTargets() {
	register("mutate", func(r *runner, v vars) ([]step, error) {
		if err := os.MkdirAll(r.mutateEnv(v)["GOTMPDIR"], 0o755); err != nil {
			return nil, err
		}
		args := []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", v.get("PARALLEL", "1")}
		if v.has("CHALLENGE") {
			args = append(args, "-challenge")
		}
		return []step{{name: "mutate", path: "go", args: args, env: r.mutateEnv(v)}}, nil
	})
	register("mutate-smoke", func(r *runner, v vars) ([]step, error) {
		if err := os.MkdirAll(r.mutateEnv(v)["GOTMPDIR"], 0o755); err != nil {
			return nil, err
		}
		return []step{{name: "mutate-smoke", path: "go", args: []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-allow-scope", "internal/rules", "-parallel", v.get("PARALLEL", "2"), "-pkgs", "internal/rules"}, env: r.mutateEnv(v)}}, nil
	})
}

func init() {
	registerGoTargets()
	registerPlaygroundTargets()
	registerArenaTargets()
	registerTourneyTargets()
	registerScriptTargets()
	registerMutateTargets()
}

func buildSteps(target string, r *runner, v vars) ([]step, error) {
	b, ok := registry[target]
	if !ok {
		return nil, fmt.Errorf("run: unknown target %q", target)
	}
	return b(r, v)
}
