package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvaluateEmptyBoard(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	score := Evaluate(&sb, domain.PlayerRed)
	assert.Equal(t, 0, score, "empty board should be neutral")
}

func TestEvaluateFavorsFourInRow(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 7; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	sb := NewSearchBoard(b)
	scoreRed := Evaluate(&sb, domain.PlayerRed)
	assert.Greater(t, scoreRed, 0, "red with 4 in a row should be positive for red")
}

func TestEvaluateDefenseMultiplier(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 6; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	sb := NewSearchBoard(b)

	scoreRed := Evaluate(&sb, domain.PlayerRed)
	scoreBlue := Evaluate(&sb, domain.PlayerBlue)
	assert.Less(t, scoreBlue, 0, "opponent of 3-in-a-row player should have negative score")
	assert.Greater(t, -scoreBlue, scoreRed,
		"defense multiplier should make opponent penalty larger than player advantage")
}
