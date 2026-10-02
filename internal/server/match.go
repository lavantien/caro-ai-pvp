package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/pattern"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The match driver: one live game at a time per room, the current board,
// per-side GameClocks, and the single completion path every game end flows
// through. The clock law holds everywhere: a side's legal-move time never
// decides the game. There is no flag fall for a human anywhere (a drained
// clock simply keeps playing) and the bot floor funds moves forever, so
// games end by win or by full board only.

// Hub event kinds of the room layer. move payloads carry the cell name in
// the rules codec, mline the Implication 1.5 bot-log line, gameend the
// outcome (red/blue/draw), series the winning series side
// (host/guest/none).
const (
	EventKindMove    = "move"
	EventKindMLine   = "mline"
	EventKindGameEnd = "gameend"
	EventKindSeries  = "series"
)

// mLineTag is the Implication 1.5 solver tag of an emitted bot line. The
// engine's SearchStats carries no VCF/VCT provenance yet, so every line
// ships untagged; when the drivers expose solver-found wins, feed
// config.BotLogTagVCF and config.BotLogTagVCT here.
const mLineTag = ""

// WonBy vocabulary derivable today, read off the position at the winner's
// last move (move n-1 per Scenario 1): "open 4" the winner held a window
// with two completion cells (the pattern tables' OpenFour class; the win
// was unstoppable one move ahead), "double 4" two or more distinct
// win-in-1 cells with no open-four window (two separate simple fours, also
// unstoppable in one move), "4" exactly one win-in-1 cell, a blockable
// four the opponent failed or was forced not to block. Three-based tags
// (double 3, cross 3-4) need distinct-threat identity: every four-in-a-row
// contains three-class sub-windows, so mere window presence cannot count
// threes, and the spec defers those formal definitions to the separate
// analytics service. Draws and forfeit sweeps carry no tag at all.
const (
	WonByOpenFour   = "open 4"
	WonByDoubleFour = "double 4"
	WonByFour       = "4"
)

// Ready marks one participant's handshake; the second ready makes game 1
// live with the host on red per the spec.
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

// startGameLocked resets every per-game surface: fresh board, fresh clocks
// under the series time control, emptied move buffer, a fresh engine
// instance per bot seat (the ephemerality rule: nothing carries across
// games), turn clock running on red. The Series state machine owns who red
// is.
func (r *Room) startGameLocked() {
	r.closeEnginesLocked()
	r.board = rules.NewBoard()
	r.moves = r.moves[:0]
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

// PlayMove applies a human move: participant, turn, and rules legality
// (opening distance, region, occupancy) all validate before the stone
// lands. A bot seat never accepts a human move for it. The move publishes
// a board event; a game end routes through the single completion path,
// which retires the room itself when the series closed.
func (r *Room) PlayMove(userID int64, cell rules.Cell) error {
	err := r.playMove(userID, cell)
	r.maybeFinish()
	return err
}

// maybeFinish retires the room after a terminal transition. Over is the
// completion path's verdict, so a mid-series game end leaves the room
// live for the next game.
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

// applyMoveLocked lands a validated stone: board, history buffer, the
// mover's clock charged the wall time from turn start (scheduling latency
// bills to the mover, and the never-negative law clamps at zero), then the
// board event.
func (r *Room) applyMoveLocked(side rules.Color, cell rules.Cell) {
	r.board.Make(cell)
	r.moves = append(r.moves, rules.Move(cell))
	r.clock[side].Commit(time.Since(r.turnStart))
	r.publishLocked(Event{Kind: EventKindMove, Payload: cellName(cell)})
}

// completeGameLocked is the one completion path: rules decided the outcome
// (last mover's five or full board), the Series machine books it, the
// write queue persists the game, the rating pair, and the series finish in
// order, then the room either resets for the next game with the rotation
// the Series machine chose, or retires. Holding the room lock across the
// blocking Send serializes per-room writes: game n+1 cannot persist before
// game n, and the rating reads inside the mutation see every earlier event.
func (r *Room) completeGameLocked(winner rules.Color, lastCell rules.Cell) error {
	redUser, blueUser := r.seatByColorLocked(rules.Red).userID, r.seatByColorLocked(rules.Blue).userID
	outcome := Draw
	var wonBy *string
	if winner != rules.Empty {
		outcome = RedWins
		if winner == rules.Blue {
			outcome = BlueWins
		}
		// The tag reads the position at move n-1: lift the winning stone,
		// classify, restore.
		r.board.Unmake()
		tag := wonByTag(r.board, winner)
		r.board.Make(lastCell)
		wonBy = &tag
	}
	if err := r.series.RecordResult(outcome); err != nil {
		return err
	}
	if err := r.persistGameLocked(redUser, blueUser, outcome, wonBy); err != nil {
		return err
	}
	r.publishLocked(Event{Kind: EventKindGameEnd, Payload: outcome.String()})
	if r.series.State() == SeriesFinished {
		r.over = true
		r.publishLocked(Event{Kind: EventKindSeries, Payload: r.series.Winner().String()})
	} else {
		r.startGameLocked()
	}
	return nil
}

// persistGameLocked writes one finished game and everything it implies.
// Bot rooms (seriesID 0) persist nothing: Scenario 2 keeps bot records in
// the tournament's separate space.
func (r *Room) persistGameLocked(redUser, blueUser int64, outcome Outcome, wonBy *string) error {
	if r.seriesID == 0 {
		return nil
	}
	seriesID, idx := r.seriesID, r.series.GamesPlayed()-1
	blob, fullTurns := encodeMoves(nil, r.moves), len(r.moves)/2
	finished := r.series.State() == SeriesFinished
	winner, finAt := r.seriesWinnerUser(), time.Now().Unix()
	return r.wq.Send(func(_ context.Context) error {
		g, err := r.store.AppendGame(Game{
			SeriesID: seriesID, IdxInSeries: idx, RedUser: redUser, BlueUser: blueUser,
			Outcome: outcome.String(), Moves: blob, FullTurns: fullTurns, WonBy: wonBy,
		})
		if err != nil {
			return err
		}
		if outcome != Draw {
			if err := r.applyRatingDeltas(g.ID, redUser, blueUser, outcome); err != nil {
				return err
			}
		}
		if finished {
			return r.store.UpdateSeries(seriesID, SeriesStateFinished, winner, &finAt)
		}
		return nil
	})
}

// Forfeit bills a mid-series quit: Series.Forfeit books every remaining
// game, the live one included, as a quitter loss, the sweep persists with
// its rating events in order, and the room retires. Quitting an open room
// (opponent never seated, no series formed) just retires it: nothing is
// billed because nothing existed.
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

// persistForfeitLocked writes the synthetic games in game order, each with
// its rating pair, then the finished series row. The first synthetic game
// is the live one: its blob keeps the partial move history actually played.
func (r *Room) persistForfeitLocked(liveMoves []rules.Move) error {
	if r.seriesID == 0 {
		return nil
	}
	seriesID := r.seriesID
	synth := r.series.SyntheticGames()
	firstLive := r.series.GamesPlayed() - len(synth) + 1
	winner, finAt := r.seriesWinnerUser(), time.Now().Unix()
	return r.wq.Send(func(_ context.Context) error {
		for _, g := range synth {
			blob, turns := []byte{}, 0
			if g.GameNo == firstLive {
				blob, turns = encodeMoves(nil, liveMoves), len(liveMoves)/2
			}
			row, err := r.store.AppendGame(Game{
				SeriesID: seriesID, IdxInSeries: g.GameNo - 1,
				RedUser: g.RedUserID, BlueUser: g.BlueUserID,
				Outcome: g.Outcome.String(), Moves: blob, FullTurns: turns,
			})
			if err != nil {
				return err
			}
			if err := r.applyRatingDeltas(row.ID, g.RedUserID, g.BlueUserID, g.Outcome); err != nil {
				return err
			}
		}
		return r.store.UpdateSeries(seriesID, SeriesStateFinished, winner, &finAt)
	})
}

// applyRatingDeltas prices one game on the two players' CURRENT stored
// ratings (the last rating event's RatingAfter, RatingStart before any)
// and appends the zero-sum pair. Runs on the write queue worker, so games
// of the same series see each other's events in order.
func (r *Room) applyRatingDeltas(gameID, redUser, blueUser int64, outcome Outcome) error {
	rRed, err := currentRating(r.store, redUser)
	if err != nil {
		return err
	}
	rBlue, err := currentRating(r.store, blueUser)
	if err != nil {
		return err
	}
	dRed, dBlue, afterRed, afterBlue := RatingDeltas(rRed, rBlue, outcome)
	pair := [2]RatingEvent{
		{GameID: gameID, UserID: redUser, Delta: dRed, RatingAfter: afterRed},
		{GameID: gameID, UserID: blueUser, Delta: dBlue, RatingAfter: afterBlue},
	}
	for _, e := range pair {
		if _, err := r.store.AppendRatingEvent(e); err != nil {
			return err
		}
	}
	return nil
}

func currentRating(st *Store, userID int64) (int, error) {
	hist, err := st.RatingHistoryByUser(userID)
	if err != nil {
		return 0, err
	}
	if len(hist) == 0 {
		return config.RatingStart, nil
	}
	return hist[len(hist)-1].RatingAfter, nil
}

// seriesWinnerUser resolves the machine's side to a stored user id, nil
// for a drawn series. Caller holds the room lock; the series lock nests
// inside it everywhere.
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

// seatOfUserLocked resolves a seat id to its seat.
func (r *Room) seatOfUserLocked(userID int64) *seat {
	if r.host.userID == userID {
		return &r.host
	}
	return &r.guest
}

// seatByColorLocked maps a stone color to the seat holding it this game;
// the Series machine owns the red rotation.
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

// ClockRemaining reports one side's remaining bank of the live game; never
// negative per the clock law, and the floor keeps funding moves at zero.
func (r *Room) ClockRemaining(c rules.Color) time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.clock[c] == nil {
		return 0
	}
	return r.clock[c].Remaining()
}

// cellName renders a validated cell in the rules codec notation through
// this package's zero-alloc mirror of rules.CellName, so the event payload
// and the M-line renderer can never drift apart.
func cellName(cell rules.Cell) string {
	return string(appendCellName(nil, rules.Move(cell)))
}

// The moves blob is one little-endian uint16 per stone in play order, the
// cell index row*BoardStride+col of the rules codec: two bytes per move,
// fixed stride, no length prefix, decodable by the playback board straight
// from the column.
func encodeMoves(dst []byte, moves []rules.Move) []byte {
	for _, m := range moves {
		dst = append(dst, byte(m), byte(m>>8))
	}
	return dst
}

func decodeMoves(blob []byte) ([]rules.Move, error) {
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

// wonByTag classifies how the winner won: open-four window presence from
// the pattern tables, then the count of distinct win-in-1 cells from the
// rules win predicate. Both are structure-identity free, which is what is
// honestly derivable before the formal definitions land.
func wonByTag(pre *rules.Board, winner rules.Color) string {
	if hasOpenFourWindow(pre, winner) {
		return WonByOpenFour
	}
	if countWinIn1(pre, winner) >= 2 {
		return WonByDoubleFour
	}
	return WonByFour
}

// hasOpenFourWindow scans the winner's windows through the pattern tables
// for one with two completion cells.
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

// countWinIn1 counts the empty cells whose placement completes a five for
// the winner right now. The side-to-move field is borrowed for the probes
// and restored; Make and Unmake keep every other board field consistent.
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

// Bot turn concurrency model: one worker goroutine per room that seats a
// bot, started when the room is created and the only writer of bot moves.
// A worker rather than the caller's goroutine because PlayMove must return
// as soon as the human's stone lands (the bot's reply arrives through the
// hub as an event, which is exactly the M6b push model), and because
// Scenario 2's bot-vs-bot matchups reuse this surface and need a driver
// with no human caller. Leak freedom: the worker parks on the wake channel
// and leaves when quit closes at retire; Close joins it through the
// WaitGroup, so no goroutine and no engine instance outlives the room.
// Searches run OUTSIDE the room lock on a private board copy, and the
// apply revalidates under the lock, so a forfeit or shutdown during a
// search discards the stale answer instead of racing it.

// searcher is the per-game bot engine surface the room drives: one Search
// per turn under a budget deadline, closed at game end.
type searcher interface {
	Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats)
	Close()
}

// singleSearcher adapts the single-threaded Engine (the easy tier: one
// core, no table), whose lifetime needs no teardown, to the same surface
// as the tiered SMP instance.
type singleSearcher struct{ e *engine.Engine }

func (s singleSearcher) Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats) {
	return s.e.Search(b, dl)
}

func (singleSearcher) Close() {}

// newBotSearcher sizes a bot from its tier: the SMP pool over a shared
// lockless table for Cores > 1, the plain engine otherwise.
func newBotSearcher(t config.Tier) searcher {
	if t.Cores > 1 {
		return engine.NewTiered(t)
	}
	return singleSearcher{e: engine.New(t.TTBytes)}
}

func (r *Room) closeEnginesLocked() {
	for i := range r.engines {
		if r.engines[i] != nil {
			r.engines[i].Close()
			r.engines[i] = nil
		}
	}
}

// mLineRecord carries the raw inputs of the last emitted bot line so the
// hub payload stays byte-checkable against the canonical renderer.
type mLineRecord struct {
	moveNumber int
	side       rules.Color
	move       rules.Move
	stats      engine.SearchStats
}

// startBotWorker launches the room's bot driver. Called once, before the
// room enters the manager map, so a published room always has its worker.
func (r *Room) startBotWorker() {
	r.wg.Add(1)
	go r.botLoop()
}

func (r *Room) botLoop() {
	defer r.wg.Done()
	for {
		if r.runBotTurn() {
			// A bot-won series ends on this goroutine: drive the same
			// lifecycle finish the human path drives.
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

// wakeBotLocked drops a coalescing token when the side to move is a bot.
// A token lost to a turn already being driven is harmless: the driver
// rechecks the turn after every move.
func (r *Room) wakeBotLocked() {
	if _, ok := r.botTurnLocked(); ok {
		select {
		case r.wake <- struct{}{}:
		default:
		}
	}
}

// botTurnLocked reports the color to move when it belongs to a bot.
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

// runBotTurn drives one bot move. The search budget comes from the side's
// GameClock.Budget() under the clock law, wrapped in a FixedBudget
// deadline; the floor grants a legal move at any drain level, so the bot
// answers even with an empty bank and a timeout can never decide the game.
// Returns whether a move landed, so the loop keeps driving while
// consecutive bot turns are pending.
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
	r.mu.Unlock()

	mv, st := eng.Search(&board, engine.NewFixedBudget(budget))

	r.mu.Lock()
	defer r.mu.Unlock()
	// Discard a stale answer: the room retired or the turn moved on while
	// the search ran.
	if r.over || r.board == nil || r.board.MoveCount != moveCount || r.board.Side != side {
		return false
	}
	if !r.board.IsLegal(rules.Cell(mv)) {
		// The engine contract guarantees legality; this is the safety net.
		mv = r.firstLegalLocked()
	}
	cell := rules.Cell(mv)
	r.applyMoveLocked(side, cell)
	r.lastM = mLineRecord{moveNumber: r.board.MoveCount, side: side, move: mv, stats: st}
	r.publishLocked(Event{Kind: EventKindMLine, Payload: MLine(r.lastM.moveNumber, side, mv, &st, mLineTag)})
	// The worker has no caller to surface a persistence error to; bot
	// rooms persist nothing today, so the completion path cannot fail here.
	if r.board.FastLastMoveWin(side, cell) {
		_ = r.completeGameLocked(side, cell)
		return true
	}
	if r.board.IsFull() {
		_ = r.completeGameLocked(rules.Empty, cell)
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
