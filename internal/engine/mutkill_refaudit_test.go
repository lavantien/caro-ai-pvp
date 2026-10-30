package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestMutKillEvalUnitWeightDeltaGuards(t *testing.T) {
	w := config.PatternClassWeights
	w[config.PatternClassOpenTwo] = 1
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	place(t, b, rules.Blue, "H10")
	e := NewEval(0, &w)
	e.eval.reset(b)
	side := b.Side
	for _, step := range []struct {
		color rules.Color
		cell  string
	}{
		{rules.Red, "I8"},
		{rules.Blue, "I10"},
	} {
		cell := mustCell(t, step.cell)
		b.Side = step.color
		e.eval.makeMove(step.color, cell)
		b.Make(cell)
		if !e.eval.stateright(b) {
			t.Fatalf("color %d at %s: incremental eval drifted from reset under unit class weights", step.color, step.cell)
		}
		b.Unmake()
		e.eval.unmakeMove(cell)
		if !e.eval.stateright(b) {
			t.Fatalf("color %d at %s: unmake eval drifted from reset under unit class weights", step.color, step.cell)
		}
	}
	b.Side = side
}

func TestMutKillSingleCandidateExclusionSearched(t *testing.T) {
	b := floodBoard(t, "E5")
	e := New(0)
	mv, stats := e.SearchExcluded(b, NewFixedBudget(30*time.Second), rules.Move(mustCell(t, "E5")))
	want := rules.Move(mustCell(t, "E5"))
	if mv != want || stats.Score != 0 {
		t.Errorf("single candidate exclusion: mv=%d score=%d, want %d and the drawn 0", mv, stats.Score, want)
	}
}

func TestMutKillRankResetClearsRanking(t *testing.T) {
	e := New(0)
	e.SearchDepth(midgameBoard(t), NewFixedBudget(30*time.Second), 2)
	e.beginSearch(midgameBoard(t))
	var dst [4]RankedMove
	if got := e.RootRanking(dst[:], 4); len(got) != 0 {
		t.Errorf("ranking after reset returned %d entries, want 0", len(got))
	}
}

func TestMutKillSingleMoveRootRankingContract(t *testing.T) {
	e := New(0)
	e.SearchDepth(floodBoard(t, "E5"), NewFixedBudget(30*time.Second), 2)
	want := rules.Move(mustCell(t, "E5"))
	var dst [4]RankedMove
	top := e.RootRanking(dst[:], 4)
	if len(top) != 1 || top[0].Move != want || top[0].Score != 0 {
		t.Errorf("single move ranking k4 = %v, want exactly [%d 0]", top, want)
	}
	if got := e.RootRanking(dst[:1], 1); len(got) != 1 || got[0].Move != want {
		t.Errorf("single move ranking k1 = %v, want exactly the move %d", got, want)
	}
}

func rankCraft(t *testing.T, moves []rules.Move, scores []int, k int) []RankedMove {
	t.Helper()
	e := New(0)
	copy(e.rankMoves[:], moves)
	copy(e.rankScores[:], scores)
	e.rankN = len(moves)
	var dst [config.SearchMaxMovesPerPly]RankedMove
	return e.RootRanking(dst[:], k)
}

func rankedPairs(top []RankedMove) [][2]int {
	out := make([][2]int, len(top))
	for i, rm := range top {
		out[i] = [2]int{int(rm.Move), rm.Score}
	}
	return out
}

func TestMutKillRootRankingEvictionCraft(t *testing.T) {
	cases := []struct {
		name   string
		moves  []rules.Move
		scores []int
		k      int
		want   [][2]int
	}{
		{"equal score smaller move evicts", []rules.Move{12, 11, 10}, []int{500, 300, 300}, 2, [][2]int{{12, 500}, {10, 300}}},
		{"equal score bigger move skipped", []rules.Move{9, 11, 12}, []int{500, 300, 300}, 2, [][2]int{{9, 500}, {11, 300}}},
		{"better score over full buffer", []rules.Move{12, 10, 11}, []int{500, 200, 300}, 2, [][2]int{{12, 500}, {11, 300}}},
		{"skip does not end the scan", []rules.Move{12, 9, 10, 11}, []int{500, 100, 50, 300}, 2, [][2]int{{12, 500}, {11, 300}}},
		{"equal score inserts after smaller", []rules.Move{9, 5, 7}, []int{300, 300, 300}, 3, [][2]int{{5, 300}, {7, 300}, {9, 300}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			top := rankCraft(t, tc.moves, tc.scores, tc.k)
			if got := rankedPairs(top); len(got) != len(tc.want) {
				t.Fatalf("ranking = %v, want %v", got, tc.want)
			} else {
				for i := range got {
					if got[i] != tc.want[i] {
						t.Fatalf("ranking = %v, want %v", got, tc.want)
					}
				}
			}
		})
	}
}

func TestMutKillNegativeStaticFallsBackClean(t *testing.T) {
	w := config.PatternClassWeights
	w[config.PatternClassOpenTwo] = -1
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8", "I8")
	b.Side = rules.Red
	e := NewEval(0, &w)
	e.resetForSearch(b)
	n := e.generate(b, 0, moveNone)
	if n == 0 {
		t.Fatal("negative-weight opening generated no candidates")
	}
	discouraged := rules.Move(mustCell(t, "G8"))
	for i := range n {
		if e.moves[0][i] == discouraged {
			t.Errorf("generate admitted G8 whose only priced window is the open two at static -1: the minStatic floor must keep it out of every pass")
		}
	}
}

type mutNeverDL struct{}

func (mutNeverDL) Exceeded() bool        { return false }
func (mutNeverDL) Stop()                 {}
func (mutNeverDL) Budget() time.Duration { return time.Nanosecond }

func TestMutKillSMPSoftExcludedStopsAtBudget(t *testing.T) {
	s := newSMP(1, 0)
	defer s.Close()
	b := floodBoard(t, "E5", "F5")
	mv, stats := s.SearchExcluded(b, mutNeverDL{}, moveNone)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("soft excluded move %d illegal", mv)
	}
	if stats.Depth != 1 || stats.RootIters != 1 {
		t.Errorf("soft excluded search depth=%d rootiters=%d, want the soft stop after the first completed iteration at 1 1", stats.Depth, stats.RootIters)
	}
}

func TestMutKillSMPHeadGuardExitsOnFirstExceeded(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(1, 0)
	defer s.Close()
	mv, stats := s.SearchDepth(b, &relapseDL{}, 3)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("relapsed smp search move %d illegal", mv)
	}
	if stats.Depth != 0 || stats.Nodes != 0 || stats.RootIters != 0 {
		t.Errorf("relapsed smp deadline: depth=%d nodes=%d rootiters=%d, want all 0: the head guard must break, not continue", stats.Depth, stats.Nodes, stats.RootIters)
	}
}

func TestMutKillTTDistrustFloorBoundary(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	e.beginSearch(b)
	for _, tc := range []struct {
		cell string
		flip int
	}{
		{"G7", 1},
		{"G8", -1},
	} {
		cell := mustCell(t, tc.cell)
		b.Make(cell)
		h := b.Hash
		b.Unmake()
		e.tt.store(h, tc.flip*config.EvalMateFloor, 0, config.SearchMaxPly, ttBoundExact, 1)
		e.tt.bumpGen()
		sc, _, cut := e.tt.probe(h, 3, -config.EvalMateMax, config.EvalMateMax, 1)
		if cut {
			t.Errorf("%s: stale-generation entry at the %d mate floor cut off, want the distrust miss", tc.cell, tc.flip)
		}
		if sc != 0 {
			t.Errorf("%s: distrust miss score = %d, want 0", tc.cell, sc)
		}
	}
}
