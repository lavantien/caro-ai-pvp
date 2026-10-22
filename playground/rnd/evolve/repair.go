package main

import "math"

func pavaNonincreasing(x []float64) {
	n := len(x)
	val := make([]float64, n)
	wt := make([]float64, n)
	cnt := make([]int, n)
	k := 0
	for i := 0; i < n; i++ {
		val[k], wt[k], cnt[k] = x[i], 1, 1
		for k > 0 && val[k-1] < val[k] {
			tw := wt[k-1] + wt[k]
			val[k-1] = (val[k-1]*wt[k-1] + val[k]*wt[k]) / tw
			wt[k-1] = tw
			cnt[k-1] += cnt[k]
			k--
		}
		k++
	}
	pos := 0
	for j := 0; j < k; j++ {
		v := val[j]
		for t := 0; t < cnt[j]; t++ {
			x[pos] = v
			pos++
		}
	}
}

func repairBox(x []float64, lo, hi []float64) {
	clipTo(x, lo, hi)
	pavaNonincreasing(x)
	clipTo(x, lo, hi)
}

func exactDecreasing(x []float64, lo, hi []float64) []float64 {
	y := append([]float64{}, x...)
	repairBox(y, lo, hi)
	return y
}

func diagonalRosenbrock(c float64, d int) float64 {
	s := 0.0
	for i := 0; i+1 < d; i++ {
		t := c - c*c
		s += 100*t*t + (1-c)*(1-c)
	}
	s += (1 - c) * (1 - c)
	return s
}

func bestDiagonal(d int) (float64, float64) {
	lo, hi := -5.0, 10.0
	phi := (math.Sqrt(5) - 1) / 2
	a, b := lo, hi
	for range 200 {
		c1 := b - phi*(b-a)
		c2 := a + phi*(b-a)
		if diagonalRosenbrock(c1, d) < diagonalRosenbrock(c2, d) {
			b = c2
		} else {
			a = c1
		}
	}
	c := (a + b) / 2
	return c, diagonalRosenbrock(c, d)
}
