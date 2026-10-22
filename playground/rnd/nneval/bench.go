package main

import (
	"fmt"
	"time"
)

func runCpuBench() error {
	fmt.Printf("cpu matmul bench: nets %v and %v, target %d ms per cell, machine under fleet load (coarse)\n", netDims[netNarrow], netDims[netWide], cpuTargetMs)
	fmt.Println("net        threads   batch   us_per_eval   evals_per_s")
	for slot := range netCount {
		n := newNet(netDims[slot], weightSeedA, weightSeedB)
		for _, threads := range [...]int{1, cpuWorkers} {
			for _, batch := range cpuBatches {
				inputs := batchInputs(batch, netDims[slot][0], inputSeedA, inputSeedB)
				outputs := make([]float32, batch)
				n.forwardBatch(inputs, outputs, threads)
				iters := 0
				start := time.Now()
				for {
					n.forwardBatch(inputs, outputs, threads)
					iters++
					if iters >= cpuMinIters && time.Since(start).Milliseconds() >= cpuTargetMs {
						break
					}
					if iters >= 100000 {
						break
					}
				}
				secs := time.Since(start).Seconds()
				perEval := secs * 1e6 / float64(iters*batch)
				fmt.Printf("%-9s  %7d  %6d   %12.3f   %12.0f\n", netName(slot), threads, batch, perEval, 1e6/perEval)
			}
		}
	}
	return nil
}

func netName(slot int) string {
	if slot == netWide {
		return "wide"
	}
	return "narrow"
}
