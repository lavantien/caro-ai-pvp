package main

import "sort"

type surrogate struct {
	dim   int
	lo    []float64
	hi    []float64
	xs    [][]float64
	ys    []float64
	theta []float64
	dirty bool
}

func newSurrogate(dim int, lo, hi []float64) *surrogate {
	return &surrogate{dim: dim, lo: lo, hi: hi, theta: make([]float64, dim+1)}
}

func screenMaxHistory() int { return 64 * dePop }

func (s *surrogate) features(x []float64) []float64 {
	f := make([]float64, s.dim+1)
	f[0] = 1
	for i := 0; i < s.dim; i++ {
		span := s.hi[i] - s.lo[i]
		if span <= 0 {
			f[i+1] = 0
			continue
		}
		f[i+1] = (x[i] - s.lo[i]) / span
	}
	return f
}

func (s *surrogate) observe(x []float64, y float64) {
	s.xs = append(s.xs, s.features(x))
	s.ys = append(s.ys, y)
	if len(s.xs) > screenMaxHistory() {
		drop := len(s.xs) - screenMaxHistory()
		s.xs = append([][]float64{}, s.xs[drop:]...)
		s.ys = append([]float64{}, s.ys[drop:]...)
	}
	s.dirty = true
}

func solveRidge(xs [][]float64, ys []float64, lambda float64) []float64 {
	d := len(xs[0])
	a := make([][]float64, d)
	for i := range a {
		a[i] = make([]float64, d)
	}
	b := make([]float64, d)
	for r := 0; r < d; r++ {
		for c := 0; c < d; c++ {
			sum := 0.0
			for k := range xs {
				sum += xs[k][r] * xs[k][c]
			}
			if r == c {
				sum += lambda
			}
			a[r][c] = sum
		}
		sum := 0.0
		for k := range xs {
			sum += xs[k][r] * ys[k]
		}
		b[r] = sum
	}
	for col := 0; col < d; col++ {
		piv := col
		for r := col + 1; r < d; r++ {
			if abs(a[r][col]) > abs(a[piv][col]) {
				piv = r
			}
		}
		a[col], a[piv] = a[piv], a[col]
		b[col], b[piv] = b[piv], b[col]
		if abs(a[col][col]) < 1e-12 {
			continue
		}
		for r := col + 1; r < d; r++ {
			f := a[r][col] / a[col][col]
			for c := col; c < d; c++ {
				a[r][c] -= f * a[col][c]
			}
			b[r] -= f * b[col]
		}
	}
	x := make([]float64, d)
	for r := d - 1; r >= 0; r-- {
		if abs(a[r][r]) < 1e-12 {
			continue
		}
		sum := b[r]
		for c := r + 1; c < d; c++ {
			sum -= a[r][c] * x[c]
		}
		x[r] = sum / a[r][r]
	}
	return x
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (s *surrogate) refit() {
	if !s.dirty || len(s.xs) == 0 {
		return
	}
	s.theta = solveRidge(s.xs, s.ys, screenLambda)
	s.dirty = false
}

func (s *surrogate) predict(x []float64) float64 {
	s.refit()
	f := s.features(x)
	sum := 0.0
	for i, v := range f {
		sum += s.theta[i] * v
	}
	return sum
}

func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(q * float64(len(sorted)-1))
	return sorted[idx]
}

func (s *surrogate) skip(x []float64, loss []float64, skipped, popLen int) bool {
	if len(s.ys) < screenWarmup {
		return false
	}
	if popLen > 0 && skipped >= int(screenSkipCap*float64(popLen)) {
		return false
	}
	ys := append([]float64{}, s.ys...)
	sort.Float64s(ys)
	pred := s.predict(x)
	return pred > quantile(ys, screenQuantile)
}
