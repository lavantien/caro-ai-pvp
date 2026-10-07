// gpuoffload drives the nvcc-built spike dll and prints the measured
// launch overhead and PCIe bandwidth the CUDA-offload assessment cites.
// Loaded at runtime through the Windows loader, never linked, so the
// module builds with no CUDA toolchain of its own.
//
//go:build windows

package main

import (
	"fmt"
	"log"
	"syscall"
	"time"
)

var (
	spike          = syscall.NewLazyDLL(`bin\spike.dll`)
	procNoop       = spike.NewProc("noop_launch")
	procBandwidth  = spike.NewProc("copy_bandwidth_dgbps")
	launchIters    = 200_000
	bandwidthBytes = 256 << 20
	bandwidthIters = 20
)

func main() {
	start := time.Now()
	rc, _, _ := procNoop.Call(uintptr(launchIters))
	if rc != 0 {
		log.Fatalf("gpuoffload: noop_launch cuda error %d", int32(rc))
	}
	perLaunch := time.Since(start) / time.Duration(launchIters)

	dgbps, _, _ := procBandwidth.Call(uintptr(bandwidthBytes), uintptr(bandwidthIters))
	fmt.Printf("kernel launch: %d launches, %v per launch\n", launchIters, perLaunch)
	fmt.Printf("pcie copy: %d MiB x %d round trips, %.1f GB/s combined\n",
		bandwidthBytes>>20, bandwidthIters, float64(int32(dgbps))/10.0)
}
