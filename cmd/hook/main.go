package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/hook"
)

func hookMain(argv0 string, args []string, stdout, stderr io.Writer) int {
	name := strings.TrimSuffix(filepath.Base(argv0), ".exe")
	var hookArgs []string
	switch name {
	case "pre-commit", "commit-msg":
		hookArgs = args
	default:
		if len(args) < 1 {
			fmt.Fprintln(stderr, "usage: go run ./cmd/hook <pre-commit|commit-msg <file>|install>")
			return 2
		}
		name = args[0]
		hookArgs = args[1:]
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	switch name {
	case "pre-commit":
		if err := hook.Run(root, stdout, os.Environ()); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "commit-msg":
		if len(hookArgs) < 1 {
			fmt.Fprintln(stderr, "usage: go run ./cmd/hook commit-msg <message-file>")
			return 2
		}
		if err := hook.CommitMsg(hookArgs[0], stdout); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	case "install":
		if err := hook.Install(root); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
	default:
		fmt.Fprintf(stderr, "hook: unknown command %q\n", name)
		return 2
	}
	return 0
}

func main() {
	os.Exit(hookMain(os.Args[0], os.Args[1:], os.Stdout, os.Stderr))
}
