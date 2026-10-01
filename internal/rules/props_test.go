package rules

import (
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestPropMakeUnmakeRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewPCG(101, 102))
	for range 300 {
		b, _, _ := randomPosition(rng.Uint64())
		before := snapshot(b)
		for range config.BoardCells {
			cell := Cell(rng.IntN(config.BoardCells))
			if !b.inRegion(cell) || b.Occupied(cell) {
				continue
			}
			b.Make(cell)
			b.Unmake()
			assertSnapEq(t, snapshot(b), before)
			break
		}
	}
}

func TestPropRandomLegalPlayTerminates(t *testing.T) {
	rng := rand.New(rand.NewPCG(42, 43))
	buf := make([]Move, config.BoardCells)
	for game := range 40 {
		b := NewBoard()
		if game%2 == 1 {
			b = NewCrossCheck()
		}
		winner := Empty
		var lastMover Color
		var lastCell Cell
		for b.MoveCount < config.BoardCells {
			if b.Wins(Red) || b.Wins(Blue) {
				t.Fatalf("game %d: play continued past a win", game)
			}
			n := b.LegalMoves(buf)
			if n == 0 {
				if !b.IsFull() {
					t.Fatalf("game %d: stuck at %d stones with region not full", game, b.MoveCount)
				}
				break
			}
			mover := b.Side
			lastMover = mover
			lastCell = Cell(buf[rng.IntN(n)])
			b.Make(lastCell)
			if got, fast := b.Wins(lastMover), b.FastLastMoveWin(lastMover, lastCell); got != fast {
				t.Fatalf("game %d move %d: Wins(%v)=%v but FastLastMoveWin=%v", game, b.MoveCount, lastMover, got, fast)
			}
			if b.Wins(lastMover) {
				winner = lastMover
				break
			}
		}
		if winner == Empty {
			if !b.IsFull() {
				t.Fatalf("game %d: ended without a win while region not full", game)
			}
			if b.Wins(Red) || b.Wins(Blue) {
				t.Fatalf("game %d: full board with a win is a draw, not a stuck state", game)
			}
		}
	}
}

func TestPropFastLastMoveWinCoversWins(t *testing.T) {
	for seed := uint64(1); seed <= 64; seed++ {
		b, _, stones := randomPosition(seed)
		for _, color := range [colorCount]Color{Red, Blue} {
			anyThrough := false
			for _, c := range stones {
				if b.At(c) == color && b.FastLastMoveWin(color, c) {
					anyThrough = true
				}
			}
			if anyThrough != b.Wins(color) {
				t.Fatalf("seed %d color %v: disjunction of FastLastMoveWin %v != Wins %v", seed, color, anyThrough, b.Wins(color))
			}
		}
	}
}

func TestPropLegalMovesMatchIsLegal(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 56))
	buf := make([]Move, config.BoardCells)
	for range 100 {
		b, _, _ := randomPosition(rng.Uint64())
		b.Side = Color(rng.IntN(colorCount))
		n := b.LegalMoves(buf)
		for _, m := range buf[:n] {
			if !b.IsLegal(Cell(m)) {
				t.Fatalf("LegalMoves yielded %d which IsLegal rejects", m)
			}
		}
		for cell := range config.BoardCells {
			if b.IsLegal(Cell(cell)) {
				found := false
				for _, m := range buf[:n] {
					if Cell(m) == Cell(cell) {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("IsLegal accepts %d but LegalMoves omitted it", cell)
				}
			}
		}
	}
}
