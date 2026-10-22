package main

import "math"

func expectedImprovement(mu, sigma, best, xi float64) float64 {
	gap := best - xi - mu
	if sigma <= 0 {
		if gap > 0 {
			return gap
		}
		return 0
	}
	z := gap / sigma
	return gap*stdNormCdf(z) + sigma*stdNormPdf(z)
}

func stdNormCdf(z float64) float64 {
	return 0.5 * math.Erfc(-z/math.Sqrt2)
}

func stdNormPdf(z float64) float64 {
	return math.Exp(-0.5*z*z) / math.Sqrt(2*math.Pi)
}
