package engine

import (
	"caro-ai-pvp/internal/domain"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// m15VCFBoard is a real game position where red wins by a two-move VCF chain:
// (9,4) makes a four, blue must block, then (9,5) completes five.
func m15VCFBoard() domain.Board {
	return domain.NewBoard().
		// Red stones
		PlaceStone(9, 8, domain.PlayerRed).
		PlaceStone(6, 7, domain.PlayerRed).
		PlaceStone(9, 7, domain.PlayerRed).
		PlaceStone(7, 7, domain.PlayerRed).
		PlaceStone(8, 9, domain.PlayerRed).
		PlaceStone(8, 5, domain.PlayerRed).
		PlaceStone(7, 6, domain.PlayerRed).
		PlaceStone(9, 6, domain.PlayerRed).
		// Blue stones
		PlaceStone(8, 8, domain.PlayerBlue).
		PlaceStone(7, 9, domain.PlayerBlue).
		PlaceStone(9, 9, domain.PlayerBlue).
		PlaceStone(8, 7, domain.PlayerBlue).
		PlaceStone(8, 6, domain.PlayerBlue).
		PlaceStone(8, 10, domain.PlayerBlue).
		PlaceStone(5, 8, domain.PlayerBlue)
}

func TestSolveVCFDepthLimit(t *testing.T) {
	b := m15VCFBoard()

	_, _, result := SolveVCFWithDepth(b, domain.PlayerRed, 1, 5000, context.Background())
	assert.Equal(t, VCFNoWin, result,
		"depth 1 only sees immediate fours; the chain needs two attacker moves")

	_, _, result = SolveVCFWithDepth(b, domain.PlayerRed, 2, 5000, context.Background())
	assert.Equal(t, VCFWin, result,
		"depth 2 must find the four-then-five chain")

	_, _, result = SolveVCF(b, domain.PlayerRed, 5000, context.Background())
	assert.Equal(t, VCFWin, result,
		"the default entry point keeps the full default sight")
}

func TestSearchPositionRespectsVCFDepthLimit(t *testing.T) {
	b := m15VCFBoard()

	limited := SearchConfig{MaxDepth: 10, TimeLimitMs: 30_000, Goroutines: 1, UseVCF: true, VCFMaxDepth: 1}
	_, _, stats := SearchPosition(b, domain.PlayerRed, limited, NewTranspositionTable(1), NewSearchHeuristics(), context.Background())
	assert.NotEqual(t, "vcf", stats.MoveType,
		"a depth-1 VCF sight must not report the two-move chain as a solver win")

	full := SearchConfig{MaxDepth: 10, TimeLimitMs: 30_000, Goroutines: 1, UseVCF: true}
	_, _, stats = SearchPosition(b, domain.PlayerRed, full, NewTranspositionTable(1), NewSearchHeuristics(), context.Background())
	assert.Equal(t, "vcf", stats.MoveType,
		"the default sight (VCFMaxDepth 0) must keep finding the chain")
}
