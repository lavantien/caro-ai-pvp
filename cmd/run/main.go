package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const goTestTimeout = "30m"

type step struct {
	name  string
	path  string
	args  []string
	dir   string
	env   map[string]string
	logTo string
}

type vars map[string]string

func (v vars) get(key, def string) string {
	if s, ok := v[key]; ok && s != "" {
		return s
	}
	return def
}

func (v vars) has(key string) bool {
	s, ok := v[key]
	return ok && s != ""
}

func parseVars(args []string) (vars, error) {
	v := vars{}
	for _, a := range args {
		k, val, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("run: expected KEY=VALUE argument, got %q", a)
		}
		v[k] = val
	}
	return v, nil
}

type runner struct {
	target string
	root   string
	goExe  string
	base   map[string]string
}

func newRunner(target string, v vars) (*runner, error) {
	root, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(root + string(os.PathSeparator) + "go.mod"); err != nil {
		return nil, fmt.Errorf("run: invoke from the repo root, %s has no go.mod", root)
	}
	goExe, err := goEnvGOEXE()
	if err != nil {
		return nil, err
	}
	base := map[string]string{
		"CGO_ENABLED": "1",
		"GOTMPDIR":    root + string(os.PathSeparator) + ".gotmp",
	}
	if t, ok := os.LookupEnv("GOTMPDIR"); ok && t != "" {
		base["GOTMPDIR"] = t
	}
	return &runner{target: target, root: root, goExe: goExe, base: base}, nil
}

func (r *runner) childEnv(stepEnv map[string]string) []string {
	merged := make(map[string]string, len(r.base)+len(stepEnv))
	for k, val := range r.base {
		merged[k] = val
	}
	for k, val := range stepEnv {
		merged[k] = val
	}
	drop := map[string]bool{}
	for k := range merged {
		drop[k] = true
	}
	out := make([]string, 0, len(merged)+8)
	for _, kv := range os.Environ() {
		k, _, ok := strings.Cut(kv, "=")
		if !ok || drop[k] {
			continue
		}
		out = append(out, kv)
	}
	for k, val := range merged {
		out = append(out, k+"="+val)
	}
	return out
}

func (r *runner) run(s step) error {
	code, err := r.runCode(s)
	if err != nil {
		return err
	}
	if code != 0 {
		return &exitCoder{code: code, err: fmt.Errorf("run: %s failed at %s: exit %d", r.target, s.name, code)}
	}
	return nil
}

func (r *runner) runCode(s step) (int, error) {
	if err := os.MkdirAll(r.base["GOTMPDIR"], 0o755); err != nil {
		return 0, fmt.Errorf("run: %s: %w", s.name, err)
	}
	started := time.Now()
	fmt.Printf("run: %s %s: %s %s", r.target, s.name, s.path, strings.Join(s.args, " "))
	if image, guarded := buildOutputImage(s, r.goExe); guarded {
		if err := guardRunningImage(s, image); err != nil {
			fmt.Printf(" [guard %s refused]\n", image)
			return 0, err
		}
		fmt.Printf(" [guard %s clear]\n", image)
	} else {
		fmt.Println()
	}
	code, err := execStep(s, r.childEnv(s.env))
	if err != nil {
		return 0, fmt.Errorf("run: %s failed at %s: %v", r.target, s.name, err)
	}
	fmt.Printf("run: %s %s: exit %d (%s)\n", r.target, s.name, code, time.Since(started).Round(time.Millisecond))
	return code, nil
}

func (r *runner) mutateEnv(v vars) map[string]string {
	tmp := v.get("MUTATE_TMP", filepath.Join(r.root, ".scratch", "tmp"))
	return map[string]string{"CGO_ENABLED": "1", "TMP": tmp, "TEMP": tmp, "GOTMPDIR": tmp}
}

func (r *runner) runAll(steps []step) error {
	for _, s := range steps {
		if err := r.run(s); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./cmd/run <target> [KEY=VALUE ...]")
		os.Exit(2)
	}
	target := os.Args[1]
	v, err := parseVars(os.Args[2:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	r, err := newRunner(target, v)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if flow, ok := flows[target]; ok {
		if err := flow(r, v); err != nil {
			fmt.Fprintln(os.Stderr, err)
			exitCode(err)
		}
		return
	}
	build, ok := registry[target]
	if !ok {
		fmt.Fprintf(os.Stderr, "run: unknown target %q\n", target)
		os.Exit(2)
	}
	steps, err := build(r, v)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if err := r.runAll(steps); err != nil {
		fmt.Fprintln(os.Stderr, err)
		exitCode(err)
	}
}
