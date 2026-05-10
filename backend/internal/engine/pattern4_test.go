package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassifyDirectionFlex3(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	// Place at (7,8) makes 3 horizontal with open ends → Flex3
	result := classifyDirection(&sb, 7, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Flex3, result)
}

func TestClassifyDirectionBlock3(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(4, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 7, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Block3, result)
}

func TestClassifyDirectionFlex4(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 8, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Flex4, result)
}

func TestClassifyDirectionBlock4(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed).
		PlaceStone(9, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 8, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Block4, result)
}

func TestClassifyDirectionExactly5(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 9, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Exactly5, result)
}

func TestClassifyDirectionBlocked5(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(4, 8, domain.PlayerBlue).
		PlaceStone(10, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 9, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4None, result)
}

func TestClassifyDirectionOverline(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed).
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(9, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 10, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Overline, result)
}

func TestClassifyDirectionFlex2(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 6, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Flex2, result)
}

func TestClassifyDirectionBlock2(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(5, 8, domain.PlayerRed).
		PlaceStone(4, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 6, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Block2, result)
}

func TestClassifyDirectionFlex1(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 8, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4Flex1, result)
}

func TestClassifyDirectionNone(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(6, 8, domain.PlayerRed).
		PlaceStone(7, 8, domain.PlayerRed).
		PlaceStone(5, 8, domain.PlayerBlue).
		PlaceStone(8, 8, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	result := classifyDirection(&sb, 6, 8, 1, 0, domain.PlayerRed)
	assert.Equal(t, P4None, result)
}

func TestHasDoubleFlex3(t *testing.T) {
	// Move at (6,5): horizontal (5,5)+(7,5)=Flex3, vertical (6,4)+(6,6)=Flex3
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(6, 4, domain.PlayerRed).
		PlaceStone(6, 6, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	assert.True(t, hasDoubleFlex3(&sb, 6, 5, domain.PlayerRed))
}

func TestHasDoubleFlex3False(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	assert.False(t, hasDoubleFlex3(&sb, 8, 8, domain.PlayerRed))
}

func TestHasFlex4PlusFlex3(t *testing.T) {
	// Move at (6,5): horizontal (5,5)+(7,5)+(8,5)=Flex4, vertical (6,4)+(6,6)=Flex3
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(6, 4, domain.PlayerRed).
		PlaceStone(6, 6, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	assert.True(t, hasFlex4PlusFlex3(&sb, 6, 5, domain.PlayerRed))
}

func TestHasFlex4PlusFlex3False(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	assert.False(t, hasFlex4PlusFlex3(&sb, 8, 8, domain.PlayerRed))
}
