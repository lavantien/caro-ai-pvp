package main

const (
	weightSeedA = 1
	weightSeedB = 3
	inputSeedA  = 5
	inputSeedB  = 7

	netNarrow = 0
	netWide   = 1
	netCount  = 2

	cpuWorkers    = 8
	cpuTargetMs   = 200
	cpuMinIters   = 3
	launchIters   = 200_000
	gpuRoundIters = 50

	searchTTMiB = 64
	searchReps  = 5

	gpuDLL = `playground\rnd\nneval\bin\nnevalcuda.dll`
)

var (
	netDims = [netCount][]int{
		{256, 64, 32, 1},
		{256, 128, 64, 1},
	}

	cpuBatches   = [...]int{1, 8, 32, 128, 512, 2048, 4096}
	gpuBatches   = [...]int{64, 256, 1024, 4096, 16384}
	searchDepths = [...]int{8, 9}
	midgameMoves = [...]string{"H8", "H9", "I8", "I9", "C3", "C4", "D3", "M12", "M13", "N12"}
)
