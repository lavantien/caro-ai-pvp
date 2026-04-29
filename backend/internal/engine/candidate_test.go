package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCandidateEmptyBoard(t *testing.T) {
	b := domain.NewBoard()
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)
	assert.Greater(t, len(candidates), 0, "empty board should return center candidates")
}

func TestCandidateNearStones(t *testing.T) {
	b := domain.NewBoard().PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)

	for _, c := range candidates {
		assert.True(t, sb.IsEmpty(c.X, c.Y), "candidate should be empty")
	}

	found := false
	for _, c := range candidates {
		if c.X == 7 && c.Y == 7 {
			found = true
		}
	}
	assert.True(t, found, "should include neighbor of placed stone")
}

func TestCandidateNoOccupied(t *testing.T) {
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(7, 7, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)

	for _, c := range candidates {
		assert.True(t, sb.IsEmpty(c.X, c.Y), "no candidate should be occupied")
	}
}
