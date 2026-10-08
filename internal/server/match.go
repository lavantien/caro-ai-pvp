package server

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

const (
	EventKindMove    = "move"
	EventKindMLine   = "mline"
	EventKindGameEnd = "gameend"
	EventKindSeries  = "series"
)
const (
	WonByOpenFour   = "open 4"
	WonByDoubleFour = "double 4"
	WonByFour       = "4"
)

func (r *Room) Ready(userID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over {
		return ErrRoomClosed
	}
	if r.series == nil {
		return ErrNotReady
	}
	before := r.series.State()
	if err := r.series.Ready(userID); err != nil {
		return err
	}
	if before == SeriesCreated && r.series.State() == SeriesReady {
		r.startGameLocked()
	}
	return nil
}
func (r *Room) startGameLocked() {
	r.closeEnginesLocked()
	r.board = rules.NewBoard()
	r.moves = r.moves[:0]
	r.mlines = r.mlines[:0]
	r.clock[rules.Red] = clock.NewGameClock(r.tcIdx)
	r.clock[rules.Blue] = clock.NewGameClock(r.tcIdx)
	for _, c := range [2]rules.Color{rules.Red, rules.Blue} {
		if seat := r.seatByColorLocked(c); seat.bot != nil {
			r.engines[c] = r.makeSearcher(*seat.bot)
		}
	}
	r.turnStart = time.Now()
	r.wakeBotLocked()
}
func (r *Room) PlayMove(userID int64, cell rules.Cell) error {
	err := r.playMove(userID, cell)
	r.maybeFinish()
	return err
}
func (r *Room) maybeFinish() {
	r.mu.Lock()
	done := r.over
	r.mu.Unlock()
	if done {
		r.finishLifecycle()
	}
}
func (r *Room) playMove(userID int64, cell rules.Cell) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over {
		return ErrRoomClosed
	}
	if userID != r.host.userID && (r.guest.userID == 0 || userID != r.guest.userID) {
		return ErrNotParticipant
	}
	if r.series == nil || r.board == nil {
		return ErrNotReady
	}
	side := r.board.Side
	mover := r.seatByColorLocked(side)
	if mover.bot != nil || mover.userID != userID {
		return ErrNotYourTurn
	}
	if !r.board.IsLegal(cell) {
		return ErrIllegalMove
	}
	r.applyMoveLocked(side, cell)
	if r.board.FastLastMoveWin(side, cell) {
		return r.completeGameLocked(side, cell)
	}
	if r.board.IsFull() {
		return r.completeGameLocked(rules.Empty, cell)
	}
	r.turnStart = time.Now()
	r.wakeBotLocked()
	return nil
}
func (r *Room) applyMoveLocked(side rules.Color, cell rules.Cell) {
	r.board.Make(cell)
	r.moves = append(r.moves, rules.Move(cell))
	r.clock[side].Commit(time.Since(r.turnStart))
	r.publishLocked(Event{Kind: EventKindMove, Payload: cellName(cell)})
}
func (r *Room) completeGameLocked(winner rules.Color, lastCell rules.Cell) error {
	r.lastMoves = append(r.lastMoves[:0], r.moves...)
	redUser, blueUser := r.seatByColorLocked(rules.Red).userID, r.seatByColorLocked(rules.Blue).userID
	outcome := Draw
	var wonBy *string
	if winner != rules.Empty {
		outcome = RedWins
		if winner == rules.Blue {
			outcome = BlueWins
		}
		r.board.Unmake()
		tag := WonByTag(r.board, winner)
		r.board.Make(lastCell)
		wonBy = &tag
	}
	if err := r.series.RecordResult(outcome); err != nil {
		return err
	}
	perr := r.persistGameLocked(redUser, blueUser, outcome, wonBy)
	r.publishLocked(Event{Kind: EventKindGameEnd, Payload: outcome.String()})
	if r.series.State() == SeriesFinished {
		r.over = true
		r.publishLocked(Event{Kind: EventKindSeries, Payload: r.series.Winner().String()})
	} else {
		r.startGameLocked()
	}
	return perr
}
func (r *Room) persistGameLocked(redUser, blueUser int64, outcome Outcome, wonBy *string) error {
	if r.seriesID == 0 {
		return nil
	}
	seriesID, idx := r.seriesID, r.series.GamesPlayed()-1
	blob, fullTurns := EncodeMoves(nil, r.moves), len(r.moves)/2
	unit := Completion{Games: []Game{{
		SeriesID: seriesID, IdxInSeries: idx, RedUser: redUser, BlueUser: blueUser,
		Outcome: outcome.String(), Moves: blob, FullTurns: fullTurns, WonBy: wonBy,
		BotName: r.botDisplayNameLocked(), StatLines: append([]GameStat(nil), r.mlines...),
	}}}
	if r.series.State() == SeriesFinished {
		unit.Finish = &SeriesFinish{
			SeriesID: seriesID, Winner: r.seriesWinnerUser(), FinishedAt: time.Now().Unix(),
		}
	}
	return r.wq.Send(func(ctx context.Context) error {
		return r.store.ApplyCompletion(ctx, unit)
	})
}
func (r *Room) botDisplayNameLocked() string {
	if tier := botTierName(r.guest.bot); tier != "" {
		return botDisplayName(tier, r.id)
	}
	return botDisplayName(botTierName(r.host.bot), r.id)
}
func botDisplayName(tier, roomID string) string {
	if tier == "" {
		return ""
	}
	return tier + "-" + roomID
}
func (r *Room) Forfeit(userID int64) error {
	r.mu.Lock()
	if r.over {
		r.mu.Unlock()
		return ErrRoomClosed
	}
	if userID != r.host.userID && (r.guest.userID == 0 || userID != r.guest.userID) {
		r.mu.Unlock()
		return ErrNotParticipant
	}
	if r.series == nil {
		r.publishLocked(Event{Kind: EventKindSeries, Payload: SideNone.String()})
		r.mu.Unlock()
		r.finishLifecycle()
		return nil
	}
	liveMoves := append([]rules.Move(nil), r.moves...)
	if err := r.series.Forfeit(userID); err != nil {
		r.mu.Unlock()
		return err
	}
	err := r.persistForfeitLocked(liveMoves)
	r.over = true
	r.publishLocked(Event{Kind: EventKindSeries, Payload: r.series.Winner().String()})
	r.mu.Unlock()
	r.finishLifecycle()
	return err
}
func (r *Room) persistForfeitLocked(liveMoves []rules.Move) error {
	if r.seriesID == 0 {
		return nil
	}
	seriesID := r.seriesID
	synth := r.series.SyntheticGames()
	firstLive := r.series.GamesPlayed() - len(synth) + 1
	unit := Completion{Games: make([]Game, 0, len(synth))}
	for _, g := range synth {
		blob, turns, stats := []byte{}, 0, []GameStat(nil)
		if g.GameNo == firstLive {
			blob, turns = EncodeMoves(nil, liveMoves), len(liveMoves)/2
			stats = append([]GameStat(nil), r.mlines...)
		}
		unit.Games = append(unit.Games, Game{
			SeriesID: seriesID, IdxInSeries: g.GameNo - 1,
			RedUser: g.RedUserID, BlueUser: g.BlueUserID,
			Outcome: g.Outcome.String(), Moves: blob, FullTurns: turns,
			StatLines: stats,
		})
	}
	unit.Finish = &SeriesFinish{
		SeriesID: seriesID, Winner: r.seriesWinnerUser(), FinishedAt: time.Now().Unix(),
	}
	return r.wq.Send(func(ctx context.Context) error {
		return r.store.ApplyCompletion(ctx, unit)
	})
}
func (r *Room) seriesWinnerUser() *int64 {
	switch r.series.Winner() {
	case SideHost:
		id := r.host.userID
		return &id
	case SideGuest:
		id := r.guest.userID
		return &id
	}
	return nil
}
func (r *Room) seatOfUserLocked(userID int64) *seat {
	if r.host.userID == userID {
		return &r.host
	}
	return &r.guest
}
func (r *Room) seatByColorLocked(c rules.Color) seat {
	red := r.series.RedUserID()
	if c == rules.Red {
		return *r.seatOfUserLocked(red)
	}
	if r.host.userID == red {
		return r.guest
	}
	return r.host
}
func (r *Room) publishLocked(ev Event) {
	r.hub.Publish(r.id, ev)
}
func (r *Room) ClockRemaining(c rules.Color) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clock[c] == nil {
		return 0
	}
	return r.clock[c].Remaining()
}
func cellName(cell rules.Cell) string {
	return string(appendCellName(nil, rules.Move(cell)))
}
func EncodeMoves(dst []byte, moves []rules.Move) []byte {
	if dst == nil {
		dst = []byte{}
	}
	for _, m := range moves {
		dst = append(dst, byte(m), byte(m>>8))
	}
	return dst
}
func DecodeMoves(blob []byte) ([]rules.Move, error) {
	if len(blob)%2 != 0 {
		return nil, errors.New("server: moves blob length is not a move count")
	}
	moves := make([]rules.Move, 0, len(blob)/2)
	for i := 0; i < len(blob); i += 2 {
		m := rules.Move(blob[i]) | rules.Move(blob[i+1])<<8
		if int(m) >= config.BoardCells {
			return nil, fmt.Errorf("server: moves blob cell %d out of board", m)
		}
		moves = append(moves, m)
	}
	return moves, nil
}
func WonByTag(pre *rules.Board, winner rules.Color) string {
	if hasOpenFourWindow(pre, winner) {
		return WonByOpenFour
	}
	if countWinIn1(pre, winner) >= 2 {
		return WonByDoubleFour
	}
	return WonByFour
}
func hasOpenFourWindow(pre *rules.Board, winner rules.Color) bool {
	for cell := range config.BoardCells {
		for dir := range config.PatternDirections {
			e := pattern.Lookup(dir, pattern.Index(pre, rules.Cell(cell), dir, winner))
			if e.Class == config.PatternClassOpenFour {
				return true
			}
		}
	}
	return false
}
func countWinIn1(pre *rules.Board, winner rules.Color) int {
	side := pre.Side
	defer func() { pre.Side = side }()
	n := 0
	for cell := range config.BoardCells {
		c := rules.Cell(cell)
		if pre.At(c) != rules.Empty {
			continue
		}
		pre.Side = winner
		pre.Make(c)
		if pre.FastLastMoveWin(winner, c) {
			n++
		}
		pre.Unmake()
	}
	return n
}

type searcher interface {
	Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string)
	StartPonder(b *rules.Board)
	StopPonder() (rules.Move, engine.SearchStats, string)
	Close()
}
type Searcher = searcher

func NewBotSearcher(t config.Tier) Searcher { return newBotSearcher(t) }

type singleSearcher struct{ e *engine.Engine }

func (s singleSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	mv, st := s.e.Search(b, dl)
	return mv, st, ""
}
func (singleSearcher) Close() {}

func (singleSearcher) StartPonder(*rules.Board) {}

func (singleSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

type tierSearcher struct{ smp *engine.SMP }

func (s tierSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	mv, st := s.smp.Search(b, dl)
	return mv, st, ""
}
func (s tierSearcher) Close() { s.smp.Close() }

func (s tierSearcher) StartPonder(b *rules.Board) { s.smp.StartPonder(b) }

func (s tierSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	mv, st := s.smp.StopPonder()
	return mv, st, ""
}

type solverSearcher struct {
	inner        searcher
	vcf          *vcf.Solver
	vct          *vcf.Solver
	cores        int
	proofPending bool
	proofMove    rules.Move
	proofStats   engine.SearchStats
	proofTag     string
}

func (s *solverSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	start := time.Now()
	grant := time.Duration(0)
	hasGrant := false
	if bg, ok := dl.(engine.Budgeter); ok {
		grant = bg.Budget()
		hasGrant = true
	}
	passes := []struct {
		solver *vcf.Solver
		tag    string
	}{{s.vcf, config.BotLogTagVCF}, {s.vct, config.BotLogTagVCT}}
	if hasGrant && grant < config.SolverMinGrantMs*time.Millisecond {
		passes = nil
	}
	for _, pass := range passes {
		if pass.solver == nil {
			continue
		}
		var share engine.Deadline
		if hasGrant {
			share = engine.NewFixedBudget(time.Duration(config.SolverBudgetShare * float64(grant-time.Since(start))))
		}
		var out vcf.SolverStats
		if pass.solver.Solve(b, config.SolverNodeBudget, share, &out) {
			return rules.Move(out.PV[0]), solverStats(s.cores, grant, &out), pass.tag
		}
	}
	rest := dl
	if hasGrant {
		rest = engine.NewFixedBudget(grant - time.Since(start))
	}
	mv, st, _ := s.inner.Search(b, rest)
	if hasGrant {
		st.AllocNs = int64(grant)
	}
	return mv, st, ""
}
func (s *solverSearcher) Close() { s.inner.Close() }

func (s *solverSearcher) StartPonder(b *rules.Board) {
	for _, pass := range []struct {
		solver *vcf.Solver
		tag    string
	}{{s.vcf, config.BotLogTagVCF}, {s.vct, config.BotLogTagVCT}} {
		if pass.solver == nil {
			continue
		}
		var out vcf.SolverStats
		if pass.solver.Solve(b, config.SolverNodeBudget, nil, &out) {
			s.proofPending = true
			s.proofMove = rules.Move(out.PV[0])
			s.proofStats = solverStats(s.cores, 0, &out)
			s.proofTag = pass.tag
			break
		}
	}
	s.inner.StartPonder(b)
}

func (s *solverSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	mv, st, tag := s.inner.StopPonder()
	if s.proofPending {
		s.proofPending = false
		return s.proofMove, s.proofStats, s.proofTag
	}
	return mv, st, tag
}
func solverStats(cores int, grant time.Duration, out *vcf.SolverStats) engine.SearchStats {
	var st engine.SearchStats
	st.Depth = out.Plies
	st.Nodes = out.Nodes
	st.Nps = engine.NpsReport(out.Nodes, out.ElapsedNs)
	st.EBFMilli = engine.EBFMilli(out.Nodes, out.Plies)
	st.Score = config.EvalMateMax - out.Plies*config.EvalMateScoreStep
	st.Threads = cores
	st.ElapsedNs = out.ElapsedNs
	st.AllocNs = int64(grant)
	st.PVLen = out.Plies
	st.PV = out.PV
	return st
}
func newBotSearcher(t config.Tier) searcher {
	var inner searcher
	if t.Cores > 1 {
		inner = tierSearcher{smp: engine.NewTiered(t)}
	} else {
		inner = singleSearcher{e: engine.New(t.TTBytes)}
	}
	if !t.VCF && !t.VCT {
		return inner
	}
	s := &solverSearcher{inner: inner, cores: t.Cores}
	if t.VCF {
		s.vcf = vcf.New(vcf.KindVCF)
	}
	if t.VCT {
		s.vct = vcf.New(vcf.KindVCT)
	}
	return s
}
func (r *Room) closeEnginesLocked() {
	for i := range r.engines {
		if r.engines[i] != nil {
			r.engines[i].Close()
			r.engines[i] = nil
		}
	}
}

type mLineRecord struct {
	moveNumber int
	side       rules.Color
	move       rules.Move
	stats      engine.SearchStats
	tag        string
}

func (r *Room) startBotWorker() {
	r.wg.Add(1)
	go r.botLoop()
}
func (r *Room) botLoop() {
	defer r.wg.Done()
	defer r.closeEnginesAtExit()
	for {
		if r.runBotTurn() {
			r.maybeFinish()
			continue
		}
		select {
		case <-r.wake:
		case <-r.quit:
			return
		}
	}
}
func (r *Room) closeEnginesAtExit() {
	r.mu.Lock()
	r.closeEnginesLocked()
	r.mu.Unlock()
}
func (r *Room) wakeBotLocked() {
	if _, ok := r.botTurnLocked(); ok {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
}
func (r *Room) botTurnLocked() (rules.Color, bool) {
	if r.over || r.board == nil {
		return 0, false
	}
	side := r.board.Side
	if r.seatByColorLocked(side).bot == nil {
		return 0, false
	}
	return side, true
}
func (r *Room) runBotTurn() bool {
	r.mu.Lock()
	side, ok := r.botTurnLocked()
	if !ok {
		r.mu.Unlock()
		return false
	}
	budget := r.clock[side].Budget()
	if r.budgetCap > 0 && budget > r.budgetCap {
		budget = r.budgetCap
	}
	board, moveCount, eng := *r.board, r.board.MoveCount, r.engines[side]
	entry := r.searchEntry
	r.mu.Unlock()
	if entry != nil {
		entry()
	}
	mv, st, tag := eng.Search(&board, engine.NewFixedBudget(budget))
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.over || r.board == nil || r.board.MoveCount != moveCount || r.board.Side != side {
		return false
	}
	if !r.board.IsLegal(rules.Cell(mv)) {
		mv = r.firstLegalLocked()
	}
	cell := rules.Cell(mv)
	r.applyMoveLocked(side, cell)
	r.lastM = mLineRecord{moveNumber: r.board.MoveCount, side: side, move: mv, stats: st, tag: tag}
	line := MLine(r.lastM.moveNumber, side, mv, &st, tag)
	r.mlines = append(r.mlines, GameStat{MoveNo: r.lastM.moveNumber, Line: line})
	r.publishLocked(Event{Kind: EventKindMLine, Payload: line})
	if r.board.FastLastMoveWin(side, cell) {
		logBotCompletion(r.completeGameLocked(side, cell))
		return true
	}
	if r.board.IsFull() {
		logBotCompletion(r.completeGameLocked(rules.Empty, cell))
		return true
	}
	r.turnStart = time.Now()
	r.wakeBotLocked()
	return true
}
func (r *Room) firstLegalLocked() rules.Move {
	n := r.board.LegalMoves(r.legalBuf[:])
	if n == 0 {
		panic("server: bot turn on a board with no legal move")
	}
	return r.legalBuf[0]
}
func logBotCompletion(err error) {
	if err != nil {
		log.Printf("server: bot game completion persistence failed: %v", err)
	}
}
