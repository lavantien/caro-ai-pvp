//go:build !race

package engine

import (
	"sync"
	"testing"
	"time"
)

const raceBudgetScale = 1
const coverBudgetScale = 5

var (
	coverOnce   sync.Once
	coverScaled bool
)

func scaledBudget(d time.Duration) time.Duration {
	coverOnce.Do(func() { coverScaled = testing.Coverage() > 0 })
	if coverScaled {
		return d * coverBudgetScale
	}
	return d * raceBudgetScale
}
