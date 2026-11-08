package engine

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type onceExceededDeadline struct {
	calls atomic.Int32
}

func (d *onceExceededDeadline) Exceeded() bool { return d.calls.Add(1) == 1 }
func (d *onceExceededDeadline) Stop()          {}

func farCornerBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	place(t, b, rules.Red, "K10")
	place(t, b, rules.Blue, "L12")
	return b
}

func driverFallback(b *rules.Board) rules.Move {
	e := New(0)
	fb := *b
	e.resetForSearch(&fb)
	return e.fallbackMove(&fb)
}

func TestSMPKillServeHotHoldsParkWindow(t *testing.T) {
	s := newSMP(1, 0)
	start := time.Now()
	if s.serveHot(s.workers[0], &s.boards[0], &s.results[0]) {
		t.Fatal("serveHot reported a served job with an empty wake channel")
	}
	window := time.Duration(config.SearchWorkerParkDelayMs) * time.Millisecond
	if elapsed := time.Since(start); elapsed < window {
		t.Errorf("idle serveHot returned after %v, want at least the park window %v", elapsed, window)
	}
}

func TestSMPKillServeHotReportsServedJob(t *testing.T) {
	s := newSMP(1, testTTBytes)
	w, b, res := s.workers[0], &s.boards[0], &s.results[0]
	*b = *midgameBoard(t)
	w.resetForSearch(b)
	s.jobMaxDepth = 2
	s.halt.Store(false)
	s.haltDL.inner = NewFixedBudget(50 * time.Millisecond)
	s.runWG.Add(1)
	s.wake <- struct{}{}
	served := s.serveHot(w, b, res)
	if !served {
		t.Fatal("serveHot must report true after serving a queued job")
	}
	s.runWG.Wait()
	if res.completed == 0 {
		t.Error("hot-served job left no completed iteration")
	}
}

func TestSMPKillSearchReportsGrantedBudget(t *testing.T) {
	s := newSMP(1, 0)
	defer s.Close()
	_, stats := s.SearchDepth(farCornerBoard(t), cappedBudget(7*time.Millisecond, 1_000_000), 1)
	if stats.AllocNs != int64(7*time.Millisecond) {
		t.Errorf("alloc ns = %d, want the granted budget %d", stats.AllocNs, int64(7*time.Millisecond))
	}
}

func TestSMPKillNoIterationFallsBackCleanly(t *testing.T) {
	b := farCornerBoard(t)
	want := driverFallback(b)
	if want == 0 {
		t.Fatal("setup: fallback move is the zero cell")
	}
	s := newSMP(2, 0)
	defer s.Close()
	mv, stats := s.SearchDepth(b, NewFixedBudget(0), 8)
	if mv != want {
		t.Fatalf("zero-budget move = %d, want the fallback %d", mv, want)
	}
	if stats.Depth != 0 || stats.Nodes != 0 || stats.PVLen != 0 {
		t.Errorf("zero-budget stats depth %d nodes %d pvlen %d, want all 0", stats.Depth, stats.Nodes, stats.PVLen)
	}
}

func TestSMPKillDepthLoopExitsWhenExceeded(t *testing.T) {
	b := farCornerBoard(t)
	want := driverFallback(b)
	s := newSMP(1, 0)
	defer s.Close()
	mv, stats := s.SearchDepth(b, &onceExceededDeadline{}, 2)
	if mv != want {
		t.Fatalf("move = %d, want the fallback %d", mv, want)
	}
	if stats.Depth != 0 || stats.Nodes != 0 {
		t.Errorf("stats depth %d nodes %d, want 0 and 0: the loop must not search past a first true reading", stats.Depth, stats.Nodes)
	}
}

type countingBudgetDeadline struct {
	calls atomic.Int32
}

func (d *countingBudgetDeadline) Exceeded() bool {
	d.calls.Add(1)
	return false
}

func (d *countingBudgetDeadline) Stop()                 {}
func (d *countingBudgetDeadline) Budget() time.Duration { return time.Nanosecond }

func twoHoleBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			if (r == 0 && c == 1) || (r == 1 && c == 0) {
				continue
			}
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	b.Side = rules.Red
	return b
}

func TestSMPKillSoftStopBreakEndsLoop(t *testing.T) {
	s := newSMP(1, testTTBytes)
	w, b, res := s.workers[0], &s.boards[0], &s.results[0]
	*b = *twoHoleBoard(t)
	w.resetForSearch(b)
	s.jobMaxDepth = 8
	s.jobSoft = true
	s.halt.Store(false)
	dl := &countingBudgetDeadline{}
	s.haltDL.inner = dl
	s.runWorker(w, b, res)
	if res.completed != 1 || res.move == moveNone {
		t.Fatalf("setup: completed %d move %d, want the quiet depth-1 bank with a real move", res.completed, res.move)
	}
	if got := dl.calls.Load(); got != 2 {
		t.Fatalf("deadline consulted %d times, want 2: after the soft rule fires the loop must end, not skip to the next head", got)
	}
}

const poisonChildScore = -(config.EvalMateMax - 2*config.EvalMateScoreStep)

const killSetupNodeBudget = 50000

func poisonWorker(b *rules.Board) *Engine {
	tt := newTT(1 << 21)
	bc := *b
	for c := range config.BoardCells {
		cell := rules.Cell(c)
		if !b.IsLegal(cell) {
			continue
		}
		bc.Make(cell)
		tt.store(bc.Hash, poisonChildScore, rules.Move(1), ttMaxDepth, ttBoundExact, 1)
		bc.Unmake()
	}
	return newEngineShared(tt)
}

func warmAndPark(t *testing.T, s *SMP, b *rules.Board) {
	t.Helper()
	_, _ = s.Search(b, cappedProbe(time.Microsecond))
	time.Sleep(3 * time.Duration(config.SearchWorkerParkDelayMs) * time.Millisecond)
}

func assertInstantWinWorker(t *testing.T, stats SearchStats, mv rules.Move) {
	t.Helper()
	if stats.Depth != config.SearchMaxPly {
		t.Errorf("depth = %d, want the full ladder %d", stats.Depth, config.SearchMaxPly)
	}
	if stats.Score != -config.EvalMateMax-1 {
		t.Errorf("score = %d, want the empty-root floor %d", stats.Score, -config.EvalMateMax-1)
	}
	if stats.PVLen != 0 || mv != moveNone {
		t.Errorf("pvlen %d move %d, want 0 and moveNone", stats.PVLen, mv)
	}
}

func TestSMPKillSelectionPrefersDeeperWorker(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.workers[1].radius = 0
	warmAndPark(t, s, b)
	mv, stats := s.Search(b, cappedNodeBudget(150_000))
	if s.results[1].completed != config.SearchMaxPly {
		t.Fatalf("setup: radius-zero worker completed %d", s.results[1].completed)
	}
	if s.results[0].completed == 0 || s.results[0].completed >= config.SearchMaxPly {
		t.Fatalf("setup: real worker completed %d, want inside (0, %d)", s.results[0].completed, config.SearchMaxPly)
	}
	assertInstantWinWorker(t, stats, mv)
}

func TestSMPKillSelectionTieBreaksByArrival(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.workers[0].radius = 0
	s.workers[1] = poisonWorker(b)
	for attempt := 1; ; attempt++ {
		warmAndPark(t, s, b)
		mv, stats := s.Search(b, cappedNodeBudget(killSetupNodeBudget))
		r0, r1 := &s.results[0], &s.results[1]
		if r0.completed != config.SearchMaxPly || r1.completed != config.SearchMaxPly {
			if attempt < 5 {
				continue
			}
			t.Fatalf("setup: completions %d and %d, want both %d", r0.completed, r1.completed, config.SearchMaxPly)
		}
		if r0.seq >= r1.seq {
			if attempt < 5 {
				continue
			}
			t.Fatalf("setup: the instant worker never arrived first in %d attempts", attempt)
		}
		if r1.nodes > 5000 {
			t.Fatalf("setup: poison worker searched %d nodes, a child entry was missed", r1.nodes)
		}
		assertInstantWinWorker(t, stats, mv)
		return
	}
}

func TestSMPKillArrivalCounterStrictlyIncreases(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.workers[0] = poisonWorker(b)
	s.workers[1].radius = 0
	for attempt := 1; ; attempt++ {
		warmAndPark(t, s, b)
		mv, stats := s.Search(b, cappedNodeBudget(killSetupNodeBudget))
		r0, r1 := &s.results[0], &s.results[1]
		if r0.completed != config.SearchMaxPly || r1.completed != config.SearchMaxPly {
			if attempt < 5 {
				continue
			}
			t.Fatalf("setup: completions %d and %d, want both %d", r0.completed, r1.completed, config.SearchMaxPly)
		}
		if r1.seq >= r0.seq {
			if attempt < 5 {
				continue
			}
			t.Fatalf("setup: the instant worker never arrived first in %d attempts", attempt)
		}
		if r0.nodes > 5000 {
			t.Fatalf("setup: poison worker searched %d nodes, a child entry was missed", r0.nodes)
		}
		assertInstantWinWorker(t, stats, mv)
		return
	}
}

func TestSMPKillTTHitPermilleSingleProbe(t *testing.T) {
	b := rules.NewBoard()
	s := newSMP(1, 1<<12)
	defer s.Close()
	bc := *b
	bc.Make(rules.Cell(config.SearchEmptyBoardCell))
	s.tt.store(bc.Hash, 37, rules.Move(1), 8, ttBoundExact, 1)
	_, stats := s.SearchDepth(b, cappedNodeBudget(500_000), 2)
	r := &s.results[0]
	if r.ttProbes != 1 || r.ttHits != 1 {
		t.Fatalf("setup: probes %d hits %d, want exactly 1 and 1", r.ttProbes, r.ttHits)
	}
	if stats.TTHitPermille != 1000 {
		t.Errorf("hit permille = %d, want 1000", stats.TTHitPermille)
	}
}

func TestSMPKillTTHitPermilleMatchesWorkers(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	_, stats := s.SearchDepth(midgameBoard(t), cappedNodeBudget(500_000), 3)
	r := &s.results[0]
	if r.ttProbes < 2 || r.ttHits == 0 {
		t.Fatalf("setup: probes %d hits %d, want a multi-probe run with hits", r.ttProbes, r.ttHits)
	}
	if want := int(r.ttHits * 1000 / r.ttProbes); stats.TTHitPermille != want {
		t.Errorf("hit permille = %d, want the worker ratio %d", stats.TTHitPermille, want)
	}
}

func TestSMPKillCutPermilleAllFirstMoveCuts(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	_, stats := s.SearchDepth(mate2Board(t), cappedNodeBudget(500_000), 1)
	r := &s.results[0]
	if r.cutNodes != 3 || r.cutFirst != 3 {
		t.Fatalf("setup: cut nodes %d first %d, want exactly 3 and 3", r.cutNodes, r.cutFirst)
	}
	if stats.FirstMoveFailHighPermille != 1000 {
		t.Errorf("cut permille = %d, want 1000", stats.FirstMoveFailHighPermille)
	}
}

func TestSMPKillCutPermilleMatchesWorkers(t *testing.T) {
	s := newSMP(1, testTTBytes)
	defer s.Close()
	_, stats := s.SearchDepth(midgameBoard(t), cappedNodeBudget(500_000), 3)
	r := &s.results[0]
	if r.cutNodes < 2 || r.cutFirst == 0 {
		t.Fatalf("setup: cut nodes %d first %d, want a multi-cutoff run", r.cutNodes, r.cutFirst)
	}
	if want := int(r.cutFirst * 1000 / r.cutNodes); stats.FirstMoveFailHighPermille != want {
		t.Errorf("cut permille = %d, want the worker ratio %d", stats.FirstMoveFailHighPermille, want)
	}

}

func TestSMPKillPVLenFromRootSlot(t *testing.T) {
	b := farCornerBoard(t)
	s := newSMP(1, testTTBytes)
	defer s.Close()
	mv, stats := s.SearchDepth(b, cappedNodeBudget(500_000), 1)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("move %d illegal", mv)
	}
	if stats.PVLen != 1 || stats.PV[0] != mv {
		t.Errorf("pv len %d head %d, want 1 and %d", stats.PVLen, stats.PV[0], mv)
	}
}

func TestSMPKillImmediateWinHaltsPool(t *testing.T) {
	s := newSMP(1, testTTBytes)
	w, b, res := s.workers[0], &s.boards[0], &s.results[0]
	*b = *mate1Board(t)
	w.resetForSearch(b)
	s.jobMaxDepth = 3
	s.halt.Store(false)
	s.haltDL.inner = NewFixedBudget(time.Second)
	s.runWorker(w, b, res)
	if !s.halt.Load() {
		t.Error("immediate win left the shared halt flag unset")
	}
	if res.completed != 2 {
		t.Errorf("completed = %d, want the win iteration plus its verification", res.completed)
	}
}

func TestSMPKillEnsureProcsExactTarget(t *testing.T) {
	old := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(old)
	runtime.GOMAXPROCS(1)
	s := newSMP(2, 0)
	s.ensureProcs()
	if got := runtime.GOMAXPROCS(0); got != 3 {
		t.Errorf("GOMAXPROCS = %d after ensureProcs, want exactly workers+1 = 3", got)
	}
}

func TestTTKillZeroDataWordNeverCuts(t *testing.T) {
	tb := newTT(1 << 10)
	score, move, cutoff := tb.probe(0, 0, -1000, 1000, 0)
	if cutoff {
		t.Error("empty slot with matching zero key produced a cutoff")
	}
	if score != 0 || move != 0 {
		t.Errorf("empty slot probe = %d %d, want 0 0", score, move)
	}
}

func TestTTKillMateFenceRoundTrip(t *testing.T) {
	ply := 3
	step := config.EvalMateScoreStep
	if got := scoreToTT(config.EvalMateFloor, ply); got != config.EvalMateFloor+ply*step {
		t.Errorf("scoreToTT(+fence) = %d, want %d", got, config.EvalMateFloor+ply*step)
	}
	if got := scoreToTT(-config.EvalMateFloor, ply); got != -config.EvalMateFloor-ply*step {
		t.Errorf("scoreToTT(-fence) = %d, want %d", got, -config.EvalMateFloor-ply*step)
	}
	if got := scoreFromTT(config.EvalMateFloor, ply); got != config.EvalMateFloor-ply*step {
		t.Errorf("scoreFromTT(+fence) = %d, want %d", got, config.EvalMateFloor-ply*step)
	}
	if got := scoreFromTT(-config.EvalMateFloor, ply); got != -config.EvalMateFloor+ply*step {
		t.Errorf("scoreFromTT(-fence) = %d, want %d", got, -config.EvalMateFloor+ply*step)
	}
}

func TestTTKillGenBumpHoldsMaxGen(t *testing.T) {
	tb := newTT(16)
	tb.gen = 62
	tb.bumpGen()
	if tb.gen != 63 {
		t.Errorf("generation after bump from 62 = %d, want 63", tb.gen)
	}
	tb.bumpGen()
	if tb.gen != 1 {
		t.Errorf("generation after wrap = %d, want 1", tb.gen)
	}
}

func TestTTKillKeyMissTuple(t *testing.T) {
	tb := newTT(1 << 10)
	tb.store(0x1234, 500, 3, 4, ttBoundExact, 0)
	score, move, cutoff := tb.probe(0x1234^1<<40, 4, -1000, 1000, 0)
	if cutoff {
		t.Error("wrong-key probe produced a cutoff")
	}
	if score != 0 || move != -1 {
		t.Errorf("wrong-key probe = %d %d, want 0 -1", score, move)
	}
}

func TestTTKillShallowEntryMissTuple(t *testing.T) {
	tb := newTT(1 << 10)
	tb.store(999, 250, 7, 5, ttBoundExact, 0)
	score, move, cutoff := tb.probe(999, 6, -1000, 1000, 0)
	if cutoff {
		t.Error("probe deeper than the stored depth produced a cutoff")
	}
	if score != 0 {
		t.Errorf("shallow probe score = %d, want 0", score)
	}
	if move != 7 {
		t.Errorf("shallow probe move = %d, want the ordering move 7", move)
	}
}

func TestTTKillUpperBoundCutsAtAlpha(t *testing.T) {
	tb := newTT(1 << 10)
	tb.store(555, -900, 4, 4, ttBoundUpper, 0)
	score, _, cutoff := tb.probe(555, 4, -900, 0, 0)
	if !cutoff {
		t.Error("upper bound at alpha failed to cut")
	}
	if score != -900 {
		t.Errorf("upper bound cut score = %d, want -900", score)
	}
}

func TestTTKillLowerBelowBetaFallsThrough(t *testing.T) {
	tb := newTT(1 << 10)
	tb.store(777, 100, 3, 6, ttBoundLower, 0)
	score, move, cutoff := tb.probe(777, 6, 0, 150, 0)
	if cutoff {
		t.Error("lower bound below beta produced a cutoff")
	}
	if score != 0 || move != 3 {
		t.Errorf("fall-through probe = %d %d, want 0 3", score, move)
	}
}

func TestTTKillHashFullSamplesWithinBudget(t *testing.T) {
	tb := newTT(2048 * ttEntryBytes)
	tb.gen = 1
	for j := 1; j < 2048; j += 2 {
		tb.entries[2*j] = uint64(tb.gen<<2|ttBoundExact) << 56
	}
	if got := tb.hashFullPermille(); got != 0 {
		t.Errorf("hash full = %d, want 0: only slots off the sample stride hold entries", got)
	}
}

func TestTTKillHashFullCountsNonzeroWord(t *testing.T) {
	tb := newTT(4 * ttEntryBytes)
	tb.gen = 0
	tb.entries[0] = 1
	if got := tb.hashFullPermille(); got != 250 {
		t.Errorf("hash full = %d, want 250 for one filled slot of four", got)
	}
}
