package engine

import "runtime"

type DifficultyProfile struct {
	Name         string
	TimeFraction float64
	Goroutines   int
	UseVCF       bool
	Ponder       bool
}

func GetDifficultyProfile(level int) DifficultyProfile {
	n := runtime.GOMAXPROCS(0)
	l5Goroutines := pow2Floor((n - 2) / 2)

	switch level {
	case 1:
		return DifficultyProfile{"Novice", 0.05, 1, false, false}
	case 2:
		return DifficultyProfile{"Beginner", 0.15, 1, false, false}
	case 3:
		return DifficultyProfile{"Intermediate", 0.40, 2, true, false}
	case 4:
		l4 := pow2Floor(l5Goroutines / 2)
		if l4 < 1 {
			l4 = 1
		}
		return DifficultyProfile{"Advanced", 0.70, l4, true, false}
	default:
		if l5Goroutines < 1 {
			l5Goroutines = 1
		}
		return DifficultyProfile{"Grandmaster", 1.0, l5Goroutines, true, true}
	}
}

func pow2Floor(n int) int {
	if n <= 0 {
		return 1
	}
	p := 1
	for p*2 <= n {
		p *= 2
	}
	return p
}

func GetEngineThreadsForLoad(activeGames int) int {
	if activeGames <= 1 {
		return runtime.GOMAXPROCS(0)
	}
	return runtime.GOMAXPROCS(0) / activeGames
}
