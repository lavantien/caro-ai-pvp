package main

import (
	"math"
	"math/rand"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

const (
	fitRestartIncrementLo = 30.0
	fitRestartIncrementHi = 7000.0
	fitRestartScaleLo     = 300.0
	fitRestartScaleHi     = 15000.0
	gdLRGrowth            = 1.05
	gdLRShrink            = 0.5
	gdLRMax               = 1e4
	gdLRMin               = 1e-10
	gdGradEps             = 1e-14
	gdMomentum            = 0.9
	gdStallWindow         = 500
	gdStallEps            = 1e-12
	pClamp                = 1e-15
	intProjectPad         = 5
)

type gdOptions struct {
	Restarts   int
	MaxSteps   int
	TraceEvery int
	LR0        float64
}

func defaultGDOptions() gdOptions {
	return gdOptions{Restarts: 10, MaxSteps: 50000, TraceEvery: 2000, LR0: 10}
}

type fitCell struct {
	F    [5]float64
	N    float64
	YSum float64
}

type fitData struct {
	Cells []fitCell
	Total float64
}

type fitResult struct {
	W        [5]float64
	S        float64
	Loss     float64
	Steps    int
	Restart  int
	Restarts int
}

func aggregateFit(samples []sample) fitData {
	index := make(map[[5]int32]int)
	d := fitData{}
	for _, s := range samples {
		i, ok := index[s.F]
		if !ok {
			i = len(d.Cells)
			index[s.F] = i
			var f [5]float64
			for c, v := range s.F {
				f[c] = float64(v)
			}
			d.Cells = append(d.Cells, fitCell{F: f})
		}
		d.Cells[i].N++
		d.Cells[i].YSum += s.Y
		d.Total++
	}
	return d
}

func clampP(p float64) float64 {
	if p < pClamp {
		return pClamp
	}
	if p > 1-pClamp {
		return 1 - pClamp
	}
	return p
}

func sigmoid(x float64) float64 {
	if x >= 0 {
		return 1 / (1 + math.Exp(-x))
	}
	e := math.Exp(x)
	return e / (1 + e)
}

func (d fitData) lossGrad(w [5]float64, s float64, wantGrad bool) (float64, [5]float64, float64) {
	var dw [5]float64
	if d.Total == 0 || s <= 0 || math.IsNaN(s) {
		return math.Inf(1), dw, 0
	}
	inv := 1 / d.Total
	loss := 0.0
	dLogS := 0.0
	for _, c := range d.Cells {
		dot := float64(config.EvalTempo)
		for j := range c.F {
			dot += c.F[j] * w[j]
		}
		t := dot / s
		p := clampP(sigmoid(t))
		loss -= c.YSum * math.Log(p)
		loss -= (c.N - c.YSum) * math.Log1p(-p)
		if wantGrad {
			k := (c.N*p - c.YSum) * inv
			for j := range c.F {
				dw[j] += k * c.F[j]
			}
			dLogS -= k * t
		}
	}
	for j := range dw {
		dw[j] /= s
	}
	return loss * inv, dw, dLogS
}

func (d fitData) lossAt(w [5]float64, s float64) float64 {
	l, _, _ := d.lossGrad(w, s, false)
	return l
}

func evalLoss(samples []sample, w [5]float64, s float64) float64 {
	return aggregateFit(samples).lossAt(w, s)
}

func projectMonotoneInt(w [5]float64) [5]int64 {
	lo, hi := w[0], w[0]
	for _, x := range w {
		lo = math.Min(lo, x)
		hi = math.Max(hi, x)
	}
	first := int(math.Floor(lo)) - intProjectPad
	levels := int(math.Ceil(hi)) + intProjectPad - first + 1
	cost := make([]float64, levels)
	for v := range cost {
		cost[v] = math.Abs(w[0] - float64(first+v))
	}
	parents := make([][]int, len(w)-1)
	for i := 1; i < len(w); i++ {
		parent := make([]int, levels)
		next := make([]float64, levels)
		next[0] = math.Inf(1)
		best := math.Inf(1)
		bestIdx := -1
		for v := 1; v < levels; v++ {
			if cost[v-1] < best {
				best = cost[v-1]
				bestIdx = v - 1
			}
			parent[v] = bestIdx
			next[v] = math.Abs(w[i]-float64(first+v)) + best
		}
		parents[i-1] = parent
		cost = next
	}
	v := 0
	for j := 1; j < levels; j++ {
		if cost[j] < cost[v] {
			v = j
		}
	}
	var out [5]int64
	for i := len(w) - 1; i >= 0; i-- {
		out[i] = int64(first + v)
		if i > 0 {
			v = parents[i-1][v]
		}
	}
	return out
}

func maxIntDrift(w [5]float64, v [5]int64) float64 {
	d := 0.0
	for i := range w {
		d = math.Max(d, math.Abs(w[i]-float64(v[i])))
	}
	return d
}

func weightsFromU(u [5]float64) [5]float64 {
	var w [5]float64
	for i := range w {
		for j := 0; j <= i; j++ {
			w[i] += math.Exp(u[j])
		}
	}
	return w
}

func uFromX(x [6]float64) [5]float64 {
	var u [5]float64
	copy(u[:], x[:5])
	return u
}

func fitGD(d fitData, opts gdOptions, rng *rand.Rand, trace func(restart, step int, loss float64)) fitResult {
	best := fitResult{Loss: math.Inf(1), Restarts: opts.Restarts}
	for r := range opts.Restarts {
		var u [5]float64
		for j := range u {
			u[j] = math.Log(fitRestartIncrementLo + rng.Float64()*(fitRestartIncrementHi-fitRestartIncrementLo))
		}
		z := math.Log(fitRestartScaleLo + rng.Float64()*(fitRestartScaleHi-fitRestartScaleLo))
		res := gdRun(d, u, z, opts, r, trace)
		if res.Loss < best.Loss {
			best = res
		}
	}
	return best
}

func gdRun(d fitData, u [5]float64, z float64, opts gdOptions, restart int, trace func(restart, step int, loss float64)) fitResult {
	x := [6]float64{u[0], u[1], u[2], u[3], u[4], z}
	loss := d.lossAt(weightsFromU(uFromX(x)), math.Exp(x[5]))
	lr := opts.LR0
	var vel [6]float64
	steps := 0
	windowBest := loss
	for steps < opts.MaxSteps {
		_, dw, dLogS := d.lossGrad(weightsFromU(uFromX(x)), math.Exp(x[5]), true)
		var grad [6]float64
		tail := 0.0
		for j := 4; j >= 0; j-- {
			tail += dw[j]
			grad[j] = math.Exp(x[j]) * tail
		}
		grad[5] = dLogS
		norm := 0.0
		for _, g := range grad {
			norm += g * g
		}
		if math.Sqrt(norm) < gdGradEps {
			break
		}
		accepted := false
		for lr >= gdLRMin {
			var nextVel, next [6]float64
			for i := range next {
				nextVel[i] = gdMomentum*vel[i] - lr*grad[i]
				next[i] = x[i] + nextVel[i]
			}
			nloss := d.lossAt(weightsFromU(uFromX(next)), math.Exp(next[5]))
			if nloss <= loss {
				x, vel = next, nextVel
				loss = nloss
				lr = math.Min(lr*gdLRGrowth, gdLRMax)
				accepted = true
				break
			}
			vel = [6]float64{}
			lr *= gdLRShrink
		}
		if !accepted {
			break
		}
		steps++
		if trace != nil && opts.TraceEvery > 0 && steps%opts.TraceEvery == 0 {
			trace(restart, steps, loss)
		}
		if steps%gdStallWindow == 0 {
			if windowBest-loss < gdStallEps*(1+math.Abs(loss)) {
				break
			}
			windowBest = loss
		}
	}
	return fitResult{W: weightsFromU(uFromX(x)), S: math.Exp(x[5]), Loss: loss, Steps: steps, Restart: restart}
}

func fitScaleGD(d fitData, w [5]float64, opts gdOptions, rng *rand.Rand) fitResult {
	z := math.Log(fitRestartScaleLo + rng.Float64()*(fitRestartScaleHi-fitRestartScaleLo))
	loss := d.lossAt(w, math.Exp(z))
	lr := opts.LR0
	steps := 0
	windowBest := loss
	for steps < opts.MaxSteps {
		_, _, dLogS := d.lossGrad(w, math.Exp(z), true)
		if math.Abs(dLogS) < gdGradEps {
			break
		}
		accepted := false
		for lr >= gdLRMin {
			nz := z - lr*dLogS
			nloss := d.lossAt(w, math.Exp(nz))
			if nloss <= loss {
				z, loss = nz, nloss
				lr = math.Min(lr*gdLRGrowth, gdLRMax)
				accepted = true
				break
			}
			lr *= gdLRShrink
		}
		if !accepted {
			break
		}
		steps++
		if steps%gdStallWindow == 0 {
			if windowBest-loss < gdStallEps*(1+math.Abs(loss)) {
				break
			}
			windowBest = loss
		}
	}
	return fitResult{W: w, S: math.Exp(z), Loss: loss, Steps: steps}
}

func splitSeriesKeys(keys []string, seed int64, trainFrac float64) map[string]bool {
	rng := rand.New(rand.NewSource(seed))
	order := rng.Perm(len(keys))
	n := int(math.Round(trainFrac * float64(len(keys))))
	train := make(map[string]bool, n)
	for i := range n {
		train[keys[order[i]]] = true
	}
	return train
}
