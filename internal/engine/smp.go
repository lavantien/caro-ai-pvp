package engine

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Ponderer is the ponder seam: searching the opponent's expected reply in
// the background while the clock is idle. The ponder implementation lands
// with the time manager and the server; until then the hooks are no-ops so
// callers can be written against the final surface now.
type Ponderer interface {
	StartPonder(b *rules.Board)
	StopPonder()
}

// workerResult is one worker's report slot. Only that worker writes it
// during the search and the driver reads it after WaitGroup.Wait, which
// gives the needed happens-before edge without any lock.
type workerResult struct {
	seq       uint32
	completed int
	score     int
	move      rules.Move
	pvLen     int
	pv        [config.SearchMaxPly]rules.Move
	nodes     uint64
	ttProbes  uint64
	ttHits    uint64
	cutNodes  uint64
	cutFirst  uint64
}

// haltDeadline folds a shared halt flag into the caller's deadline so one
// worker's proven immediate win stops its siblings inside their normal
// node-check cadence. Preallocated once per SMP instance, re-armed per
// search, never allocated in the solve loop.
type haltDeadline struct {
	inner Deadline
	halt  *atomic.Bool
}

func (h *haltDeadline) Exceeded() bool { return h.halt.Load() || h.inner.Exceeded() }

func (h *haltDeadline) Stop() { h.inner.Stop() }

// SMP is the tier engine: one shared lockless transposition table plus one
// Engine of private search state per worker, one per instance, never shared
// across instances or board kinds since the zobrist key does not encode the
// region. Every worker runs the same iterative deepening on the same root
// and differs only in timing, which the shared table converts into tree
// coverage. The reported move is the mainline of the deepest completed
// iteration, first arrival breaking ties.
//
// Workers are a persistent pool: locked to their threads once and parked on
// a wake channel between searches, so a search itself allocates nothing.
// The pool starts lazily on the first search and Close releases it. Close
// joins an in-flight Search instead of racing it; Search after Close and
// concurrent Searches on one instance both panic loudly.
type SMP struct {
	tt      *ttTable
	workers []*Engine
	boards  []rules.Board
	results []workerResult
	seq     atomic.Uint32
	halt    atomic.Bool
	haltDL  haltDeadline

	jobMaxDepth int
	jobSoft     bool
	wake        chan struct{}
	quit        chan struct{}
	runWG       sync.WaitGroup
	exitWG      sync.WaitGroup

	// mu serializes the lifecycle: pool start, job dispatch, and close.
	// Holding it across the dispatch handshake makes Close racing a
	// starting search deterministic instead of a data race: either Close
	// wins and dispatch panics before any token is counted, or dispatch
	// wins and every token is queued before quit closes, which the
	// workers drain on their way out.
	mu        sync.Mutex
	started   bool
	closed    bool
	searching bool
}

// NewTiered sizes an instance from a config tier: worker count from Cores,
// shared table from TTBytes. Easy's zero bytes disables the table.
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

// StartPonder is the ponder seam, a documented no-op until ponder lands.
func (s *SMP) StartPonder(b *rules.Board) {}

// StopPonder halts a ponder run started by StartPonder, a no-op until then.
func (s *SMP) StopPonder() {}

var _ Ponderer = (*SMP)(nil)

// Close stops the worker pool and waits for every worker to leave its
// thread. Idempotent. No allocation. An in-flight Search is joined: its
// workers serve their dispatched jobs and the driver returns before the
// last worker exits.
func (s *SMP) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	if s.started {
		close(s.quit)
		s.mu.Unlock()
		s.exitWG.Wait()
		return
	}
	s.mu.Unlock()
}

// startPool launches the persistent workers exactly once. The caller holds
// mu: closed is only written under mu and dispatch checks it before calling
// this, so a Close arriving later is guaranteed to see started and join,
// and the pool can never leak.
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

// worker is the persistent locked-thread loop: serve jobs while hot, park
// on the wake channel once idle past SearchWorkerParkDelayMs, and leave on
// Close. The hot window is what keeps a bench or a ponder loop free of
// blocking channel operations, and parking is what keeps an idle instance
// off the cores.
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

// serveQueuedJob serves a dispatched job that is still queued when its
// worker sees quit. The driver counts every wake token in runWG before any
// worker can observe quit, since dispatch and close serialize on mu, so
// leaving one unserved would hang SearchDepth on runWG.Wait.
func (s *SMP) serveQueuedJob(w *Engine, b *rules.Board, res *workerResult) bool {
	select {
	case <-s.wake:
		s.serveJob(w, b, res)
		return true
	default:
		return false
	}
}

// serveJob runs one dispatched search and releases its runWG slot, deferred
// so even a panicking search cannot hang the driver's Wait.
func (s *SMP) serveJob(w *Engine, b *rules.Board, res *workerResult) {
	defer s.runWG.Done()
	s.runWorker(w, b, res)
}

// serveHot spins over the non-blocking wake check for the park delay
// window and reports whether a job was served. The spin deliberately never
// yields the P: Gosched would hand the worker's thread around the scheduler
// and allocate, while holding the P is what keeps the driver and the hot
// workers running side by side. time.Now is the preemption point. Quit is
// left to the parked select: Close simply waits out the remaining spin.
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

// SearchDepth is the capped SMP driver behind Search, mirroring the single
// threaded Engine.SearchDepth contract: only completed iterations count, a
// legal fallback covers budgets too small for one iteration, and no soft
// stop, so a fixed target depth is reached unless the hard deadline fires.
// Zero allocation once the pool runs.
func (s *SMP) SearchDepth(b *rules.Board, dl Deadline, maxDepth int) (rules.Move, SearchStats) {
	return s.searchDepth(b, dl, maxDepth, false)
}

func (s *SMP) searchDepth(b *rules.Board, dl Deadline, maxDepth int, soft bool) (rules.Move, SearchStats) {
	start := time.Now()
	var stats SearchStats
	stats.Threads = len(s.workers)
	if bg, ok := dl.(Budgeter); ok {
		stats.AllocNs = int64(bg.Budget())
	}
	if b.IsFull() {
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
	s.searching = true
	s.mu.Unlock()
	defer s.clearSearching()
	s.tt.bumpGen()
	s.halt.Store(false)
	s.seq.Store(0)
	s.haltDL.inner = dl
	s.jobMaxDepth = maxDepth
	s.jobSoft = soft
	for i := range s.workers {
		// Each worker searches its own value copy of the root: Make and
		// Unmake mutate the board, only the table is shared.
		s.boards[i] = *b
		s.workers[i].resetForSearch(&s.boards[i])
		s.results[i] = workerResult{}
	}
	fallback := s.workers[0].fallbackMove(&s.boards[0])
	s.ensureProcs()
	s.dispatch()
	s.runWG.Wait()

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
	bestMove := fallback
	if best >= 0 {
		r := &s.results[best]
		stats.Depth = r.completed
		stats.Score = r.score
		stats.PVLen = r.pvLen
		stats.PV = r.pv
		bestMove = r.move
	}
	stats.ElapsedNs = int64(time.Since(start))
	stats.Nps = npsReport(stats.Nodes, stats.ElapsedNs)
	stats.EBFMilli = ebfMilli(stats.Nodes, stats.Depth)
	if ttProbes > 0 {
		stats.TTHitPermille = int(ttHits * 1000 / ttProbes)
	}
	stats.HashFullPermille = s.tt.hashFullPermille()
	if cutNodes > 0 {
		stats.FirstMoveFailHighPermille = int(cutFirst * 1000 / cutNodes)
	}
	return bestMove, stats
}

// runWorker is one locked-thread lazy SMP solve: plain iterative deepening
// against the shared table, reporting every completed iteration into the
// worker's slot. Soft jobs consult the same soft stop rule as the single
// threaded driver, against the grant carried by the wrapped inner deadline
// and this worker's own banked depth. Zero allocation by construction, same
// code path as the single threaded driver.
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

// clearSearching releases the single-search latch on every exit path of
// SearchDepth, including panics from the dispatch handshakes.
func (s *SMP) clearSearching() {
	s.mu.Lock()
	s.searching = false
	s.mu.Unlock()
}

// dispatch hands every worker one wake token under mu. The closed recheck
// is what makes Close racing a starting search loud: a Close that slipped
// in since the entry check is caught here, before any token is counted, so
// the pool never spawns into an instance nobody will join.
func (s *SMP) dispatch() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		panic("engine: Search raced Close on an SMP instance")
	}
	s.startPool()
	s.runWG.Add(len(s.workers))
	for range s.workers {
		s.wake <- struct{}{}
	}
	s.mu.Unlock()
}

// ensureProcs gives the locked workers and the driver room to run in
// parallel: one P for the driver plus one per hot worker, since a spinning
// worker holds its P for the whole park delay window. The spec pins
// instance threads with LockOSThread and GOMAXPROCS(N); a library must not
// lower the host process's parallelism, so this only raises the ceiling.
// Bounding instances against each other belongs to the tournament
// conductor.
func (s *SMP) ensureProcs() {
	want := len(s.workers) + 1
	if runtime.GOMAXPROCS(0) < want {
		runtime.GOMAXPROCS(want)
	}
}
