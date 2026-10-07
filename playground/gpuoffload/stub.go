//go:build !windows

package main

import "fmt"

func main() {
	fmt.Println("gpuoffload: the spike needs the Windows loader and nvcc, see ANALYSIS.md for the recorded measurements")
}
