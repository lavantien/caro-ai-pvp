//go:build !windows

package main

import "fmt"

func gpuPresent() bool { return false }

func runGpuBench() error {
	return fmt.Errorf("nneval gpu bench needs the Windows loader and nvcc")
}
