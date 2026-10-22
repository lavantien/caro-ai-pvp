package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"strings"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
)

var demoLengths = []int{20, 30, 40, 50, 60}

func demoObjective(x []float64) float64 {
	g := config.PIDGains{Kp: x[0], Ki: x[1], Kd: x[2] * demoKdStep}
	worst := math.Inf(-1)
	for _, length := range demoLengths {
		for _, factors := range demoSpendModels(demoTcIdx, length) {
			worst = math.Max(worst, demoWorstDeviation(demoTcIdx, g, factors))
		}
	}
	return worst
}

func demoSpendModels(tcIdx, length int) [][]float64 {
	models := make([][]float64, 0, 3)
	exact := make([]float64, length)
	for i := range exact {
		exact[i] = 1
	}
	models = append(models, exact)
	rng := rand.New(rand.NewPCG(demoSeedHi, demoSeedLo+uint64(uint32(tcIdx))<<32|uint64(uint32(length))))
	ov := make([]float64, length)
	for i := range ov {
		ov[i] = demoOvLo + (demoOvHi-demoOvLo)*rng.Float64()
	}
	models = append(models, ov)
	ss := make([]float64, length)
	for i := range ss {
		ss[i] = 1
	}
	for i := range min(demoSlowStartMoves, length) {
		ss[i] = demoSlowStartFactor
	}
	models = append(models, ss)
	return models
}

func demoWorstDeviation(tcIdx int, g config.PIDGains, factors []float64) float64 {
	nsPerMs := float64(time.Millisecond)
	c := clock.NewGameClockWithGains(tcIdx, g)
	dev := func() float64 {
		return math.Abs(float64(c.Remaining())/nsPerMs - float64(c.Target())/nsPerMs)
	}
	maxDev := dev()
	for _, factor := range factors {
		budget := float64(c.Budget()) / nsPerMs
		c.Commit(time.Duration(budget * factor * nsPerMs))
		maxDev = math.Max(maxDev, dev())
	}
	return maxDev
}

type demoGridRef struct {
	calls int
	bestF float64
	bestX []float64
}

func demoGridReference() demoGridRef {
	ref := demoGridRef{bestF: math.Inf(1)}
	for i := 0; i <= 5; i++ {
		for j := 0; j <= 5; j++ {
			for k := 0; k < demoKdLevels; k++ {
				x := []float64{demoKpLo + 0.01*float64(i), demoKiLo + 0.005*float64(j), float64(k)}
				f := demoObjective(x)
				ref.calls++
				if f < ref.bestF {
					ref.bestF = f
					ref.bestX = x
				}
			}
		}
	}
	return ref
}

func demoStream(budget int, gridBest float64) (runResult, int) {
	dims := []dim{
		{"kp", demoKpLo, demoKpHi, false},
		{"ki", demoKiLo, demoKiHi, false},
		{"kd", 0, demoKdLevels - 1, true},
	}
	o := newOptimizer(dims, demoSeedLo)
	match := 0
	for i := 1; i <= budget; i++ {
		x := o.suggest()
		f := demoObjective(x)
		o.observe(x, f)
		if match == 0 && f <= gridBest {
			match = i
		}
	}
	return runResult{bestX: o.bestX, bestF: o.bestF}, match
}

func writeDemoArtifact(path string) error {
	ref := demoGridReference()
	best, match := demoStream(demoBudget, ref.bestF)
	var b strings.Builder
	fmt.Fprintf(&b, "# tune lane clock dry-run\n\n")
	fmt.Fprintf(&b, "objective: worst drain-trajectory deviation in ms at tc %d (%d+%d) over lengths %v, cost models exact, overshoot[%g,%g], slowstart %dx%g\n\n",
		demoTcIdx, config.TimeControls[demoTcIdx].InitialMin, config.TimeControls[demoTcIdx].IncrementSec, demoLengths, demoOvLo, demoOvHi, demoSlowStartMoves, demoSlowStartFactor)
	fmt.Fprintf(&b, "grid reference: %d cells (the clocktune 6x6x3 grid), best {Kp %.3f, Ki %.3f, Kd %.3f} worst deviation %.0f ms\n\n",
		ref.calls, ref.bestX[0], ref.bestX[1], ref.bestX[2]*demoKdStep, ref.bestF)
	fmt.Fprintf(&b, "bayes stream: budget %d, best {Kp %.4f, Ki %.4f, Kd %.3f} worst deviation %.0f ms\n\n",
		demoBudget, best.bestX[0], best.bestX[1], best.bestX[2]*demoKdStep, best.bestF)
	if match > 0 {
		fmt.Fprintf(&b, "trials to first value at or below the full-grid best: %d of %d\n\n", match, demoBudget)
	} else {
		fmt.Fprintf(&b, "trials to first value at or below the full-grid best: not reached within %d\n\n", demoBudget)
	}
	fmt.Fprintf(&b, "the sweep objective is monotone in Kp at increment controls (the clocktune structural finding), so this dry-run measures sample efficiency on a real repo surface, not a new optimum\n")
	return writeFile(path, b.String())
}
