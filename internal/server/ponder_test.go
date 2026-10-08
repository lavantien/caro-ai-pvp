package server

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func TestGrantDepthRingEmptyHasNoReference(t *testing.T) {
	var ring GrantDepthRing
	if ref, ok := ring.ReferenceDepth(1_000_000); ok || ref != 0 {
		t.Fatalf("empty ring reference = (%d, %t), want (0, false)", ref, ok)
	}
}

func TestGrantDepthRingExactGrantCompares(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(1_000_000, 12)
	if ref, ok := ring.ReferenceDepth(1_000_000); !ok || ref != 12 {
		t.Fatalf("same-grant reference = (%d, %t), want (12, true)", ref, ok)
	}
}

func TestGrantDepthRingBandBoundaries(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(500, 5)
	ring.Append(2000, 7)
	ring.Append(499, 9)
	ring.Append(2001, 11)
	if ref, ok := ring.ReferenceDepth(1000); !ok || ref != 7 {
		t.Fatalf("band reference = (%d, %t), want the max in-band depth (7, true)", ref, ok)
	}
	if _, ok := ring.ReferenceDepth(501); !ok {
		t.Error("grant 501 lost every comparable: 500 sits in [250.5, 1002]")
	}
	if _, ok := ring.ReferenceDepth(1999); !ok {
		t.Error("grant 1999 lost every comparable: 2000 sits in [999.5, 3998]")
	}
}

func TestGrantDepthRingEvictsOldestAtCapacity(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(100, 42)
	for range config.PonderDepthHistory {
		ring.Append(100, 10)
	}
	if ref, ok := ring.ReferenceDepth(100); !ok || ref != 10 {
		t.Fatalf("reference after the wrap = (%d, %t), want the oldest entry evicted so (10, true)", ref, ok)
	}
}

func TestGrantDepthRingZeroGrantBand(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(0, 4)
	ring.Append(1, 9)
	if ref, ok := ring.ReferenceDepth(0); !ok || ref != 4 {
		t.Fatalf("zero-grant reference = (%d, %t), want only the zero-grant entry (4, true)", ref, ok)
	}
}

func stablePonderStats(t *testing.T, depth, rootIters int, move rules.Move, window []int) engine.SearchStats {
	t.Helper()
	if len(window) != config.PonderStableIters {
		t.Fatalf("window = %d entries, want PonderStableIters %d", len(window), config.PonderStableIters)
	}
	st := engine.SearchStats{Depth: depth, RootIters: rootIters}
	n := min(config.PonderDepthHistory, rootIters)
	for i := range n {
		st.RootMoves[i] = move
		st.RootScores[i] = window[0]
	}
	if n >= len(window) {
		for k := range window {
			st.RootScores[n-len(window)+k] = window[k]
		}
	}
	return st
}

func TestAdoptPonderSolverProofIsExempt(t *testing.T) {
	var ring GrantDepthRing
	mv := rules.Move(mustCellT(t, "H8"))
	for _, tag := range []string{config.BotLogTagVCF, config.BotLogTagVCT} {
		if !AdoptPonder(engine.SearchStats{}, tag, 0, 1, ring, 1) {
			t.Errorf("%s proof with an empty ring and zero history must adopt", tag)
		}
		if !AdoptPonder(stablePonderStats(t, 0, 0, mv, []int{0, 0, 0}), tag, 0, 1, ring, 1) {
			t.Errorf("%s proof must adopt regardless of the clauses", tag)
		}
	}
}

func TestAdoptPonderRejectsTheEmptyZeroNodeStop(t *testing.T) {
	var ring GrantDepthRing
	if AdoptPonder(engine.SearchStats{}, "", 0, int64(time.Hour), ring, 1) {
		t.Fatal("zero-node stop with an empty ring must reject: no depth, no elapsed, no history")
	}
}

func TestAdoptPonderTimeAdequacyBoundary(t *testing.T) {
	var ring GrantDepthRing
	mv := rules.Move(mustCellT(t, "H8"))
	budget := int64(2 * time.Second)
	st := stablePonderStats(t, 10, config.PonderStableIters, mv, []int{100, 100, 100})
	if !AdoptPonder(st, "", int64(time.Second), budget, ring, 1) {
		t.Error("ponder elapsed at exactly the adopt fraction of the budget must adopt")
	}
	if AdoptPonder(st, "", int64(time.Second)-1, budget, ring, 1) {
		t.Error("ponder elapsed one nanosecond under the adopt fraction must reject")
	}
}

func TestAdoptPonderDepthAdequacy(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	var ring GrantDepthRing
	ring.Append(int64(time.Second), 10)
	huge := int64(time.Hour)
	atRef := stablePonderStats(t, 10, config.PonderStableIters, mv, []int{100, 100, 100})
	if !AdoptPonder(atRef, "", 0, huge, ring, int64(time.Second)) {
		t.Error("ponder depth at the comparable reference with no elapsed time must adopt")
	}
	below := stablePonderStats(t, 9, config.PonderStableIters, mv, []int{100, 100, 100})
	if AdoptPonder(below, "", 0, huge, ring, int64(time.Second)) {
		t.Error("ponder depth under the comparable reference must reject when time is inadequate")
	}
}

func TestAdoptPonderOutOfBandGrantsGiveNoReference(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	var ring GrantDepthRing
	ring.Append(10*int64(time.Second), 30)
	st := stablePonderStats(t, 20, config.PonderStableIters, mv, []int{100, 100, 100})
	if AdoptPonder(st, "", 0, int64(time.Hour), ring, int64(time.Second)) {
		t.Error("a ring with no in-band comparables cannot satisfy depth adequacy")
	}
}

func TestAdoptPonderStabilityWindow(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	other := rules.Move(mustCellT(t, "I8"))
	budget := int64(time.Second)
	elapsed := int64(2 * time.Second)
	var ring GrantDepthRing

	short := stablePonderStats(t, 10, config.PonderStableIters-1, mv, []int{0, 0, 0})
	if AdoptPonder(short, "", elapsed, budget, ring, 1) {
		t.Errorf("root history of %d iterations under PonderStableIters must reject", config.PonderStableIters-1)
	}

	st := stablePonderStats(t, 10, 5, mv, []int{100, 100, 100})
	st.RootMoves[4] = other
	if AdoptPonder(st, "", elapsed, budget, ring, 1) {
		t.Error("a best-move change in the final completed iteration must reject")
	}

	unstableMid := stablePonderStats(t, 10, 5, mv, []int{100, 100, 100})
	unstableMid.RootMoves[5-config.PonderStableIters] = other
	if AdoptPonder(unstableMid, "", elapsed, budget, ring, 1) {
		t.Error("an unstable tail over the stable window must reject")
	}

	if !AdoptPonder(stablePonderStats(t, 10, 5, mv, []int{100, 100, 100}), "", elapsed, budget, ring, 1) {
		t.Error("a stable tail with adequate time must adopt")
	}
}

func TestAdoptPonderScoreDropMargin(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	budget := int64(time.Second)
	elapsed := int64(2 * time.Second)
	var ring GrantDepthRing

	atMargin := stablePonderStats(t, 10, config.PonderStableIters, mv,
		[]int{100, 100, 100 + config.PonderScoreDropMargin})
	if !AdoptPonder(atMargin, "", elapsed, budget, ring, 1) {
		t.Error("a window drop at exactly the margin must adopt")
	}
	overMargin := stablePonderStats(t, 10, config.PonderStableIters, mv,
		[]int{100, 100, 100 + config.PonderScoreDropMargin + 1})
	if AdoptPonder(overMargin, "", elapsed, budget, ring, 1) {
		t.Error("a window drop one point over the margin must reject")
	}
	rise := stablePonderStats(t, 10, config.PonderStableIters, mv,
		[]int{100, 100, 100 - config.PonderScoreDropMargin - 1})
	if AdoptPonder(rise, "", elapsed, budget, ring, 1) {
		t.Error("a window rise beyond the margin must reject under the absolute drop law")
	}
}

func TestAdoptPonderRequiresEveryClause(t *testing.T) {
	mv := rules.Move(mustCellT(t, "H8"))
	other := rules.Move(mustCellT(t, "I8"))
	var ring GrantDepthRing
	ring.Append(int64(time.Second), 10)
	budget := int64(time.Second)
	elapsed := int64(2 * time.Second)

	unstable := stablePonderStats(t, 10, config.PonderStableIters, mv, []int{100, 100, 100})
	unstable.RootMoves[config.PonderStableIters-1] = other
	if AdoptPonder(unstable, "", elapsed, budget, ring, int64(time.Second)) {
		t.Error("time adequacy must not rescue an unstable tail")
	}

	drifty := stablePonderStats(t, 10, config.PonderStableIters, mv,
		[]int{100, 100, 100 + config.PonderScoreDropMargin + 1})
	if AdoptPonder(drifty, "", elapsed, budget, ring, int64(time.Second)) {
		t.Error("time adequacy must not rescue a score drop beyond the margin")
	}
}

type recordingInner struct {
	mu        sync.Mutex
	starts    int
	stops     int
	lastBoard *rules.Board
	stopMove  rules.Move
	stopStats engine.SearchStats
	stopTag   string
}

func (f *recordingInner) Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (f *recordingInner) StartPonder(b *rules.Board) {
	f.mu.Lock()
	f.starts++
	f.lastBoard = b
	f.mu.Unlock()
}

func (f *recordingInner) StopPonder() (rules.Move, engine.SearchStats, string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	return f.stopMove, f.stopStats, f.stopTag
}

func (f *recordingInner) Close() {}

func TestSingleSearcherNeverPonders(t *testing.T) {
	s := singleSearcher{e: engine.New(config.TierEasy.TTBytes)}
	defer s.Close()
	b := anchorBoard(t, rules.Red, r(0, 0), x(1, 1))
	s.StartPonder(b)
	mv, st, tag := s.StopPonder()
	if mv != 0 || st != (engine.SearchStats{}) || tag != "" {
		t.Fatalf("single searcher ponder stop = %d %+v %q, want zero values", mv, st, tag)
	}
}

func TestTierSearcherPonderDelegates(t *testing.T) {
	probe := config.Tier{Name: "ponder-probe", Cores: 2, TTBytes: 1 << 20}
	s := tierSearcher{smp: engine.NewTiered(probe)}
	defer s.Close()
	b := anchorBoard(t, rules.Red, r(0, 0), r(0, 1), x(1, 1), x(1, 2))
	s.StartPonder(b)
	mv, st, tag := s.StopPonder()
	if tag != "" {
		t.Fatalf("tier ponder tag = %q, want the empty string: adoption stamps [PONDER]", tag)
	}
	if mv != 0 && !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("tier ponder move %v is not legal on the ponder board", cellName(rules.Cell(mv)))
	}
	if n := min(config.PonderDepthHistory, st.RootIters); st.RootIters > 0 && st.RootMoves[n-1] != mv {
		t.Fatalf("tier ponder newest history move %d disagrees with the reported move %d", st.RootMoves[n-1], mv)
	}
	if mv2, st2, tag2 := s.StopPonder(); mv2 != 0 || st2 != (engine.SearchStats{}) || tag2 != "" {
		t.Fatalf("second tier ponder stop = %d %+v %q, want zero values", mv2, st2, tag2)
	}
}

func TestSolverSearcherPonderRecordsVCFProof(t *testing.T) {
	inner := &recordingInner{}
	s := &solverSearcher{inner: inner, vcf: vcf.New(vcf.KindVCF), vct: vcf.New(vcf.KindVCT), cores: 2}
	b := openFourBoard(t)
	s.StartPonder(b)
	inner.mu.Lock()
	starts, last := inner.starts, inner.lastBoard
	inner.mu.Unlock()
	if starts != 1 || last != b {
		t.Fatalf("inner start = %d on %p, want 1 on the ponder board", starts, last)
	}
	mv, st, tag := s.StopPonder()
	if tag != config.BotLogTagVCF {
		t.Fatalf("ponder proof tag = %q, want %q", tag, config.BotLogTagVCF)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Fatalf("ponder proof move = %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; st.Score != want {
		t.Fatalf("ponder proof score = %d, want the M1 lattice point %d", st.Score, want)
	}
	if st.Depth != 1 || st.Threads != 2 {
		t.Fatalf("ponder proof depth %d threads %d, want 1 and 2", st.Depth, st.Threads)
	}
	inner.mu.Lock()
	stops := inner.stops
	inner.mu.Unlock()
	if stops != 1 {
		t.Fatalf("inner stops = %d, want the collected inward result", stops)
	}
	mv2, _, tag2 := s.StopPonder()
	if tag2 != "" || mv2 != 0 {
		t.Fatalf("second ponder stop = %d %q, want the consumed proof gone", mv2, tag2)
	}
}

func TestSolverSearcherPonderRecordsVCTProof(t *testing.T) {
	inner := &recordingInner{}
	s := &solverSearcher{inner: inner, vct: vcf.New(vcf.KindVCT), cores: 1}
	s.StartPonder(doubleThreeBoard(t))
	mv, _, tag := s.StopPonder()
	if tag != config.BotLogTagVCT {
		t.Fatalf("ponder vct tag = %q, want %q", tag, config.BotLogTagVCT)
	}
	if mv == 0 {
		t.Fatal("ponder vct proof returned no move")
	}
}

func TestSolverSearcherPonderMissKeepsInwardResult(t *testing.T) {
	inner := &recordingInner{}
	want := engine.SearchStats{Depth: 3, Nodes: 77}
	inner.stopMove = rules.Move(mustCellT(t, "K10"))
	inner.stopStats = want
	s := &solverSearcher{inner: inner, vcf: vcf.New(vcf.KindVCF), cores: 1}
	s.StartPonder(quietBoard(t))
	mv, st, tag := s.StopPonder()
	if mv != inner.stopMove || st != want || tag != "" {
		t.Fatalf("ponder miss stop = %d %+v %q, want the inward result", mv, st, tag)
	}
}
