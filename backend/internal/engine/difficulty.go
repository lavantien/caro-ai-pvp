package engine

import (
	"caro-ai-pvp/internal/domain"
	"runtime"
)

type DifficultyProfile struct {
	Name         string
	TimeFraction float64
	MaxDepth     int
	Goroutines   int
	UseVCF       bool
	VCFDepth     int
	Ponder       bool
	TTSizeMB     int
}

// Levels are strength-based first (depth caps, solver sight and parallel
// gating) and time-fraction scaled second, so L(k) is stronger than L(k-1) on
// any host. L3/L4 caps stay at or below 5: measured at bullet, ID depth past
// ~6 stops buying strength in self-play, so the ladder keeps those levels
// below the plateau and scales VCF sight instead.
func GetDifficultyProfile(level int) DifficultyProfile {
	n := runtime.GOMAXPROCS(0)
	l5Goroutines := pow2Floor((n - 2) / 2)

	switch level {
	case 1:
		return DifficultyProfile{"Novice", 0.05, 2, 1, false, 0, false, 64}
	case 2:
		return DifficultyProfile{"Beginner", 0.15, 4, 1, false, 0, false, 64}
	case 3:
		return DifficultyProfile{"Intermediate", 0.40, 4, 2, true, 2, false, 256}
	case 4:
		l4 := pow2Floor(l5Goroutines / 2)
		if l4 < 1 {
			l4 = 1
		}
		return DifficultyProfile{"Advanced", 0.70, 5, l4, true, 4, false, domain.DefaultTTSizeMB}
	default:
		if l5Goroutines < 1 {
			l5Goroutines = 1
		}
		return DifficultyProfile{"Grandmaster", 1.0, domain.AbsoluteMaxDepth, l5Goroutines, true, domain.VCFSearchDepth, true, domain.DefaultTTSizeMB}
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

// DefaultSessionTTSizeMB bounds the table used when no difficulty level is set.
const DefaultSessionTTSizeMB = 256
