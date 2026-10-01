package vcf

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type absStone struct {
	r, c int
	red  bool
}

func buildAbs(t *testing.T, stones []absStone, side rules.Color) (*rules.Board, opos) {
	t.Helper()
	b := rules.NewBoard()
	var p opos
	p.nb = *rules.NewNaiveBoard()
	for r := range config.BoardSize {
		for c := range config.BoardSize {
			p.region[r*config.BoardStride+c] = true
		}
	}
	for _, s := range stones {
		color := rules.Blue
		if s.red {
			color = rules.Red
		}
		b.Side = color
		b.Make(cellOf(s.r, s.c))
		p.nb.Set(cellOf(s.r, s.c), color)
		cell := s.r*config.BoardStride + s.c
		p.occ[cell] = true
		p.color[cell] = uint8(color)
	}
	b.Side = side
	if p.nb.Wins(rules.Red) || p.nb.Wins(rules.Blue) {
		t.Fatalf("construction already holds a win")
	}
	return b, p
}

// deadFrameFirstCells is the adversarial counterexample root: Blue's broken
// four (6,7),(6,9),(6,10),(6,11) forces Red to answer (6,8), whose reply
// creates the win cell (7,8) completing two exact fives at once, the dead
// horizontal frame with both ends Blue at (7,3) and (7,9), and the live
// vertical frame with near end Blue at (2,8) and open far end (8,8).
func deadFrameFirstCells() []absStone {
	return []absStone{
		{3, 8, true}, {4, 8, true}, {5, 8, true},
		{7, 4, true}, {7, 5, true}, {7, 6, true}, {7, 7, true},
		{8, 9, true}, {8, 10, true}, {8, 11, true},
		{2, 8, false}, {7, 3, false}, {7, 9, false},
		{6, 7, false}, {6, 9, false}, {6, 10, false}, {6, 11, false},
		{0, 0, false}, {15, 15, false}, {0, 15, false},
	}
}

// TestFiveFramesEnumeratesEveryDirection pins the witness enumeration: the
// win cell (7,8) after Red's forced block (6,8) completes exact fives in
// two directions at once, the dead horizontal frame (7,3)-(7,9) and the
// live vertical frame (2,8)-(8,8).
func TestFiveFramesEnumeratesEveryDirection(t *testing.T) {
	stones := append(deadFrameFirstCells(), absStone{6, 8, true})
	b, _ := buildAbs(t, stones, rules.Blue)
	var frames [config.PatternDirections]fiveFrame
	n := fiveFrames(b, rules.Red, cellOf(7, 8), frames[:])
	if n != 2 {
		t.Fatalf("frames = %d, want the dead horizontal and the live vertical", n)
	}
	got := map[[4]int]bool{}
	for i := range n {
		got[[4]int{frames[i].br, frames[i].bc, frames[i].fr, frames[i].fc}] = true
	}
	if !got[[4]int{7, 3, 7, 9}] || !got[[4]int{2, 8, 8, 8}] {
		t.Fatalf("frames = %v, want (7,3)-(7,9) and (2,8)-(8,8)", got)
	}
}

// TestDefusingCoversLiveFrameFarEnd is the white box regression: the
// defusing set of (7,8) must hold both the win cell and the live vertical
// frame's far end (8,8). Sampling only the first exact-five frame returned
// the dead horizontal one and omitted (8,8), Blue's real saving move.
func TestDefusingCoversLiveFrameFarEnd(t *testing.T) {
	stones := append(deadFrameFirstCells(), absStone{6, 8, true})
	b, _ := buildAbs(t, stones, rules.Blue)
	s := New(KindVCF)
	nA, _ := s.probeWins(b, rules.Red, s.winA[:])
	if nA != 1 || s.winA[0] != cellOf(7, 8) {
		t.Fatalf("setup: win cells n=%d first=%v, want exactly (7,8)", nA, s.winA[0])
	}
	n := s.defusing(b, rules.Red, nA, 0)
	var hasWin, hasFar bool
	for i := range n {
		switch s.defends[0][i] {
		case cellOf(7, 8):
			hasWin = true
		case cellOf(8, 8):
			hasFar = true
		}
	}
	if !hasWin || !hasFar {
		t.Fatalf("defusing set of %d cells lacks the win cell or the live far end (8,8)", n)
	}
}

// TestDeadFrameFirstIsNoForcedWin is the end to end soundness regression:
// the full width oracle proves Blue holds within 5 plies (saving with
// (8,8) after the forced Red (6,8)), so neither kind may claim a win.
func TestDeadFrameFirstIsNoForcedWin(t *testing.T) {
	b, p := buildAbs(t, deadFrameFirstCells(), rules.Red)
	for _, kind := range []Kind{KindVCF, KindVCT} {
		soundnessCheck(t, b, p, kind, config.SolverNodeBudget)
		s := New(kind)
		s.maxPly = min(s.maxPly, oracleClaimPlyMax)
		var stats SolverStats
		if s.Solve(b, config.SolverNodeBudget, nil, &stats) {
			var buf [64]byte
			t.Fatalf("kind %d claimed %d plies %q against the proven save",
				kind, stats.Plies, string(stats.AppendPV(buf[:0])))
		}
	}
}
