package engine

import (
	"runtime"
	"strings"
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

func TestSearchRootRingMatchesIterations(t *testing.T) {
	b := playout(t, 4242, 24)
	dl := NewFixedBudget(scaledBudget(30 * time.Second))
	const maxIter = config.PonderDepthHistory + 2
	var moves [maxIter]rules.Move
	var scores [maxIter]int
	for d := 1; d <= maxIter; d++ {
		s := newSMP(1, testTTBytes)
		mv, stats := s.SearchDepth(b, dl, d)
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
