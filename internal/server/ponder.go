package server

import (
	"sync"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type GrantDepthRing struct {
	grants [config.PonderDepthHistory]int64
	depths [config.PonderDepthHistory]int
	head   int
	n      int
}

func (g *GrantDepthRing) Append(grantNs int64, depth int) {
	g.grants[g.head] = grantNs
	g.depths[g.head] = depth
	g.head = (g.head + 1) % config.PonderDepthHistory
	if g.n < config.PonderDepthHistory {
		g.n++
	}
}

func (g *GrantDepthRing) ReferenceDepth(grantNs int64) (int, bool) {
	ref := -1
	for i := range g.n {
		slot := (g.head - 1 - i + config.PonderDepthHistory) % config.PonderDepthHistory
		if g.grants[slot] >= grantNs/2 && g.grants[slot] <= grantNs*2 {
			if g.depths[slot] > ref {
				ref = g.depths[slot]
			}
		}
	}
	if ref < 0 {
		return 0, false
	}
	return ref, true
}

func AdoptPonder(stats engine.SearchStats, tag string, elapsedNs, budgetNs int64, ring GrantDepthRing, grantNs int64) bool {
	if tag != "" {
		return true
	}
	ref, ok := ring.ReferenceDepth(grantNs)
	if !ok || stats.Depth < ref {
		if elapsedNs < int64(config.PonderAdoptFraction*float64(budgetNs)) {
			return false
		}
	}
	n := min(config.PonderDepthHistory, stats.RootIters)
	if n < config.PonderStableIters {
		return false
	}
	for i := n - config.PonderStableIters; i < n; i++ {
		if stats.RootMoves[i] != stats.RootMoves[n-1] {
			return false
		}
	}
	drop := stats.RootScores[n-1] - stats.RootScores[n-config.PonderStableIters]
	if drop < 0 {
		drop = -drop
	}
	if drop > config.PonderScoreDropMargin {
		return false
	}
	return stats.RootMoves[n-1] == stats.RootMoves[n-2]
}

type ponderResult struct {
	active  bool
	move    rules.Move
	stats   engine.SearchStats
	tag     string
	predict rules.Move
	base    int
}

type ponderCmdKind uint8

const (
	ponderStart ponderCmdKind = iota
	ponderStop
	ponderClose
)

type ponderCmd struct {
	kind    ponderCmdKind
	eng     searcher
	board   rules.Board
	predict rules.Move
	base    int
	done    chan struct{}
	result  *ponderResult
}

type ponderLane struct {
	mu        sync.Mutex
	cond      *sync.Cond
	queue     []ponderCmd
	closed    bool
	closeDone chan struct{}
	active    *ponderCmd
}

func newPonderLane() *ponderLane {
	l := &ponderLane{}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *ponderLane) start(eng searcher, board *rules.Board, predict rules.Move, base int) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.queue = append(l.queue, ponderCmd{kind: ponderStart, eng: eng, board: *board, predict: predict, base: base})
	l.cond.Signal()
	l.mu.Unlock()
}

func (l *ponderLane) stop() ponderResult {
	var res ponderResult
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return res
	}
	done := make(chan struct{})
	l.queue = append(l.queue, ponderCmd{kind: ponderStop, done: done, result: &res})
	l.cond.Signal()
	l.mu.Unlock()
	<-done
	return res
}

func (l *ponderLane) close() {
	l.mu.Lock()
	if l.closed {
		done := l.closeDone
		l.mu.Unlock()
		if done != nil {
			<-done
		}
		return
	}
	l.closed = true
	l.closeDone = make(chan struct{})
	l.queue = append(l.queue, ponderCmd{kind: ponderClose, done: l.closeDone})
	l.cond.Signal()
	l.mu.Unlock()
	<-l.closeDone
}

func (l *ponderLane) run() {
	for {
		l.mu.Lock()
		for len(l.queue) == 0 {
			l.cond.Wait()
		}
		cmd := l.queue[0]
		l.queue = l.queue[1:]
		l.mu.Unlock()
		switch cmd.kind {
		case ponderStart:
			cmd.eng.StartPonder(&cmd.board)
			l.active = &cmd
		case ponderStop:
			res := l.halt()
			*cmd.result = res
			close(cmd.done)
		case ponderClose:
			l.halt()
			close(cmd.done)
			return
		}
	}
}

func (l *ponderLane) halt() ponderResult {
	if l.active == nil {
		return ponderResult{}
	}
	a := l.active
	l.active = nil
	mv, st, tag := a.eng.StopPonder()
	return ponderResult{active: true, move: mv, stats: st, tag: tag, predict: a.predict, base: a.base}
}
