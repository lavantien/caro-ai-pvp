//go:build !race

package engine_test

import "testing"

func stallNodeScale() uint64 {
	if testing.Coverage() > 0 {
		return 5
	}
	return 1
}
