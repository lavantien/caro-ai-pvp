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

const usage = "usage: mutate [-pkgs comma,separated,patterns] [-timeout 30s] [-allow file] [-allow-scope path,prefixes] [-resume prior-run.log] [-parallel 1] [-challenge false]"

func splitPkgs(s string) []string {
	var out []string
	for p := range strings.SplitSeq(s, ",") {
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

func summaryLine(res result, err error) string {
	if err != nil || res.total == 0 {
		return ""
	}
	return fmt.Sprintf("mutate: %d/%d run, %d killed, %d survived, %d allowed\n", res.run, res.total, res.killed, res.survived, res.allowed)
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
	allowScope := fs.String("allow-scope", "", "comma-separated path prefixes; allowlist entries outside them are dropped, so a scoped gate skips the packages it did not run")
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
	if *allowScope != "" {
		var prefixes []string
		for p := range strings.SplitSeq(*allowScope, ",") {
			if p = strings.TrimSpace(p); p != "" {
				prefixes = append(prefixes, p)
			}
		}
		allows = scopeAllows(allows, prefixes)
		_, _ = fmt.Fprintf(stdout, "mutate: allowlist scoped to %d entries\n", len(allows))
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cacheDir, cerr := newRunCache()
	if cerr != nil {
		_, _ = fmt.Fprintln(stderr, "mutate:", cerr)
		return 2
	}
	defer func() { removeAllRetried(cacheDir) }()
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
					removeAllRetried(d)
				}
				return 2
			}
			dirs = append(dirs, extra)
			done = len(dirs) == *parallel
		}
		defer func() {
			for _, d := range dirs[1:] {
				removeAllRetried(d)
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
	if s := summaryLine(res, err); s != "" {
		_, _ = fmt.Fprint(stdout, s)
	}
	return exitCode(err, res)
}

func main() {
	code := 0
	defer func() { os.Exit(code) }()
	defer func() {
		if rerr := residueCheck(os.TempDir()); rerr != nil {
			_, _ = fmt.Fprintln(os.Stderr, "mutate:", rerr)
			code = 1
		}
	}()
	sweepStaleTemp(os.TempDir())
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
	defer func() { removeAllRetried(iso) }()
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
