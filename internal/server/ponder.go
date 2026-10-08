package server

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
)

type GrantDepthRing struct {
	grants [config.PonderDepthHistory]int64
	depths [config.PonderDepthHistory]int
	head   int
	n      int
}

func (g *GrantDepthRing) Append(grantNs int64, depth int) {
	g.grants[g.head] = grantNs
	g.depths[g.head] = depth
	g.head = (g.head + 1) % config.PonderDepthHistory
	if g.n < config.PonderDepthHistory {
		g.n++
	}
}

func (g *GrantDepthRing) ReferenceDepth(grantNs int64) (int, bool) {
	ref := -1
	for i := range g.n {
		slot := (g.head - 1 - i + config.PonderDepthHistory) % config.PonderDepthHistory
		if g.grants[slot] >= grantNs/2 && g.grants[slot] <= grantNs*2 {
			if g.depths[slot] > ref {
				ref = g.depths[slot]
			}
		}
	}
	if ref < 0 {
		return 0, false
	}
	return ref, true
}

func AdoptPonder(stats engine.SearchStats, tag string, elapsedNs, budgetNs int64, ring GrantDepthRing, grantNs int64) bool {
	if tag != "" {
		return true
	}
	ref, ok := ring.ReferenceDepth(grantNs)
	if !ok || stats.Depth < ref {
		if elapsedNs < int64(config.PonderAdoptFraction*float64(budgetNs)) {
			return false
		}
	}
	n := min(config.PonderDepthHistory, stats.RootIters)
	if n < config.PonderStableIters {
		return false
	}
	for i := n - config.PonderStableIters; i < n; i++ {
		if stats.RootMoves[i] != stats.RootMoves[n-1] {
			return false
		}
	}
	drop := stats.RootScores[n-1] - stats.RootScores[n-config.PonderStableIters]
	if drop < 0 {
		drop = -drop
	}
	if drop > config.PonderScoreDropMargin {
		return false
	}
	return stats.RootMoves[n-1] == stats.RootMoves[n-2]
}
