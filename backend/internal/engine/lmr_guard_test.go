package engine

import (
	"caro-ai-pvp/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLMRReductionSkipsTacticalMoves(t *testing.T) {
	assert.Equal(t, 0, lmrReduction(8, 12, true, -100),
		"a forcing move must never be reduced, however late it is ordered")
	assert.Equal(t, 3, lmrReduction(8, 12, false, -100),
		"quiet late move with poor history keeps the deep reduction")
	assert.Equal(t, 2, lmrReduction(8, 12, false, 500),
		"quiet late move with good history keeps the base reduction")
	assert.Equal(t, 1, lmrReduction(8, 5, false, 500),
		"early quiet move keeps the light reduction")
	assert.Equal(t, 0, lmrReduction(2, 12, false, 500),
		"shallow nodes are never reduced (depth below LMRMinDepth)")
	assert.Equal(t, 2, lmrReduction(3, 12, false, -100),
		"reduction stays capped below the node depth (3 would equal depth)")
}

func TestPickerFlagsWinningMovesAsTactical(t *testing.T) {
	b := redHasOpenFour() // red to move can complete an open four at (2,5) or (7,5)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)

	picker := NewMovePicker(candidates, &sb, domain.PlayerRed, 6, nil, NewSearchHeuristics(), domain.Position{X: -1, Y: -1})
	sawTactical, sawQuiet := false, false
	for {
		m, ok := picker.Next()
		if !ok {
			break
		}
		if picker.LastMoveTactical() {
			sawTactical = true
			assert.True(t, wouldWin(&sb, m.X, m.Y, domain.PlayerRed),
				"only winning completions may be flagged tactical from the winning stage")
		} else {
			sawQuiet = true
		}
	}
	assert.True(t, sawTactical, "winning completions must be flagged")
	assert.True(t, sawQuiet, "quiet moves must not be flagged")
}

func TestPickerFlagsMustBlockMovesAsTactical(t *testing.T) {
	// Blue holds an open four on row 5 (columns 3-6); red to move must block
	// (2,5) or (7,5) or lose immediately.
	b := domain.NewBoard()
	for x := 3; x <= 6; x++ {
		b = b.PlaceStone(x, 5, domain.PlayerBlue)
	}
	b = b.PlaceStone(10, 10, domain.PlayerRed)
	sb := NewSearchBoard(b)
	candidates := GetCandidates(&sb, domain.MaxSearchRadius)

	picker := NewMovePicker(candidates, &sb, domain.PlayerRed, 6, nil, NewSearchHeuristics(), domain.Position{X: -1, Y: -1})
	sawBlockTactical := false
	for {
		m, ok := picker.Next()
		if !ok {
			break
		}
		if (m.X == 2 || m.X == 7) && m.Y == 5 {
			if picker.LastMoveTactical() {
				sawBlockTactical = true
			}
		}
	}
	assert.True(t, sawBlockTactical,
		"the only moves that stop an opponent open four must be flagged tactical")
}
