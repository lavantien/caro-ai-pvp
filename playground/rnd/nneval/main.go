package main

import (
	"fmt"
	"os"
)

func main() {
	mode := "all"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	var err error
	switch mode {
	case "search":
		err = runSearchProbe()
	case "cpu":
		err = runCpuBench()
	case "gpu":
		err = runGpuBench()
	case "all":
		err = runAll()
	default:
		err = fmt.Errorf("unknown mode %q, want search|cpu|gpu|all", mode)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "nneval:", err)
		os.Exit(1)
	}
}

func runAll() error {
	for _, step := range []func() error{runSearchProbe, runCpuBench, runGpuBench} {
		if err := step(); err != nil {
			return err
		}
		fmt.Println()
	}
	return nil
}
