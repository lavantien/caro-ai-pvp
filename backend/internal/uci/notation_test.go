package uci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoveToString(t *testing.T) {
	assert.Equal(t, "aa", MoveToString(0, 0))
	assert.Equal(t, "bd", MoveToString(3, 1))
	assert.Equal(t, "pp", MoveToString(15, 15))
}

func TestParseMove(t *testing.T) {
	x, y, ok := ParseMove("aa")
	assert.True(t, ok)
	assert.Equal(t, 0, x)
	assert.Equal(t, 0, y)

	x, y, ok = ParseMove("bd")
	assert.True(t, ok)
	assert.Equal(t, 3, x)
	assert.Equal(t, 1, y)

	_, _, ok = ParseMove("z")
	assert.False(t, ok)
}
