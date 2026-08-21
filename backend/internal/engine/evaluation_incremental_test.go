package engine

import (
	"caro-ai-pvp/internal/domain"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The incremental pattern aggregates must be indistinguishable from the
// from-scratch classification after any sequence of makes and unmakes:
// counts first (the eval's early returns and ±MaxEval clamp could mask
// count drift), then the score itself.
func TestIncrementalEvaluateMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewSource(20260821))
	sb := NewSearchBoard(domain.NewBoard())
	var depth int

	verify := func(step int) {
		for _, p := range []domain.Player{domain.PlayerRed, domain.PlayerBlue} {
			assert.Equal(t, ClassifyBoard(&sb, p), sb.patterns(p),
				"step %d: pattern counts diverged for %v", step, p)
			assert.Equal(t, evaluateSlow(&sb, p), Evaluate(&sb, p),
				"step %d: eval diverged for %v", step, p)
		}
	}

	for step := range 400 {
		verify(step)

		if depth > 0 && rng.Intn(3) == 0 {
			sb.UnmakeMove()
			depth--
			continue
		}
		for range 200 {
			x, y := rng.Intn(domain.BoardSize), rng.Intn(domain.BoardSize)
			if !sb.IsEmpty(x, y) {
				continue
			}
			player := domain.PlayerRed
			if depth%2 == 1 {
				player = domain.PlayerBlue
			}
			sb.MakeMove(x, y, player)
			depth++
			break
		}
	}

	verify(-1)
	require.True(t, depth > 0, "playout must have placed stones")
}

func BenchmarkEvaluateIncremental(b *testing.B) {
	rng := rand.New(rand.NewSource(1))
	board := domain.NewBoard()
	player := domain.PlayerRed
	for range 40 {
		for {
			x, y := rng.Intn(domain.BoardSize), rng.Intn(domain.BoardSize)
			if board.IsEmptyAt(x, y) {
				board = board.PlaceStone(x, y, player)
				player = player.Opponent()
				break
			}
		}
	}
	sb := NewSearchBoard(board)

	b.Run("incremental", func(b *testing.B) {
		for b.Loop() {
			Evaluate(&sb, domain.PlayerRed)
		}
	})
	b.Run("reference", func(b *testing.B) {
		for b.Loop() {
			evaluateSlow(&sb, domain.PlayerRed)
		}
	})
}
