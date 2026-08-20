package api

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func openingStones(s *GameSession) (int, int, int, int) {
	var redX, redY, blueX, blueY = -1, -1, -1, -1
	for x := range domain.BoardSize {
		for y := range domain.BoardSize {
			switch s.game.Board.GetPlayerAt(x, y) {
			case domain.PlayerRed:
				redX, redY = x, y
			case domain.PlayerBlue:
				blueX, blueY = x, y
			}
		}
	}
	return redX, redY, blueX, blueY
}

func TestRandomOpeningDeterministicPerSeed(t *testing.T) {
	s1 := NewGameSession("3+0", 180_000, 0, domain.GameModeAivAI, nil, nil, nil, func() int { return 1 })
	s1.applyRandomOpening(42)
	r1x, r1y, b1x, b1y := openingStones(s1)

	s2 := NewGameSession("3+0", 180_000, 0, domain.GameModeAivAI, nil, nil, nil, func() int { return 1 })
	s2.applyRandomOpening(42)
	r2x, r2y, b2x, b2y := openingStones(s2)

	assert.Equal(t, r1x, r2x)
	assert.Equal(t, r1y, r2y)
	assert.Equal(t, b1x, b2x)
	assert.Equal(t, b1y, b2y)

	// Red starts near the center, blue responds nearby.
	assert.InDelta(t, 7.5, float64(r1x), 3.5)
	assert.InDelta(t, 7.5, float64(r1y), 3.5)
	cheb := max(abs(b1x-r1x), abs(b1y-r1y))
	assert.LessOrEqual(t, cheb, 3, "blue's reply must stay local to red's first stone")
	assert.Equal(t, 2, s1.game.MoveNumber)
	assert.Equal(t, domain.PlayerRed, s1.game.CurrentPlayer)
}

func TestRandomOpeningVariesAcrossSeeds(t *testing.T) {
	seen := map[int]bool{}
	for seed := int64(1); seed <= 40; seed++ {
		s := NewGameSession("3+0", 180_000, 0, domain.GameModeAivAI, nil, nil, nil, func() int { return 1 })
		s.applyRandomOpening(seed)
		rx, ry, _, _ := openingStones(s)
		seen[ry*domain.BoardSize+rx] = true
	}
	assert.Greater(t, len(seen), 10, "40 seeds must produce a varied set of openings")
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
