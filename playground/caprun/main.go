package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	capMiB := flag.Int64("cap", 2048, "job commit cap in MiB")
	killAfter := flag.Duration("kill-after", 5*time.Minute, "outer kill window")
	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "usage: caprun [-cap MiB] [-kill-after d] command...")
		os.Exit(2)
	}
	if !capSupported() {
		fmt.Fprintln(os.Stderr, "caprun: requires windows job objects")
		os.Exit(2)
	}
	argv := flag.Args()
	ctx, cancel := context.WithTimeout(context.Background(), *killAfter)
	defer cancel()
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	start := time.Now()
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "caprun: start: %v\n", err)
		os.Exit(2)
	}
	if err := applyCap(cmd, *capMiB); err != nil {
		fmt.Fprintf(os.Stderr, "caprun: %v\n", err)
		os.Exit(2)
	}
	werr := cmd.Wait()
	elapsed := time.Since(start)
	fmt.Printf("caprun: exit=%v elapsed=%s ctx_err=%v output_bytes=%d\n", werr, elapsed.Round(time.Second), ctx.Err(), out.Len())
	if out.Len() > 0 {
		fmt.Printf("caprun: output tail:\n%s\n", tail(out.String(), 3000))
	}
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n:]
}
