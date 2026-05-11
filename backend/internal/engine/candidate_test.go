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

func TestFilterOpenRule(t *testing.T) {
	// One red stone at (8,8), no blue stones → open rule applies
	b := domain.NewBoard().PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)
	filtered := FilterOpenRule(candidates, &sb, domain.PlayerRed)
	for _, c := range filtered {
		dx := c.X - 8
		dy := c.Y - 8
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		assert.True(t, dx >= 3 || dy >= 3, "filtered candidate should be >=3 away, got (%d,%d)", c.X, c.Y)
	}
}

func TestFilterOpenRuleBluePassThrough(t *testing.T) {
	b := domain.NewBoard().PlaceStone(8, 8, domain.PlayerRed)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)
	filtered := FilterOpenRule(candidates, &sb, domain.PlayerBlue)
	assert.Equal(t, len(candidates), len(filtered), "blue player should not be filtered")
}

func TestFilterOpenRuleMultipleRed(t *testing.T) {
	// Multiple red stones → open rule does not apply
	b := domain.NewBoard().
		PlaceStone(8, 8, domain.PlayerRed).
		PlaceStone(7, 7, domain.PlayerRed)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, 2)
	filtered := FilterOpenRule(candidates, &sb, domain.PlayerRed)
	assert.Equal(t, len(candidates), len(filtered), "multiple reds should not be filtered")
}

func TestCreatesOpenFourBlocked(t *testing.T) {
	// Blocked four: XXXX with only one open end → NOT an open four
	// Red at (5,5)(6,5)(7,5), placing at (8,5). (9,5) blocked by Blue, (4,5) open.
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(9, 5, domain.PlayerBlue).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	// Placing at (8,5) creates 4 in a row: (5,5)(6,5)(7,5)(8,5)
	// Left end (4,5) is open, right end (9,5) is Blue (blocked)
	result := createsOpenFour(&sb, 8, 5, domain.PlayerRed)
	assert.False(t, result, "blocked four (one end) should NOT be detected as open four")
}

func TestCreatesOpenFourBothEndsOpen(t *testing.T) {
	// Open four: .XXXX. with both ends open → IS an open four
	b := domain.NewBoard().
		PlaceStone(5, 5, domain.PlayerRed).
		PlaceStone(6, 5, domain.PlayerRed).
		PlaceStone(7, 5, domain.PlayerRed).
		PlaceStone(0, 0, domain.PlayerBlue)
	sb := NewSearchBoard(b)
	// Placing at (8,5) creates 4: (5,5)(6,5)(7,5)(8,5)
	// Left (4,5) open, right (9,5) open
	result := createsOpenFour(&sb, 8, 5, domain.PlayerRed)
	assert.True(t, result, "open four (both ends open) should be detected")
}
