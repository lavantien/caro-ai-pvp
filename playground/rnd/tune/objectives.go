package main

import (
	"math"
	"math/rand/v2"
)

func branin(x []float64) float64 {
	b := 5.1 / (4 * math.Pi * math.Pi)
	c := 4 / math.Pi
	r := 6.0
	s := 10.0
	t := 1 / (8 * math.Pi)
	inner := x[1] - b*x[0]*x[0] + c*x[0] - r
	return inner*inner + s*(1-t)*math.Cos(x[0]) + s
}

var hartmann3Alpha = [4]float64{1.0, 1.2, 3.0, 3.2}

var hartmann3A = [4][3]float64{
	{3.0, 10.0, 30.0},
	{0.1, 10.0, 35.0},
	{3.0, 10.0, 30.0},
	{0.1, 10.0, 35.0},
}

var hartmann3P = [4][3]float64{
	{3689, 1170, 2673},
	{4699, 4387, 7470},
	{1091, 8732, 5547},
	{381, 5743, 8828},
}

func hartmann3(x []float64) float64 {
	sum := 0.0
	for i := range hartmann3Alpha {
		inner := 0.0
		for j := range x {
			p := hartmann3P[i][j] * 1e-4
			d := x[j] - p
			inner += hartmann3A[i][j] * d * d
		}
		sum += hartmann3Alpha[i] * math.Exp(-inner)
	}
	return -sum
}

func quadratic3(x []float64) float64 {
	d0 := x[0] - quadCenter0
	d1 := x[1] - quadCenter1
	d2 := x[2] - quadCenter2
	return d0*d0 + d1*d1 + d2*d2
}

func noised(obj func([]float64) float64, rng *rand.Rand) func([]float64) float64 {
	return func(x []float64) float64 {
		return obj(x) + noiseSigma*rng.NormFloat64()
	}
}
