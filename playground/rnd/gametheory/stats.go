package main

import "math"

func wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	p := float64(k) / float64(n)
	z := wilsonZ
	den := 1 + z*z/float64(n)
	center := (p + z*z/(2*float64(n))) / den
	half := z * math.Sqrt(p*(1-p)/float64(n)+z*z/(4*float64(n)*float64(n))) / den
	lo = center - half
	hi = center + half
	if lo < 0 {
		lo = 0
	}
	if hi > 1 {
		hi = 1
	}
	return lo, hi
}

func quantile(sorted []int, q float64) int {
	if len(sorted) == 0 {
		return 0
	}
	idx := int(math.Round(q * float64(len(sorted)-1)))
	return sorted[idx]
}
