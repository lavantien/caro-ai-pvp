package main

import "math"

type gp struct {
	dims    int
	lengths []float64
	sigmaF  float64
	sigmaN  float64
	xs      [][]float64
	ys      []float64
	l       [][]float64
	alpha   []float64
}

func newGP(dims int, lengths []float64, sigmaF, sigmaN float64) *gp {
	return &gp{dims: dims, lengths: append([]float64(nil), lengths...), sigmaF: sigmaF, sigmaN: sigmaN}
}

func (g *gp) kernel(a, b []float64) float64 {
	s := 0.0
	for i := range a {
		d := (a[i] - b[i]) / g.lengths[i]
		s += d * d
	}
	return g.sigmaF * g.sigmaF * math.Exp(-0.5*s)
}

func (g *gp) fit(xs [][]float64, ys []float64) {
	g.xs = xs
	g.ys = ys
	n := len(xs)
	k := make([][]float64, n)
	for i := range k {
		k[i] = make([]float64, n)
		for j := 0; j <= i; j++ {
			v := g.kernel(xs[i], xs[j])
			k[i][j] = v
			k[j][i] = v
		}
		k[i][i] += g.sigmaN * g.sigmaN
	}
	g.l = cholesky(k)
	g.alpha = backSolve(g.l, forwardSolve(g.l, append([]float64(nil), ys...)))
}

func (g *gp) predict(x []float64) (float64, float64) {
	kstar := make([]float64, len(g.xs))
	for i, xi := range g.xs {
		kstar[i] = g.kernel(x, xi)
	}
	mu := 0.0
	for i := range kstar {
		mu += kstar[i] * g.alpha[i]
	}
	v := forwardSolve(g.l, kstar)
	ss := 0.0
	for _, e := range v {
		ss += e * e
	}
	variance := g.kernel(x, x) - ss
	if variance < 0 {
		variance = 0
	}
	return mu, math.Sqrt(variance)
}

func (g *gp) logMarginal() float64 {
	n := len(g.ys)
	logdet := 0.0
	for i := 0; i < n; i++ {
		logdet += math.Log(g.l[i][i])
	}
	quad := 0.0
	for i := range g.ys {
		quad += g.ys[i] * g.alpha[i]
	}
	return -0.5*quad - logdet - 0.5*float64(n)*math.Log(2*math.Pi)
}

func (g *gp) fitMap(xs [][]float64, ys []float64, dims []dim) {
	span := make([]float64, g.dims)
	p := make([]float64, g.dims)
	priorMean := make([]float64, g.dims)
	floor := make([]float64, g.dims)
	ceil := make([]float64, g.dims)
	for i, d := range dims {
		if d.hi-d.lo > 0 {
			span[i] = d.hi - d.lo
		} else {
			span[i] = 1
		}
		priorMean[i] = math.Log(priorMeanFrac * span[i])
		p[i] = priorMean[i]
		floor[i] = math.Log(lengthFloorFrac * span[i])
		ceil[i] = math.Log(lengthCeilFrac * span[i])
	}
	obj := func(pp []float64) float64 {
		for i := range g.lengths {
			g.lengths[i] = math.Exp(pp[i])
		}
		g.fit(xs, ys)
		pen := 0.0
		for i := range pp {
			d := pp[i] - priorMean[i]
			pen += d * d
		}
		return g.logMarginal() - pen/(2*priorSd*priorSd)
	}
	cur := obj(p)
	for step := 0; step < mapSteps; step++ {
		grad := make([]float64, g.dims)
		for i := range grad {
			up := append([]float64(nil), p...)
			dn := append([]float64(nil), p...)
			up[i] += mapH
			dn[i] -= mapH
			grad[i] = (obj(up) - obj(dn)) / (2 * mapH)
		}
		norm := 0.0
		for _, gr := range grad {
			norm += gr * gr
		}
		norm = math.Sqrt(norm)
		if norm < 1e-9 {
			break
		}
		next := make([]float64, g.dims)
		for i := range next {
			next[i] = p[i] + mapStepSize*grad[i]/norm
			next[i] = math.Max(next[i], floor[i])
			next[i] = math.Min(next[i], ceil[i])
		}
		v := obj(next)
		if v <= cur {
			break
		}
		p = next
		cur = v
	}
	obj(p)
}

func cholesky(a [][]float64) [][]float64 {
	n := len(a)
	l := make([][]float64, n)
	for i := range l {
		l[i] = make([]float64, n)
	}
	for i := 0; i < n; i++ {
		for j := 0; j <= i; j++ {
			s := a[i][j]
			for k := 0; k < j; k++ {
				s -= l[i][k] * l[j][k]
			}
			if i == j {
				if s < choleskyFloor {
					s = choleskyFloor
				}
				l[i][i] = math.Sqrt(s)
			} else {
				l[i][j] = s / l[j][j]
			}
		}
	}
	return l
}

func forwardSolve(l [][]float64, b []float64) []float64 {
	x := append([]float64(nil), b...)
	for i := range l {
		for k := 0; k < i; k++ {
			x[i] -= l[i][k] * x[k]
		}
		x[i] /= l[i][i]
	}
	return x
}

func backSolve(l [][]float64, b []float64) []float64 {
	n := len(l)
	x := append([]float64(nil), b...)
	for i := n - 1; i >= 0; i-- {
		for k := i + 1; k < n; k++ {
			x[i] -= l[k][i] * x[k]
		}
		x[i] /= l[i][i]
	}
	return x
}
