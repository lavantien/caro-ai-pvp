package server

import (
	"strings"
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

func TestPonderLaneStartStopHandshake(t *testing.T) {
	eng := &recordingInner{}
	eng.stopMove = rules.Move(mustCellT(t, "K10"))
	eng.stopStats = engine.SearchStats{Depth: 7, Nodes: 123}
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	b := rules.NewBoard()
	b.Make(mustCellT(t, "H8"))
	predict := rules.Move(mustCellT(t, "I8"))
	lane.start(eng, rules.Red, b, predict, 1)

	done := make(chan ponderResult, 1)
	go func() { done <- lane.stop(rules.Red) }()
	var res ponderResult
	select {
	case res = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lane stop handshake hung")
	}
	if !res.active {
		t.Fatal("stop after a queued start must report the drained ponder")
	}
	if res.move != eng.stopMove || res.stats != eng.stopStats {
		t.Fatalf("stop result = %d %+v, want the engine's %d %+v", res.move, res.stats, eng.stopMove, eng.stopStats)
	}
	if res.predict != predict || res.base != 1 {
		t.Fatalf("stop carries predict %d base %d, want %d and 1", res.predict, res.base, predict)
	}
	eng.mu.Lock()
	starts, stops, last := eng.starts, eng.stops, eng.lastBoard
	eng.mu.Unlock()
	if starts != 1 || stops != 1 {
		t.Fatalf("engine calls = %d starts %d stops, want 1 and 1", starts, stops)
	}
	if last == nil || last.MoveCount != 1 {
		t.Fatalf("ponder board = %+v, want the start board copy", last)
	}

	go func() { done <- lane.stop(rules.Red) }()
	select {
	case idle := <-done:
		if idle.active {
			t.Fatal("stop with no active ponder must report idle")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("idle stop hung")
	}

	lane.close()
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("lane did not exit after close")
	}
}

func TestPonderLaneStopAfterCloseIsEmpty(t *testing.T) {
	lane := newPonderLane()
	go lane.run()
	lane.close()
	res := make(chan ponderResult, 1)
	go func() { res <- lane.stop(rules.Red) }()
	select {
	case r := <-res:
		if r.active {
			t.Fatal("stop after close must write an empty result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop after close hung")
	}
}

func TestPonderLaneStartAfterCloseIsNoOp(t *testing.T) {
	eng := &recordingInner{}
	lane := newPonderLane()
	go lane.run()
	lane.close()
	lane.start(eng, rules.Red, rules.NewBoard(), 0, 0)
	time.Sleep(20 * time.Millisecond)
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if eng.starts != 0 {
		t.Fatalf("start after close reached the engine %d times, want none", eng.starts)
	}
}

func TestPonderLaneCloseWhilePonderingJoins(t *testing.T) {
	eng := &recordingInner{}
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	lane.start(eng, rules.Red, rules.NewBoard(), 0, 0)
	closed := make(chan struct{})
	go func() {
		lane.close()
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(2 * time.Second):
		t.Fatal("close while ponding hung")
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("lane did not exit after close while ponding")
	}
	eng.mu.Lock()
	defer eng.mu.Unlock()
	if eng.stops != 1 {
		t.Fatalf("close stopped the engine %d times, want the one active ponder halted", eng.stops)
	}
}

func TestPonderLaneEnqueueNeverBlocks(t *testing.T) {
	eng := &recordingInner{}
	lane := newPonderLane()
	b := rules.NewBoard()
	t0 := time.Now()
	lane.start(eng, rules.Red, b, 0, 0)
	if d := time.Since(t0); d > 100*time.Millisecond {
		t.Fatalf("enqueue blocked the caller for %s", d)
	}
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	done := make(chan ponderResult, 1)
	go func() { done <- lane.stop(rules.Red) }()
	select {
	case res := <-done:
		if !res.active {
			t.Fatal("the queued start must run before the stop result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop never drained the queued start")
	}
	lane.close()
	<-exited
}

func TestPonderLanePerSideIndependence(t *testing.T) {
	red, blue := &recordingInner{}, &recordingInner{}
	red.stopMove = rules.Move(mustCellT(t, "H8"))
	blue.stopMove = rules.Move(mustCellT(t, "K10"))
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	b := rules.NewBoard()
	lane.start(red, rules.Red, b, 1, 1)
	lane.start(blue, rules.Blue, b, 2, 2)

	done := make(chan ponderResult, 2)
	go func() { done <- lane.stop(rules.Red) }()
	var res ponderResult
	select {
	case res = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("red stop hung while blue still ponders")
	}
	if !res.active || res.move != red.stopMove || res.predict != 1 {
		t.Fatalf("red stop = %+v, want the red engine's drained ponder", res)
	}
	blue.mu.Lock()
	blueStops := blue.stops
	blue.mu.Unlock()
	if blueStops != 0 {
		t.Fatalf("red stop halted the blue engine %d times, want the seats independent", blueStops)
	}
	go func() { done <- lane.stop(rules.Blue) }()
	select {
	case res = <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("blue stop hung")
	}
	if !res.active || res.move != blue.stopMove || res.predict != 2 {
		t.Fatalf("blue stop = %+v, want the blue engine's drained ponder", res)
	}
	lane.close()
	<-exited
}

func TestPonderLaneSkipsStartWhileSeatPonders(t *testing.T) {
	first, second := &recordingInner{}, &recordingInner{}
	first.stopMove = rules.Move(mustCellT(t, "H8"))
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	b := rules.NewBoard()
	lane.start(first, rules.Red, b, 1, 1)
	lane.start(second, rules.Red, b, 2, 2)
	done := make(chan ponderResult, 1)
	go func() { done <- lane.stop(rules.Red) }()
	select {
	case res := <-done:
		if !res.active || res.predict != 1 {
			t.Fatalf("stop = %+v, want the first start's ponder kept", res)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop hung behind the duplicate start")
	}
	second.mu.Lock()
	starts := second.starts
	second.mu.Unlock()
	if starts != 0 {
		t.Fatalf("second start reached the engine %d times, want it dropped while the seat ponders", starts)
	}
	lane.close()
	<-exited
}

type ponderAnswer struct {
	move  rules.Move
	pv    []rules.Move
	stats engine.SearchStats
}

type ponderBot struct {
	mu        sync.Mutex
	closed    bool
	searches  int
	starts    int
	stops     int
	answers   []ponderAnswer
	stopMove  rules.Move
	stopStats engine.SearchStats
	stopTag   string
	lastStart rules.Board
}

func (b *ponderBot) Search(bd *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.searches++
	if len(b.answers) == 0 {
		var buf [config.BoardCells]rules.Move
		n := bd.LegalMoves(buf[:])
		st := engine.SearchStats{Depth: 1, Threads: 1}
		st.PV[0] = buf[n-1]
		st.PVLen = 1
		return buf[n-1], st, ""
	}
	a := b.answers[0]
	b.answers = b.answers[1:]
	st := a.stats
	st.PVLen = len(a.pv)
	for i, m := range a.pv {
		st.PV[i] = m
	}
	return a.move, st, ""
}

func (b *ponderBot) StartPonder(bd *rules.Board) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.starts++
	b.lastStart = *bd
}

func (b *ponderBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.stops++
	return b.stopMove, b.stopStats, b.stopTag
}

func (b *ponderBot) Close() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
}

func (b *ponderBot) counts() (searches, starts, stops int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.searches, b.starts, b.stops
}

func (b *ponderBot) isClosed() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.closed
}

func adoptableStopStats(move rules.Move, depth int, elapsedNs int64) engine.SearchStats {
	st := engine.SearchStats{Depth: depth, Nodes: 500, ElapsedNs: elapsedNs,
		RootIters: config.PonderStableIters, Threads: 8}
	for i := range config.PonderStableIters {
		st.RootMoves[i] = move
		st.RootScores[i] = 100
	}
	st.PV[0] = move
	st.PVLen = 1
	return st
}

func ponderRoom(t *testing.T, s *stack, bot *ponderBot) (User, *Room) {
	t.Helper()
	alice := seedUser(t, s.store, "alice")
	tier := config.TierMaster
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier)
	if err != nil {
		t.Fatalf("create vs master: %v", err)
	}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.budgetCap = 3 * time.Millisecond
	r.mu.Unlock()
	return alice, r
}

func waitBotMove(t *testing.T, r *Room, moveCount int, want rules.Move) {
	t.Helper()
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.board != nil && r.board.MoveCount == moveCount && r.moves[moveCount-1] == want
	})
}

func TestPonderArmsAfterOwnMoveAndAdoptsOnHit(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "K10", "J10"})
	d4, h8, p16, k10, j10 := ms[0], ms[1], ms[2], ms[3], ms[4]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
		{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
	}
	bot.stopMove = k10
	bot.stopStats = adoptableStopStats(k10, 12, int64(10*time.Millisecond))
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	waitFor(t, func() bool {
		_, starts, _ := bot.counts()
		return starts == 1
	})
	bot.mu.Lock()
	startBoard := bot.lastStart
	bot.mu.Unlock()
	if startBoard.MoveCount != 3 || startBoard.Side != rules.Blue {
		t.Fatalf("ponder board = %d stones side %v, want 3 with blue to move", startBoard.MoveCount, startBoard.Side)
	}

	if err := r.PlayMove(alice.ID, rules.Cell(p16)); err != nil {
		t.Fatalf("predicted reply: %v", err)
	}
	waitBotMove(t, r, 4, k10)

	searches, starts, stops := bot.counts()
	if searches != 1 {
		t.Fatalf("searches after the adopted turn = %d, want 1: adoption must not search", searches)
	}
	if starts != 1 || stops != 1 {
		t.Fatalf("ponder counts = %d starts %d stops, want 1 and 1", starts, stops)
	}
	r.mu.Lock()
	rec := r.lastM
	line := r.mlines[len(r.mlines)-1].Line
	r.mu.Unlock()
	if rec.tag != config.BotLogTagPonder || rec.move != k10 {
		t.Fatalf("last mline record = move %d tag %q, want the adopted %d with %q", rec.move, rec.tag, k10, config.BotLogTagPonder)
	}
	if want := int64(3 * time.Millisecond); rec.stats.AllocNs != want {
		t.Fatalf("adopted alloc = %d, want the budget draw %d", rec.stats.AllocNs, want)
	}
	if want := engine.NpsReport(500, int64(10*time.Millisecond)); rec.stats.Nps != want {
		t.Fatalf("adopted nps = %d, want the ponder search's %d", rec.stats.Nps, want)
	}
	if rec.stats.Depth != 12 || rec.stats.ElapsedNs < 0 {
		t.Fatalf("adopted stats depth %d elapsed %d, want the ponder depth 12 and a real turn wall", rec.stats.Depth, rec.stats.ElapsedNs)
	}
	if !strings.Contains(line, ", [PONDER], pv=K10") {
		t.Fatalf("adopted line = %q, want the ponder tag before the pv", line)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderMissFallsThroughToSearch(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "M4", "J10"})
	d4, h8, p16, m4, j10 := ms[0], ms[1], ms[2], ms[3], ms[4]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
		{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
	}
	bot.stopMove = rules.Move(mustCellT(t, "K10"))
	bot.stopStats = adoptableStopStats(bot.stopMove, 12, int64(10*time.Millisecond))
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	if err := r.PlayMove(alice.ID, rules.Cell(m4)); err != nil {
		t.Fatalf("surprise reply: %v", err)
	}
	waitBotMove(t, r, 4, j10)
	searches, starts, stops := bot.counts()
	if searches != 2 {
		t.Fatalf("searches after the missed ponder = %d, want the normal second search", searches)
	}
	if starts != 1 || stops != 1 {
		t.Fatalf("ponder counts = %d starts %d stops, want the one armed and drained", starts, stops)
	}
	r.mu.Lock()
	rec := r.lastM
	r.mu.Unlock()
	if rec.tag != "" || rec.move != j10 {
		t.Fatalf("last record = move %d tag %q, want the searched move untagged", rec.move, rec.tag)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderGateRejectFallsThroughToSearch(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "K10", "J10"})
	d4, h8, p16, k10, j10 := ms[0], ms[1], ms[2], ms[3], ms[4]
	other := rules.Move(mustCellT(t, "L9"))
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
		{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
	}
	bot.stopMove = k10
	bot.stopStats = adoptableStopStats(k10, 12, int64(10*time.Millisecond))
	bot.stopStats.RootMoves[config.PonderStableIters-1] = other
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	if err := r.PlayMove(alice.ID, rules.Cell(p16)); err != nil {
		t.Fatalf("predicted reply: %v", err)
	}
	waitBotMove(t, r, 4, j10)
	searches, _, stops := bot.counts()
	if searches != 2 {
		t.Fatalf("searches after the rejected ponder = %d, want the fallback search", searches)
	}
	if stops != 1 {
		t.Fatalf("stops = %d, want the drained ponder", stops)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderSolverProofAdoptsWithoutClauses(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "K10"})
	d4, h8, p16, k10 := ms[0], ms[1], ms[2], ms[3]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
	}
	bot.stopMove = k10
	bot.stopStats = engine.SearchStats{}
	bot.stopTag = config.BotLogTagVCF
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	if err := r.PlayMove(alice.ID, rules.Cell(p16)); err != nil {
		t.Fatalf("predicted reply: %v", err)
	}
	waitBotMove(t, r, 4, k10)
	searches, _, _ := bot.counts()
	if searches != 1 {
		t.Fatalf("searches after the proof adoption = %d, want 1", searches)
	}
	r.mu.Lock()
	rec := r.lastM
	r.mu.Unlock()
	if rec.tag != config.BotLogTagVCF {
		t.Fatalf("proof adoption tag = %q, want %q", rec.tag, config.BotLogTagVCF)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderNoArmWithoutPredictedReply(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "J10"})
	d4, h8, p16, j10 := ms[0], ms[1], ms[2], ms[3]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
		{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
	}
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	time.Sleep(100 * time.Millisecond)
	if _, starts, _ := bot.counts(); starts != 0 {
		t.Fatalf("starts after a PVLen-1 move = %d, want no arm without a predicted reply", starts)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(p16)); err != nil {
		t.Fatalf("human reply: %v", err)
	}
	waitBotMove(t, r, 4, j10)
	if searches, _, stops := bot.counts(); searches != 2 || stops != 0 {
		t.Fatalf("counts = %d searches %d stops, want the idle handshakes only", searches, stops)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderCloseMidPonderJoins(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16"})
	d4, h8, p16 := ms[0], ms[1], ms[2]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
	}
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	waitFor(t, func() bool {
		_, starts, _ := bot.counts()
		return starts == 1
	})
	closed := make(chan struct{})
	go func() {
		defer close(closed)
		r.Close()
	}()
	select {
	case <-closed:
	case <-time.After(5 * time.Second):
		t.Fatal("room close hung mid-ponder")
	}
	waitFor(t, bot.isClosed)
	if _, _, stops := bot.counts(); stops != 1 {
		t.Fatalf("stops during close = %d, want the drained ponder", stops)
	}
}

func TestPonderForfeitMidPonderJoins(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16"})
	d4, h8, p16 := ms[0], ms[1], ms[2]
	bot := &ponderBot{}
	bot.answers = []ponderAnswer{
		{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
	}
	alice, r := ponderRoom(t, s, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	waitFor(t, func() bool {
		_, starts, _ := bot.counts()
		return starts == 1
	})
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit mid-ponder: %v", err)
	}
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	waitFor(t, bot.isClosed)
}

func TestPonderGameResetDrainsMidPonder(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	tier := config.TierMaster
	var gmu sync.Mutex
	var bots []*ponderBot
	scatter := []string{"H8", "N14", "L12", "J10"}
	dfive := []string{"D4", "D8", "D5", "D6", "D7"}
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier)
	if err != nil {
		t.Fatalf("create vs master: %v", err)
	}
	r.mu.Lock()
	r.budgetCap = 3 * time.Millisecond
	predicted := movesOf(t, []string{"P16"})[0]
	game1Answers := make([]ponderAnswer, 0, len(scatter))
	for i, name := range scatter {
		ans := ponderAnswer{move: movesOf(t, []string{name})[0], stats: engine.SearchStats{Depth: 12, Threads: 8}}
		if i == len(scatter)-1 {
			ans.pv = []rules.Move{ans.move, predicted}
		} else {
			ans.pv = []rules.Move{ans.move}
		}
		game1Answers = append(game1Answers, ans)
	}
	r.makeSearcher = func(config.Tier) searcher {
		b := &ponderBot{}
		gmu.Lock()
		if len(bots) == 0 {
			b.answers = append([]ponderAnswer(nil), game1Answers...)
		}
		bots = append(bots, b)
		gmu.Unlock()
		return b
	}
	r.mu.Unlock()
	game1 := func() *ponderBot {
		gmu.Lock()
		defer gmu.Unlock()
		return bots[0]
	}

	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for i := range 4 {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, dfive[i])); err != nil {
			t.Fatalf("human move %s: %v", dfive[i], err)
		}
	}
	waitFor(t, func() bool {
		_, starts, _ := game1().counts()
		return starts == 1
	})
	if err := r.PlayMove(alice.ID, mustCellT(t, dfive[4])); err != nil {
		t.Fatalf("winning move: %v", err)
	}
	var game2 *ponderBot
	waitFor(t, func() bool {
		gmu.Lock()
		defer gmu.Unlock()
		if len(bots) < 2 {
			return false
		}
		game2 = bots[1]
		return true
	})
	waitFor(t, game1().isClosed)
	if _, _, stops := game1().counts(); stops != 1 {
		t.Fatalf("game-1 stops = %d, want the reset-drained ponder", stops)
	}
	finalBot := game2
	waitFor(t, func() bool {
		searches, _, _ := finalBot.counts()
		return searches >= 1
	})
	r.Close()
}

func TestBotVsBotPonderMLineObserved(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "P16", "H8", "N14", "J10"})
	d4, p16, h8, n14, j10 := ms[0], ms[1], ms[2], ms[3], ms[4]
	gate := make(chan struct{})
	var once sync.Once
	master := &ponderBot{}
	master.answers = []ponderAnswer{
		{move: d4, pv: []rules.Move{d4, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
		{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
	}
	master.stopMove = h8
	master.stopStats = adoptableStopStats(h8, 12, int64(time.Hour))
	hard := &ponderBot{}
	hard.answers = []ponderAnswer{
		{move: p16, pv: []rules.Move{p16}, stats: engine.SearchStats{Depth: 9, Threads: 4}},
		{move: n14, pv: []rules.Move{n14}, stats: engine.SearchStats{Depth: 9, Threads: 4}},
	}
	s.rm.makeSearcher = func(tier config.Tier) searcher {
		if tier.Name != config.TierMaster.Name {
			return hard
		}
		return &startGatedPonderBot{gate: gate, once: &once, inner: master}
	}
	r, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierHard, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create master vs hard: %v", err)
	}
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	events := make(chan Event, 64)
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range sub.Events() {
			events <- ev
		}
	}()
	close(gate)

	var seq []string
	var mlines []string
	deadline := time.After(10 * time.Second)
	for len(seq) < 10 {
		select {
		case ev := <-events:
			switch ev.Kind {
			case EventKindMove:
				seq = append(seq, ev.Payload)
			case EventKindMLine:
				mlines = append(mlines, ev.Payload)
			}
		case <-deadline:
			t.Fatalf("moves observed = %v, want the five scripted stones", seq)
		}
	}
	want := []string{"D4", "P16", "H8", "N14", "J10"}
	for i, w := range want {
		if seq[i] != w {
			t.Fatalf("move %d = %s, want %s (seq %v)", i, seq[i], w, seq)
		}
	}
	sub.Unsubscribe()
	<-drained
	ponderLines := 0
	for _, line := range mlines {
		if strings.Contains(line, ", [PONDER],") {
			ponderLines++
		}
	}
	if ponderLines == 0 {
		searches, starts, stops := master.counts()
		t.Fatalf("no [PONDER] mline observed through the subscription: master counts %d/%d/%d, mlines %v",
			searches, starts, stops, mlines)
	}
	if _, starts, stops := master.counts(); starts != 1 || stops != 1 {
		t.Fatalf("master ponder counts = %d starts %d stops, want the one arm and drain", starts, stops)
	}
	r.Close()
}

type startGatedPonderBot struct {
	gate  chan struct{}
	once  *sync.Once
	inner *ponderBot
}

func (b *startGatedPonderBot) Search(bd *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.once.Do(func() { <-b.gate })
	return b.inner.Search(bd, dl)
}

func (b *startGatedPonderBot) StartPonder(bd *rules.Board) { b.inner.StartPonder(bd) }

func (b *startGatedPonderBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return b.inner.StopPonder()
}

func (b *startGatedPonderBot) Close() { b.inner.Close() }
