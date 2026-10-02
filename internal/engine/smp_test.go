package engine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestNewTieredMapsConfig(t *testing.T) {
	cases := [...]struct {
		tier    config.Tier
		workers int
		slots   int
	}{
		{config.TierEasy, 1, 0},
		{config.TierMedium, 2, 1 << 24},
		{config.TierHard, 4, 1 << 26},
	}
	for _, c := range cases {
		s := NewTiered(c.tier)
		if s.Workers() != c.workers {
			t.Errorf("%s workers = %d, want %d", c.tier.Name, s.Workers(), c.workers)
		}
		if got := len(s.tt.entries) / 2; got != c.slots {
			t.Errorf("%s tt slots = %d, want %d", c.tier.Name, got, c.slots)
		}
		for _, w := range s.workers {
			if w.tt != s.tt {
				t.Errorf("%s worker does not share the instance table", c.tier.Name)
			}
		}
		for i := range s.workers {
			for j := i + 1; j < len(s.workers); j++ {
				if s.workers[i] == s.workers[j] {
					t.Errorf("%s workers %d and %d are the same engine", c.tier.Name, i, j)
				}
			}
		}
	}
	if s := newSMP(0, 0); s.Workers() != 1 {
		t.Errorf("worker clamp = %d, want 1", s.Workers())
	}
}

func TestSMPSearchReturnsLegalMove(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(4, testTTBytes)
	budget := scaledBudget(80 * time.Millisecond)
	mv, stats := s.Search(b, NewFixedBudget(budget))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("smp move %d illegal", mv)
	}
	if stats.Threads != 4 {
		t.Errorf("threads = %d, want 4", stats.Threads)
	}
	if stats.Nodes == 0 || stats.Nps == 0 {
		t.Errorf("nodes %d nps %d, both must be positive", stats.Nodes, stats.Nps)
	}
	if stats.Depth < 2 {
		t.Errorf("depth = %d, want at least 2 in %v across 4 workers", stats.Depth, budget)
	}
	if stats.PVLen == 0 || stats.PV[0] != mv {
		t.Errorf("pv head %d len %d, want the reported move %d", stats.PV[0], stats.PVLen, mv)
	}
}

func TestSMPSingleWorkerMatchesEngine(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(1, 1<<20)
	e := New(1 << 20)
	mvA, statsA := s.SearchDepth(b, NewFixedBudget(scaledBudget(time.Second)), 4)
	mvB, statsB := e.SearchDepth(b, NewFixedBudget(scaledBudget(time.Second)), 4)
	if mvA != mvB || statsA.Score != statsB.Score || statsA.Nodes != statsB.Nodes || statsA.Depth != statsB.Depth {
		t.Errorf("1 worker diverged from the engine: %d/%d/%d/%d vs %d/%d/%d/%d",
			mvA, statsA.Score, statsA.Nodes, statsA.Depth, mvB, statsB.Score, statsB.Nodes, statsB.Depth)
	}
	if statsA.Threads != 1 {
		t.Errorf("threads = %d, want 1", statsA.Threads)
	}
}

func TestSMPStatsAggregateAcrossWorkers(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(4, testTTBytes)
	mv, stats := s.SearchDepth(b, NewFixedBudget(scaledBudget(120*time.Millisecond)), 64)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("move %d illegal", mv)
	}
	var nodes, maxCompleted uint64
	maxCompleted = 0
	for i := range s.results {
		r := &s.results[i]
		nodes += r.nodes
		if uint64(r.completed) > maxCompleted {
			maxCompleted = uint64(r.completed)
		}
	}
	if stats.Nodes != nodes {
		t.Errorf("aggregated nodes %d, want the worker sum %d", stats.Nodes, nodes)
	}
	if uint64(stats.Depth) != maxCompleted {
		t.Errorf("reported depth %d, want max completed %d", stats.Depth, maxCompleted)
	}
	if stats.HashFullPermille == 0 {
		t.Error("shared table hash full permille 0 after a contended search")
	}
}

func TestSMPFindsForcedMateInOne(t *testing.T) {
	b := mate1Board(t)
	s := newSMP(2, testTTBytes)
	start := time.Now()
	mv, stats := s.Search(b, NewFixedBudget(scaledBudget(2*time.Second)))
	if mv != rules.Move(mustCell(t, "I9")) {
		t.Fatalf("smp mate in 1 move %d, want I9", mv)
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("smp mate in 1 score = %d, want %d", stats.Score, want)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("immediate win took %v, siblings must halt instead of burning the budget", elapsed)
	}
}

func TestSMPFindsForcedMateInTwoAndThree(t *testing.T) {
	b2 := mate2Board(t)
	s2 := newSMP(2, testTTBytes)
	mv2, stats2 := s2.SearchDepth(b2, NewFixedBudget(scaledBudget(time.Second)), 6)
	if !b2.IsLegal(rules.Cell(mv2)) {
		t.Fatalf("mate in 2 move %d illegal", mv2)
	}
	if want := config.EvalMateMax - 3*config.EvalMateScoreStep; stats2.Score != want {
		t.Errorf("smp mate in 2 score = %d, want %d", stats2.Score, want)
	}
	b3 := mate3Board(t)
	s3 := newSMP(4, testTTBytes)
	mv3, stats3 := s3.SearchDepth(b3, NewFixedBudget(scaledBudget(2*time.Second)), 8)
	if mv3 != rules.Move(mustCell(t, "H9")) {
		t.Fatalf("smp mate in 3 move %d, want the cross point H9", mv3)
	}
	if want := config.EvalMateMax - 5*config.EvalMateScoreStep; stats3.Score != want {
		t.Errorf("smp mate in 3 score = %d, want %d", stats3.Score, want)
	}
}

func TestSMPDeclinesOverlineTrap(t *testing.T) {
	b := overlineTrapBoard(t)
	s := newSMP(2, testTTBytes)
	mv, stats := s.SearchDepth(b, NewFixedBudget(scaledBudget(time.Second)), 4)
	if mv != rules.Move(mustCell(t, "E9")) {
		t.Fatalf("smp trap move %d, want E9", mv)
	}
	if want := config.EvalMateMax - config.EvalMateScoreStep; stats.Score != want {
		t.Errorf("score = %d, want mate in 1 %d", stats.Score, want)
	}
}

func TestSMPTinyBudgetStillLegal(t *testing.T) {
	for _, budget := range []time.Duration{0, 30} {
		b := midgameBoard(t)
		s := newSMP(3, 0)
		mv, stats := s.Search(b, NewFixedBudget(budget*time.Microsecond))
		if !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("budget %v: illegal move %d", budget, mv)
		}
		if stats.Threads != 3 {
			t.Errorf("threads = %d, want 3", stats.Threads)
		}
	}
}

func TestSMPTinyBudgetWithExternalStop(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(4, testTTBytes)
	dl := NewFixedBudget(5 * time.Second)
	var stopped atomic.Bool
	go func() {
		time.Sleep(2 * time.Millisecond)
		dl.Stop()
		stopped.Store(true)
	}()
	mv, _ := s.Search(b, dl)
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("externally stopped search returned illegal move %d", mv)
	}
	if !stopped.Load() {
		t.Error("stop goroutine never ran")
	}
}

func TestSMPFullBoardReturnsNoMove(t *testing.T) {
	b := rules.NewCrossCheck()
	for r := range config.CrossCheckSize {
		for c := range config.CrossCheckSize {
			b.Side = rules.Color((r + c) % 2)
			b.Make(rules.Cell(r*config.BoardStride + c))
		}
	}
	if !b.IsFull() {
		t.Fatal("setup did not fill the board")
	}
	s := NewTiered(config.TierEasy)
	mv, stats := s.Search(b, NewFixedBudget(time.Millisecond))
	if mv != moveNone {
		t.Errorf("full board move = %d, want moveNone", mv)
	}
	if stats.Depth != 0 {
		t.Errorf("full board depth = %d, want 0", stats.Depth)
	}
}

func TestSMPWorkerPoolLifecycle(t *testing.T) {
	before := runtime.NumGoroutine()
	s := newSMP(4, testTTBytes)
	for range 3 {
		b := midgameBoard(t)
		if mv, _ := s.Search(b, NewFixedBudget(30*time.Millisecond)); !b.IsLegal(rules.Cell(mv)) {
			t.Fatalf("lifecycle probe move %d illegal", mv)
		}
	}
	s.Close()
	s.Close() // idempotent
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked after Close: %d now vs %d before", runtime.NumGoroutine(), before)
}

func TestSMPCloseBeforeFirstSearchIsInert(t *testing.T) {
	s := newSMP(2, testTTBytes)
	s.Close()
	if s.started {
		t.Error("Close before any search must not start the pool")
	}
}

func TestSMPSearchAfterClosePanics(t *testing.T) {
	s := newSMP(2, 0)
	if mv, _ := s.Search(midgameBoard(t), NewFixedBudget(time.Microsecond)); !midgameBoard(t).IsLegal(rules.Cell(mv)) {
		t.Fatalf("warmup move %d illegal", mv)
	}
	s.Close()
	defer func() {
		if recover() == nil {
			t.Error("Search after Close must panic")
		}
	}()
	_, _ = s.Search(midgameBoard(t), NewFixedBudget(time.Microsecond))
}

// TestSMPDispatchAfterClosePanics drives the dispatch handshake itself into
// a Close that slipped past the entry check: the pool must never spawn into
// an instance nobody will join.
func TestSMPDispatchAfterClosePanics(t *testing.T) {
	s := newSMP(2, testTTBytes)
	s.closed = true
	defer func() {
		if recover() == nil {
			t.Fatal("dispatch after Close must panic loudly")
		}
		if s.started {
			t.Error("dispatch spawned the pool despite Close")
		}
	}()
	s.dispatch()
}

// TestSMPCloseJoinsInFlightSearch is the Close-versus-search race: every
// interleaving must leave no worker behind and hang nothing, with the
// in-flight driver either finishing its jobs or failing loudly at entry.
func TestSMPCloseJoinsInFlightSearch(t *testing.T) {
	before := runtime.NumGoroutine()
	for i := range 12 {
		s := newSMP(2, testTTBytes)
		b := midgameBoard(t)
		done := make(chan struct{})
		go func() {
			defer close(done)
			defer func() { _ = recover() }()
			_, _ = s.Search(b, NewFixedBudget(60*time.Millisecond))
		}()
		time.Sleep(time.Duration(i%9) * time.Millisecond)
		s.Close()
		s.Close() // idempotent under the join
		<-done
	}
	for range 200 {
		if runtime.NumGoroutine() <= before {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("goroutines leaked by Close racing searches: %d now vs %d before", runtime.NumGoroutine(), before)
}

func TestSMPConcurrentSearchPanicsLoudly(t *testing.T) {
	s := newSMP(2, testTTBytes)
	b := midgameBoard(t)
	start := make(chan struct{})
	var wg sync.WaitGroup
	var panicked atomic.Int32
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			defer func() {
				if recover() != nil {
					panicked.Add(1)
				}
			}()
			_, _ = s.Search(b, NewFixedBudget(80*time.Millisecond))
		}()
	}
	close(start)
	wg.Wait()
	if panicked.Load() == 0 {
		t.Fatal("concurrent Search on one instance must fail loudly")
	}
	s.Close()
}

// TestSMPQuitBranchServesQueuedJob covers the drain that keeps
// SearchDepth's runWG.Wait hang-proof when Close lands between the token
// send and a worker's parked receive.
func TestSMPQuitBranchServesQueuedJob(t *testing.T) {
	s := newSMP(1, testTTBytes)
	w, b, res := s.workers[0], &s.boards[0], &s.results[0]
	*b = *midgameBoard(t)
	w.resetForSearch(b)
	s.jobMaxDepth = 2
	s.halt.Store(false)
	s.haltDL.inner = NewFixedBudget(50 * time.Millisecond)
	s.runWG.Add(1)
	s.wake <- struct{}{}
	if !s.serveQueuedJob(w, b, res) {
		t.Fatal("queued job not served on the quit path")
	}
	s.runWG.Wait()
	if res.completed == 0 {
		t.Error("queued job left no completed iteration")
	}
	if s.serveQueuedJob(w, b, res) {
		t.Fatal("empty wake channel reported a queued job")
	}
}

func TestSMPInstantWinStatsReportNps(t *testing.T) {
	s := newSMP(2, testTTBytes)
	_, stats := s.Search(mate1Board(t), NewFixedBudget(time.Second))
	if stats.Nodes == 0 {
		t.Fatal("setup: no nodes searched")
	}
	if stats.Nps == 0 {
		t.Errorf("nps 0 on an instant win: nodes %d elapsed %d", stats.Nodes, stats.ElapsedNs)
	}
}

func TestSMPRaceHammer(t *testing.T) {
	if testing.Short() {
		t.Skip("hammer needs wall clock budget")
	}
	for _, workers := range [...]int{2, 4} {
		s := newSMP(workers, testTTBytes)
		for seed := range uint64(4) {
			b := playout(t, 400*seed+uint64(workers), int(10+seed*5))
			mv, stats := s.Search(b, NewFixedBudget(40*time.Millisecond))
			if !b.IsLegal(rules.Cell(mv)) {
				t.Fatalf("workers %d seed %d: illegal move %d", workers, seed, mv)
			}
			if stats.Threads != workers {
				t.Fatalf("threads = %d, want %d", stats.Threads, workers)
			}
		}
	}
}

func TestSMPEnsureProcsRaisesOnly(t *testing.T) {
	old := runtime.GOMAXPROCS(0)
	defer runtime.GOMAXPROCS(old)
	runtime.GOMAXPROCS(1)
	s := newSMP(2, 0)
	s.ensureProcs()
	if got := runtime.GOMAXPROCS(0); got < 2 {
		t.Errorf("GOMAXPROCS = %d after ensureProcs, want at least 2", got)
	}
	runtime.GOMAXPROCS(old)
	s.ensureProcs()
	if got := runtime.GOMAXPROCS(0); got != old {
		t.Errorf("GOMAXPROCS lowered to %d, must never drop below %d", got, old)
	}
}

func TestSMPPonderSeamIsNoOp(t *testing.T) {
	var p Ponderer = NewTiered(config.TierEasy)
	p.StartPonder(rules.NewBoard())
	p.StopPonder()
}

func TestSMPHaltDeadlineSemantics(t *testing.T) {
	dl := NewFixedBudget(time.Hour)
	var halt atomic.Bool
	h := haltDeadline{inner: dl, halt: &halt}
	if h.Exceeded() {
		t.Fatal("fresh halt deadline exceeded")
	}
	h.Stop()
	if !dl.Exceeded() || !h.Exceeded() {
		t.Error("Stop must forward to the inner deadline")
	}
	halt.Store(true)
	if !h.Exceeded() {
		t.Error("halt flag must force exceeded")
	}
}

func TestSMPWorkerLoopZeroAllocs(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	w := s.workers[0]
	res := &s.results[0]
	dl := NewFixedBudget(0)
	s.jobMaxDepth = config.SearchMaxPly
	if n := testing.AllocsPerRun(20, func() {
		s.tt.bumpGen()
		w.resetForSearch(b)
		s.halt.Store(false)
		s.haltDL.inner = dl
		dl.Reset(300 * time.Microsecond)
		s.runWorker(w, b, res)
	}); n != 0 {
		t.Fatalf("worker solve loop: %v allocs, want 0", n)
	}
	if res.nodes == 0 {
		t.Fatal("worker run produced no nodes")
	}
}

// TestSMPSearchZeroAllocs pins the whole SMP search, pool included: once the
// workers are parked, a search allocates nothing, matching the single
// threaded Engine contract.
func TestSMPSearchZeroAllocs(t *testing.T) {
	b := midgameBoard(t)
	s := newSMP(2, testTTBytes)
	dl := NewFixedBudget(0)
	dl.Reset(time.Millisecond)
	if mv, _ := s.Search(b, dl); !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("pool warmup move %d illegal", mv)
	}
	defer s.Close()
	if n := testing.AllocsPerRun(20, func() {
		dl.Reset(time.Millisecond)
		_, _ = s.Search(b, dl)
	}); n != 0 {
		t.Fatalf("SMP Search: %v allocs, want 0", n)
	}
}
