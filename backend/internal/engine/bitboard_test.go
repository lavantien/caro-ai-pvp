package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitBoardSetAndGet(t *testing.T) {
	var bb BitBoard
	bb.Set(0, 0)
	assert.True(t, bb.Get(0, 0))
	assert.False(t, bb.Get(1, 0))

	bb.Set(15, 15)
	assert.True(t, bb.Get(15, 15))
}

func TestBitBoardClear(t *testing.T) {
	var bb BitBoard
	bb.Set(5, 5)
	bb.Clear(5, 5)
	assert.False(t, bb.Get(5, 5))
}

func TestBitBoardOr(t *testing.T) {
	var a, b BitBoard
	a.Set(0, 0)
	b.Set(1, 0)
	c := a.Or(b)
	assert.True(t, c.Get(0, 0))
	assert.True(t, c.Get(1, 0))
}

func TestBitBoardCount(t *testing.T) {
	var bb BitBoard
	bb.Set(0, 0)
	bb.Set(1, 0)
	bb.Set(2, 0)
	assert.Equal(t, 3, bb.Count())
}

func TestBitBoardDilate(t *testing.T) {
	var bb BitBoard
	bb.Set(8, 8)
	dilated := bb.Dilate()
	assert.True(t, dilated.Get(7, 7), "diagonal up-left")
	assert.True(t, dilated.Get(8, 8), "center preserved")
	assert.True(t, dilated.Get(9, 9), "diagonal down-right")
	assert.True(t, dilated.Get(7, 8), "left")
	assert.True(t, dilated.Get(9, 8), "right")
	assert.True(t, dilated.Get(8, 7), "up")
	assert.True(t, dilated.Get(8, 9), "down")
}

func TestBitBoardFromDomain(t *testing.T) {
	b := domain.NewBoard().PlaceStone(3, 4, domain.PlayerRed)
	red, blue := BitBoardsFromDomain(b)
	assert.True(t, red.Get(3, 4))
	assert.False(t, blue.Get(3, 4))
}

func TestBitBoardAnd(t *testing.T) {
	var a, b BitBoard
	a.Set(0, 0)
	a.Set(1, 0)
	b.Set(1, 0)
	b.Set(2, 0)
	c := a.And(b)
	assert.False(t, c.Get(0, 0))
	assert.True(t, c.Get(1, 0))
	assert.False(t, c.Get(2, 0))
}

func TestBitBoardXor(t *testing.T) {
	var a, b BitBoard
	a.Set(0, 0)
	a.Set(1, 0)
	b.Set(1, 0)
	b.Set(2, 0)
	c := a.Xor(b)
	assert.True(t, c.Get(0, 0))
	assert.False(t, c.Get(1, 0))
	assert.True(t, c.Get(2, 0))
}

func TestBitBoardNot(t *testing.T) {
	var bb BitBoard
	bb.Set(0, 0)
	n := bb.Not()
	assert.False(t, n.Get(0, 0))
	assert.True(t, n.Get(1, 0))
}

func TestBitBoardIsZero(t *testing.T) {
	var bb BitBoard
	assert.True(t, bb.IsZero())
	bb.Set(5, 5)
	assert.False(t, bb.IsZero())
}
