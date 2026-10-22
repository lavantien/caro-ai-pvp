//go:build windows

package main

import (
	"fmt"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	cudaDLL     = syscall.NewLazyDLL(gpuDLL)
	procNoop    = cudaDLL.NewProc("noop_launch")
	procSetup   = cudaDLL.NewProc("mlp_setup")
	procCheck   = cudaDLL.NewProc("mlp_check")
	procUsPerIt = cudaDLL.NewProc("mlp_us_per_iter")
	procH2D     = cudaDLL.NewProc("copy_h2d_dgbps")
)

func gpuPresent() bool {
	_, err := os.Stat(gpuDLL)
	if err != nil {
		return false
	}
	return cudaDLL.Load() == nil
}

func gpuSetupNet(slot int, n *net) error {
	if len(n.layers) != 3 {
		return fmt.Errorf("gpu net needs exactly 3 layers, got %d", len(n.layers))
	}
	rc, _, _ := procSetup.Call(
		uintptr(slot),
		uintptr(n.layers[0].in), uintptr(n.layers[0].out), uintptr(n.layers[1].out),
		uintptr(unsafe.Pointer(&n.layers[0].w[0])), uintptr(unsafe.Pointer(&n.layers[0].b[0])),
		uintptr(unsafe.Pointer(&n.layers[1].w[0])), uintptr(unsafe.Pointer(&n.layers[1].b[0])),
		uintptr(unsafe.Pointer(&n.layers[2].w[0])), uintptr(unsafe.Pointer(&n.layers[2].b[0])),
	)
	if int32(rc) != 0 {
		return fmt.Errorf("mlp_setup slot %d cuda code %d", slot, int32(rc))
	}
	return nil
}

func gpuCheckSample(slot int, input []float32) (float32, error) {
	var out float32
	rc, _, _ := procCheck.Call(uintptr(slot), uintptr(unsafe.Pointer(&input[0])), uintptr(unsafe.Pointer(&out)))
	if int32(rc) != 0 {
		return 0, fmt.Errorf("mlp_check slot %d cuda code %d", slot, int32(rc))
	}
	return out, nil
}

func gpuUsPerIter(slot, batch, iters int) (float64, error) {
	rc, _, _ := procUsPerIt.Call(uintptr(slot), uintptr(batch), uintptr(iters))
	if int32(rc) < 0 {
		return 0, fmt.Errorf("mlp_us_per_iter slot %d batch %d code %d", slot, batch, int32(rc))
	}
	return float64(int32(rc)), nil
}

func runGpuBench() error {
	if !gpuPresent() {
		return fmt.Errorf("gpu dll absent at %s, run build-gpu.ps1 first", gpuDLL)
	}
	start := time.Now()
	rc, _, _ := procNoop.Call(uintptr(launchIters))
	if int32(rc) != 0 {
		return fmt.Errorf("noop_launch cuda code %d", int32(rc))
	}
	perLaunch := time.Since(start) / time.Duration(launchIters)
	fmt.Printf("kernel launch: %d launches, %v per launch\n", launchIters, perLaunch)
	for _, bytes := range []int{256 << 10, 1 << 20, 8 << 20} {
		dgbps, _, _ := procH2D.Call(uintptr(bytes), uintptr(gpuRoundIters))
		fmt.Printf("pcie h2d: %d KiB blocks, %.1f GB/s one-way\n", bytes>>10, float64(int32(dgbps))/10.0)
	}
	fmt.Println("gpu mlp bench: weights resident, per iter = input upload + kernel + output readback + sync")
	fmt.Println("net        batch   us_per_iter   us_per_eval   evals_per_s")
	for slot := range netCount {
		n := newNet(netDims[slot], weightSeedA, weightSeedB)
		if err := gpuSetupNet(slot, n); err != nil {
			return err
		}
		inputs := batchInputs(1, netDims[slot][0], inputSeedA, inputSeedB)
		ref := make([]float32, 1)
		n.forwardSample(inputs[0], n.makeScratch(), &ref[0])
		got, err := gpuCheckSample(slot, inputs[0])
		if err != nil {
			return err
		}
		delta := got - ref[0]
		if delta < 0 {
			delta = -delta
		}
		if delta > 1e-3 {
			return fmt.Errorf("gpu/cpu mismatch slot %d: gpu %f cpu %f", slot, got, ref[0])
		}
		for _, batch := range gpuBatches {
			us, err := gpuUsPerIter(slot, batch, gpuRoundIters)
			if err != nil {
				return err
			}
			fmt.Printf("%-9s  %6d   %12.1f   %12.4f   %12.0f\n", netName(slot), batch, us, us/float64(batch), 1e6*float64(batch)/us)
		}
	}
	return nil
}
