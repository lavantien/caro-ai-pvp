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

const usage = "usage: mutate [-pkgs comma,separated,patterns] [-timeout 30s] [-allow file]"

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
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
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
	res, err := runMutation(ctx, stdout, workDir, patterns, execRunner{dir: workDir, timeout: *timeout}, allows)
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
	before, err := treeHash(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate:", err)
		os.Exit(1)
	}
	iso, err := isolateModule(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate:", err)
		os.Exit(1)
	}
	defer func() { _ = os.RemoveAll(iso) }()
	code := runCLI(os.Args[1:], os.Stdout, os.Stderr, iso)
	after, err := treeHash(".")
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "mutate: live tree check failed:", err)
		os.Exit(1)
	}
	for path, sum := range before {
		if after[path] != sum {
			_, _ = fmt.Fprintln(os.Stderr, "mutate: live tree modified during run:", path)
			os.Exit(1)
		}
	}
	os.Exit(code)
}
