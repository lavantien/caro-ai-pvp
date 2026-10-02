//go:build race

package engine

import "time"

// The race detector multiplies the wall-clock cost of the search path by an
// order of magnitude, so budgets in tests that assert search progress
// (completed iterations, node counts, specific moves) scale with it.
const raceBudgetScale = 20

func scaledBudget(d time.Duration) time.Duration { return d * raceBudgetScale }
