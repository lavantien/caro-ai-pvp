package server

import (
	"github.com/lavantien/caro-ai-pvp/internal/config"
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
