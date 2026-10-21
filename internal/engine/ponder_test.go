package engine

import (
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestPondererSurfaceMatchesSMP(t *testing.T) {
	s := NewTiered(config.TierEasy)
	var p Ponderer = s
	p.StartPonder(midgameBoard(t))
	p.StopPonder()
	s.Close()
}

func TestSMPPonderWidthReportsActiveWorkers(t *testing.T) {
	s := newSMPPonder(4, testTTBytes, 2)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	_, stats := s.StopPonder()
	if stats.Threads != 2 {
		t.Fatalf("ponder stop threads = %d, want the 2-worker ponder width", stats.Threads)
	}
	_, stats = s.SearchDepth(midgameBoard(t), NewFixedBudget(scaledBudget(120*time.Millisecond)), 4)
	if stats.Threads != 4 {
		t.Fatalf("post-ponder search threads = %d, want the full 4-worker width", stats.Threads)
	}
}

func TestSMPPonderWidthZeroNeverPonders(t *testing.T) {
	s := newSMPPonder(2, testTTBytes, 0)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	mv, stats := s.StopPonder()
	if mv != 0 || stats != (SearchStats{}) {
		t.Fatalf("zero-width ponder stop = %d %+v, want the no-op zeros", mv, stats)
	}
}

func TestSMPPonderWidthOnePonders(t *testing.T) {
	s := newSMPPonder(2, testTTBytes, 1)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	time.Sleep(scaledBudget(500 * time.Millisecond))
	_, stats := s.StopPonder()
	if stats.RootIters < 1 {
		t.Fatalf("width-1 ponder banked %d iterations, want the dispatched worker searching", stats.RootIters)
	}
	if stats.Threads != 1 {
		t.Fatalf("width-1 ponder threads = %d, want the single dispatched worker", stats.Threads)
	}
}

func TestSMPPonderWidthEngagesOnlyDispatchedWorkers(t *testing.T) {
	s := newSMPPonder(4, testTTBytes, 2)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	time.Sleep(scaledBudget(500 * time.Millisecond))
	s.StopPonder()
	active := 0
	for i := range s.results {
		if s.results[i].nodes > 0 {
			active++
		}
	}
	if active != 2 {
		t.Fatalf("ponder engaged %d workers, want the dispatched width 2", active)
	}
}

func TestSMPPonderWidthClampsToPoolAndFloor(t *testing.T) {
	s := newSMPPonder(2, testTTBytes, 5)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	if _, stats := s.StopPonder(); stats.Threads != 2 {
		t.Fatalf("over-wide ponder threads = %d, want the clamp to the 2-worker pool", stats.Threads)
	}
	z := newSMPPonder(2, testTTBytes, -1)
	defer z.Close()
	z.StartPonder(midgameBoard(t))
	if mv, stats := z.StopPonder(); mv != 0 || stats != (SearchStats{}) {
		t.Fatalf("negative-width ponder stop = %d %+v, want the no-op zeros", mv, stats)
	}
}

func TestSearchRootRingMatchesIterations(t *testing.T) {
	b := playout(t, 4242, 24)
	const maxIter = config.PonderDepthHistory + 2
	var moves [maxIter]rules.Move
	var scores [maxIter]int
	for d := 1; d <= maxIter; d++ {
		s := newSMP(1, testTTBytes)
		mv, stats := s.SearchDepth(b, NewStoppable(), d)
		s.Close()
		if stats.Depth != d {
			t.Fatalf("depth %d run reached only depth %d, budget too small", d, stats.Depth)
		}
		if stats.RootIters != d {
			t.Fatalf("depth %d run reports %d root iterations", d, stats.RootIters)
		}
		moves[d-1], scores[d-1] = mv, stats.Score
		n := min(d, config.PonderDepthHistory)
		for j := range n {
			want := d - n + j
			if stats.RootMoves[j] != moves[want] {
				t.Fatalf("depth %d ring slot %d move %d, want iteration %d move %d", d, j, stats.RootMoves[j], want+1, moves[want])
			}
			if stats.RootScores[j] != scores[want] {
				t.Fatalf("depth %d ring slot %d score %d, want iteration %d score %d", d, j, stats.RootScores[j], want+1, scores[want])
			}
		}
	}
}

func TestEngineSearchRootRingMatchesIterations(t *testing.T) {
	b := playout(t, 4242, 24)
	const maxIter = config.PonderDepthHistory + 2
	var moves [maxIter]rules.Move
	var scores [maxIter]int
	for d := 1; d <= maxIter; d++ {
		e := New(testTTBytes)
		mv, stats := e.SearchDepth(b, NewStoppable(), d)
		if stats.Depth != d {
			t.Fatalf("depth %d run reached only depth %d, ladder starved", d, stats.Depth)
		}
		if stats.RootIters != d {
			t.Fatalf("depth %d run reports %d root iterations", d, stats.RootIters)
		}
		moves[d-1], scores[d-1] = mv, stats.Score
		n := min(d, config.PonderDepthHistory)
		for j := range n {
			want := d - n + j
			if stats.RootMoves[j] != moves[want] {
				t.Fatalf("depth %d ring slot %d move %d, want iteration %d move %d", d, j, stats.RootMoves[j], want+1, moves[want])
			}
			if stats.RootScores[j] != scores[want] {
				t.Fatalf("depth %d ring slot %d score %d, want iteration %d score %d", d, j, stats.RootScores[j], want+1, scores[want])
			}
		}
	}
}

func TestEngineRingMatchesSMPWidthOne(t *testing.T) {
	b := playout(t, 4242, 24)
	const depth = config.PonderDepthHistory
	e := New(testTTBytes)
	engMv, engStats := e.SearchDepth(b, NewStoppable(), depth)
	s := newSMP(1, testTTBytes)
	defer s.Close()
	smpMv, smpStats := s.SearchDepth(b, NewStoppable(), depth)
	if engStats.RootIters != depth || smpStats.RootIters != depth {
		t.Fatalf("ring tails %d and %d, want the capped %d on both engines", engStats.RootIters, smpStats.RootIters, depth)
	}
	for j := range config.PonderDepthHistory {
		if engStats.RootMoves[j] != smpStats.RootMoves[j] {
			t.Fatalf("ring slot %d move %d, want the smp ring move %d", j, engStats.RootMoves[j], smpStats.RootMoves[j])
		}
		if engStats.RootScores[j] != smpStats.RootScores[j] {
			t.Fatalf("ring slot %d score %d, want the smp ring score %d", j, engStats.RootScores[j], smpStats.RootScores[j])
		}
	}
	if engMv != smpMv || engStats.Score != smpStats.Score {
		t.Fatalf("engine root %d/%d diverged from smp %d/%d", engMv, engStats.Score, smpMv, smpStats.Score)
	}
}

func TestPonderStopReturnsLegalMove(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.StartPonder(b)
	time.Sleep(scaledBudget(80 * time.Millisecond))
	mv, stats := s.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("ponder move %d illegal", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("ponder depth = %d after %v, want at least 2", stats.Depth, scaledBudget(80*time.Millisecond))
	}
	if stats.Nodes == 0 || stats.Nps == 0 {
		t.Errorf("ponder nodes %d nps %d, both must be positive", stats.Nodes, stats.Nps)
	}
	if stats.Threads != 2 {
		t.Errorf("threads = %d, want 2", stats.Threads)
	}
	if stats.ElapsedNs <= 0 {
		t.Errorf("ponder elapsed %d, must span the ponder window", stats.ElapsedNs)
	}
	if stats.RootIters < 2 {
		t.Errorf("ponder root history = %d iterations, want at least 2", stats.RootIters)
	}
	tail := min(config.PonderDepthHistory, stats.RootIters)
	if stats.RootMoves[tail-1] != mv {
		t.Errorf("newest root history move %d, want the reported move %d", stats.RootMoves[tail-1], mv)
	}
	if stats.RootScores[tail-1] != stats.Score {
		t.Errorf("newest root history score %d, want the reported score %d", stats.RootScores[tail-1], stats.Score)
	}
	mv2, stats2 := s.StopPonder()
	if mv2 != 0 || stats2 != (SearchStats{}) {
		t.Errorf("second StopPonder = %d %+v, want zero values", mv2, stats2)
	}
}

func TestStopPonderWhenNotPonderingReturnsZeros(t *testing.T) {
	s := newSMP(2, testTTBytes)
	defer s.Close()
	if mv, _ := s.Search(midgameBoard(t), NewFixedBudget(time.Millisecond)); mv == moveNone {
		t.Fatal("warmup search returned no move")
	}
	mv, stats := s.StopPonder()
	if mv != 0 || stats != (SearchStats{}) {
		t.Errorf("StopPonder without a ponder = %d %+v, want zero values", mv, stats)
	}
}

func TestPonderImplicitStopOnSearch(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.StartPonder(b)
	time.Sleep(scaledBudget(20 * time.Millisecond))
	mv, stats := s.Search(b, NewFixedBudget(scaledBudget(80*time.Millisecond)))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("search after ponder move %d illegal", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("search depth = %d, want at least 2", stats.Depth)
	}
	if mv2, stats2 := s.StopPonder(); mv2 != 0 || stats2 != (SearchStats{}) {
		t.Errorf("StopPonder after implicit stop = %d %+v, want zero values", mv2, stats2)
	}
}

func TestPonderImplicitStopOnClose(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := range 6 {
		s := newSMP(2, testTTBytes)
		s.StartPonder(midgameBoard(t))
		time.Sleep(time.Duration(i) * time.Millisecond)
		s.Close()
		s.Close()
	}
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked after closing pondering instances: %d now vs %d before", runtime.NumGoroutine(), before)
}

func TestStartPonderWhilePonderingPanics(t *testing.T) {
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.StartPonder(midgameBoard(t))
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("StartPonder while pondering must panic")
		}
		if msg, ok := r.(string); !ok || !strings.HasPrefix(msg, "engine:") {
			t.Fatalf("panic %v, want an engine: prefix", r)
		}
	}()
	s.StartPonder(midgameBoard(t))
}

func TestStartPonderWhileSearchingPanics(t *testing.T) {
	s := newSMP(2, testTTBytes)
	defer s.Close()
	b := midgameBoard(t)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = s.Search(b, NewFixedBudget(scaledBudget(150*time.Millisecond)))
	}()
	for {
		s.mu.Lock()
		busy := s.searching
		s.mu.Unlock()
		if busy {
			break
		}
		time.Sleep(time.Millisecond)
	}
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("StartPonder while searching must panic")
		}
		if msg, ok := r.(string); !ok || !strings.HasPrefix(msg, "engine:") {
			t.Fatalf("panic %v, want an engine: prefix", r)
		}
		<-done
	}()
	s.StartPonder(b)
}

func TestStartPonderAfterCloseIsSilent(t *testing.T) {
	s := newSMP(2, testTTBytes)
	s.Close()
	s.StartPonder(midgameBoard(t))
	if mv, stats := s.StopPonder(); mv != 0 || stats != (SearchStats{}) {
		t.Errorf("StopPonder after a closed no-op start = %d %+v, want zero values", mv, stats)
	}
}

func TestStartPonderFullBoardLeavesLatchUnset(t *testing.T) {
	b := rules.NewBoard()
	var buf [config.BoardCells]rules.Move
	for {
		n := b.LegalMoves(buf[:])
		if n == 0 {
			break
		}
		b.Make(rules.Cell(buf[0]))
	}
	if !b.IsFull() {
		t.Fatal("setup did not fill the board")
	}
	s := newSMP(2, testTTBytes)
	defer s.Close()
	s.StartPonder(b)
	if mv, stats := s.StopPonder(); mv != 0 || stats != (SearchStats{}) {
		t.Errorf("StopPonder after a full board start = %d %+v, want zero values", mv, stats)
	}
}

func TestPonderDepthGrowsWithWallTime(t *testing.T) {
	b := playout(t, 77, 4)
	s := newSMP(2, 1<<12)
	defer s.Close()
	s.StartPonder(b)
	time.Sleep(scaledBudget(300 * time.Millisecond))
	mv, stats := s.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("ponder move %d illegal on the opening board", mv)
	}
	if stats.Depth < 2 {
		t.Fatalf("ponder depth = %d after %v on an opening board, want at least 2", stats.Depth, scaledBudget(300*time.Millisecond))
	}
}

func TestPonderStatsCoverOnlyTheLastJob(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	defer s.Close()
	_, first := s.Search(b, NewFixedBudget(scaledBudget(120*time.Millisecond)))
	s.StartPonder(b)
	time.Sleep(scaledBudget(60 * time.Millisecond))
	_, p1 := s.StopPonder()
	_, second := s.Search(b, NewFixedBudget(scaledBudget(120*time.Millisecond)))
	s.StartPonder(b)
	time.Sleep(scaledBudget(60 * time.Millisecond))
	mv, p2 := s.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("ponder move %d illegal", mv)
	}
	if first.Nodes == 0 || p1.Nodes == 0 || second.Nodes == 0 || p2.Nodes == 0 {
		t.Fatalf("a job reported zero nodes: search %d ponder %d search %d ponder %d", first.Nodes, p1.Nodes, second.Nodes, p2.Nodes)
	}
	prior := first.Nodes + p1.Nodes + second.Nodes
	if p2.Nodes > prior {
		t.Fatalf("second ponder nodes %d exceeds the prior jobs' whole total %d: stats are not per job", p2.Nodes, prior)
	}
	window := scaledBudget(2 * time.Second)
	if d := time.Duration(p2.ElapsedNs); d > window {
		t.Fatalf("second ponder elapsed %v spans past its own window, want at most %v", d, window)
	}
}

func TestPonderZeroAllocs(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	if mv, _ := s.Search(b, NewFixedBudget(time.Millisecond)); !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("warmup move %d illegal", mv)
	}
	defer s.Close()
	var sinkMove rules.Move
	var sinkStats SearchStats
	if n := testing.AllocsPerRun(20, func() {
		s.StartPonder(b)
		sinkMove, sinkStats = s.StopPonder()
	}); n != 0 {
		t.Fatalf("ponder cycle: %v allocs, want 0", n)
	}
	if !b.IsLegal(rules.Cell(sinkMove)) {
		t.Fatalf("sink ponder move %d illegal", sinkMove)
	}
	if sinkStats.Threads != 2 {
		t.Errorf("sink ponder threads = %d, want 2", sinkStats.Threads)
	}
}

func TestEnginePonderLifecycle(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	defer e.Close()
	var p Ponderer = e
	p.StartPonder(b)
	e.mu.Lock()
	ponding := e.ponding
	e.mu.Unlock()
	if !ponding {
		t.Fatal("StartPonder left the ponder latch unset")
	}
	mv, stats := p.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("ponder stop move %d illegal", mv)
	}
	if stats.Threads != 1 {
		t.Errorf("ponder stop threads = %d, want 1", stats.Threads)
	}
	e.mu.Lock()
	ponding = e.ponding
	e.mu.Unlock()
	if ponding {
		t.Fatal("StopPonder left the ponder latch set")
	}
	if mv2, stats2 := p.StopPonder(); mv2 != 0 || stats2 != (SearchStats{}) {
		t.Errorf("second StopPonder = %d %+v, want zero values", mv2, stats2)
	}
}

func TestPonderOutlastsFundedGrant(t *testing.T) {
	b := midgameBoard(t)
	grant := scaledBudget(120 * time.Millisecond)
	pacer := New(testTTBytes)
	pacerBoard := *b
	paceOnce := func() {
		pacer.Search(&pacerBoard, NewFixedBudget(grant))
	}
	fundedUnderPace := func(search func() SearchStats) int {
		paced := make(chan struct{})
		go func() {
			defer close(paced)
			paceOnce()
		}()
		stats := search()
		<-paced
		return stats.Depth
	}
	e := New(testTTBytes)
	defer e.Close()
	engFunded := fundedUnderPace(func() SearchStats {
		_, stats := e.Search(b, NewFixedBudget(grant))
		return stats
	})
	e.StartPonder(b)
	paceOnce()
	paceOnce()
	e.mu.Lock()
	still := e.ponding
	e.mu.Unlock()
	if !still {
		t.Fatal("engine ponder self-terminated without a stop")
	}
	mv, stats := e.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("engine ponder move %d illegal", mv)
	}
	if stats.Depth < engFunded || stats.RootIters < engFunded {
		t.Fatalf("engine ponder banked depth %d root iters %d, want at least the funded %d", stats.Depth, stats.RootIters, engFunded)
	}
	s := newSMP(2, testTTBytes)
	defer s.Close()
	smpFunded := fundedUnderPace(func() SearchStats {
		_, stats := s.Search(b, NewFixedBudget(grant))
		return stats
	})
	s.StartPonder(b)
	paceOnce()
	paceOnce()
	s.mu.Lock()
	still = s.ponding
	s.mu.Unlock()
	if !still {
		t.Fatal("smp ponder self-terminated without a stop")
	}
	mv, stats = s.StopPonder()
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("smp ponder move %d illegal", mv)
	}
	if stats.Depth < smpFunded || stats.RootIters < smpFunded {
		t.Fatalf("smp ponder banked depth %d root iters %d, want at least the funded %d", stats.Depth, stats.RootIters, smpFunded)
	}
}

func TestEnginePonderImplicitStopOnSearch(t *testing.T) {
	b := midgameBoard(t)
	e := New(testTTBytes)
	defer e.Close()
	e.StartPonder(b)
	e.mu.Lock()
	ponding := e.ponding
	e.mu.Unlock()
	if !ponding {
		t.Fatal("StartPonder left the ponder latch unset")
	}
	mv, stats := e.Search(b, NewFixedBudget(scaledBudget(80*time.Millisecond)))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("search after ponder move %d illegal", mv)
	}
	if stats.Depth < 2 {
		t.Errorf("search depth = %d, want at least 2", stats.Depth)
	}
	e.mu.Lock()
	ponding = e.ponding
	e.mu.Unlock()
	if ponding {
		t.Fatal("Search left the ponder latch set")
	}
	if mv2, stats2 := e.StopPonder(); mv2 != 0 || stats2 != (SearchStats{}) {
		t.Errorf("StopPonder after implicit stop = %d %+v, want zero values", mv2, stats2)
	}
}

func TestEnginePonderImplicitStopOnClose(t *testing.T) {
	before := runtime.NumGoroutine()
	for range 6 {
		e := New(testTTBytes)
		e.StartPonder(midgameBoard(t))
		e.Close()
		e.Close()
	}
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked after closing pondering engines: %d now vs %d before", runtime.NumGoroutine(), before)
}

func fullBoard(t *testing.T) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	var buf [config.BoardCells]rules.Move
	for {
		n := b.LegalMoves(buf[:])
		if n == 0 {
			break
		}
		b.Make(rules.Cell(buf[0]))
	}
	return b
}

func assertEnginePanic(t *testing.T, name string, fn func()) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatalf("%s must panic", name)
		}
		if msg, ok := r.(string); !ok || !strings.HasPrefix(msg, "engine:") {
			t.Fatalf("%s panic %v, want an engine: prefix", name, r)
		}
	}()
	fn()
}

func TestEngineStartPonderGuards(t *testing.T) {
	e := New(testTTBytes)
	e.StartPonder(fullBoard(t))
	if mv, stats := e.StopPonder(); mv != 0 || stats != (SearchStats{}) {
		t.Fatalf("full-board ponder stop = %d %+v, want zero values", mv, stats)
	}
	e.Close()
	e.StartPonder(midgameBoard(t))
	if mv, stats := e.StopPonder(); mv != 0 || stats != (SearchStats{}) {
		t.Fatalf("closed-engine ponder stop = %d %+v, want zero values", mv, stats)
	}
	assertEnginePanic(t, "Search after Close", func() { e.Search(midgameBoard(t), NewStoppable()) })

	hot := New(testTTBytes)
	defer hot.Close()
	hot.StartPonder(midgameBoard(t))
	assertEnginePanic(t, "StartPonder while pondering", func() { hot.StartPonder(midgameBoard(t)) })
	hot.StopPonder()

	busy := New(testTTBytes)
	defer busy.Close()
	b := midgameBoard(t)
	guard := *b
	done := make(chan struct{})
	go func() {
		defer close(done)
		busy.Search(b, NewFixedBudget(scaledBudget(150*time.Millisecond)))
	}()
	for {
		busy.mu.Lock()
		searching := busy.searching
		busy.mu.Unlock()
		if searching {
			break
		}
		runtime.Gosched()
	}
	assertEnginePanic(t, "StartPonder while searching", func() { busy.StartPonder(&guard) })
	<-done
}

func TestEnginePonderConcurrentLifecycle(t *testing.T) {
	if testing.Short() {
		t.Skip("hammer needs wall clock budget")
	}
	b := midgameBoard(t)
	e := New(testTTBytes)
	before := runtime.NumGoroutine()
	quit := make(chan struct{})
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for {
				select {
				case <-quit:
					return
				default:
				}
				if mv, _ := e.StopPonder(); mv != 0 && !b.IsLegal(rules.Cell(mv)) {
					t.Errorf("hammer stop returned illegal move %d", mv)
					return
				}
			}
		})
	}
	for range 6 {
		e.StartPonder(b)
		time.Sleep(scaledBudget(20 * time.Millisecond))
		if mv, _ := e.StopPonder(); mv != 0 && !b.IsLegal(rules.Cell(mv)) {
			t.Errorf("hammer cycle move %d illegal", mv)
		}
		if mv, _ := e.Search(b, NewFixedBudget(scaledBudget(10*time.Millisecond))); mv != 0 && !b.IsLegal(rules.Cell(mv)) {
			t.Errorf("hammer search move %d illegal", mv)
		}
	}
	e.StartPonder(b)
	time.Sleep(scaledBudget(20 * time.Millisecond))
	e.Close()
	e.Close()
	close(quit)
	wg.Wait()
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked after the engine ponder hammer: %d now vs %d before", runtime.NumGoroutine(), before)
}

func TestPonderConcurrentStartStopHammer(t *testing.T) {
	if testing.Short() {
		t.Skip("hammer needs wall clock budget")
	}
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	before := runtime.NumGoroutine()
	quit := make(chan struct{})
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() {
			for {
				select {
				case <-quit:
					return
				default:
				}
				if mv, _ := s.StopPonder(); mv != 0 && !b.IsLegal(rules.Cell(mv)) {
					t.Errorf("hammer stop returned illegal move %d", mv)
					return
				}
			}
		})
	}
	for range 6 {
		s.StartPonder(b)
		time.Sleep(scaledBudget(20 * time.Millisecond))
		if mv, _ := s.StopPonder(); mv != 0 && !b.IsLegal(rules.Cell(mv)) {
			t.Errorf("hammer cycle move %d illegal", mv)
		}
	}
	s.StartPonder(b)
	time.Sleep(scaledBudget(20 * time.Millisecond))
	s.Close()
	s.Close()
	close(quit)
	wg.Wait()
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked after the ponder hammer: %d now vs %d before", runtime.NumGoroutine(), before)
}
