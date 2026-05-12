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

func TestEvaluateZeroSumProperty(t *testing.T) {
	boards := []domain.Board{
		domain.NewBoard(),
		domain.NewBoard().PlaceStone(8, 8, domain.PlayerRed).PlaceStone(7, 7, domain.PlayerBlue),
		domain.NewBoard().
			PlaceStone(5, 5, domain.PlayerRed).PlaceStone(6, 5, domain.PlayerRed).
			PlaceStone(7, 5, domain.PlayerRed).
			PlaceStone(0, 0, domain.PlayerBlue).PlaceStone(1, 1, domain.PlayerBlue),
		domain.NewBoard().
			PlaceStone(3, 3, domain.PlayerRed).PlaceStone(4, 3, domain.PlayerRed).
			PlaceStone(5, 3, domain.PlayerRed).PlaceStone(6, 3, domain.PlayerRed).
			PlaceStone(10, 10, domain.PlayerBlue),
	}
	for i, b := range boards {
		sb := NewSearchBoard(b)
		scoreRed := Evaluate(&sb, domain.PlayerRed)
		scoreBlue := Evaluate(&sb, domain.PlayerBlue)
		assert.Equal(t, scoreRed, -scoreBlue, "zero-sum violated for board %d: red=%d blue=%d", i, scoreRed, scoreBlue)
	}
}

func TestEvaluateOpponentThreatsPenalized(t *testing.T) {
	b := domain.NewBoard()
	for x := 3; x < 6; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerRed)
	}
	sb := NewSearchBoard(b)
	scoreRed := Evaluate(&sb, domain.PlayerRed)
	scoreBlue := Evaluate(&sb, domain.PlayerBlue)
	assert.Greater(t, scoreRed, 0, "player with 3-in-a-row should have positive score")
	assert.Equal(t, -scoreRed, scoreBlue, "scores must be zero-sum")
}
