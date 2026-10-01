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

const usage = "usage: mutate [-pkgs comma,separated,patterns] [-timeout 30s]"

func splitPkgs(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
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
	if err := fs.Parse(args); err != nil || fs.NArg() > 0 {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	patterns := splitPkgs(*pkgs)
	if len(patterns) == 0 || *timeout < time.Duration(config.MutateMinTimeoutMs)*time.Millisecond {
		_, _ = fmt.Fprintln(stderr, usage)
		return 2
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	res, err := runMutation(ctx, stdout, workDir, patterns, execRunner{dir: workDir, timeout: *timeout})
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "mutate:", err)
	}
	if err == nil && res.total == 0 {
		_, _ = fmt.Fprintln(stdout, "mutate: no mutants found in", strings.Join(patterns, ","))
	}
	if res.total > 0 {
		_, _ = fmt.Fprintf(stdout, "mutate: %d/%d run, %d killed, %d survived\n", res.run, res.total, res.killed, res.survived)
	}
	return exitCode(err, res)
}

func main() {
	os.Exit(runCLI(os.Args[1:], os.Stdout, os.Stderr, "."))
}
