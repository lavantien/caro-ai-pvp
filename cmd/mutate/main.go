package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const usage = "usage: mutate [-pkgs comma,separated,patterns] [-timeout 30s] [-allow file] [-resume prior-run.log] [-parallel 1] [-challenge false]"

func splitPkgs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			if !strings.HasPrefix(p, ".") {
				p = "./" + p
			}
			out = append(out, p)
		}
	}
	return out
}

func exitCode(err error, res result) int {
	switch {
	case err != nil:
		return 1
	case res.total == 0:
		return 1
	case res.survived > 0 || res.run < res.total:
		return 1
	}
	return 0
}

func runCLI(args []string, stdout, stderr io.Writer, workDir string) int {
	fs := flag.NewFlagSet("mutate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	pkgs := fs.String("pkgs", config.MutatePackages, "comma-separated package patterns to mutate")
	timeout := fs.Duration("timeout", time.Duration(config.MutateTimeoutMs)*time.Millisecond, "per-mutant test timeout")
	allowPath := fs.String("allow", "", "equivalence allowlist, one 'file:line:col descriptor # proof' entry per line; empty or missing file means no allowances")
	resumePath := fs.String("resume", "", "prior run log whose KILLED verdicts are replayed instead of re-run, for continuing a gate after a host failure")
	parallel := fs.Int("parallel", 1, "worker count: mutants run concurrently in isolated module copies, survivors re-verified serially")
	challenge := fs.Bool("challenge", false, "run the suite under allowlisted mutants too; a serially confirmed kill demotes the entry and fails the gate")
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 || *parallel < 1 || *parallel > config.MutateMaxParallel {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	resumeKilled := map[string]bool{}
	if *resumePath != "" {
		rf, rerr := os.Open(*resumePath)
		if rerr != nil {
			_, _ = fmt.Fprintln(stderr, "mutate:", rerr)
			return 2
		}
		resumeKilled = loadResumeLog(rf)
		_ = rf.Close()
		_, _ = fmt.Fprintf(stdout, "mutate: resuming, %d prior kills replayed\n", len(resumeKilled))
	}
	patterns := splitPkgs(*pkgs)
	if len(patterns) == 0 || *timeout < time.Duration(config.MutateMinTimeoutMs)*time.Millisecond {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	allows, aerr := loadAllowlist(*allowPath)
	if aerr != nil {
		_, _ = fmt.Fprintln(stderr, "mutate:", aerr)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cacheDir, cerr := newRunCache()
	if cerr != nil {
		_, _ = fmt.Fprintln(stderr, "mutate:", cerr)
		return 2
	}
	defer func() { _ = os.RemoveAll(cacheDir) }()
	var res result
	var err error
	if *parallel > 1 {
		dirs := []string{workDir}
		done := false
		for !done {
			extra, ierr := isolateModule(workDir)
			if ierr != nil {
				_, _ = fmt.Fprintln(stderr, "mutate:", ierr)
				for _, d := range dirs[1:] {
					_ = os.RemoveAll(d)
				}
				return 2
			}
			dirs = append(dirs, extra)
			done = len(dirs) == *parallel
		}
		defer func() {
			for _, d := range dirs[1:] {
				_ = os.RemoveAll(d)
			}
		}()
		res, err = runMutationParallel(ctx, stdout, dirs, patterns, cacheDir, func(dir string) runner {
			return execRunner{dir: dir, cacheDir: cacheDir, timeout: *timeout}
		}, allows, resumeKilled, *challenge)
	} else {
		res, err = runMutation(ctx, stdout, workDir, patterns, cacheDir, execRunner{dir: workDir, cacheDir: cacheDir, timeout: *timeout}, allows, resumeKilled, *challenge)
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "mutate:", err)
	}
	if err == nil && res.total == 0 {
		_, _ = fmt.Fprintln(stdout, "mutate: no mutants found in", strings.Join(patterns, ","))
	}
	if res.total > 0 {
		_, _ = fmt.Fprintf(stdout, "mutate: %d/%d run, %d killed, %d survived, %d allowed\n", res.run, res.total, res.killed, res.survived, res.allowed)
	}
	return exitCode(err, res)
}

func main() {
	code := 0
	// The exit defer runs last, after the cleanup defers registered below
	// it, so isolated copies are removed even on error paths.
	defer func() { os.Exit(code) }()
	sweepStaleIsolates()
	before, err := treeHash(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate:", err)
		code = 1
		return
	}
	iso, err := isolateModule(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate:", err)
		code = 1
		return
	}
	defer func() { _ = os.RemoveAll(iso) }()
	code = runCLI(os.Args[1:], os.Stdout, os.Stderr, iso)
	after, err := treeHash(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate: live tree check failed:", err)
		code = 1
		return
	}
	for path, sum := range before {
		if after[path] != sum {
			_, _ = fmt.Fprintln(os.Stderr, "mutate: live tree modified during run:", path)
			code = 1
			return
		}
	}
}
