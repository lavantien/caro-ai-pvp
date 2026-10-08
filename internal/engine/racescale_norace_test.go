//go:build !race

package engine

import (
	"testing"
	"time"
)

const raceBudgetScale = 1
const coverBudgetScale = 5

func scaledBudget(d time.Duration) time.Duration {
	if testing.Coverage() > 0 {
		return d * coverBudgetScale
	}
	return d * raceBudgetScale
}
