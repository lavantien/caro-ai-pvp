package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type flowFunc func(r *runner, v vars) error

var flows = map[string]flowFunc{}

func registerFlow(name string, f flowFunc) {
	flows[name] = f
}

func runRegistered(r *runner, v vars, target string) error {
	if f, ok := flows[target]; ok {
		return f(r, v)
	}
	steps, err := buildSteps(target, r, v)
	if err != nil {
		return err
	}
	return r.runAll(steps)
}

func registerDoctor() {
	registerFlow("doctor", func(r *runner, v vars) error {
		out, code, err := execCapture("go", []string{"version"}, r.childEnv(nil), r.root)
		if err != nil || code != 0 {
			return fmt.Errorf("doctor: go version failed: %v", err)
		}
		fmt.Printf("go: %s\n", out)
		cc, err := exec.LookPath("gcc")
		if err != nil {
			cc, err = exec.LookPath("clang")
		}
		if err != nil {
			return fmt.Errorf("doctor: no C toolchain (gcc or clang) in PATH")
		}
		ccOut, code, err := execCapture(cc, []string{"--version"}, r.childEnv(nil), r.root)
		if err != nil || code != 0 {
			return fmt.Errorf("doctor: %s --version failed: %v", cc, err)
		}
		ccLine, _, _ := strings.Cut(ccOut, "\n")
		fmt.Printf("cc: %s\n", ccLine)
		verOut, code, err := execCapture("go", []string{"env", "GOVERSION"}, r.childEnv(nil), r.root)
		if err != nil || code != 0 {
			return fmt.Errorf("doctor: go env GOVERSION failed: %v", err)
		}
		score, ok := goVersionScore(strings.TrimPrefix(verOut, "go"))
		if !ok {
			return fmt.Errorf("doctor: unparsable go version %q", verOut)
		}
		if score < 1027001 {
			return fmt.Errorf("doctor: need go >= 1.27.1, got %s", strings.TrimPrefix(verOut, "go"))
		}
		cgo, code, err := execCapture("go", []string{"env", "CGO_ENABLED"}, r.childEnv(map[string]string{"CGO_ENABLED": "1"}), r.root)
		if err != nil || code != 0 {
			return fmt.Errorf("doctor: go env CGO_ENABLED failed: %v", err)
		}
		if cgo != "1" {
			return fmt.Errorf("doctor: CGO_ENABLED=1 not accepted")
		}
		fmt.Println("doctor: ok")
		return nil
	})
}

func goVersionScore(v string) (int, bool) {
	major, rest, ok := strings.Cut(v, ".")
	if !ok {
		return 0, false
	}
	minor := rest
	patch := "0"
	if r2, p2, ok := strings.Cut(rest, "."); ok {
		minor, patch = r2, p2
	}
	m, err1 := strconv.Atoi(major)
	n, err2 := strconv.Atoi(minor)
	p, err3 := strconv.Atoi(patch)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return m*1000000 + n*1000 + p, true
}

func gofmtPath() string {
	root, code, err := execCapture("go", []string{"env", "GOROOT"}, nil, "")
	if err != nil || code != 0 {
		return "gofmt"
	}
	name := "gofmt"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	c := filepath.Join(root, "bin", name)
	if _, err := os.Stat(c); err == nil {
		return c
	}
	return "gofmt"
}

func trackedGoFiles() ([]string, error) {
	out, code, err := execCapture("git", []string{"ls-files", "*.go"}, nil, "")
	if err != nil || code != 0 {
		return nil, fmt.Errorf("git ls-files: %v", err)
	}
	if out == "" {
		return nil, nil
	}
	return strings.Fields(out), nil
}

func registerFmt() {
	registerFlow("fmt", func(r *runner, v vars) error {
		files, err := trackedGoFiles()
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		return r.run(step{name: "fmt", path: gofmtPath(), args: append([]string{"-w"}, files...)})
	})
	registerFlow("fmt-check", func(r *runner, v vars) error {
		files, err := trackedGoFiles()
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return nil
		}
		out, code, err := execCapture(gofmtPath(), append([]string{"-l"}, files...), nil, "")
		if err != nil || code != 0 {
			return fmt.Errorf("fmt-check: gofmt failed: %v", err)
		}
		if out != "" {
			fmt.Println(out)
			return fmt.Errorf("fmt-check: run go run ./cmd/run fmt")
		}
		return nil
	})
}

var fuzzGroups = []struct {
	dir      string
	pkgArg   string
	inCDir   bool
	targets  []string
	fuzztime string
}{
	{"internal/rules", "./internal/rules", false, []string{"FuzzRulesDifferential"}, "60s"},
	{"internal/server", "./internal/server", false, []string{"FuzzRatingLaw", "FuzzSeriesDrive", "FuzzMovesBlob", "FuzzGrantDepthRing", "FuzzAdoptPonder"}, "30s"},
	{"internal/tourney", "./internal/tourney", false, []string{"FuzzPairings", "FuzzFoldZeroSum"}, "30s"},
	{"internal/book", "./internal/book", false, []string{"FuzzBookLoad"}, "30s"},
	{"playground/bookgen", ".", true, []string{"FuzzLoadCorpus", "FuzzReadOverlay"}, "30s"},
}

func registerFuzz() {
	registerFlow("fuzz", func(r *runner, v vars) error {
		for _, g := range fuzzGroups {
			if _, err := os.Stat(g.dir); err != nil {
				continue
			}
			for _, t := range g.targets {
				args := []string{"test", "-run=^$", "-fuzz=" + t, "-fuzztime=" + g.fuzztime, g.pkgArg}
				s := goStep("fuzz "+t, args...)
				if g.inCDir {
					s = step{name: "fuzz " + t, path: "go", args: append([]string{"-C", g.dir}, args...), dir: g.dir, env: map[string]string{"CGO_ENABLED": "1"}}
				}
				if err := r.run(s); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

var mutateVerdictRe = regexp.MustCompile(`(?m)^mutate: [0-9][0-9]*/[0-9]* run`)

func hasMutateVerdict(logPath string) bool {
	b, err := os.ReadFile(logPath)
	if err != nil {
		return false
	}
	return mutateVerdictRe.Match(b)
}

func tailLines(path string, n int) []string {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(b), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}

func mutateStepArgs(v vars, parallelDefault string, extra []string) []string {
	args := []string{"run", "./cmd/mutate", "-allow", ".mutate-allow", "-parallel", v.get("PARALLEL", parallelDefault)}
	if v.has("CHALLENGE") {
		args = append(args, "-challenge")
	}
	args = append(args, splitArgs(v)...)
	return append(args, extra...)
}

func registerMutateFlows() {
	registerFlow("mutate-resume", func(r *runner, v vars) error {
		log := v.get("LOG", "")
		if log == "" {
			return fmt.Errorf("mutate-resume: LOG=<prior-run.log> required")
		}
		for attempt := 1; attempt <= 5; attempt++ {
			s := step{name: "mutate-resume", path: "go", args: mutateStepArgs(v, "1", []string{"-resume", log}), env: r.mutateEnv(v)}
			if err := r.run(s); err == nil {
				return nil
			}
			if hasMutateVerdict(log) {
				for _, l := range tailLines(log, 2) {
					fmt.Println(l)
				}
				return fmt.Errorf("mutate-resume: verdict failure in %s", log)
			}
			fmt.Printf("mutate-resume: attempt %d crashed host-side, resuming from %s\n", attempt, log)
		}
		return fmt.Errorf("mutate-resume: no definitive verdict after 5 attempts")
	})
	registerFlow("mutate-full", func(r *runner, v vars) error {
		if err := os.MkdirAll("logs/archive", 0o755); err != nil {
			return err
		}
		log := filepath.Join("logs", "archive", "mutate-"+v.get("LABEL", "run")+".log")
		if err := os.WriteFile(log, nil, 0o644); err != nil {
			return err
		}
		for attempt := 1; attempt <= 5; attempt++ {
			s := step{name: "mutate-full", path: "go", args: mutateStepArgs(v, "4", []string{"-resume", log}), env: r.mutateEnv(v), logTo: log}
			if err := r.run(s); err == nil {
				return nil
			}
			if hasMutateVerdict(log) {
				for _, l := range tailLines(log, 2) {
					fmt.Println(l)
				}
				return fmt.Errorf("mutate-full: verdict failure in %s", log)
			}
			fmt.Printf("mutate-full: attempt %d crashed host-side, resuming from %s\n", attempt, log)
		}
		for _, l := range tailLines(log, 3) {
			fmt.Println(l)
		}
		return fmt.Errorf("mutate-full: no definitive verdict after 5 attempts")
	})
}

var regateArms = []string{"regate21-r1", "regate21-r2", "regate21-r3", "regate32-r1"}

func newestRunDir(arm string) string {
	matches, err := filepath.Glob(filepath.Join("logs", "tourny", "*-"+arm))
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Slice(matches, func(i, j int) bool {
		fi, erri := os.Stat(matches[i])
		fj, errj := os.Stat(matches[j])
		if erri != nil || errj != nil {
			return false
		}
		return fi.ModTime().After(fj.ModTime())
	})
	return matches[0]
}

func registerChain() {
	registerFlow("tourney-regate-chain", func(r *runner, v vars) error {
		for _, arm := range regateArms {
			dir := ""
			done := false
			for attempt := 1; attempt <= 5; attempt++ {
				steps, err := buildSteps("tourney-"+arm, r, v)
				if err != nil {
					return err
				}
				if code, err := r.runCode(steps[len(steps)-1]); err != nil {
					return err
				} else if code != 0 {
					fmt.Printf("chain: %s exited nonzero on attempt %d\n", arm, attempt)
				}
				dir = newestRunDir(arm)
				if dir != "" {
					if _, err := os.Stat(filepath.Join(dir, "summary.txt")); err == nil {
						done = true
						break
					}
				}
				fmt.Printf("chain: %s has no summary.txt after attempt %d, relaunching to resume\n", arm, attempt)
			}
			if !done {
				return fmt.Errorf("chain: %s never reached summary.txt in 5 attempts", arm)
			}
		}
		return nil
	})
}

const (
	mermaidVersion   = "12.0.0"
	puppeteerVersion = "25.12.0"
	diagramsDir      = "docs/diagrams"
	diagramBg        = "rgb(13,17,23)"
	diagramScale     = "3"
)

func diagramNeedsRender(png, mmd string) bool {
	st, err := os.Stat(png)
	if err != nil {
		return true
	}
	for _, dep := range []string{mmd, filepath.Join(diagramsDir, "mermaid-config.json"), filepath.Join(diagramsDir, "puppeteer.json")} {
		d, err := os.Stat(dep)
		if err != nil || d.ModTime().After(st.ModTime()) {
			return true
		}
	}
	return false
}

func registerDiagrams() {
	registerFlow("diagrams", func(r *runner, v vars) error {
		if err := r.run(step{name: "diagrams-browser", path: "npx", args: []string{"-y", "puppeteer@" + puppeteerVersion, "browsers", "install", "chrome-headless-shell"}}); err != nil {
			return err
		}
		mmds, err := filepath.Glob(filepath.Join(diagramsDir, "*.mmd"))
		if err != nil {
			return err
		}
		for _, mmd := range mmds {
			png := strings.TrimSuffix(mmd, ".mmd") + ".png"
			if !diagramNeedsRender(png, mmd) {
				continue
			}
			err := r.run(step{name: "diagram " + filepath.Base(mmd), path: "npx", args: []string{
				"-y",
				"-p", "@mermaid-js/mermaid-cli@" + mermaidVersion,
				"-p", "puppeteer@" + puppeteerVersion,
				"mmdc",
				"-i", mmd,
				"-o", png,
				"-e", "png",
				"-b", diagramBg,
				"-s", diagramScale,
				"-c", filepath.Join(diagramsDir, "mermaid-config.json"),
				"-p", filepath.Join(diagramsDir, "puppeteer.json"),
			}})
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func registerCi() {
	registerFlow("ci", func(r *runner, v vars) error {
		for _, t := range []string{"playground-modules", "contrast", "fmt-check", "lint", "vet", "build", "bookgen-test", "evalfit-test", "cover"} {
			if err := runRegistered(r, v, t); err != nil {
				return err
			}
		}
		fmt.Println("ci: all green")
		return nil
	})
}

func init() {
	registerDoctor()
	registerFmt()
	registerFuzz()
	registerMutateFlows()
	registerChain()
	registerDiagrams()
	registerCi()
}
