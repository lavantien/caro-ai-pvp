package main

const gameDim = 6

func measuredP(margin float64) float64 {
	list := seedList()
	sum := 0.0
	for _, sd := range list {
		g := newGameObj(gameDim, margin, sd)
		sum += clampP(0.5 + (1-g.skill(g.Reference()))*margin)
	}
	return sum / float64(len(list))
}

func solveMargin(target float64) float64 {
	lo, hi := 0.0, 4.0
	for i := 0; i < 200; i++ {
		mid := (lo + hi) / 2
		if measuredP(mid) < target {
			lo = mid
		} else {
			hi = mid
		}
	}
	return (lo + hi) / 2
}
