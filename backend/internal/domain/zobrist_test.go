package domain

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestZobristKeysAreNonZero(t *testing.T) {
	for x := 0; x < BoardSize; x++ {
		for y := 0; y < BoardSize; y++ {
			assert.NotZero(t, ZobristKey(x, y, PlayerRed), "red key (%d,%d)", x, y)
			assert.NotZero(t, ZobristKey(x, y, PlayerBlue), "blue key (%d,%d)", x, y)
		}
	}
}

func TestZobristKeysAreDistinct(t *testing.T) {
	seen := make(map[uint64]string)
	for x := 0; x < BoardSize; x++ {
		for y := 0; y < BoardSize; y++ {
			kr := ZobristKey(x, y, PlayerRed)
			loc, exists := seen[kr]
			require.False(t, exists, "duplicate key with %d,%d red (also %s)", x, y, loc)
			seen[kr] = fmt.Sprintf("%d,%d red", x, y)

			kb := ZobristKey(x, y, PlayerBlue)
			loc, exists = seen[kb]
			require.False(t, exists, "duplicate key with %d,%d blue (also %s)", x, y, loc)
			seen[kb] = fmt.Sprintf("%d,%d blue", x, y)
		}
	}
}

func TestZobristDeterministic(t *testing.T) {
	k1 := ZobristKey(5, 5, PlayerRed)
	k2 := ZobristKey(5, 5, PlayerRed)
	assert.Equal(t, k1, k2)
}
