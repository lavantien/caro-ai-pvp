package engine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type Ponderer interface {
	StartPonder(b *rules.Board)
	StopPonder() (rules.Move, SearchStats)
}

type workerResult struct {
	seq        uint32
	completed  int
	score      int
	move       rules.Move
	pvLen      int
	pv         [config.SearchMaxPly]rules.Move
	nodes      uint64
	ttProbes   uint64
	ttHits     uint64
	cutNodes   uint64
	cutFirst   uint64
	rootMoves  [config.PonderDepthHistory]rules.Move
	rootScores [config.PonderDepthHistory]int
	rootIters  int
}

type haltDeadline struct {
	inner Deadline
	halt  *atomic.Bool
}

func (h *haltDeadline) Exceeded() bool { return h.halt.Load() || h.inner.Exceeded() }

func (h *haltDeadline) Stop() { h.inner.Stop() }

type ponderDeadline struct {
	stopped atomic.Bool
}

func (p *ponderDeadline) Exceeded() bool { return p.stopped.Load() }

func (p *ponderDeadline) Stop() { p.stopped.Store(true) }

type SMP struct {
	tt       *ttTable
	workers  []*Engine
	boards   []rules.Board
	results  []workerResult
	seq      atomic.Uint32
	halt     atomic.Bool
	haltDL   haltDeadline
	ponderDL ponderDeadline

	jobMaxDepth int
	jobSoft     bool
	ponderStart time.Time
	wake        chan struct{}
	quit        chan struct{}
	runWG       sync.WaitGroup
	exitWG      sync.WaitGroup

	mu        sync.Mutex
	started   bool
	closed    bool
	searching bool
	ponding   bool
}

func NewTiered(t config.Tier) *SMP {
	return newSMP(t.Cores, t.TTBytes)
}

func newSMP(workers int, ttBytes int64) *SMP {
	if workers < 1 {
		workers = 1
	}
	s := &SMP{tt: newTT(ttBytes), wake: make(chan struct{}, workers), quit: make(chan struct{})}
	for range workers {
		s.workers = append(s.workers, newEngineShared(s.tt))
		s.boards = append(s.boards, rules.Board{})
		s.results = append(s.results, workerResult{})
	}
	s.haltDL.halt = &s.halt
	return s
}

func (s *SMP) Workers() int { return len(s.workers) }

func (s *SMP) StartPonder(b *rules.Board) {
	if b.IsFull() {
		return
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	if s.searching {
		s.mu.Unlock()
		panic("engine: StartPonder while a search is running")
	}
	if s.ponding {
		s.mu.Unlock()
		panic("engine: StartPonder while already pondering")
	}
	s.ponding = true
	s.ponderStart = time.Now()
	s.ponderDL.stopped.Store(false)
	s.setupJob(b, config.SearchMaxPly, false, &s.ponderDL)
	s.ensureProcs()
	s.dispatchLocked()
	s.mu.Unlock()
}

func (s *SMP) StopPonder() (rules.Move, SearchStats) {
	s.mu.Lock()
	if !s.ponding {
		s.mu.Unlock()
		var zero rules.Move
		return zero, SearchStats{}
	}
	s.stopPonderLocked()
	mv, stats := s.collectResults(s.ponderStart)
	s.mu.Unlock()
	return mv, stats
}

var _ Ponderer = (*SMP)(nil)

func (s *SMP) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.ponding {
		s.stopPonderLocked()
	}
	if s.started {
		close(s.quit)
		s.mu.Unlock()
		s.exitWG.Wait()
		return
	}
	s.mu.Unlock()
}

func (s *SMP) startPool() {
	if s.started {
		return
	}
	s.started = true
	s.exitWG.Add(len(s.workers))
	for i := range s.workers {
		go s.worker(s.workers[i], &s.boards[i], &s.results[i])
	}
}

func (s *SMP) worker(w *Engine, b *rules.Board, res *workerResult) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	defer s.exitWG.Done()
	for {
		if s.serveHot(w, b, res) {
			continue
		}
		select {
		case <-s.wake:
			s.serveJob(w, b, res)
		case <-s.quit:
			if !s.serveQueuedJob(w, b, res) {
				return
			}
		}
	}
}

func (s *SMP) serveQueuedJob(w *Engine, b *rules.Board, res *workerResult) bool {
	select {
	case <-s.wake:
		s.serveJob(w, b, res)
		return true
	default:
		return false
	}
}

func (s *SMP) serveJob(w *Engine, b *rules.Board, res *workerResult) {
	defer s.runWG.Done()
	s.runWorker(w, b, res)
}

func (s *SMP) serveHot(w *Engine, b *rules.Board, res *workerResult) bool {
	until := time.Now().Add(time.Duration(config.SearchWorkerParkDelayMs) * time.Millisecond)
	for {
		select {
		case <-s.wake:
			s.serveJob(w, b, res)
			return true
		default:
		}
		if !time.Now().Before(until) {
			return false
		}
	}
}

func (s *SMP) Search(b *rules.Board, dl Deadline) (rules.Move, SearchStats) {
	return s.searchDepth(b, dl, config.SearchMaxPly, true)
}

func (s *SMP) SearchDepth(b *rules.Board, dl Deadline, maxDepth int) (rules.Move, SearchStats) {
	return s.searchDepth(b, dl, maxDepth, false)
}

func (s *SMP) searchDepth(b *rules.Board, dl Deadline, maxDepth int, soft bool) (rules.Move, SearchStats) {
	start := time.Now()
	var allocNs int64
	if bg, ok := dl.(Budgeter); ok {
		allocNs = int64(bg.Budget())
	}
	if b.IsFull() {
		var stats SearchStats
		stats.Threads = len(s.workers)
		stats.AllocNs = allocNs
		stats.ElapsedNs = int64(time.Since(start))
		return moveNone, stats
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		panic("engine: Search on a closed SMP instance")
	}
	if s.searching {
		s.mu.Unlock()
		panic("engine: concurrent Search on one SMP instance")
	}
	if s.ponding {
		s.stopPonderLocked()
	}
	s.searching = true
	s.mu.Unlock()
	defer s.clearSearching()
	s.setupJob(b, maxDepth, soft, dl)
	s.ensureProcs()
	s.dispatch()
	s.runWG.Wait()
	bestMove, stats := s.collectResults(start)
	stats.AllocNs = allocNs
	return bestMove, stats
}

func (s *SMP) setupJob(b *rules.Board, maxDepth int, soft bool, dl Deadline) {
	s.tt.bumpGen()
	s.halt.Store(false)
	s.seq.Store(0)
	s.haltDL.inner = dl
	s.jobMaxDepth = maxDepth
	s.jobSoft = soft
	for i := range s.workers {
		s.boards[i] = *b
		s.workers[i].resetForSearch(&s.boards[i])
		s.results[i] = workerResult{}
	}
}

func (s *SMP) stopPonderLocked() {
	s.halt.Store(true)
	s.ponderDL.stopped.Store(true)
	s.runWG.Wait()
	s.ponding = false
}

func (s *SMP) collectResults(start time.Time) (rules.Move, SearchStats) {
	var stats SearchStats
	stats.Threads = len(s.workers)
	best := -1
	var ttProbes, ttHits, cutNodes, cutFirst uint64
	for i := range s.results {
		r := &s.results[i]
		stats.Nodes += r.nodes
		ttProbes += r.ttProbes
		ttHits += r.ttHits
		cutNodes += r.cutNodes
		cutFirst += r.cutFirst
		if r.completed == 0 {
			continue
		}
		if best < 0 {
			best = i
			continue
		}
		cur := &s.results[best]
		if r.completed > cur.completed || (r.completed == cur.completed && r.seq < cur.seq) {
			best = i
		}
	}
	bestMove := s.workers[0].fallbackMove(&s.boards[0])
	if best >= 0 {
		r := &s.results[best]
		stats.Depth = r.completed
		stats.Score = r.score
		stats.PVLen = r.pvLen
		stats.PV = r.pv
		bestMove = r.move
		n := min(config.PonderDepthHistory, r.rootIters)
		for j := 0; j < n; j++ {
			slot := (r.rootIters - n + j) % config.PonderDepthHistory
			stats.RootMoves[j] = r.rootMoves[slot]
			stats.RootScores[j] = r.rootScores[slot]
		}
		stats.RootIters = r.rootIters
	}
	stats.ElapsedNs = int64(time.Since(start))
	stats.Nps = NpsReport(stats.Nodes, stats.ElapsedNs)
	stats.EBFMilli = EBFMilli(stats.Nodes, stats.Depth)
	if ttProbes > 0 {
		stats.TTHitPermille = int(ttHits * 1000 / ttProbes)
	}
	stats.HashFullPermille = s.tt.hashFullPermille()
	if cutNodes > 0 {
		stats.FirstMoveFailHighPermille = int(cutFirst * 1000 / cutNodes)
	}
	return bestMove, stats
}

func (s *SMP) runWorker(w *Engine, b *rules.Board, res *workerResult) {
	dl := Deadline(&s.haltDL)
	start := time.Now()
	var budget time.Duration
	hasBudget := false
	if bg, ok := s.haltDL.inner.(Budgeter); ok {
		budget = bg.Budget()
		hasBudget = true
	}
	completed := 0
	for depth := 1; depth <= s.jobMaxDepth; depth++ {
		if w.stopped || dl.Exceeded() {
			break
		}
		if s.jobSoft && hasBudget && softStop(time.Since(start), budget, completed) {
			break
		}
		score, move := w.searchRoot(b, depth, dl)
		if w.stopped {
			break
		}
		res.seq = s.seq.Add(1)
		res.completed = depth
		res.score = score
		res.move = move
		res.pv = w.pv[0]
		res.pvLen = w.pvLen[0]
		res.rootMoves[(depth-1)%config.PonderDepthHistory] = move
		res.rootScores[(depth-1)%config.PonderDepthHistory] = score
		res.rootIters = depth
		completed = depth
		if score >= config.EvalMateMax-config.EvalMateScoreStep {
			s.halt.Store(true)
			break
		}
	}
	res.nodes = w.nodes
	res.ttProbes = w.ttProbes
	res.ttHits = w.ttHits
	res.cutNodes = w.cutNodes
	res.cutFirst = w.cutFirst
}

func (s *SMP) clearSearching() {
	s.mu.Lock()
	s.searching = false
	s.mu.Unlock()
}

func (s *SMP) dispatch() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		panic("engine: Search raced Close on an SMP instance")
	}
	s.dispatchLocked()
	s.mu.Unlock()
}

func (s *SMP) dispatchLocked() {
	s.startPool()
	s.runWG.Add(len(s.workers))
	for range s.workers {
		s.wake <- struct{}{}
	}
}

func (s *SMP) ensureProcs() {
	want := len(s.workers) + 1
	if runtime.GOMAXPROCS(0) < want {
		runtime.GOMAXPROCS(want)
	}
}
