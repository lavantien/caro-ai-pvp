package engine

// Mutation-killer pins for internal/engine/search.go. Each test names the
// survivor keys (file:line:col descriptor) it kills; the survivors not
// covered here are either proven equivalent in .mutate-allow or flagged for
// deletion (the dead cutoff-PV writes). Every pinned number is the
// deterministic clean-code value of a fixed board and budget, so any splice
// that changes traversal, scores, PVs, TT traffic, or stats arithmetic fails
// exactly one of these pins.

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// countdownDL is a deterministic deadline: Exceeded flips true on the n-th
// call and stays true, which lands the stop at an exact node count instead
// of wall-clock time.
type mutCountdownDL struct{ calls int }

func (d *mutCountdownDL) Exceeded() bool { d.calls--; return d.calls <= 0 }
func (d *mutCountdownDL) Stop()          {}

// floodBoard fills every cell blue except the red anchor H8 and the named
// free cells. With exactly one red stone and red to move the opening
// constraint keeps only free cells at Chebyshev >= 3 from the anchor, so the
// free-cell set is exactly the candidate set.
func floodBoard(t *testing.T, free ...string) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	anchor := mustCell(t, "H8")
	freeSet := make(map[rules.Cell]bool, len(free))
	for _, f := range free {
		freeSet[mustCell(t, f)] = true
	}
	place(t, b, rules.Red, "H8")
	for r := range config.BoardSize {
		for c := range config.BoardSize {
			cell := rules.Cell(r*config.BoardStride + c)
			if cell == anchor || freeSet[cell] {
				continue
			}
			b.Side = rules.Blue
			b.Make(cell)
		}
	}
	b.Side = rules.Red
	return b
}

// seedChildTT stores an exact-bound mate entry at the hash reached by one
// move, tuned so a probe at probePly decodes to want. With want = M-16 and
// storePly = probePly+1 the decode is exactly M (or -M for negative want).
func seedChildTT(t *testing.T, e *Engine, b *rules.Board, cell rules.Cell, want int, storePly int) {
	t.Helper()
	b.Make(cell)
	h := b.Hash
	b.Unmake()
	e.tt.store(h, want, 0, config.SearchMaxPly, ttBoundExact, storePly)
}

// kills 26:33, 26:35 x2, 27:33, 27:35 x2: pv and pvLen must have the +1
// row because negamax at ply SearchMaxPly-1 writes pvLen[ply+1] before the
// depth guard runs; a shorter array panics, a longer one admits slop.

// pvEquals compares a stats PV against the wanted cell list.
func pvEquals(stats *SearchStats, want ...rules.Cell) bool {
	if stats.PVLen != len(want) {
		return false
	}
	for i, c := range want {
		if stats.PV[i] != rules.Move(c) {
			return false
		}
	}
	return true
}

func TestMutKillPVArraysSizedForMaxPly(t *testing.T) {
	e := New(0)
	if got := len(e.pv); got != config.SearchMaxPly+1 {
		t.Errorf("len(pv) = %d, want SearchMaxPly+1", got)
	}
	if got := len(e.pvLen); got != config.SearchMaxPly+1 {
		t.Errorf("len(pvLen) = %d, want SearchMaxPly+1", got)
	}
}

// kills 71:58..70: resetForSearch must zero every stats counter so two
// searches on one engine report independent counts.
func TestMutKillResetZeroesCounters(t *testing.T) {
	e := New(testTTBytes)
	e.nodes, e.ttProbes, e.ttHits, e.cutNodes, e.cutFirst = 7, 7, 7, 7, 7
	e.resetForSearch(midgameBoard(t))
	for _, c := range []struct {
		name string
		got  uint64
	}{{"nodes", e.nodes}, {"ttProbes", e.ttProbes}, {"ttHits", e.ttHits}, {"cutNodes", e.cutNodes}, {"cutFirst", e.cutFirst}} {
		if c.got != 0 {
			t.Errorf("%s = %d after reset, want 0", c.name, c.got)
		}
	}
}

// kills 99:31 x2, 99:53 x2, 101:16: a stopped deadline before the first
// iteration must leave the zero inits of best/completed visible in stats,
// and the || guard must break on the deadline alone.
func TestMutKillStoppedSearchInits(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := NewFixedBudget(time.Second)
	dl.Stop()
	mv, stats := e.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("stopped search move %d illegal", mv)
	}
	if stats.Score != 0 || stats.Depth != 0 || stats.PVLen != 0 {
		t.Errorf("stopped search stats score=%d depth=%d pvlen=%d, want all 0", stats.Score, stats.Depth, stats.PVLen)
	}
}

// kills 124:16, 125:17, 125:20: the precomputed fallback is the first
// generated candidate of ply 0 in scan order, here F8.
func TestMutKillFallbackMoveIsFirstCandidate(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	place(t, b, rules.Blue, "H9")
	place(t, b, rules.Red, "I8")
	e := New(0)
	dl := NewFixedBudget(time.Second)
	dl.Stop()
	mv, _ := e.Search(b, dl)
	if mv != rules.Move(mustCell(t, "F8")) {
		t.Errorf("fallback move = %d, want F8", mv)
	}
}

// kills 135:18, 136:40 x2, 139:18, 140:54 x2, 140:59: the permille stats
// arithmetic, exercised straight through finishStats with hand-set counters.
func TestMutKillFinishStatsPermilles(t *testing.T) {
	run := func(probes, hits, cutNodes, cutFirst uint64) SearchStats {
		e := New(0)
		e.ttProbes, e.ttHits, e.cutNodes, e.cutFirst = probes, hits, cutNodes, cutFirst
		var st SearchStats
		e.finishStats(&st, 0, 1, time.Now())
		return st
	}
	if st := run(1, 1, 0, 0); st.TTHitPermille != 1000 {
		t.Errorf("1/1 probes: tt hit permille = %d, want 1000", st.TTHitPermille)
	}
	if st := run(2, 2, 0, 0); st.TTHitPermille != 1000 {
		t.Errorf("2/2 probes: tt hit permille = %d, want 1000", st.TTHitPermille)
	}
	if st := run(0, 0, 1, 1); st.FirstMoveFailHighPermille != 1000 {
		t.Errorf("1/1 cuts: fh1 permille = %d, want 1000", st.FirstMoveFailHighPermille)
	}
	if st := run(0, 0, 7, 5); st.FirstMoveFailHighPermille != 714 {
		t.Errorf("5/7 cuts: fh1 permille = %d, want 714", st.FirstMoveFailHighPermille)
	}
}

// kills 113:12, 113:33, 114:4: the mate early break stops iterative
// deepening after the depth-1 win, so the whole search is 2 nodes at depth
// 1; without the break the extra iterations inflate the node count.
func TestMutKillMateEarlyBreakNodes(t *testing.T) {
	e := New(0)
	_, stats := e.SearchDepth(mate1Board(t), NewFixedBudget(time.Second), 4)
	if stats.Depth != 1 || stats.Nodes != 2 {
		t.Errorf("mate in 1 depth=%d nodes=%d, want 1 and 2", stats.Depth, stats.Nodes)
	}
}

// kills 68:18, 68:37 (stale killer slots reorder the first search, A1 is a
// live midgame candidate), 150:36, 166:39, 168:28, 168:29, 168:47, 250:13,
// 253:28, 253:39, 253:57, 254:7, 254:9, 254:17, 254:22, 254:29, 255:29,
// 255:51, 263:8, 266:8, 276:8, 280:31, 280:36, 284:5, 286:12 and the bulk
// of the remaining window and ordering traversal splices: exact node, move,
// score and PV pins on fixed boards. Depth 2 and 3 with the table off pin
// the bare alpha-beta; the mate pins below pin forced-line behavior.
func TestMutKillMidgameBarePins(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 2)
	if stats.Nodes != 107 || mv != rules.Move(mustCell(t, "E3")) || stats.Score != 1000 || stats.PVLen != 2 {
		t.Errorf("mid d2: nodes=%d mv=%d score=%d pvlen=%d, want 107 E3 1000 2", stats.Nodes, mv, stats.Score, stats.PVLen)
	}
	e = New(0)
	mv, stats = e.SearchDepth(b, NewFixedBudget(time.Second), 3)
	if stats.Nodes != 1047 || mv != rules.Move(mustCell(t, "E3")) || stats.Score != 7000 || !pvEquals(&stats, mustCell(t, "E3"), mustCell(t, "B3"), mustCell(t, "G8")) {
		t.Errorf("mid d3: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

// kills 268:7, 268:9, 268:12 (cutFirst accounting), 284:5, 284:10, 286:12,
// 286:17 (bound selection feeds probing) and reinforces the bare pins with
// the table on: fresh and warm searches on one engine have distinct exact
// node counts.
func TestMutKillTTTraversalPins(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	_, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 3)
	if stats.Nodes != 702 || stats.FirstMoveFailHighPermille != 825 {
		t.Errorf("mid d3 tt: nodes=%d fh1=%d, want 702 and 825", stats.Nodes, stats.FirstMoveFailHighPermille)
	}
	_, stats = e.SearchDepth(b, NewFixedBudget(time.Second), 3)
	if stats.Nodes != 42 {
		t.Errorf("mid d3 tt warm: nodes=%d, want 42", stats.Nodes)
	}
}

// kills 178:9 (tie move must not rewrite the root PV head), 182:26 x2,
// 249:15, 249:21 (root/interior pvLen bookkeeping on the draw line), and
// 248:8 x2 (the interior draw scores exactly 0).
func TestMutKillTwoHoleDrawPins(t *testing.T) {
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			if r == 0 && c == 0 || r == config.CrossCheckSize-1 && c == config.CrossCheckSize-1 {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	e := New(0)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 2)
	if stats.Score != 0 || mv != 0 || stats.PVLen != 2 || stats.PV[0] != 0 || stats.PV[1] != rules.Move(mustCell(t, "H8")) {
		t.Errorf("two hole draw: score=%d mv=%d pvlen=%d pv=%v, want 0 A1 2 [A1 H8]", stats.Score, mv, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

// kills 162:17: the root filling draw leaves a length-1 PV.
func TestMutKillFillingDrawPVLen(t *testing.T) {
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			if r == 0 && c == 0 {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	e := New(0)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 2)
	if stats.PVLen != 1 || stats.PV[0] != mv {
		t.Errorf("filling draw: pvlen=%d pv0=%d mv=%d, want 1 and mv", stats.PVLen, stats.PV[0], mv)
	}
}

// kills 159:12 (root win PV length is 1 even when the winning move is
// ordered after the two open-four blocks). Decoy: red four E9..H9 with the
// win at I9, blue open four F2..I2 whose block cells E2/J2 share the top
// static, so the win lands at i=2 after two recursions.
func TestMutKillDecoyRootWinPVLen(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H9")
	place(t, b, rules.Blue, "D9", "F2", "G2", "H2", "I2")
	b.Side = rules.Red
	e := New(0)
	mv, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 3)
	if mv != rules.Move(mustCell(t, "I9")) || stats.Score != config.EvalMateMax-config.EvalMateScoreStep || stats.PVLen != 1 {
		t.Errorf("decoy: mv=%d score=%d pvlen=%d, want I9 mate pvlen 1", mv, stats.Score, stats.PVLen)
	}
}

// kills 216:14 (leaf one ply earlier misses the mate), 164:28 (shallower
// mainline misses the mate at depth 3), 246:15, 246:16 x2, 246:21 (interior
// win-case pvLen), 253:57, 255:51 (scout/re-search ply skew shifts mate
// distance by one step), plus window mutants on the forced line.
func TestMutKillMate2Depth3Pins(t *testing.T) {
	e := New(0)
	mv, stats := e.SearchDepth(mate2Board(t), NewFixedBudget(time.Second), 3)
	want := config.EvalMateMax - 3*config.EvalMateScoreStep
	if stats.Nodes != 682 || mv != rules.Move(mustCell(t, "E9")) || stats.Score != want || !pvEquals(&stats, mustCell(t, "E9"), mustCell(t, "D9"), mustCell(t, "I9")) {
		t.Errorf("mate2 d3: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

// kills 212:40 (extension flips to the mover's own fours) and 251:28
// (mainline depth): depth 4 finds the 5-ply mate only through the
// forced-defense extension, with exact node and PV pins.
func TestMutKillMate3Depth4Pins(t *testing.T) {
	e := New(0)
	mv, stats := e.SearchDepth(mate3Board(t), NewFixedBudget(time.Second), 4)
	want := config.EvalMateMax - 5*config.EvalMateScoreStep
	if stats.Nodes != 3058 || mv != rules.Move(mustCell(t, "H9")) || stats.Score != want || !pvEquals(&stats, mustCell(t, "H9"), mustCell(t, "I9"), mustCell(t, "H6"), mustCell(t, "I7"), mustCell(t, "H5")) {
		t.Errorf("mate3 d4: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

// kills 198:20, 198:32, 198:35 x2: the node-check cadence decides exactly
// which node observes the flipped deadline; every cadence splice moves the
// stop node and the completed depth.
func TestMutKillPollCadence(t *testing.T) {
	e := New(0)
	_, stats := e.SearchDepth(midgameBoard(t), &mutCountdownDL{calls: 5}, 5)
	if stats.Nodes != 2048 || stats.Depth != 3 {
		t.Errorf("countdown stop: nodes=%d depth=%d, want 2048 and 3", stats.Nodes, stats.Depth)
	}
}

// kills 228:6, 228:8, 228:11 (0 -> 1): the boundary move value 0 (cell A1)
// must still be adopted as the TT move. Blue B1..D1 make A1 a live
// candidate; seeding move 0 at the G7 child reorders that child and changes
// the exact node count.
func TestMutKillTTMoveZeroAdopted(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8", "I9", "C3")
	place(t, b, rules.Blue, "H9", "I8", "C4", "D3", "B1", "C1", "D1")
	e := New(testTTBytes)
	e.beginSearch(b)
	b.Make(mustCell(t, "G7"))
	h := b.Hash
	b.Unmake()
	e.tt.store(h, 0, 0, 0, ttBoundLower, 1)
	_, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 3)
	if stats.Nodes != 410 {
		t.Errorf("seeded A1 ttm: nodes=%d, want 410", stats.Nodes)
	}
}

// kills 216:23: negamax at ply SearchMaxPly must return the leaf eval
// before touching the ply-indexed move stack, which has no row 64.
func TestMutKillMaxPlyLeafGuard(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	e.resetForSearch(b)
	want := e.eval.eval(b.Side)
	got := e.negamax(b, 1, -config.EvalMateMax, config.EvalMateMax, config.SearchMaxPly, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if got != want {
		t.Errorf("ply=SearchMaxPly negamax = %d, want leaf eval %d", got, want)
	}
}

// kills 186:58 x2: the root store passes ply 0, so probing the root entry
// at ply 0 decodes back to exactly the reported root score.
func TestMutKillRootStorePlyDecodes(t *testing.T) {
	b := mate1Board(t)
	e := New(testTTBytes)
	_, stats := e.SearchDepth(b, NewFixedBudget(time.Second), 4)
	score, _, cutoff := e.tt.probe(b.Hash, 1, -config.EvalMateMax, config.EvalMateMax, 0)
	if !cutoff || score != stats.Score || score != config.EvalMateMax-config.EvalMateScoreStep {
		t.Errorf("root entry decode: cutoff=%v score=%d, want exact %d", cutoff, score, config.EvalMateMax-config.EvalMateScoreStep)
	}
}

// kills 167:7, 167:9, 167:17, 167:29 (alpha-boundary scout) and 167:22
// (beta-boundary scout): both root moves decode to exactly -M (C1) or the
// second to exactly +M (C2) through seeded entries, so the re-search guard
// is exercised on both boundaries; any splice that fires the re-search adds
// a third node.
func TestMutKillRootPVSWindowCraft(t *testing.T) {
	m := config.EvalMateMax
	step := config.EvalMateScoreStep

	fb := floodBoard(t, "E5", "F5")
	e := New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), m-step, 2)
	seedChildTT(t, e, fb, mustCell(t, "F5"), m-step, 2)
	sc, mv := e.searchRoot(fb, 3, NewFixedBudget(time.Second))
	if e.nodes != 2 || sc != -m || mv != rules.Move(mustCell(t, "E5")) {
		t.Errorf("craft C1: nodes=%d score=%d mv=%d, want 2 -M E5", e.nodes, sc, mv)
	}

	fb = floodBoard(t, "E5", "F5")
	e = New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), m-step, 2)
	seedChildTT(t, e, fb, mustCell(t, "F5"), -(m - step), 2)
	sc, mv = e.searchRoot(fb, 3, NewFixedBudget(time.Second))
	if e.nodes != 2 || sc != m || mv != rules.Move(mustCell(t, "F5")) {
		t.Errorf("craft C2: nodes=%d score=%d mv=%d, want 2 +M F5", e.nodes, sc, mv)
	}
}

// kills 147:26 (1 -> 0) and 148:10, 148:15 x2: every iteration scores both
// root moves exactly -M, which never beats alpha, so the move-0 sentinel
// must still record the best move and the pvLen[0] reset must hold through
// the last iteration's snapshot.
func TestMutKillRootSentinelCraft(t *testing.T) {
	m := config.EvalMateMax
	fb := floodBoard(t, "E5", "F5")
	e := New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), m-config.EvalMateScoreStep, 2)
	seedChildTT(t, e, fb, mustCell(t, "F5"), m-config.EvalMateScoreStep, 2)
	mv, stats := e.SearchDepth(fb, NewFixedBudget(time.Second), 3)
	if mv != rules.Move(mustCell(t, "E5")) || stats.Score != -m || stats.PVLen != 0 || stats.Depth != 3 {
		t.Errorf("root sentinel craft: mv=%d score=%d pvlen=%d depth=%d, want E5 -M 0 3", mv, stats.Score, stats.PVLen, stats.Depth)
	}
}

// kills 234:41 and 234:43 (1 -> 0): the single interior candidate decodes
// to exactly -M, one below the fail-soft sentinel, so best/bestMove must
// still update and the stored best move must be the candidate.
func TestMutKillInteriorSentinelCraft(t *testing.T) {
	m := config.EvalMateMax
	fb := floodBoard(t, "E5", "F5")
	e := New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), m-config.EvalMateScoreStep, 3)
	seedChildTT(t, e, fb, mustCell(t, "F5"), m-config.EvalMateScoreStep, 3)
	rv := e.negamax(fb, 3, -m, m, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if rv != -m || e.tt.move(fb.Hash) != rules.Move(mustCell(t, "E5")) {
		t.Errorf("interior sentinel craft: rv=%d ttmove=%d, want -M E5", rv, e.tt.move(fb.Hash))
	}
}

// checkerBoard fills the board with alternating colors except a free 3x3
// block: no two adjacent stones share a color anywhere, so no move can ever
// complete a five and every subtree is a pure eval walk.
func mutCheckerBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for r := range config.BoardSize {
		for c := range config.BoardSize {
			if r >= 6 && r <= 8 && c >= 6 && c <= 8 {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	return b
}

// kills 212:13, 212:15 x2, 212:45: the extension must fire exactly when
// extLeft is positive. Blue to move against red's four (fours[red] = 5):
// extLeft 0 stays a depth-1 node (3 nodes), extLeft 1 extends to depth 2
// (80 nodes).
func TestMutKillExtensionBudget(t *testing.T) {
	b := mate1Board(t)
	b.Side = rules.Blue
	e := New(0)
	e.resetForSearch(b)
	e.negamax(b, 1, -config.EvalMateMax, config.EvalMateMax, 1, 0, NewFixedBudget(time.Second))
	if e.nodes != 3 {
		t.Errorf("extLeft=0: nodes=%d, want 3 (no extension)", e.nodes)
	}
	e = New(0)
	e.resetForSearch(b)
	e.negamax(b, 1, -config.EvalMateMax, config.EvalMateMax, 1, 1, NewFixedBudget(time.Second))
	if e.nodes != 80 {
		t.Errorf("extLeft=1: nodes=%d, want 80 (extension fires)", e.nodes)
	}
}

// kills 284:10, 286:17: a fail-high exactly at beta stores a lower bound
// and a fail-low exactly at the original alpha stores an upper bound; the
// discriminating probe windows cutoff only under the exact bound the
// spliced condition would produce. The single candidate E5 is seeded to
// return -7000, so the node's best is exactly 7000.
func TestMutKillBoundClassification(t *testing.T) {
	fb := floodBoard(t, "E5", "F5")
	e := New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), -7000, 2)
	if rv := e.negamax(fb, 3, 0, 7000, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second)); rv != 7000 {
		t.Fatalf("lower craft rv=%d, want 7000", rv)
	}
	if _, _, cut := e.tt.probe(fb.Hash, 3, 0, 7001, 1); cut {
		t.Error("lower bound cutoff at beta 7001, want miss")
	}
	if _, _, cut := e.tt.probe(fb.Hash, 3, 0, 7000, 1); !cut {
		t.Error("lower bound miss at beta 7000, want cutoff")
	}

	fb = floodBoard(t, "E5", "F5")
	e = New(testTTBytes)
	e.beginSearch(fb)
	seedChildTT(t, e, fb, mustCell(t, "E5"), -7000, 2)
	if rv := e.negamax(fb, 3, 7000, 7001, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second)); rv != 7000 {
		t.Fatalf("upper craft rv=%d, want 7000", rv)
	}
	if _, _, cut := e.tt.probe(fb.Hash, 3, 6999, config.EvalMateMax, 1); cut {
		t.Error("upper bound cutoff at alpha 6999, want miss")
	}
	if _, _, cut := e.tt.probe(fb.Hash, 3, 7000, config.EvalMateMax, 1); !cut {
		t.Error("upper bound miss at alpha 7000, want cutoff")
	}
}

// kills 166:41 and 253:41: the scout window is exactly [-(alpha+1), -alpha].
// The first root move (H7 on this board) is seeded to return 1, so alpha is
// -1 and the second move's true value 0 sits exactly on the window edge:
// the clean scout stores an upper bound at the child, making its own
// full-window re-search traverse everything, while the widened scout stores
// an exact bound whose re-search probe cuts off immediately. The ply-2
// variant drives the interior scout through a direct negamax call.
func TestMutKillScoutWindowBoundary(t *testing.T) {
	cb := mutCheckerBoard(t)
	e := New(testTTBytes)
	e.beginSearch(cb)
	seedChildTT(t, e, cb, mustCell(t, "H7"), 1, 1)
	sc, mv := e.searchRoot(cb, 3, NewFixedBudget(time.Second))
	if e.nodes != 1481 || sc != 0 || mv != rules.Move(mustCell(t, "G8")) {
		t.Errorf("root scout craft: nodes=%d score=%d mv=%d, want 1481 0 G8", e.nodes, sc, mv)
	}

	e = New(testTTBytes)
	e.beginSearch(cb)
	seedChildTT(t, e, cb, mustCell(t, "H7"), 1, 2)
	rv := e.negamax(cb, 3, -config.EvalMateMax, config.EvalMateMax, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if e.nodes != 1181 || rv != 0 {
		t.Errorf("interior scout craft: nodes=%d rv=%d, want 1181 0", e.nodes, rv)
	}
}
