package engine

import (
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const moveNone = rules.Move(config.BoardCells)

type Engine struct {
	tt        *ttTable
	radius    int
	moves     [config.SearchMaxPly][config.SearchMaxMovesPerPly]rules.Move
	scores    [config.SearchMaxPly][config.SearchMaxMovesPerPly]int
	killers   [config.SearchMaxPly][2]rules.Move
	history   [2][config.BoardCells]int
	pv        [config.SearchMaxPly + 1][config.SearchMaxPly]rules.Move
	pvLen     [config.SearchMaxPly + 1]int
	eval      evaluator
	nodes     uint64
	ttProbes  uint64
	ttHits    uint64
	cutNodes  uint64
	cutFirst  uint64
	nodeCheck int
	stopped   bool
	noOrder   bool
}

func New(ttBytes int64) *Engine {
	return NewEval(ttBytes, nil)
}

func NewEval(ttBytes int64, w *[config.PatternClassCount]int) *Engine {
	e := newEngineShared(newTT(ttBytes))
	if w != nil {
		e.eval.w = w
	}
	return e
}

func newEngineShared(tt *ttTable) *Engine {
	return &Engine{radius: config.SearchRingRadius, tt: tt}
}

func mateWin(ply int) int {
	return config.EvalMateMax - (ply+1)*config.EvalMateScoreStep
}

func (e *Engine) beginSearch(b *rules.Board) {
	e.tt.bumpGen()
	e.resetForSearch(b)
}

func (e *Engine) resetForSearch(b *rules.Board) {
	clear(e.history[:])
	for ply := range e.killers {
		e.killers[ply][0], e.killers[ply][1] = moveNone, moveNone
	}
	clear(e.pvLen[:])
	e.nodes, e.ttProbes, e.ttHits, e.cutNodes, e.cutFirst = 0, 0, 0, 0, 0
	e.nodeCheck = config.SearchNodeCheckInterval
	e.stopped = false
	e.eval.reset(b)
}

func (e *Engine) Search(b *rules.Board, dl Deadline) (rules.Move, SearchStats) {
	return e.search(b, dl, config.SearchMaxPly, true)
}

func (e *Engine) SearchDepth(b *rules.Board, dl Deadline, maxDepth int) (rules.Move, SearchStats) {
	return e.search(b, dl, maxDepth, false)
}

func softStop(elapsed, budget time.Duration, completed int) bool {
	if completed < 1 {
		return false
	}
	if elapsed <= 0 {
		elapsed = time.Duration(config.SearchClockQuantumMs) * time.Millisecond
	}
	return elapsed > time.Duration(config.SearchSoftStopFraction*float64(budget))
}

func (e *Engine) search(b *rules.Board, dl Deadline, maxDepth int, soft bool) (rules.Move, SearchStats) {
	start := time.Now()
	e.beginSearch(b)
	var stats SearchStats
	stats.Threads = 1
	var budget time.Duration
	hasBudget := false
	if bg, ok := dl.(Budgeter); ok {
		budget = bg.Budget()
		stats.AllocNs = int64(budget)
		hasBudget = true
	}
	if b.IsFull() {
		stats.ElapsedNs = int64(time.Since(start))
		return moveNone, stats
	}
	best, bestMove, completed := 0, e.fallbackMove(b), 0
	var rootMoves [config.PonderDepthHistory]rules.Move
	var rootScores [config.PonderDepthHistory]int
	rootIters := 0
	for depth := 1; depth <= maxDepth; depth++ {
		if e.stopped || dl.Exceeded() {
			break
		}
		if soft && hasBudget && softStop(time.Since(start), budget, completed) {
			break
		}
		score, move := e.searchRoot(b, depth, dl)
		if e.stopped {
			break
		}
		best, bestMove, completed = score, move, depth
		rootMoves[(depth-1)%config.PonderDepthHistory] = move
		rootScores[(depth-1)%config.PonderDepthHistory] = score
		rootIters = depth
		stats.PV = e.pv[0]
		stats.PVLen = e.pvLen[0]
		if score >= config.EvalMateMax-config.EvalMateScoreStep {
			break
		}
	}
	n := min(config.PonderDepthHistory, rootIters)
	for j := range n {
		slot := (rootIters - n + j) % config.PonderDepthHistory
		stats.RootMoves[j] = rootMoves[slot]
		stats.RootScores[j] = rootScores[slot]
	}
	stats.RootIters = rootIters
	e.finishStats(&stats, best, completed, start)
	return bestMove, stats
}

func (e *Engine) fallbackMove(b *rules.Board) rules.Move {
	n := e.generate(b, 0, moveNone)
	pickMax(e.moves[0][:n], e.scores[0][:n], 0, n)
	return e.moves[0][0]
}

func (e *Engine) finishStats(stats *SearchStats, score int, depth int, start time.Time) {
	stats.Depth = depth
	stats.Nodes = e.nodes
	stats.Score = score
	stats.ElapsedNs = int64(time.Since(start))
	stats.Nps = NpsReport(stats.Nodes, stats.ElapsedNs)
	stats.EBFMilli = EBFMilli(e.nodes, depth)
	if e.ttProbes > 0 {
		stats.TTHitPermille = int(e.ttHits * 1000 / e.ttProbes)
	}
	stats.HashFullPermille = e.tt.hashFullPermille()
	if e.cutNodes > 0 {
		stats.FirstMoveFailHighPermille = int(e.cutFirst * 1000 / e.cutNodes)
	}
}

func (e *Engine) searchRoot(b *rules.Board, depth int, dl Deadline) (int, rules.Move) {
	n := e.generate(b, 0, e.tt.move(b.Hash))
	alpha, beta := -config.EvalMateMax, config.EvalMateMax
	best, bestMove := alpha-1, moveNone
	e.pvLen[0] = 0
	for i := 0; i < n; i++ {
		pickMax(e.moves[0][:n], e.scores[0][:n], i, n)
		m := e.moves[0][i]
		side := b.Side
		e.eval.makeMove(side, rules.Cell(m))
		b.Make(rules.Cell(m))
		var s int
		switch {
		case b.FastLastMoveWin(side, rules.Cell(m)):
			s = mateWin(0)
			e.pvLen[1] = 0
		case b.IsFull():
			s = 0
			e.pvLen[1] = 0
		case i == 0:
			s = -e.negamax(b, depth-1, -beta, -alpha, 1, config.SearchExtensionMaxPly, dl)
		default:
			s = -e.negamax(b, depth-1, -(alpha + 1), -alpha, 1, config.SearchExtensionMaxPly, dl)
			if s > alpha && s < beta && !e.stopped {
				s = -e.negamax(b, depth-1, -beta, -alpha, 1, config.SearchExtensionMaxPly, dl)
			}
		}
		e.eval.unmakeMove(rules.Cell(m))
		b.Unmake()
		if e.stopped {
			return 0, moveNone
		}
		if s > best {
			best, bestMove = s, m
			if s > alpha {
				alpha = s
				e.pv[0][0] = m
				copy(e.pv[0][1:], e.pv[1][:e.pvLen[1]])
				e.pvLen[0] = e.pvLen[1] + 1
			}
		}
	}
	e.tt.store(b.Hash, best, bestMove, depth, ttBoundExact, 0)
	return best, bestMove
}

func (e *Engine) negamax(b *rules.Board, depth int, alpha, beta int, ply int, extLeft int, dl Deadline) int {
	e.nodes++
	if e.nodeCheck--; e.nodeCheck <= 0 {
		e.nodeCheck = config.SearchNodeCheckInterval
		if dl.Exceeded() {
			e.stopped = true
		}
	}
	if e.stopped {
		return 0
	}
	e.pvLen[ply] = 0
	if extLeft > 0 && e.eval.fours[b.Side^1] > 0 {
		depth++
		extLeft--
	}
	if depth <= 0 || ply >= config.SearchMaxPly {
		return e.eval.eval(b.Side)
	}
	ttm := moveNone
	if e.tt.enabled() {
		e.ttProbes++
		score, m, cutoff := e.tt.probe(b.Hash, depth, alpha, beta, ply)
		if cutoff {
			e.ttHits++
			return score
		}
		if m >= 0 {
			ttm = rules.Move(m)
		}
	}
	n := e.generate(b, ply, ttm)
	best, bestMove := -(config.EvalMateMax + 1), moveNone
	a0 := alpha
	for i := 0; i < n; i++ {
		pickMax(e.moves[ply][:n], e.scores[ply][:n], i, n)
		m := e.moves[ply][i]
		side := b.Side
		e.eval.makeMove(side, rules.Cell(m))
		b.Make(rules.Cell(m))
		var s int
		switch {
		case b.FastLastMoveWin(side, rules.Cell(m)):
			s = mateWin(ply)
			e.pvLen[ply+1] = 0
		case b.IsFull():
			s = 0
			e.pvLen[ply+1] = 0
		case i == 0:
			s = -e.negamax(b, depth-1, -beta, -alpha, ply+1, extLeft, dl)
		default:
			s = -e.negamax(b, depth-1, -(alpha + 1), -alpha, ply+1, extLeft, dl)
			if s > alpha && s < beta && !e.stopped {
				s = -e.negamax(b, depth-1, -beta, -alpha, ply+1, extLeft, dl)
			}
		}
		e.eval.unmakeMove(rules.Cell(m))
		b.Unmake()
		if e.stopped {
			return 0
		}
		if s > best {
			best, bestMove = s, m
		}
		if s >= beta {
			e.cutNodes++
			if i == 0 {
				e.cutFirst++
			}
			e.recordCutoff(m, side, ply, depth)
			break
		}
		if s > alpha {
			alpha = s
			e.pv[ply][0] = m
			copy(e.pv[ply][1:], e.pv[ply+1][:e.pvLen[ply+1]])
			e.pvLen[ply] = e.pvLen[ply+1] + 1
		}
	}
	bound := ttBoundExact
	if best >= beta {
		bound = ttBoundLower
	} else if best <= a0 {
		bound = ttBoundUpper
	}
	e.tt.store(b.Hash, best, bestMove, depth, bound, ply)
	return best
}
