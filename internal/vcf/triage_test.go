package vcf

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func primeSolver(kind Kind, plyCap int) *Solver {
	s := New(kind)
	s.nodes, s.budget, s.check, s.aborted = 0, config.SolverNodeBudget, config.SolverNodeCheckInterval, false
	s.plyCap = plyCap
	return s
}

func TestBlockFourChainWins(t *testing.T) {
	spec := []placed{
		o(-8, -3), o(-8, -2), o(-8, -1),
		o(-5, -3), o(-5, -2), o(-5, -1),
		o(0, 0),
		x(-4, 0), x(-3, 0), x(-2, 0), x(-1, 0), x(-5, 1),
	}
	b := buildAt(t, false, 10, 5, rules.Red, spec)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 5 {
		t.Fatalf("block four chain: plies=%d want 5", stats.Plies)
	}
}

func TestCornerThreatMoveWins(t *testing.T) {
	stones := []absStone{
		{0, 1, true}, {0, 2, true}, {0, 3, true},
		{1, 1, true}, {2, 2, true}, {3, 3, true},
		{0, 5, false}, {5, 5, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(0, 0, testGeom)) {
		t.Fatalf("corner threat move: plies=%d pv0=%d want the corner cell", stats.Plies, stats.PV[0])
	}
}

func TestCornerForcedBlockWins(t *testing.T) {
	stones := []absStone{
		{0, 1, false}, {0, 2, false}, {0, 3, false}, {0, 4, false}, {0, 5, true},
		{1, 1, true}, {2, 2, true}, {3, 3, true},
		{1, 0, true}, {2, 0, true}, {3, 0, true},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(0, 0, testGeom)) {
		t.Fatalf("corner forced block: plies=%d pv0=%d want the corner cell", stats.Plies, stats.PV[0])
	}
}

func TestForcedBlockCapWins(t *testing.T) {
	spec := []placed{
		o(0, 0), o(-3, 6), o(-2, 6), o(-1, 6),
		x(0, 1), x(0, 2), x(0, 3), x(0, 4),
	}
	b := buildAt(t, false, 8, 4, rules.Red, spec)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(8, 10, testGeom)) {
		t.Fatalf("forced block cap: plies=%d pv0=%d want the cap cell", stats.Plies, stats.PV[0])
	}
}

func TestForcedBlockDeadFrameOccupiedEnd(t *testing.T) {
	stones := []absStone{
		{4, 8, true}, {10, 8, true}, {10, 4, true}, {10, 5, true}, {10, 6, true},
		{5, 8, false}, {6, 8, false}, {7, 8, false}, {9, 8, false},
		{8, 5, false}, {8, 6, false}, {8, 7, false}, {8, 9, false},
		{10, 3, false}, {10, 9, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
}

func TestDefendFirstReplySaves(t *testing.T) {
	stones := []absStone{{0, 1, true}, {0, 2, true}, {0, 3, true}, {0, 4, true}, {0, 5, false}}
	b, _ := buildAbs(t, stones, rules.Blue)
	s := primeSolver(KindVCF, 9)
	if r := s.defend(b, cellOf(0, 4, testGeom), 1); r != resFail {
		t.Fatalf("defend first reply: result %d want %d", r, resFail)
	}
}

func TestDefendSingleDefenderWinRefutes(t *testing.T) {
	stones := []absStone{
		{8, 4, true}, {8, 5, true}, {8, 6, true}, {8, 7, true}, {2, 9, true},
		{2, 10, false}, {2, 11, false}, {2, 12, false}, {2, 13, false},
	}
	b, _ := buildAbs(t, stones, rules.Blue)
	s := primeSolver(KindVCF, 9)
	if r := s.defend(b, cellOf(8, 7, testGeom), 1); r != resFail {
		t.Fatalf("defend single defender win: result %d want %d", r, resFail)
	}
}

func TestDefendBrokenThreeWindowReplySaves(t *testing.T) {
	stones := []absStone{
		{1, 8, true}, {2, 8, true}, {3, 8, true},
		{5, 8, false},
	}
	b, _ := buildAbs(t, stones, rules.Blue)
	s := primeSolver(KindVCT, 9)
	if r := s.defend(b, cellOf(3, 8, testGeom), 1); r != resFail {
		t.Fatalf("defend broken three window reply: result %d want %d", r, resFail)
	}
}

func TestForcedBlockFourFramesFillCandidates(t *testing.T) {
	spec := []placed{
		x(0, -4), x(0, -3), x(0, -2), x(0, -1),
		x(-4, 0), x(-3, 0), x(-2, 0), x(-1, 0),
		x(-4, -4), x(-3, -3), x(-2, -2), x(-1, -1),
		x(-4, 4), x(-3, 3), x(-2, 2), x(-1, 1),
		o(0, -5), o(-5, 0), o(-5, -5), o(-5, 5),
	}
	b := buildAt(t, false, 8, 8, rules.Red, spec)
	s := primeSolver(KindVCF, 9)
	if r := s.forcedBlock(b, rules.Red, cellOf(8, 8, testGeom), 0); r != resFail {
		t.Fatalf("forced block four frames: result %d want %d", r, resFail)
	}
}

func TestPickBestPrefersHigherScore(t *testing.T) {
	stones := []absStone{
		{4, 1, true}, {4, 2, true}, {4, 3, true}, {1, 4, true}, {2, 4, true}, {3, 4, true},
		{4, 0, false}, {0, 4, false},
		{12, 9, true}, {12, 10, true}, {12, 11, true},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(12, 8, testGeom)) {
		t.Fatalf("pick best higher score: plies=%d pv0=%d want the open four cell", stats.Plies, stats.PV[0])
	}
}

func TestPickBestTieKeepsFirst(t *testing.T) {
	stones := []absStone{
		{4, 1, true}, {4, 2, true}, {4, 3, true}, {1, 4, true}, {2, 4, true}, {3, 4, true},
		{4, 0, false}, {0, 4, false},
		{12, 9, true}, {12, 10, true}, {12, 11, true}, {9, 12, true}, {10, 12, true}, {11, 12, true},
		{12, 8, false}, {8, 12, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(4, 4, testGeom)) {
		t.Fatalf("pick best tie: plies=%d pv0=%d want the first tied cell", stats.Plies, stats.PV[0])
	}
}

func TestAppendPVCapsAtPVLen(t *testing.T) {
	var stats SolverStats
	stats.Plies = config.SolverMaxPly + 1
	var want []byte
	for i := range stats.PV {
		stats.PV[i] = rules.Move(cellOf(i/config.BoardSize, i%config.BoardSize, testGeom))
		if i > 0 {
			want = append(want, ' ')
		}
		want = rules.AppendCellName(want, rules.Cell(stats.PV[i]))
	}
	got := stats.AppendPV(make([]byte, 0, 4*config.SolverMaxPly))
	if string(got) != string(want) {
		t.Fatalf("append pv cap: got %d bytes want %d bytes", len(got), len(want))
	}
}

func TestWinInOneNodeCount(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specWinIn1)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Nodes != 1 {
		t.Fatalf("win in one node count: nodes=%d want 1", stats.Nodes)
	}
}

func TestBudgetBoundaryPastShallowRounds(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	shallow := New(KindVCF)
	shallow.maxPly = 5
	var shallowStats SolverStats
	shallow.Solve(b, config.SolverNodeBudget, nil, &shallowStats)
	mustNotFound(t, b, KindVCF, int(shallowStats.Nodes)+1)
}

func TestBudgetBoundaryLastNode(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	full := New(KindVCF)
	var fullStats SolverStats
	full.Solve(b, config.SolverNodeBudget, nil, &fullStats)
	mustNotFound(t, b, KindVCF, int(fullStats.Nodes)-1)
}

func TestDeadlineFiresAtFirstNode(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specWinIn1)
	s := New(KindVCF)
	s.nodes, s.budget, s.check, s.aborted = 0, config.SolverNodeBudget, 1, false
	s.plyCap = config.SolverMaxPly
	s.deadline = &countdownDeadline{after: 0}
	if r := s.attack(b, 0); r != resAbort {
		t.Fatalf("deadline at first node: result %d want %d", r, resAbort)
	}
}

func TestDeadlineQuietBeforeFirstPoll(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specWinIn1)
	s := New(KindVCF)
	s.nodes, s.budget, s.check, s.aborted = 0, config.SolverNodeBudget, 2, false
	s.plyCap = config.SolverMaxPly
	s.deadline = &countdownDeadline{after: 0}
	if r := s.attack(b, 0); r != resWin {
		t.Fatalf("deadline before first poll: result %d want %d", r, resWin)
	}
}

func TestOkRefusesAfterAbort(t *testing.T) {
	s := New(KindVCF)
	s.aborted = true
	if s.ok() {
		t.Fatal("ok: must refuse every node once aborted")
	}
}

func TestPickBestSecondIndexMax(t *testing.T) {
	stones := []absStone{
		{1, 4, true}, {2, 4, true}, {3, 4, true},
		{4, 6, true}, {4, 7, true}, {4, 8, true},
		{1, 1, true}, {2, 2, true}, {3, 3, true},
		{0, 4, false}, {5, 5, false}, {0, 0, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 3 || stats.PV[0] != rules.Move(cellOf(4, 5, testGeom)) {
		t.Fatalf("pick best second index: plies=%d pv0=%d want the first open four cell", stats.Plies, stats.PV[0])
	}
}

func TestAddCellDedups(t *testing.T) {
	var buf [4]rules.Cell
	buf[0] = cellOf(8, 4, testGeom)
	n := addCell(buf[:], 1, cellOf(8, 4, testGeom))
	if n != 1 || buf[1] != 0 {
		t.Fatalf("add cell dedup: n=%d buf1=%v want the duplicate refused", n, buf[1])
	}
}

func TestWarmMemoRootRoundResearched(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specTwoDeadFours)
	s := New(KindVCF)
	var first SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &first) {
		t.Fatalf("warm memo: two dead fours must stay unfound")
	}
	s.maxPly = 3
	var second SolverStats
	s.Solve(b, config.SolverNodeBudget, nil, &second)
	if second.Nodes < 4 {
		t.Fatalf("warm memo root round: nodes=%d want a real re-search", second.Nodes)
	}
}

func TestThreatScoreCornerDoubleFour(t *testing.T) {
	stones := []absStone{
		{0, 1, true}, {0, 2, true}, {0, 3, true},
		{1, 1, true}, {2, 2, true}, {3, 3, true},
		{0, 5, false}, {5, 5, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	s := New(KindVCF)
	reach := dirReach(b, rules.Red)
	if r := threatRank(b, cellOf(0, 0, testGeom), rules.Red, &reach); r != config.PatternClassFour {
		t.Fatalf("threat rank corner: %d want %d", r, config.PatternClassFour)
	}
	want := uint8(config.PatternClassFour)<<2 | 2
	if score := s.threatScore(b, cellOf(0, 0, testGeom), rules.Red, &reach); score != want {
		t.Fatalf("threat score corner: %d want %d", score, want)
	}
}

func TestThreatScoreFourFourPoint(t *testing.T) {
	stones := []absStone{
		{8, 5, true}, {8, 6, true}, {8, 7, true},
		{5, 8, true}, {6, 8, true}, {7, 8, true},
		{5, 5, true}, {6, 6, true}, {7, 7, true},
		{5, 11, true}, {6, 10, true}, {7, 9, true},
		{8, 4, false}, {4, 8, false}, {4, 4, false}, {4, 12, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	s := New(KindVCF)
	reach := dirReach(b, rules.Red)
	want := uint8(config.PatternClassFour)<<2 | 3
	if score := s.threatScore(b, cellOf(8, 8, testGeom), rules.Red, &reach); score != want {
		t.Fatalf("threat score four four: %d want %d", score, want)
	}
}

func boundsBoard(t *testing.T) *rules.Board {
	t.Helper()
	stones := []absStone{
		{0, 5, true}, {1, 0, true}, {3, 0, true}, {4, 15, true}, {5, 0, true}, {6, 0, true},
		{2, 3, false},
	}
	b, _ := buildAbs(t, stones, rules.Red)
	return b
}

func TestCellAtBounds(t *testing.T) {
	b := boundsBoard(t)
	cases := []struct {
		r, c  int
		color rules.Color
		want  bool
	}{
		{-1, 0, rules.Red, false},
		{0, 5, rules.Red, true},
		{0, 5, rules.Blue, false},
		{2, 3, rules.Blue, true},
		{5, 16, rules.Red, false},
		{5, -1, rules.Red, false},
		{16, 5, rules.Red, false},
		{-2, 3, rules.Blue, false},
	}
	for _, tc := range cases {
		if got := cellAt(b, tc.r, tc.c, tc.color); got != tc.want {
			t.Fatalf("cellAt(%d,%d,%v)=%v want %v", tc.r, tc.c, tc.color, got, tc.want)
		}
	}
}

func TestPlayableBounds(t *testing.T) {
	b := boundsBoard(t)
	cases := []struct {
		r, c int
		want bool
	}{
		{-1, 5, false},
		{0, 0, true},
		{0, 5, false},
		{2, 3, false},
		{3, -1, false},
		{7, 16, false},
		{16, 0, false},
	}
	for _, tc := range cases {
		if got := playable(b, tc.r, tc.c); got != tc.want {
			t.Fatalf("playable(%d,%d)=%v want %v", tc.r, tc.c, got, tc.want)
		}
	}
}

func TestFiveFramesSplitRun(t *testing.T) {
	stones := []absStone{{8, 6, true}, {8, 7, true}, {8, 9, true}, {8, 10, true}}
	b, _ := buildAbs(t, stones, rules.Red)
	var frames [config.PatternDirections]fiveFrame
	n := fiveFrames(b, rules.Red, cellOf(8, 8, testGeom), frames[:])
	if n != 1 || frames[0] != (fiveFrame{8, 5, 8, 11}) {
		t.Fatalf("five frames split run: n=%d frame=%+v want the (8,5)-(8,11) frame", n, frames[0])
	}
}

func TestDefusingCoversHorizontalFrameFarEnd(t *testing.T) {
	stones := []absStone{{7, 4, true}, {7, 5, true}, {7, 6, true}, {7, 7, true}, {7, 3, false}}
	b, _ := buildAbs(t, stones, rules.Red)
	s := New(KindVCF)
	s.winA[0] = cellOf(7, 8, testGeom)
	n := s.defusing(b, rules.Red, 1, 0)
	if n != 2 || s.defends[0][0] != cellOf(7, 8, testGeom) || s.defends[0][1] != cellOf(7, 9, testGeom) {
		t.Fatalf("defusing horizontal frame: n=%d defends=%v want the win cell and the far end", n, s.defends[0][:n])
	}
}

func TestDefendFortyEightWinCells(t *testing.T) {
	b, _ := buildAbs(t, []absStone{{14, 14, false}}, rules.Blue)
	for r := 2; r <= 5; r++ {
		for c := 2; c <= 5; c++ {
			b.Side = rules.Red
			b.Make(cellOf(r, c, testGeom))
		}
	}
	for r := 2; r <= 5; r++ {
		for c := 8; c <= 11; c++ {
			b.Side = rules.Red
			b.Make(cellOf(r, c, testGeom))
		}
	}
	for _, r := range [...]int{8, 10} {
		for _, c := range [...]int{2, 3, 4, 5, 8, 9, 10, 11} {
			b.Side = rules.Red
			b.Make(cellOf(r, c, testGeom))
		}
	}
	b.Side = rules.Blue
	s := primeSolver(KindVCF, config.SolverMaxPly)
	nA, _ := s.probeWins(b, rules.Red, s.winA[:])
	if nA != maxWinCells {
		t.Fatalf("forty eight win cells: n=%d want %d", nA, maxWinCells)
	}
	if r := s.defend(b, cellOf(2, 2, testGeom), 1); r != resWin {
		t.Fatalf("forty eight win cells: defend result %d want %d", r, resWin)
	}
}

func TestRealThreeRightSideProof(t *testing.T) {
	stones := []absStone{{8, 4, true}, {8, 5, true}, {8, 6, true}, {8, 3, false}}
	b, _ := buildAbs(t, stones, rules.Red)
	if !realThree(b, rules.Red, cellOf(8, 6, testGeom)) {
		t.Fatal("real three right side: the closed three must stay real")
	}
}

func TestRealThreeFourIsNotThree(t *testing.T) {
	stones := []absStone{{8, 3, true}, {8, 4, true}, {8, 5, true}, {8, 6, true}, {8, 2, false}}
	b, _ := buildAbs(t, stones, rules.Red)
	if realThree(b, rules.Red, cellOf(8, 6, testGeom)) {
		t.Fatal("real three four: a blocked four is not a three")
	}
}

func TestSquareDirReachTopWord(t *testing.T) {
	coords := [][2]int{{14, 5}, {15, 10}, {12, 12}, {13, 1}}
	b := rules.NewBoard()
	for _, rc := range coords {
		b.Side = rules.Red
		b.Make(cellOf(rc[0], rc[1], testGeom))
	}
	b.Side = rules.Red
	reach := dirReach(b, rules.Red)
	for d := range testGeom.Dirs {
		dr, dc := testGeom.Dirs[d].Dr, testGeom.Dirs[d].Dc
		want := make(map[rules.Cell]bool)
		for _, rc := range coords {
			r, c := rc[0], rc[1]
			want[rules.Cell(r*config.BoardStride+c)] = true
			for k := 1; k <= windowHalf; k++ {
				for _, sign := range [2]int{k, -k} {
					rr, cc := r+sign*dr, c+sign*dc
					if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
						continue
					}
					want[rules.Cell(rr*config.BoardStride+cc)] = true
				}
			}
		}
		for cell := 0; cell < config.BoardCells; cell++ {
			m := uint64(1) << (uint(cell) % wordBits)
			got := reach[d][cell/wordBits]&m != 0
			if got != want[rules.Cell(cell)] {
				t.Fatalf("dir %d cell %d: reach %v want %v", d, cell, got, want[rules.Cell(cell)])
			}
		}
	}
}

func TestSquareDilateMatchesNeighbors(t *testing.T) {
	coords := [][2]int{{0, 0}, {3, 15}, {4, 0}, {7, 15}, {8, 0}, {11, 15}, {12, 0}, {15, 15}, {8, 8}}
	b := rules.NewBoard()
	for _, rc := range coords {
		b.Side = rules.Red
		b.Make(cellOf(rc[0], rc[1], testGeom))
	}
	b.Side = rules.Red
	got := dilate(stones(b, rules.Red), 1, testGeom)
	want := make(map[rules.Cell]bool)
	for _, rc := range coords {
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				rr, cc := rc[0]+dr, rc[1]+dc
				if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
					continue
				}
				want[rules.Cell(rr*config.BoardStride+cc)] = true
			}
		}
	}
	for cell := 0; cell < config.BoardCells; cell++ {
		m := uint64(1) << (uint(cell) % wordBits)
		has := got[cell/wordBits]&m != 0
		if has != want[rules.Cell(cell)] {
			t.Fatalf("cell %d: dilated %v want %v", cell, has, want[rules.Cell(cell)])
		}
	}
}

func hexDilateBoard(t *testing.T, coords [][2]int) (map[rules.Cell]bool, map[rules.Cell]bool) {
	t.Helper()
	g := hexGeom(t)
	b := rules.NewBoardFor(g)
	for _, rc := range coords {
		b.Side = rules.Red
		b.Make(rules.Cell(rc[0]*g.Stride + rc[1]))
	}
	b.Side = rules.Red
	got := dilate(stones(b, rules.Red), 1, g)
	want := make(map[rules.Cell]bool)
	gotCells := make(map[rules.Cell]bool)
	for _, rc := range coords {
		want[rules.Cell(rc[0]*g.Stride+rc[1])] = true
		for _, n := range g.Neighbors {
			qq, rr := rc[0]+n[0], rc[1]+n[1]
			if qq < 0 || qq >= g.Rows || rr < 0 || rr >= g.Cols {
				continue
			}
			cell := qq*g.Stride + rr
			if g.Contains(cell) {
				want[rules.Cell(cell)] = true
			}
		}
	}
	for cell := 0; cell < g.Cells; cell++ {
		m := uint64(1) << (uint(cell) % wordBits)
		if got[cell/wordBits]&m != 0 && g.Contains(cell) {
			gotCells[rules.Cell(cell)] = true
		}
	}
	return gotCells, want
}

func TestHexDilateRimWrapGuards(t *testing.T) {
	west := [][2]int{{11, 0}, {1, 17}, {2, 17}, {8, 8}, {5, 21}, {21, 10}, {22, 6}, {22, 11}}
	got, want := hexDilateBoard(t, west)
	for cell := range want {
		if !got[cell] {
			t.Fatalf("west rim cell %d: missing from dilate", cell)
		}
	}
	for cell := range got {
		if !want[cell] {
			t.Fatalf("west rim cell %d: wrap artifact in dilate", cell)
		}
	}
	east := [][2]int{{11, 22}, {1, 17}, {8, 8}, {5, 21}, {21, 10}, {22, 6}}
	got, want = hexDilateBoard(t, east)
	for cell := range want {
		if !got[cell] {
			t.Fatalf("east rim cell %d: missing from dilate", cell)
		}
	}
	for cell := range got {
		if !want[cell] {
			t.Fatalf("east rim cell %d: wrap artifact in dilate", cell)
		}
	}
}
