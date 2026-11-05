package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func redWinIn1Board(t testing.TB) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8", "H9", "H10", "H11")
	place(t, b, rules.Blue, "H7", "P16")
	return b
}

func TestRootRankingWinIn1First(t *testing.T) {
	b := redWinIn1Board(t)
	if b.Side != rules.Red {
		t.Fatalf("side to move = %d, want red", b.Side)
	}
	e := New(testTTBytes)
	mv, _ := e.SearchDepth(b, NewFixedBudget(30*time.Second), 2)
	if want := rules.Move(mustCell(t, "H12")); mv != want {
		t.Fatalf("best move = %v, want the win in 1 %v", mv, want)
	}
	var dst [config.SearchMaxMovesPerPly]RankedMove
	top := e.RootRanking(dst[:], 4)
	if len(top) < 2 {
		t.Fatalf("ranking length = %d, want at least 2 ranked candidates", len(top))
	}
	if want := rules.Move(mustCell(t, "H12")); top[0].Move != want {
		t.Fatalf("rank 1 = %v, want %v", top[0].Move, want)
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; top[0].Score != want {
		t.Fatalf("rank 1 score = %d, want mate %d", top[0].Score, want)
	}
	if top[1].Score >= config.EvalMateMax/2 {
		t.Fatalf("rank 2 score %d sits inside the mate band", top[1].Score)
	}
	for i := 1; i < len(top); i++ {
		if top[i-1].Score < top[i].Score {
			t.Fatalf("ranking not monotone at %d: %d then %d", i, top[i-1].Score, top[i].Score)
		}
	}
}

func TestRootRankingTieBreakByCell(t *testing.T) {
	e := New(0)
	e.rankMoves[0], e.rankMoves[1] = rules.Move(119), rules.Move(118)
	e.rankScores[0], e.rankScores[1] = 500, 500
	e.rankN = 2
	var dst [4]RankedMove
	top := e.RootRanking(dst[:], 2)
	if len(top) != 2 {
		t.Fatalf("ranking length = %d, want 2", len(top))
	}
	if top[0].Move != 118 || top[1].Move != 119 {
		t.Fatalf("tie order = %v %v, want smaller cell 118 first", top[0].Move, top[1].Move)
	}
}

func TestRootRankingClampsAndEmpty(t *testing.T) {
	e := New(0)
	var dst [4]RankedMove
	if got := e.RootRanking(dst[:], 4); len(got) != 0 {
		t.Fatalf("fresh engine ranking length = %d, want 0", len(got))
	}
	b := midgameBoard(t)
	e2 := New(testTTBytes)
	_, _ = e2.SearchDepth(b, NewFixedBudget(30*time.Second), 3)
	if got := e2.RootRanking(dst[:], 0); len(got) != 0 {
		t.Fatalf("k 0 ranking length = %d, want 0", len(got))
	}
	if got := e2.RootRanking(dst[:], 2); len(got) != 2 {
		t.Fatalf("k 2 ranking length = %d, want 2", len(got))
	}
	if got := e2.RootRanking(dst[:], config.SearchMaxMovesPerPly+8); len(got) > len(dst) {
		t.Fatalf("k beyond buffer returned %d entries", len(got))
	}
}

func TestRootRankingHoldsLastCompletedIteration(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewNodeBudget(2048), 8)
	if stats.Depth < 1 {
		t.Fatal("no iteration completed before the trip")
	}
	var dst [config.SearchMaxMovesPerPly]RankedMove
	top := e.RootRanking(dst[:], 1)
	if len(top) != 1 {
		t.Fatalf("ranking length = %d, want 1", len(top))
	}
	if top[0].Move != mv {
		t.Fatalf("ranking top %v differs from returned move %v", top[0].Move, mv)
	}
	if top[0].Score != stats.Score {
		t.Fatalf("ranking top score %d differs from returned score %d", top[0].Score, stats.Score)
	}
}

func TestRootMovesRingUnchanged(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(30*time.Second), 3)
	if stats.RootIters != 3 {
		t.Fatalf("root iters = %d, want 3", stats.RootIters)
	}
	if stats.RootMoves[2] != mv {
		t.Fatalf("ring last = %v, returned %v", stats.RootMoves[2], mv)
	}
	if stats.RootScores[2] != stats.Score {
		t.Fatalf("ring last score = %d, returned %d", stats.RootScores[2], stats.Score)
	}
}

func TestFixedBudgetSearchUnchanged(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, NewFixedBudget(30*time.Second), 4)
	if stats.Depth != 4 {
		t.Fatalf("depth = %d, want 4", stats.Depth)
	}
	if got, want := int(mv), 36; got != want {
		t.Fatalf("pinned move = %d, want %d", got, want)
	}
	if got, want := stats.Score, 5050; got != want {
		t.Fatalf("pinned score = %d, want %d", got, want)
	}
	if got, want := stats.Nodes, uint64(1061); got != want {
		t.Fatalf("pinned nodes = %d, want %d", got, want)
	}
}

func TestZeroAllocsRootRanking(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	_, _ = e.SearchDepth(b, NewFixedBudget(time.Second), 2)
	var dst [config.SearchMaxMovesPerPly]RankedMove
	if n := testing.AllocsPerRun(1000, func() { _ = e.RootRanking(dst[:], 4) }); n != 0 {
		t.Fatalf("RootRanking: %v allocs, want 0", n)
	}
}

func sparseContestBoard(t testing.TB) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8", "I9", "J10")
	place(t, b, rules.Blue, "H9", "I8", "K11")
	return b
}

func exactRanked(t *testing.T, b *rules.Board, depth int, bare bool) (rules.Move, int, []RankedMove) {
	t.Helper()
	e := New(testTTBytes)
	defer e.Close()
	e.SetExactRootRanking(true)
	if bare {
		e.noOrder = true
	}
	mv, st := e.SearchDepth(b, NewFixedBudget(30*time.Second), depth)
	var dst [config.SearchMaxMovesPerPly]RankedMove
	return mv, st.Score, e.RootRanking(dst[:], 6)
}

func TestExactRootRankingIsOrderIndependent(t *testing.T) {
	cases := []struct {
		name  string
		board func(testing.TB) *rules.Board
		depth int
	}{
		{"midgame depth 3", midgameBoard, 3},
		{"midgame depth 4", midgameBoard, 4},
		{"win in 1 depth 2", redWinIn1Board, 2},
		{"sparse contest depth 3", sparseContestBoard, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.board(t)
			mvA, scA, topA := exactRanked(t, b, tc.depth, false)
			mvB, scB, topB := exactRanked(t, b, tc.depth, true)
			if mvA != mvB || scA != scB {
				t.Fatalf("best move or score depends on ordering: (%v %d) versus (%v %d)", mvA, scA, mvB, scB)
			}
			if len(topA) != len(topB) {
				t.Fatalf("ranking lengths differ: %d versus %d", len(topA), len(topB))
			}
			for i := range topA {
				if topA[i] != topB[i] {
					t.Fatalf("rank %d differs under no-order: %+v versus %+v, a bound leaked into exact ranking", i, topA[i], topB[i])
				}
			}
		})
	}
}

func TestDefaultRankingTailCarriesNullWindowBounds(t *testing.T) {
	b := sparseContestBoard(t)
	_, _, exact := exactRanked(t, b, 3, false)
	e := New(testTTBytes)
	defer e.Close()
	_, st := e.SearchDepth(b, NewFixedBudget(30*time.Second), 3)
	if st.Depth != 3 {
		t.Fatalf("depth = %d, want 3", st.Depth)
	}
	var dst [config.SearchMaxMovesPerPly]RankedMove
	def := e.RootRanking(dst[:], 6)
	exactScore := make(map[rules.Move]int, len(exact))
	for _, rm := range exact {
		exactScore[rm.Move] = rm.Score
	}
	bounded := 0
	for _, rm := range def {
		if want, ok := exactScore[rm.Move]; ok && rm.Score != want {
			bounded++
		}
	}
	if bounded == 0 {
		t.Fatal("default PVS ranking carries no bound on this fixture, the demonstration lost its power")
	}
	if def[0].Score != exact[0].Score {
		t.Fatalf("rank 1 score differs between default %d and exact %d", def[0].Score, exact[0].Score)
	}
}

func TestExactRankingFlagLeavesDefaultPathPinned(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	defer e.Close()
	mv, stats := e.SearchDepth(b, NewFixedBudget(30*time.Second), 4)
	if got, want := int(mv), 36; got != want {
		t.Fatalf("pinned move = %d, want %d", got, want)
	}
	if got, want := stats.Score, 5050; got != want {
		t.Fatalf("pinned score = %d, want %d", got, want)
	}
	if got, want := stats.Nodes, uint64(1061); got != want {
		t.Fatalf("pinned nodes = %d, want %d", got, want)
	}
}
