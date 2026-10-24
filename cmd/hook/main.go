package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lavantien/caro-ai-pvp/internal/hook"
)

func main() {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	var hookArgs []string
	switch name {
	case "pre-commit", "commit-msg":
		hookArgs = os.Args[1:]
	default:
		if len(os.Args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: go run ./cmd/hook <pre-commit|commit-msg <file>|install>")
			os.Exit(2)
		}
		name = os.Args[1]
		hookArgs = os.Args[2:]
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	switch name {
	case "pre-commit":
		if err := hook.Run(root, os.Stdout, os.Environ()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "commit-msg":
		if len(hookArgs) < 1 {
			fmt.Fprintln(os.Stderr, "usage: go run ./cmd/hook commit-msg <message-file>")
			os.Exit(2)
		}
		if err := hook.CommitMsg(hookArgs[0], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	case "install":
		if err := hook.Install(root); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "hook: unknown command %q\n", name)
		os.Exit(2)
	}
}
