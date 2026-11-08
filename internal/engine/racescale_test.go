//go:build race

package engine

import "time"

const raceBudgetScale = 20

func scaledBudget(d time.Duration) time.Duration { return d * raceBudgetScale }

func nodeBudgetScale() uint64 { return raceBudgetScale }
