package rules

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func randomPosition(seed uint64) (*Board, *NaiveBoard, []Cell) {
	rng := rand.New(rand.NewPCG(seed, seed*2+1))
	var b *Board
	var nb *NaiveBoard
	if rng.IntN(2) == 1 {
		b, nb = NewCrossCheck(), NewNaiveCrossCheck()
	} else {
		b, nb = NewBoard(), NewNaiveBoard()
	}
	placed := make([]Cell, 0, 64)
	count := rng.IntN(64)
	for i := 0; i < count; i++ {
		cell := Cell(rng.IntN(config.BoardCells))
		if !b.inRegion(cell) || b.Occupied(cell) {
			i--
			continue
		}
		color := Red
		if i%2 == 1 {
			color = Blue
		}
		setStone(b, cell, color)
		nb.Set(cell, color)
		placed = append(placed, cell)
	}
	return b, nb, placed
}

type sweepState struct {
	b     *Board
	nb    *NaiveBoard
	slots []Cell
	trits []int
	t     *testing.T
}

func (s *sweepState) place(i int) {
	color := Blue
	if s.trits[i] == 1 {
		color = Red
	}
	setStone(s.b, s.slots[i], color)
	s.nb.cells[s.slots[i]] = naiveOf(color)
}

func (s *sweepState) clear(i int) {
	color := Blue
	if s.trits[i] == 1 {
		color = Red
	}
	unsetStone(s.b, s.slots[i], color)
	s.nb.cells[s.slots[i]] = naiveEmpty
}

func (s *sweepState) check() {
	s.t.Helper()
	for _, color := range [colorCount]Color{Red, Blue} {
		if got, want := s.b.Wins(color), s.nb.Wins(color); got != want {
			s.t.Fatalf("sweep %v: Wins = %v, naive %v, pattern %v slots %v", color, got, want, s.trits, s.slots)
		}
	}
	for i, v := range s.trits {
		if v == 0 {
			continue
		}
		color := Blue
		if v == 1 {
			color = Red
		}
		if got, want := s.b.FastLastMoveWin(color, s.slots[i]), s.nb.WinsThrough(color, s.slots[i]); got != want {
			s.t.Fatalf("sweep %v cell %d: FastLastMoveWin = %v, naive through %v, pattern %v slots %v", color, s.slots[i], got, want, s.trits, s.slots)
		}
	}
}

func sweepWindow(t *testing.T, b *Board, nb *NaiveBoard, slots []Cell) {
	s := sweepState{b: b, nb: nb, slots: slots, trits: make([]int, len(slots)), t: t}
	s.check()
	total := 1
	for range slots {
		total *= 3
	}
	stride := 1
	if testing.Short() {
		stride = 97
	}
	for idx := stride; idx < total; idx += stride {
		for range stride {
			i := 0
			for i < len(slots) {
				if s.trits[i] != 0 {
					s.clear(i)
				}
				s.trits[i]++
				if s.trits[i] < 3 {
					if s.trits[i] != 0 {
						s.place(i)
					}
					break
				}
				s.trits[i] = 0
				i++
			}
		}
		s.check()
	}
	for i := range s.trits {
		if s.trits[i] != 0 {
			s.clear(i)
		}
	}
}

func TestExhaustiveWindowSweep(t *testing.T) {
	for _, region := range []struct {
		name  string
		cross bool
	}{
		{"full", false},
		{"crosscheck", true},
	} {
		for d := range lineDirs {
			t.Run(fmt.Sprintf("%s/dir%d", region.name, d), func(t *testing.T) {
				t.Parallel()
				var b *Board
				var nb *NaiveBoard
				if region.cross {
					b, nb = NewCrossCheck(), NewNaiveCrossCheck()
				} else {
					b, nb = NewBoard(), NewNaiveBoard()
				}
				size := config.BoardSize
				if region.cross {
					size = config.CrossCheckSize
				}
				window := 2*config.WinLength - 1
				dr, dc := lineDirs[d][0], lineDirs[d][1]
				var slots []Cell
				for r0 := -(window - 1); r0 < size; r0++ {
					for c0 := -(window - 1); c0 < size; c0++ {
						slots = slots[:0]
						for i := range window {
							r, c := r0+i*dr, c0+i*dc
							if r >= 0 && r < size && c >= 0 && c < size {
								slots = append(slots, Cell(r*config.BoardStride+c))
							}
						}
						if len(slots) == 0 {
							continue
						}
						sweepWindow(t, b, nb, slots)
					}
				}
			})
		}
	}
}

func TestEveryCellAsLastMoveRandomCorpus(t *testing.T) {
	for _, seed := range [...]uint64{1, 2, 3, 4, 5, 6, 7, 8} {
		b, nb, cells := randomPosition(seed)
		for _, c := range cells {
			color := b.At(c)
			if color == Empty {
				t.Fatalf("seed %d: corpus cell %d not a stone", seed, c)
			}
			if got, want := b.FastLastMoveWin(color, c), nb.WinsThrough(color, c); got != want {
				t.Fatalf("seed %d cell %d color %v: FastLastMoveWin = %v, naive %v", seed, c, color, got, want)
			}
		}
		for _, color := range [colorCount]Color{Red, Blue} {
			if got, want := b.Wins(color), nb.Wins(color); got != want {
				t.Fatalf("seed %d color %v: Wins = %v, naive %v", seed, color, got, want)
			}
		}
	}
}
