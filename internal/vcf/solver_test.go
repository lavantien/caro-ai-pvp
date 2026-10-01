package vcf

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type placed struct {
	dr, dc int
	red    bool
}

func o(dr, dc int) placed { return placed{dr, dc, true} }
func x(dr, dc int) placed { return placed{dr, dc, false} }

// Construction specs are relative to an anchor so the same shape runs on
// the full board and inside the 8x8 cross-check region.

var (
	specWinIn1    = []placed{o(0, 1), o(0, 2), o(0, 3), o(0, 4)}
	specOpenFour  = []placed{o(0, 2), o(0, 3), o(0, 4)}
	specFourChain = []placed{
		o(0, 1), o(0, 2), o(0, 3), x(0, 0), x(-1, 4),
		o(2, 4), o(3, 4),
		o(1, 6), o(1, 7),
	}
	specDoubleFour = []placed{
		o(0, 1), o(0, 2), o(0, 3), x(0, 0),
		o(-3, 4), o(-2, 4), o(-1, 4), x(-4, 4),
	}
	specCross34 = []placed{
		o(0, 1), o(0, 2), o(0, 3), x(0, 0),
		o(-2, 4), o(-1, 4),
	}
	specOverlineDecline = []placed{o(0, 1), o(0, 2), o(0, 3), o(0, 4), o(0, 6), x(3, 1)}
	specOverlineTrap    = []placed{o(0, 1), o(0, 2), o(0, 3), o(0, 4), o(0, 6), x(0, -1), x(0, 5)}
	specCounterFour     = []placed{
		o(0, 3), o(0, 4), o(0, 5), x(0, 6),
		x(-3, 1), x(-2, 1), x(-1, 1),
	}
	specFarBlock = []placed{
		x(2, 1), x(2, 2), x(2, 3), x(2, 5),
		o(2, 0), o(2, 6), o(2, 7), o(2, 8),
	}
	specVCTDoubleThree = []placed{
		o(0, -2), o(0, -1),
		o(-2, 0), o(-1, 0),
		x(3, -5), x(3, -4),
	}
	specVCTCounterFour = []placed{
		o(0, -2), o(0, -1),
		o(-2, 0), o(-1, 0),
		x(3, -5), x(3, -4), x(3, -3),
	}
	specPhantomThree = []placed{o(0, 2), o(0, 3), x(0, 0), x(0, 6)}
	specQuiet        = []placed{o(0, 0), o(3, 3), x(1, 2), x(4, 5)}
	specTwoDeadFours = []placed{
		o(0, 1), o(0, 2), o(0, 3), x(0, 0),
		o(3, 1), o(3, 2), o(3, 3), x(3, 0),
	}
)

func buildAt(t testing.TB, cross bool, ar, ac int, side rules.Color, spec []placed) *rules.Board {
	t.Helper()
	var b *rules.Board
	if cross {
		b = rules.NewCrossCheck()
	} else {
		b = rules.NewBoard()
	}
	for _, p := range spec {
		b.Side = rules.Red
		if !p.red {
			b.Side = rules.Blue
		}
		b.Make(cellOf(ar+p.dr, ac+p.dc))
	}
	b.Side = side
	return b
}

// verifyForcedPV replays a claimed line on a fresh copy of the board: every
// entry legal, no intermediate win, the final attacker placement a verified
// win through rules.FastLastMoveWin.
func verifyForcedPV(t testing.TB, b *rules.Board, stats SolverStats) {
	t.Helper()
	if !stats.Found || stats.Plies%2 == 0 || stats.Plies == 0 {
		t.Fatalf("pv check: bad claim found=%v plies=%d", stats.Found, stats.Plies)
	}
	clone := *b
	for i := 0; i < stats.Plies; i++ {
		c := rules.Cell(stats.PV[i])
		if !clone.IsLegal(c) {
			t.Fatalf("pv check: illegal entry %d cell %d", i, c)
		}
		side := clone.Side
		clone.Make(c)
		won := clone.FastLastMoveWin(side, c)
		if i == stats.Plies-1 {
			if !won {
				t.Fatalf("pv check: final entry %d does not win", c)
			}
		} else if won {
			t.Fatalf("pv check: intermediate win at entry %d", i)
		}
	}
}

type stoppedDeadline struct {
	stopped bool
}

func (d *stoppedDeadline) Exceeded() bool { return d.stopped }
func (d *stoppedDeadline) Stop()          { d.stopped = true }

type countdownDeadline struct{ after int }

func (d *countdownDeadline) Exceeded() bool {
	d.after--
	return d.after < 0
}
func (d *countdownDeadline) Stop() {}

func mustFound(t *testing.T, b *rules.Board, kind Kind, budget int) SolverStats {
	t.Helper()
	s := New(kind)
	var stats SolverStats
	if !s.Solve(b, budget, nil, &stats) {
		t.Fatalf("kind %d: expected forced win", kind)
	}
	verifyForcedPV(t, b, stats)
	return stats
}

func mustNotFound(t *testing.T, b *rules.Board, kind Kind, budget int) {
	t.Helper()
	s := New(kind)
	var stats SolverStats
	if s.Solve(b, budget, nil, &stats) {
		t.Fatalf("kind %d: expected not found, got a claim at plies=%d", kind, stats.Plies)
	}
	if stats.Plies != 0 || stats.PV[0] != 0 {
		t.Fatalf("kind %d: not-found solve leaked pv plies=%d", kind, stats.Plies)
	}
}

func TestWinInOne(t *testing.T) {
	for _, tc := range []struct {
		cross    bool
		ar, ac   int
		expected rules.Cell
	}{
		{false, 8, 4, cellOf(8, 4)},
		{true, 1, 1, cellOf(1, 1)},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specWinIn1)
		stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
		if stats.Plies != 1 || stats.PV[0] != rules.Move(tc.expected) {
			t.Fatalf("win in 1: plies=%d pv0=%d want pv0=%d", stats.Plies, stats.PV[0], tc.expected)
		}
		mustFound(t, b, KindVCT, config.SolverNodeBudget)
		var buf [16]byte
		got := stats.AppendPV(buf[:0])
		name, err := rules.CellName(tc.expected)
		if err != nil {
			t.Fatalf("cell name: %v", err)
		}
		if string(got) != name {
			t.Fatalf("append pv: got %q want %q", got, name)
		}
	}
}

func TestOpenFourWin(t *testing.T) {
	for _, tc := range []struct {
		cross  bool
		ar, ac int
	}{
		{false, 8, 4},
		{true, 1, 1},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specOpenFour)
		stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
		if stats.Plies != 3 {
			t.Fatalf("open four: plies=%d want 3", stats.Plies)
		}
		mustFound(t, b, KindVCT, config.SolverNodeBudget)
	}
}

func TestFourChain(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 7 {
		t.Fatalf("four chain: plies=%d want 7", stats.Plies)
	}
	mustFound(t, b, KindVCT, config.SolverNodeBudget)
}

func TestDoubleFourCross(t *testing.T) {
	for _, tc := range []struct {
		cross  bool
		ar, ac int
	}{
		{false, 8, 4},
		{true, 4, 2},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specDoubleFour)
		stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
		if stats.Plies != 3 {
			t.Fatalf("double four: plies=%d want 3", stats.Plies)
		}
	}
}

func TestCrossThreeFour(t *testing.T) {
	for _, tc := range []struct {
		cross  bool
		ar, ac int
	}{
		{false, 8, 4},
		{true, 4, 2},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specCross34)
		stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
		if stats.Plies != 5 {
			t.Fatalf("cross 3-4: plies=%d want 5", stats.Plies)
		}
		mustFound(t, b, KindVCT, config.SolverNodeBudget)
	}
}

func TestOverlineDecline(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specOverlineDecline)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 1 || stats.PV[0] != rules.Move(cellOf(8, 4)) {
		t.Fatalf("overline decline: plies=%d pv0=%d want the near win cell", stats.Plies, stats.PV[0])
	}
	mustNotFound(t, buildAt(t, false, 8, 5, rules.Red, specOverlineTrap), KindVCF, config.SolverNodeBudget)
	mustNotFound(t, buildAt(t, false, 8, 5, rules.Red, specOverlineTrap), KindVCT, config.SolverNodeBudget)
}

func TestCounterFourRefutes(t *testing.T) {
	for _, tc := range []struct {
		cross  bool
		ar, ac int
	}{
		{false, 8, 4},
		{true, 4, 1},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specCounterFour)
		mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
		mustNotFound(t, b, KindVCT, config.SolverNodeBudget)
	}
}

func TestFarBlock(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFarBlock)
	mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
	mustNotFound(t, b, KindVCT, config.SolverNodeBudget)
}

func TestVCTDoubleThree(t *testing.T) {
	for _, tc := range []struct {
		cross  bool
		ar, ac int
	}{
		{false, 8, 9},
		{true, 4, 6},
	} {
		b := buildAt(t, tc.cross, tc.ar, tc.ac, rules.Red, specVCTDoubleThree)
		mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
		stats := mustFound(t, b, KindVCT, config.SolverNodeBudget)
		if stats.Plies != 5 {
			t.Fatalf("double three: plies=%d want 5", stats.Plies)
		}
	}
}

func TestVCTCounterFourRefutes(t *testing.T) {
	b := buildAt(t, false, 8, 9, rules.Red, specVCTCounterFour)
	mustNotFound(t, b, KindVCT, config.SolverNodeBudget)
}

func TestVCTPhantomThree(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specPhantomThree)
	mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
	mustNotFound(t, b, KindVCT, config.SolverNodeBudget)
}

func TestQuietNegative(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specQuiet)
	mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
	mustNotFound(t, b, KindVCT, config.SolverNodeBudget)
	cross := buildAt(t, true, 1, 1, rules.Blue, specQuiet)
	mustNotFound(t, cross, KindVCF, config.SolverNodeBudget)
	mustNotFound(t, cross, KindVCT, config.SolverNodeBudget)
}

func TestBudgetExhaustionNeverWins(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	for _, budget := range []int{0, 1, 2, 3, 5, 9} {
		mustNotFound(t, b, KindVCF, budget)
		mustNotFound(t, b, KindVCT, budget)
	}
	s := New(KindVCF)
	var stats SolverStats
	s.Solve(b, 2, nil, &stats)
	if stats.Nodes > uint64(2)+1 {
		t.Fatalf("budget: nodes=%d over budget 2", stats.Nodes)
	}
}

func TestDeadlineStop(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	s := New(KindVCF)
	dl := &countdownDeadline{after: 0}
	var stats SolverStats
	if s.Solve(b, config.SolverNodeBudget, dl, &stats) {
		t.Fatalf("deadline already exceeded: unexpected win")
	}
	heavy := buildAt(t, false, 8, 9, rules.Red, specVCTCounterFour)
	s2 := New(KindVCT)
	dl2 := &countdownDeadline{after: 1}
	if s2.Solve(heavy, config.SolverNodeBudget, dl2, &stats) {
		t.Fatalf("deadline after one poll: unexpected win")
	}
	if stats.Nodes > uint64(2*config.SolverNodeCheckInterval)+100 {
		t.Fatalf("deadline: search ran past the poll, nodes=%d", stats.Nodes)
	}
	s3 := New(KindVCT)
	dl3 := &stoppedDeadline{stopped: true}
	if s3.Solve(b, config.SolverNodeBudget, dl3, &stats) {
		t.Fatalf("stopped deadline: unexpected win")
	}
}

func TestMaxPlyAbort(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	s := New(KindVCF)
	s.maxPly = 4
	var stats SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &stats) {
		t.Fatalf("max ply 4: unexpected win on a 7-ply chain")
	}
}

func TestMemoReuse(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specTwoDeadFours)
	s := New(KindVCF)
	var first, second SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &first) {
		t.Fatalf("memo: two dead fours must stay unfound")
	}
	s.Solve(b, config.SolverNodeBudget, nil, &second)
	if second.Found {
		t.Fatalf("memo: second solve changed its mind")
	}
	if second.Nodes >= first.Nodes {
		t.Fatalf("memo: nodes not reduced, first=%d second=%d", first.Nodes, second.Nodes)
	}
}

func TestBoardRestored(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	hash, count, side := b.Hash, b.MoveCount, b.Side
	s := New(KindVCT)
	var stats SolverStats
	s.Solve(b, config.SolverNodeBudget, nil, &stats)
	if b.Hash != hash || b.MoveCount != count || b.Side != side {
		t.Fatalf("board not restored: hash=%d/%d moves=%d/%d side=%d/%d",
			b.Hash, hash, b.MoveCount, count, b.Side, side)
	}
}

func TestStatsOverwrite(t *testing.T) {
	win := buildAt(t, false, 8, 4, rules.Red, specOpenFour)
	quiet := buildAt(t, false, 8, 4, rules.Red, specQuiet)
	s := New(KindVCF)
	var stats SolverStats
	if !s.Solve(win, config.SolverNodeBudget, nil, &stats) || !stats.Found {
		t.Fatalf("stats overwrite: expected win first")
	}
	if s.Solve(quiet, config.SolverNodeBudget, nil, &stats) || stats.Found {
		t.Fatalf("stats overwrite: expected miss second")
	}
	if stats.Plies != 0 || stats.PV != ([config.SolverMaxPly]rules.Move{}) {
		t.Fatalf("stats overwrite: stale pv plies=%d", stats.Plies)
	}
}

func TestNilStatsAndDeadline(t *testing.T) {
	b := buildAt(t, false, 8, 4, rules.Red, specOpenFour)
	s := New(KindVCF)
	if !s.Solve(b, config.SolverNodeBudget, nil, nil) {
		t.Fatalf("nil stats: expected win")
	}
}

func TestBlueAttacker(t *testing.T) {
	spec := []placed{
		o(0, 1), o(0, 2), o(0, 3), x(1, 1), x(1, 2), x(1, 3), x(1, 4),
	}
	b := buildAt(t, false, 8, 4, rules.Blue, spec)
	stats := mustFound(t, b, KindVCF, config.SolverNodeBudget)
	if stats.Plies != 1 {
		t.Fatalf("blue attacker: plies=%d want 1", stats.Plies)
	}
}

func TestFullBoardIsFail(t *testing.T) {
	// Checkerboard fill: no two same color stones adjacent, so no five and
	// no threat anywhere; the board is full.
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			color := rules.Red
			if (r+c)%2 == 1 {
				color = rules.Blue
			}
			b.Side = color
			b.Make(cellOf(r, c))
		}
	}
	b.Side = rules.Red
	if !b.IsFull() {
		t.Fatalf("checkerboard fill expected a full board")
	}
	s := New(KindVCF)
	var stats SolverStats
	if s.Solve(b, config.SolverNodeBudget, nil, &stats) {
		t.Fatalf("full board: unexpected win")
	}
}

func TestPhantomFourFailsVCF(t *testing.T) {
	spec := []placed{o(0, 1), o(0, 2), o(0, 3), x(0, -1), x(0, 5)}
	b := buildAt(t, false, 8, 5, rules.Red, spec)
	mustNotFound(t, b, KindVCF, config.SolverNodeBudget)
}

func TestAppendPVSeparatesEntries(t *testing.T) {
	var stats SolverStats
	stats.Plies = 3
	stats.PV[0] = rules.Move(cellOf(8, 5))
	stats.PV[1] = rules.Move(cellOf(8, 9))
	stats.PV[2] = rules.Move(cellOf(8, 4))
	var buf [24]byte
	got := string(stats.AppendPV(buf[:0]))
	if got != "F9 J9 E9" {
		t.Fatalf("append pv: got %q want %q", got, "F9 J9 E9")
	}
}

func TestDeadlineMidSearchPoll(t *testing.T) {
	board, _, ok := genRandom(14, 0x5851F42D4C957F2D, false)
	if !ok {
		t.SkipNow()
	}
	s := New(KindVCT)
	dl := &countdownDeadline{after: 1}
	var stats SolverStats
	if s.Solve(board, config.SolverNodeBudget, dl, &stats) {
		t.Fatalf("mid search deadline: unexpected win")
	}
	if stats.Nodes > uint64(2*config.SolverNodeCheckInterval)+100 {
		t.Fatalf("mid search deadline: ran past the poll, nodes=%d", stats.Nodes)
	}
}

func TestDefendDefenderWinsInOne(t *testing.T) {
	// White box: the attacker's threat moves and forced blocks always
	// remove the defender's win cells before defend runs, so reach the
	// guard directly with a board where the defender has a live four.
	b := buildAt(t, false, 8, 4, rules.Red, []placed{
		x(2, 1), x(2, 2), x(2, 3), x(2, 4),
		o(2, 0),
	})
	b.Side = rules.Blue
	s := New(KindVCF)
	s.nodes, s.budget, s.check, s.aborted = 0, config.SolverNodeBudget, config.SolverNodeCheckInterval, false
	s.plyCap = config.SolverMaxPly
	if r := s.defend(b, cellOf(10, 5), 1); r != resFail {
		t.Fatalf("defend with defender win in 1: result %d want %d", r, resFail)
	}
}

func TestDefusingSkipsNonWinCells(t *testing.T) {
	// White box: defusing trusts its win cell list, fiveFrames reports no
	// exact five for a plain stone, and the loop adds the cell alone.
	b := buildAt(t, false, 8, 4, rules.Red, specQuiet)
	s := New(KindVCF)
	s.winA[0] = cellOf(8, 4)
	if n := s.defusing(b, rules.Red, 1, 0); n != 1 {
		t.Fatalf("defusing: n=%d want the win cell alone", n)
	}
	var frames [config.PatternDirections]fiveFrame
	if n := fiveFrames(b, rules.Red, cellOf(8, 4), frames[:]); n != 0 {
		t.Fatalf("fiveFrames: quiet stone reported %d fives", n)
	}
}
