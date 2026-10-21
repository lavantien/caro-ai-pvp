package server

import (
	"log"
	"sync"
	"time"

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

type SoftLimitCrossed bool

func AssessSoftLimit(elapsedNs, budgetNs int64) SoftLimitCrossed {
	return SoftLimitCrossed(elapsedNs >= int64(config.PonderAdoptFraction*float64(budgetNs)))
}

func AdoptPonder(stats engine.SearchStats, tag string, soft SoftLimitCrossed, ring GrantDepthRing, grantNs int64) bool {
	if tag != "" {
		return true
	}
	ref, ok := ring.ReferenceDepth(grantNs)
	if (!ok || stats.Depth < ref) && !bool(soft) {
		return false
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
	return drop <= config.PonderScoreDropMargin
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
	side    rules.Color
	eng     searcher
	board   rules.Board
	predict rules.Move
	base    int
	adm     *Admission
	cores   int
	done    chan struct{}
	result  *ponderResult
}

type ponderLane struct {
	mu        sync.Mutex
	cond      *sync.Cond
	queue     []ponderCmd
	closed    bool
	closeDone chan struct{}
	active    [2]*ponderCmd
}

func newPonderLane() *ponderLane {
	l := &ponderLane{}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *ponderLane) start(eng searcher, side rules.Color, board *rules.Board, predict rules.Move, base int, adm *Admission, cores int) {
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}
	l.queue = append(l.queue, ponderCmd{kind: ponderStart, side: side, eng: eng, board: *board, predict: predict, base: base, adm: adm, cores: cores})
	l.cond.Signal()
	l.mu.Unlock()
}

func (l *ponderLane) stop(side rules.Color) ponderResult {
	var res ponderResult
	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return res
	}
	done := make(chan struct{})
	l.queue = append(l.queue, ponderCmd{kind: ponderStop, side: side, done: done, result: &res})
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
	done := l.closeDone
	l.queue = append(l.queue, ponderCmd{kind: ponderClose, done: done})
	l.cond.Signal()
	l.mu.Unlock()
	<-done
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
		if !l.dispatch(cmd) {
			return
		}
	}
}

func (l *ponderLane) dispatch(cmd ponderCmd) (keep bool) {
	keep = true
	defer func() {
		if r := recover(); r != nil {
			log.Printf("server: ponder lane aborted after an engine panic: %v", r)
			if cmd.kind != ponderClose && cmd.done != nil {
				close(cmd.done)
			}
			l.abort()
			keep = false
		}
	}()
	switch cmd.kind {
	case ponderStart:
		if l.active[cmd.side] == nil {
			if cmd.cores > 0 && cmd.adm != nil && !cmd.adm.TryAcquireCores(cmd.cores) {
				break
			}
			l.active[cmd.side] = &cmd
			cmd.eng.StartPonder(&cmd.board)
		}
	case ponderStop:
		res := l.halt(cmd.side)
		*cmd.result = res
		close(cmd.done)
	case ponderClose:
		l.halt(rules.Red)
		l.halt(rules.Blue)
		close(cmd.done)
		keep = false
	}
	return keep
}

func (l *ponderLane) abort() {
	l.mu.Lock()
	l.closed = true
	pending := l.queue
	l.queue = nil
	done := l.closeDone
	l.closeDone = nil
	l.mu.Unlock()
	for _, cmd := range pending {
		if cmd.kind != ponderClose && cmd.done != nil {
			close(cmd.done)
		}
	}
	if done != nil {
		close(done)
	}
}

func (l *ponderLane) drain() {
	l.mu.Lock()
	held := [2]*ponderCmd{}
	for side, a := range l.active {
		held[side] = a
		l.active[side] = nil
	}
	l.mu.Unlock()
	for _, a := range held {
		if a != nil && a.cores > 0 && a.adm != nil {
			a.adm.ReleaseCores(a.cores)
		}
	}
}

func (l *ponderLane) halt(side rules.Color) ponderResult {
	a := l.active[side]
	if a == nil {
		return ponderResult{}
	}
	l.active[side] = nil
	mv, st, tag := a.eng.StopPonder()
	if a.cores > 0 && a.adm != nil {
		a.adm.ReleaseCores(a.cores)
	}
	return ponderResult{active: true, move: mv, stats: st, tag: tag, predict: a.predict, base: a.base}
}

func (r *Room) collectPonder(side rules.Color, budgetNs int64, turnStart time.Time) (rules.Move, engine.SearchStats, string, bool) {
	if r.ponder == nil {
		return 0, engine.SearchStats{}, "", false
	}
	pres := r.ponder.stop(side)
	if !pres.active {
		return 0, engine.SearchStats{}, "", false
	}
	r.mu.Lock()
	mc := 0
	var last rules.Move
	if r.board != nil {
		mc = r.board.MoveCount
	}
	if mc > 0 && mc <= len(r.moves) {
		last = r.moves[mc-1]
	}
	ring := r.ponderRing[side]
	r.mu.Unlock()
	if mc == 0 || mc != pres.base+1 || last != pres.predict {
		return 0, engine.SearchStats{}, "", false
	}
	if !AdoptPonder(pres.stats, pres.tag, AssessSoftLimit(pres.stats.ElapsedNs, budgetNs), ring, budgetNs) {
		return 0, engine.SearchStats{}, "", false
	}
	st, tag := ComposePonderStats(pres.stats, pres.tag, turnStart, budgetNs)
	return pres.move, st, tag, true
}

func ComposePonderStats(src engine.SearchStats, tag string, turnStart time.Time, budgetNs int64) (engine.SearchStats, string) {
	nps := engine.NpsReport(src.Nodes, src.ElapsedNs)
	src.ElapsedNs = int64(time.Since(turnStart))
	src.Nps = nps
	src.AllocNs = budgetNs
	if tag == "" {
		tag = config.BotLogTagPonder
	}
	return src, tag
}

type ponderArm struct {
	ok      bool
	eng     searcher
	side    rules.Color
	board   rules.Board
	predict rules.Move
	base    int
	cores   int
}

type ttReplySearcher interface {
	TTMove(b *rules.Board) rules.Move
}

func (s tierSearcher) TTMove(b *rules.Board) rules.Move { return s.smp.TTMove(b) }

func (s *solverSearcher) TTMove(b *rules.Board) rules.Move { return searchTTMove(s.inner, b) }

func (s *bookSearcher) TTMove(b *rules.Board) rules.Move { return searchTTMove(s.inner, b) }

func searchTTMove(eng searcher, b *rules.Board) rules.Move {
	if p, ok := eng.(ttReplySearcher); ok {
		return p.TTMove(b)
	}
	return rules.Move(config.BoardCells)
}

func ttPonderPredict(eng searcher, b *rules.Board) (rules.Move, bool) {
	mv := searchTTMove(eng, b)
	if !b.IsLegal(rules.Cell(mv)) {
		return 0, false
	}
	return mv, true
}

func (r *Room) ponderArmLocked(side rules.Color, eng searcher, st *engine.SearchStats) ponderArm {
	var arm ponderArm
	if r.ponder == nil || st.PVLen < 1 {
		return arm
	}
	tier := r.seatByColorLocked(side).bot
	if tier.PonderThreads < 1 {
		return arm
	}
	predict := st.PV[1]
	if st.PVLen == 1 {
		ttPredict, ok := ttPonderPredict(eng, r.board)
		if !ok {
			return arm
		}
		predict = ttPredict
	}
	arm.eng = eng
	arm.side = side
	arm.board = *r.board
	arm.predict = predict
	arm.base = arm.board.MoveCount
	arm.cores = tier.PonderThreads
	arm.ok = true
	return arm
}

func (r *Room) dispatchPonderArm(arm *ponderArm) {
	if !arm.ok || r.ponder == nil {
		return
	}
	if !arm.board.IsLegal(rules.Cell(arm.predict)) {
		return
	}
	arm.board.Make(rules.Cell(arm.predict))
	r.mu.Lock()
	current := !r.over && !r.finished && !r.quitClosed() && r.board != nil && r.board.MoveCount == arm.base &&
		r.board.Side == arm.side.Opponent() && r.engines[arm.side] == arm.eng
	if current {
		r.ponder.start(arm.eng, arm.side, &arm.board, arm.predict, arm.base, r.manager.adm, arm.cores)
	}
	r.mu.Unlock()
}
