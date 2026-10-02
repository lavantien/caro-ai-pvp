//go:build !race

package engine

import "time"

const raceBudgetScale = 1

func scaledBudget(d time.Duration) time.Duration { return d * raceBudgetScale }
