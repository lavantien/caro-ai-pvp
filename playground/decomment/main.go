package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: decomment file.go [file.go ...]")
		os.Exit(2)
	}
	for _, path := range os.Args[1:] {
		if err := strip(path); err != nil {
			fmt.Fprintln(os.Stderr, path+":", err)
			os.Exit(1)
		}
	}
}

func strip(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		kept = append(kept, line)
	}
	out := strings.Join(kept, "\n") + "\n"
	return os.WriteFile(path, []byte(out), 0o644)
}
