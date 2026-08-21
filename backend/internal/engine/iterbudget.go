package engine

// Iteration-cost prediction keeps per-move spend inside the soft budget. An
// iterative-deepening loop that only checks elapsed time when between depths
// will start an iteration it cannot finish and then burn to the hard bound,
// so nearly every move costs the hard bound instead of the soft target.

const (
	iterGrowthMin     = 1.5
	iterGrowthMax     = 6.0
	iterGrowthDefault = 4.0
)

// iterationGrowth estimates how much costlier the next depth is than the last
// completed one. A warm TT can make an iteration cheaper, but predictions
// never assume shrinkage; one noisy re-search must not predict runaway.
func iterationGrowth(lastMs, prevMs int64) float64 {
	if lastMs <= 0 || prevMs <= 0 {
		return iterGrowthDefault
	}
	ratio := float64(lastMs) / float64(prevMs)
	return min(max(ratio, iterGrowthMin), iterGrowthMax)
}

// nextIterationFits reports whether starting another depth is predicted to
// finish inside the soft budget. softMs <= 0 disables the gate (hard bound
// still applies through the TimeMonitor).
func nextIterationFits(elapsedMs, lastIterMs, prevIterMs, softMs int64) bool {
	if softMs <= 0 || lastIterMs <= 0 {
		return true
	}
	predicted := int64(float64(lastIterMs) * iterationGrowth(lastIterMs, prevIterMs))
	return elapsedMs+predicted <= softMs
}
