package engine

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type mutCountdownDL struct{ calls int }

func (d *mutCountdownDL) Exceeded() bool { d.calls--; return d.calls <= 0 }
func (d *mutCountdownDL) Stop()          {}

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

func seedChildTT(t *testing.T, e *Engine, b *rules.Board, cell rules.Cell, want int, storePly int) {
	t.Helper()
	b.Make(cell)
	h := b.Hash
	b.Unmake()
	e.tt.store(h, want, 0, config.SearchMaxPly, ttBoundExact, storePly)
}

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
	if got := len(New(0).pv); got != config.SearchMaxPly+1 {
		t.Errorf("len(pv) = %d, want SearchMaxPly+1", got)
	}
	if got := len(New(0).pvLen); got != config.SearchMaxPly+1 {
		t.Errorf("len(pvLen) = %d, want SearchMaxPly+1", got)
	}
}

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

func TestMutKillStoppedSearchInits(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := cappedProbe(time.Second)
	dl.Stop()
	mv, stats := e.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("stopped search move %d illegal", mv)
	}
	if stats.Score != 0 || stats.Depth != 0 || stats.PVLen != 0 {
		t.Errorf("stopped search stats score=%d depth=%d pvlen=%d, want all 0", stats.Score, stats.Depth, stats.PVLen)
	}
}

func TestMutKillFallbackMoveIsFirstCandidate(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	place(t, b, rules.Blue, "H9")
	place(t, b, rules.Red, "I8")
	e := New(0)
	dl := cappedProbe(time.Second)
	dl.Stop()
	mv, _ := e.Search(b, dl)
	if mv != rules.Move(mustCell(t, "F8")) {
		t.Errorf("fallback move = %d, want F8", mv)
	}
}

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

func TestMutKillMateEarlyBreakNodes(t *testing.T) {
	e := New(0)
	_, stats := e.SearchDepth(mate1Board(t), cappedNodeBudget(500_000), 4)
	if stats.Depth != 2 || stats.Nodes != 3 || stats.RootIters != 2 {
		t.Errorf("mate in 1 depth=%d nodes=%d rootiters=%d, want the verified halt at 2 3 2", stats.Depth, stats.Nodes, stats.RootIters)
	}
}

func TestMutKillMidgameBarePins(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	mv, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 2)
	if stats.Nodes != 142 || mv != rules.Move(mustCell(t, "G8")) || stats.Score != 300 || stats.PVLen != 2 {
		t.Errorf("mid d2: nodes=%d mv=%d score=%d pvlen=%d, want 142 G8 300 2", stats.Nodes, mv, stats.Score, stats.PVLen)
	}
	e = New(0)
	mv, stats = e.SearchDepth(b, cappedNodeBudget(500_000), 3)
	if stats.Nodes != 697 || mv != rules.Move(mustCell(t, "E3")) || stats.Score != 7700 || !pvEquals(&stats, mustCell(t, "E3"), mustCell(t, "F3"), mustCell(t, "G8")) {
		t.Errorf("mid d3: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

func TestMutKillTTTraversalPins(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	_, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 3)
	if stats.Nodes != 486 || stats.FirstMoveFailHighPermille != 770 {
		t.Errorf("mid d3 tt: nodes=%d fh1=%d, want 486 and 770", stats.Nodes, stats.FirstMoveFailHighPermille)
	}
	_, stats = e.SearchDepth(b, cappedNodeBudget(500_000), 3)
	if stats.Nodes != 44 {
		t.Errorf("mid d3 tt warm: nodes=%d, want 44", stats.Nodes)
	}
}

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
	mv, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 2)
	if stats.Score != 0 || mv != 0 || stats.PVLen != 2 || stats.PV[0] != 0 || stats.PV[1] != rules.Move(mustCell(t, "H8")) {
		t.Errorf("two hole draw: score=%d mv=%d pvlen=%d pv=%v, want 0 A1 2 [A1 H8]", stats.Score, mv, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

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
	mv, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 2)
	if stats.PVLen != 1 || stats.PV[0] != mv {
		t.Errorf("filling draw: pvlen=%d pv0=%d mv=%d, want 1 and mv", stats.PVLen, stats.PV[0], mv)
	}
}

func TestMutKillDecoyRootWinPVLen(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "E9", "F9", "G9", "H9")
	place(t, b, rules.Blue, "D9", "F2", "G2", "H2", "I2")
	b.Side = rules.Red
	e := New(0)
	mv, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 3)
	if mv != rules.Move(mustCell(t, "I9")) || stats.Score != config.EvalMateMax-config.EvalMateScoreStep || stats.PVLen != 1 {
		t.Errorf("decoy: mv=%d score=%d pvlen=%d, want I9 mate pvlen 1", mv, stats.Score, stats.PVLen)
	}
}

func TestMutKillMate2Depth3Pins(t *testing.T) {
	e := New(0)
	mv, stats := e.SearchDepth(mate2Board(t), cappedNodeBudget(500_000), 3)
	want := config.EvalMateMax - 3*config.EvalMateScoreStep
	if stats.Nodes != 345 || mv != rules.Move(mustCell(t, "I9")) || stats.Score != want || !pvEquals(&stats, mustCell(t, "I9"), mustCell(t, "E9"), mustCell(t, "J9")) {
		t.Errorf("mate2 d3: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

func TestMutKillMate3Depth4Pins(t *testing.T) {
	e := New(0)
	mv, stats := e.SearchDepth(mate3Board(t), cappedNodeBudget(500_000), 4)
	want := config.EvalMateMax - 5*config.EvalMateScoreStep
	if stats.Nodes != 1976 || mv != rules.Move(mustCell(t, "H9")) || stats.Score != want || !pvEquals(&stats, mustCell(t, "H9"), mustCell(t, "I9"), mustCell(t, "H10"), mustCell(t, "H11"), mustCell(t, "H6")) {
		t.Errorf("mate3 d4: nodes=%d mv=%d score=%d pvlen=%d pv=%v", stats.Nodes, mv, stats.Score, stats.PVLen, stats.PV[:stats.PVLen])
	}
}

func TestMutKillPollCadence(t *testing.T) {
	e := New(0)
	_, stats := e.SearchDepth(midgameBoard(t), &mutCountdownDL{calls: 5}, 5)
	if stats.Nodes != 1714 || stats.Depth != 4 {
		t.Errorf("countdown stop: nodes=%d depth=%d, want 1753 and 4", stats.Nodes, stats.Depth)
	}
}

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
	_, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 3)
	if stats.Nodes != 217 {
		t.Errorf("seeded A1 ttm: nodes=%d, want 217", stats.Nodes)
	}
}

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

func TestMutKillRootStorePlyDecodes(t *testing.T) {
	b := mate1Board(t)
	e := New(testTTBytes)
	_, stats := e.SearchDepth(b, cappedNodeBudget(500_000), 4)
	score, _, cutoff := e.tt.probe(b.Hash, 1, -config.EvalMateMax, config.EvalMateMax, 0)
	if !cutoff || score != stats.Score || score != config.EvalMateMax-config.EvalMateScoreStep {
		t.Errorf("root entry decode: cutoff=%v score=%d, want exact %d", cutoff, score, config.EvalMateMax-config.EvalMateScoreStep)
	}
}

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

func TestMutKillRootSentinelCraft(t *testing.T) {
	m := config.EvalMateMax
	fb := floodBoard(t, "E5", "F5")
	e := New(testTTBytes)
	e.beginSearch(fb)
	e.tt.bumpGen()
	seedChildTT(t, e, fb, mustCell(t, "E5"), m-config.EvalMateScoreStep, 2)
	seedChildTT(t, e, fb, mustCell(t, "F5"), m-config.EvalMateScoreStep, 2)
	e.tt.gen--
	mv, stats := e.SearchDepth(fb, cappedNodeBudget(500_000), 3)
	if mv != rules.Move(mustCell(t, "E5")) || stats.Score != -m || stats.PVLen != 0 || stats.Depth != 3 {
		t.Errorf("root sentinel craft: mv=%d score=%d pvlen=%d depth=%d, want E5 -M 0 3", mv, stats.Score, stats.PVLen, stats.Depth)
	}
}

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

func TestMutKillExtensionBudget(t *testing.T) {
	b := mate1Board(t)
	b.Side = rules.Blue
	e := New(0)
	e.resetForSearch(b)
	e.negamax(b, 1, -config.EvalMateMax, config.EvalMateMax, 1, 0, NewFixedBudget(time.Second))
	if e.nodes != 2 {
		t.Errorf("extLeft=0: nodes=%d, want 2 (no extension)", e.nodes)
	}
	e = New(0)
	e.resetForSearch(b)
	e.negamax(b, 1, -config.EvalMateMax, config.EvalMateMax, 1, 1, NewFixedBudget(time.Second))
	if e.nodes != 82 {
		t.Errorf("extLeft=1: nodes=%d, want 82 (extension fires)", e.nodes)
	}
}

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

func TestMutKillScoutWindowBoundary(t *testing.T) {
	cb := mutCheckerBoard(t)
	e := New(testTTBytes)
	e.beginSearch(cb)
	seedChildTT(t, e, cb, mustCell(t, "H7"), 1, 1)
	sc, mv := e.searchRoot(cb, 3, NewFixedBudget(time.Second))
	if e.nodes != 100 || sc != 24900 || mv != rules.Move(mustCell(t, "H8")) {
		t.Errorf("root scout craft: nodes=%d score=%d mv=%d, want 100 24900 H8", e.nodes, sc, mv)
	}

	e = New(testTTBytes)
	e.beginSearch(cb)
	seedChildTT(t, e, cb, mustCell(t, "H7"), 1, 2)
	rv := e.negamax(cb, 3, -config.EvalMateMax, config.EvalMateMax, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if e.nodes != 101 || rv != 24900 {
		t.Errorf("interior scout craft: nodes=%d rv=%d, want 101 24900", e.nodes, rv)
	}
}

func TestMutKillRootWinClearsChildRow(t *testing.T) {
	b := mate1Board(t)
	e := New(0)
	e.beginSearch(b)
	e.pvLen[1] = 2
	e.pv[1][0] = rules.Move(mustCell(t, "A1"))
	e.pv[1][1] = rules.Move(mustCell(t, "A2"))
	sc, mv := e.searchRoot(b, 2, NewFixedBudget(time.Second))
	if mv != rules.Move(mustCell(t, "I9")) || sc != config.EvalMateMax-config.EvalMateScoreStep {
		t.Fatalf("root win craft: mv=%d score=%d, want I9 and mate in 1", mv, sc)
	}
	if e.pvLen[0] != 1 || e.pv[0][0] != mv {
		t.Fatalf("winning pv len=%d head=%d, want 1 and the move: a stale child row leaked", e.pvLen[0], e.pv[0][0])
	}
}

func TestMutKillInteriorWinClearsChildRow(t *testing.T) {
	m := config.EvalMateMax
	b := mate1Board(t)
	e := New(0)
	e.beginSearch(b)
	e.pvLen[2] = 2
	e.pv[2][0] = rules.Move(mustCell(t, "A1"))
	e.pv[2][1] = rules.Move(mustCell(t, "A2"))
	rv := e.negamax(b, 2, m-3*config.EvalMateScoreStep, m, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if rv != m-2*config.EvalMateScoreStep {
		t.Fatalf("interior win craft: rv=%d, want mateWin(1)", rv)
	}
	if e.pvLen[1] != 1 || e.pv[1][0] != rules.Move(mustCell(t, "I9")) {
		t.Fatalf("interior win pv len=%d head=%d, want 1 and I9: a stale child row leaked", e.pvLen[1], e.pv[1][0])
	}
}

type relapseDL struct{ calls int }

func (d *relapseDL) Exceeded() bool { d.calls++; return d.calls == 1 || d.calls >= 3 }
func (d *relapseDL) Stop()          {}

func TestMutKillHeadGuardExitsOnFirstExceeded(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	mv, stats := e.Search(b, &relapseDL{})
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("relapsed search move %d illegal", mv)
	}
	if stats.Depth != 0 || stats.Nodes != 0 {
		t.Errorf("relapsed deadline: depth=%d nodes=%d, want 0 and 0: the head guard must exit on the first exceeded consult", stats.Depth, stats.Nodes)
	}
}

func drawBoard(t *testing.T) *rules.Board {
	t.Helper()
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
	return b
}

func TestMutKillRootDrawClearsChildRow(t *testing.T) {
	b := drawBoard(t)
	e := New(0)
	e.beginSearch(b)
	e.pvLen[1] = 2
	e.pv[1][0] = rules.Move(mustCell(t, "H8"))
	e.pv[1][1] = rules.Move(mustCell(t, "I9"))
	sc, mv := e.searchRoot(b, 3, NewFixedBudget(time.Second))
	if sc != 0 || mv != rules.Move(mustCell(t, "A1")) {
		t.Fatalf("root draw craft: sc=%d mv=%d, want 0 A1", sc, mv)
	}
	if e.pvLen[0] != 1 || e.pv[0][0] != mv {
		t.Fatalf("drawn pv len=%d head=%d, want 1 and the move: a stale child row leaked", e.pvLen[0], e.pv[0][0])
	}
}

func TestMutKillInteriorDrawClearsChildRow(t *testing.T) {
	b := drawBoard(t)
	e := New(0)
	e.beginSearch(b)
	e.pvLen[2] = 2
	e.pv[2][0] = rules.Move(mustCell(t, "H8"))
	e.pv[2][1] = rules.Move(mustCell(t, "I9"))
	rv := e.negamax(b, 1, -1, 1, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if rv != 0 {
		t.Fatalf("interior draw craft: rv=%d, want 0", rv)
	}
	if e.pvLen[1] != 1 || e.pv[1][0] != rules.Move(mustCell(t, "A1")) {
		t.Fatalf("interior drawn pv len=%d head=%d, want 1 and A1: a stale child row leaked", e.pvLen[1], e.pv[1][0])
	}
}

type consultDL struct {
	exceeded bool
	calls    int
}

func (d *consultDL) Exceeded() bool {
	d.calls++
	return d.calls > fakeDeadlineCallCap || d.exceeded
}
func (d *consultDL) Stop()                 {}
func (d *consultDL) Budget() time.Duration { return time.Nanosecond }

func TestMutKillHeadGuardConsultCount(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := &consultDL{exceeded: true}
	e.Search(b, dl)
	if dl.calls != 1 {
		t.Errorf("exceeded-at-entry search consulted the deadline %d times, want 1: the head guard must break, not continue", dl.calls)
	}
}

func TestMutKillSoftStopBreakNotContinue(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	dl := &consultDL{}
	mv, stats := e.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("soft-stopped search move %d illegal", mv)
	}
	if stats.Depth != 1 {
		t.Errorf("depth = %d, want 1: the head after the first banked iteration must be refused", stats.Depth)
	}
	if dl.calls != 2 {
		t.Errorf("soft-stopped search consulted the deadline %d times, want 2: the refused head must break, not continue", dl.calls)
	}
}
