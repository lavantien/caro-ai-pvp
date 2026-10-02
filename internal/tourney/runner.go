package tourney

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// MatchSource is the conductor's one seam onto the match surface: start one
// bot-vs-bot series and hand back its event stream. Production wires
// RoomSource over the room manager; tests wire scripted sources.
type MatchSource interface {
	StartSeries(host, guest *config.Tier, tcIdx, boLen int) (SeriesStream, error)
}

// SeriesStream is one live bot-vs-bot series from the conductor's view: the
// ordered event stream ending at the series event, plus retirement.
type SeriesStream interface {
	// Events delivers the room's events in publish order. The channel
	// closes when the subscription ends; a close before the series event is
	// a missed-event gap and fails the run.
	Events() <-chan server.Event
	// Err explains why Events ended: server.ErrSlowConsumer after an
	// eviction, server.ErrHubClosed after a hub close.
	Err() error
	// Close retires the series room and joins its resources. Idempotent.
	Close()
}

// RoomSource adapts the room manager onto MatchSource. CreateBotVsBot
// returns with game 1 already live, so the subscription registers on the
// very next statement (the SSE handler's subscribe-before-liveness
// discipline); the residual window inside the create is closed by the
// per-game replay validation, which fails any series whose delivered moves
// do not replay onto the room's own terminal position.
type RoomSource struct{ RM *server.RoomManager }

// StartSeries opens the bot-vs-bot room and subscribes before returning the
// stream, then rechecks liveness exactly like the SSE handler: a room that
// already retired published its terminal event before this subscription
// registered, so waiting would park forever. That is a missed-event gap and
// a hard error, never a silent partial record. A failed subscribe or the
// liveness rejection retires the room it opened.
func (s RoomSource) StartSeries(host, guest *config.Tier, tcIdx, boLen int) (SeriesStream, error) {
	room, err := s.RM.CreateBotVsBot(host, guest, tcIdx, boLen)
	if err != nil {
		return nil, err
	}
	sub, err := room.Subscribe()
	if err != nil {
		room.Close()
		return nil, err
	}
	if _, live := room.Info(); !live {
		sub.Unsubscribe()
		room.Close()
		return nil, errors.New("tourney: the room retired before the subscription registered, the series was missed")
	}
	return roomStream{room: room, sub: sub}, nil
}

// roomStream is one live room's subscription wrapper.
type roomStream struct {
	room *server.Room
	sub  *server.Subscription
}

func (s roomStream) Events() <-chan server.Event { return s.sub.Events() }
func (s roomStream) Err() error                  { return s.sub.Err() }

// Close drops the subscription first, then retires the room and joins its
// bot worker, so no engine instance outlives the series.
func (s roomStream) Close() {
	s.sub.Unsubscribe()
	s.room.Close()
}

// SeriesResult is one pairing's completed line of the RunResult.
type SeriesResult struct {
	PairingSlot   int
	RedFirst      Participant
	BlueFirst     Participant
	WinnerSlot    *int
	RedFirstWins  int
	BlueFirstWins int
}

// RunResult is the conductor's return: the persisted run header, one line
// per pairing in schedule order, and the final leaderboard snapshot.
type RunResult struct {
	Run    Run
	Series []SeriesResult
	Board  []Standings
}

// Conductor executes whole tournament runs over a MatchSource: it persists
// the schedule, drives every pairing as one bot-vs-bot bo series on the
// room surface, records the games and the per-series txt logs, and closes
// the run with the standings snapshot.
type Conductor struct {
	source MatchSource
}

// NewConductor wires a conductor onto its match source.
func NewConductor(source MatchSource) *Conductor {
	return &Conductor{source: source}
}

// Run executes one tournament. The roster's tiers resolve through the config
// table and the worst-case live-search demand (parallel rooms, one search
// at a time each, at the roster's largest tier core count) must fit
// config.MachineCores; both refuse before anything persists. The schedule
// lands in CreateRun, every pairing runs under the semaphore, and the run
// finishes with the standings snapshot.
func (c *Conductor) Run(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int) (RunResult, error) {

	run, tiers, err := c.startRun(ctx, store, roster, tcIdx, boLen, startRating, parallel)
	if err != nil {
		return RunResult{}, err
	}
	return c.drive(ctx, store, run, roster, tiers, parallel)
}

// startRun is the synchronous half of a run: resolve the roster tiers, refuse
// a core budget the machine cannot book, and persist the run. It returns
// once the run row exists, before any series starts, so the UI manager can
// hand the id out for a redirect. Split from Run as the M7 service seam; the
// conductor's behavior is unchanged.
func (c *Conductor) startRun(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int) (Run, []*config.Tier, error) {

	tiers, err := tiersOfRoster(roster)
	if err != nil {
		return Run{}, nil, err
	}
	if err := checkCoreBudget(parallel, tiers); err != nil {
		return Run{}, nil, err
	}
	run, err := store.CreateRun(ctx, tcIdx, boLen, startRating, roster)
	if err != nil {
		return Run{}, nil, err
	}
	return run, tiers, nil
}

// drive executes one already-persisted run: read back the schedule and
// cross-check it against the pairing plan, run every pairing under the
// semaphore, close the run, and read the final leaderboard. The first series
// error aborts the run: pending pairings stop at the semaphore, live streams
// retire through their Close, the run row stays ongoing for the post-mortem,
// and the error surfaces. Cancelling ctx is the same abort with the
// context's error.
func (c *Conductor) drive(ctx context.Context, store *Store, run Run, roster []Participant,
	tiers []*config.Tier, parallel int) (RunResult, error) {

	schedule, err := store.Schedule(ctx, run.ID)
	if err != nil {
		return RunResult{}, err
	}
	pairs := Pairings(roster)
	if len(schedule) != len(pairs) {
		return RunResult{}, fmt.Errorf("tourney: run %d holds %d series rows for %d pairings",
			run.ID, len(schedule), len(pairs))
	}
	for slot := range pairs {
		if schedule[slot].RedFirstSlot != pairs[slot].RedFirst.Slot ||
			schedule[slot].BlueFirstSlot != pairs[slot].BlueFirst.Slot {
			return RunResult{}, fmt.Errorf("tourney: run %d series %d seats (%d, %d) disagree with pairing %d (%d, %d)",
				run.ID, slot, schedule[slot].RedFirstSlot, schedule[slot].BlueFirstSlot,
				slot, pairs[slot].RedFirst.Slot, pairs[slot].BlueFirst.Slot)
		}
	}

	logs := NewLogs()
	res := RunResult{Run: run, Series: make([]SeriesResult, len(pairs))}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The dispatch loop takes the semaphore slot itself before spawning, so
	// pairings start in schedule order whenever slots free in order (under
	// parallel 1 the run is fully serial and deterministic) while parallel
	// slots keep overlapping freely.
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	var once sync.Once
	var failure error
	for slot := range pairs {
		res.Series[slot] = SeriesResult{
			PairingSlot: slot, RedFirst: pairs[slot].RedFirst, BlueFirst: pairs[slot].BlueFirst,
		}
	}
dispatch:
	for slot := range pairs {
		select {
		case sem <- struct{}{}:
		case <-runCtx.Done():
			break dispatch
		}
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			defer func() { <-sem }()
			line, serr := c.series(runCtx, store, logs, run, schedule[slot], pairs[slot], tiers)
			if serr != nil {
				once.Do(func() {
					failure = fmt.Errorf("tourney: pairing %d (%s vs %s): %w",
						slot, pairs[slot].RedFirst.Name, pairs[slot].BlueFirst.Name, serr)
					cancel()
				})
				return
			}
			res.Series[slot] = line
		}(slot)
	}
	wg.Wait()
	closeErr := logs.Close()
	switch {
	case failure != nil:
		return res, errors.Join(failure, closeErr)
	case ctx.Err() != nil:
		return res, errors.Join(fmt.Errorf("tourney: run %d aborted: %w", run.ID, ctx.Err()), closeErr)
	case closeErr != nil:
		return res, closeErr
	}
	if err := store.FinishRun(ctx, run.ID, time.Now().Unix()); err != nil {
		return res, fmt.Errorf("tourney: finish run %d: %w", run.ID, err)
	}
	res.Board, err = store.Leaderboard(ctx, run.ID)
	if err != nil {
		return res, err
	}
	return res, nil
}

// tiersOfRoster resolves every roster tier name onto the config tier table,
// refusing unknown names before anything persists.
func tiersOfRoster(roster []Participant) ([]*config.Tier, error) {
	out := make([]*config.Tier, len(roster))
	for i, p := range roster {
		found := false
		for j := range config.Tiers {
			if config.Tiers[j].Name == p.Tier {
				out[i] = &config.Tiers[j]
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("tourney: roster slot %d (%s): unknown tier %q", i, p.Name, p.Tier)
		}
	}
	return out, nil
}

// checkCoreBudget refuses runs whose worst-case live-search demand exceeds
// the machine budget: a bot-vs-bot room runs one search at a time (bot turns
// alternate inside the room's single worker), so parallel rooms each book
// the largest tier core count the roster can seat.
func checkCoreBudget(parallel int, tiers []*config.Tier) error {
	if parallel < 1 {
		return fmt.Errorf("tourney: parallel %d, want at least 1", parallel)
	}
	maxCores := 0
	for _, t := range tiers {
		if t.Cores > maxCores {
			maxCores = t.Cores
		}
	}
	if need := parallel * maxCores; need > config.MachineCores {
		return fmt.Errorf("tourney: %d parallel rooms at %d live-search cores each need %d cores, machine budget is %d",
			parallel, maxCores, need, config.MachineCores)
	}
	return nil
}

// series drives one pairing end to end: open the series log, run the bo on
// the match surface with the red-first participant as host, persist every
// game as it ends, and close the series record against the room's own
// verdict. Every deviation from the room contract (an unparsable event, a
// move list that does not replay onto the verdict, a stream that ends
// before the series event, a verdict that disagrees with the billed line)
// fails the series and with it the run.
func (c *Conductor) series(ctx context.Context, store *Store, logs *Logs, run Run,
	row Series, pair Pairing, tiers []*config.Tier) (line SeriesResult, err error) {

	line = SeriesResult{PairingSlot: row.PairingSlot, RedFirst: pair.RedFirst, BlueFirst: pair.BlueFirst}
	if err = logs.WriteSeriesHeader(run.ID, row.ID, run.TCIdx, run.BOLen, pair.RedFirst, pair.BlueFirst); err != nil {
		return line, err
	}
	defer func() {
		err = errors.Join(err, logs.CloseSeries(run.ID, row.ID))
	}()

	stream, serr := c.source.StartSeries(tiers[pair.RedFirst.Slot], tiers[pair.BlueFirst.Slot], run.TCIdx, run.BOLen)
	if serr != nil {
		return line, serr
	}
	defer stream.Close()

	// Per-series state: the current game's moves, the game index, and the
	// red seat fold. Game 1 seats the host (the red-first participant) on
	// red per the room contract; after that the loser-takes-red law folds
	// from the outcomes, mirroring the room's own rotation, and the verdict
	// cross-check below fails the run if the two ever disagree.
	var moves []rules.Move
	redIsRedFirst := true
	games := 0
	for {
		var ev server.Event
		var open bool
		select {
		case <-ctx.Done():
			return line, fmt.Errorf("series aborted: %w", ctx.Err())
		case ev, open = <-stream.Events():
		}
		if !open {
			if eerr := stream.Err(); eerr != nil {
				return line, fmt.Errorf("event stream ended before the series event: %w", eerr)
			}
			return line, errors.New("event stream ended before the series event")
		}
		switch ev.Kind {
		case server.EventKindMove:
			cell, perr := rules.ParseCell(ev.Payload)
			if perr != nil {
				return line, fmt.Errorf("game %d move %q: %w", games+1, ev.Payload, perr)
			}
			moves = append(moves, rules.Move(cell))
		case server.EventKindMLine:
			if lerr := logs.WriteSeriesLine(run.ID, row.ID, ev.Payload); lerr != nil {
				return line, lerr
			}
		case server.EventKindGameEnd:
			outcome, ok := outcomeOf(ev.Payload)
			if !ok {
				return line, fmt.Errorf("game %d end %q: not a room outcome", games+1, ev.Payload)
			}
			wonBy, rerr := replayGame(moves, outcome)
			if rerr != nil {
				return line, fmt.Errorf("game %d: %w", games+1, rerr)
			}
			redSlot, blueSlot := pair.RedFirst.Slot, pair.BlueFirst.Slot
			if !redIsRedFirst {
				redSlot, blueSlot = blueSlot, redSlot
			}
			if _, aerr := store.AppendGame(ctx, Game{
				RunID: run.ID, SeriesID: row.ID, IdxInSeries: games,
				RedSlot: redSlot, BlueSlot: blueSlot, Outcome: outcome,
				WonBy: wonBy, FullTurns: len(moves) / 2, Moves: moves,
			}); aerr != nil {
				return line, fmt.Errorf("persist game %d: %w", games+1, aerr)
			}
			switch outcome {
			case server.RedWins:
				if redIsRedFirst {
					line.RedFirstWins++
				} else {
					line.BlueFirstWins++
				}
				redIsRedFirst = !redIsRedFirst // the winner was red, red passes
			case server.BlueWins:
				if redIsRedFirst {
					line.BlueFirstWins++
				} else {
					line.RedFirstWins++
				}
			}
			moves = nil
			games++
		case server.EventKindSeries:
			return c.closeSeries(logs, run, row, line, games, ev.Payload)
		default:
			return line, fmt.Errorf("event kind %q is not part of the room contract", ev.Kind)
		}
	}
}

// closeSeries settles the series record: the verdict line lands in the log,
// and the room's verdict must match the bo law folded over the billed
// games, the same settleSeries the store settles the row with. A mismatch
// means seat bookkeeping or the event stream corrupted, so it fails the
// run instead of persisting a lying record.
func (c *Conductor) closeSeries(logs *Logs, run Run, row Series, line SeriesResult,
	games int, verdict string) (SeriesResult, error) {

	winner, finished := settleSeries(run.BOLen, line.RedFirst.Slot, line.BlueFirst.Slot,
		line.RedFirstWins, line.BlueFirstWins, games)
	if !finished {
		return line, fmt.Errorf("room closed the series at %d-%d over %d games, before the bo%d law settles",
			line.RedFirstWins, line.BlueFirstWins, games, run.BOLen)
	}
	want := server.SideNone.String()
	if winner != nil {
		want = server.SideGuest.String()
		if *winner == line.RedFirst.Slot {
			want = server.SideHost.String()
		}
	}
	if verdict != want {
		return line, fmt.Errorf("room verdict %q disagrees with the billed line %d-%d (%s)",
			verdict, line.RedFirstWins, line.BlueFirstWins, want)
	}
	if lerr := logs.WriteSeriesLine(run.ID, row.ID,
		fmt.Sprintf("series %s %d-%d", verdict, line.RedFirstWins, line.BlueFirstWins)); lerr != nil {
		return line, lerr
	}
	line.WinnerSlot = winner
	return line, nil
}

// replayGame validates one finished game's delivered moves and derives the
// won-by tag. The replay is the conductor's missed-event net: a prefix lost
// to a subscription gap replays onto the wrong board, and the terminal
// predicate (the winner's exact five on the final stone, or a full board
// for a draw) then fails the series instead of persisting corrupt state.
// The tag reuses server.WonByTag on the position at move n-1, the same
// classification the room's completion path writes for PvP games.
func replayGame(moves []rules.Move, outcome server.Outcome) (*string, error) {
	if len(moves) == 0 {
		return nil, fmt.Errorf("%s verdict on an empty board", outcome)
	}
	b := rules.NewBoard()
	for _, mv := range moves {
		if !b.IsLegal(rules.Cell(mv)) {
			return nil, fmt.Errorf("move %d is illegal on the replayed board", b.MoveCount+1)
		}
		b.Make(rules.Cell(mv))
	}
	last := rules.Cell(moves[len(moves)-1])
	if outcome == server.Draw {
		if !b.IsFull() {
			return nil, fmt.Errorf("draw verdict on a board with %d of %d cells", b.MoveCount, config.BoardCells)
		}
		if b.FastLastMoveWin(rules.Red, last) || b.FastLastMoveWin(rules.Blue, last) {
			return nil, errors.New("draw verdict but the final stone completes a five")
		}
		return nil, nil
	}
	winner := rules.Blue
	if outcome == server.RedWins {
		winner = rules.Red
	}
	if !b.FastLastMoveWin(winner, last) {
		return nil, fmt.Errorf("%s verdict but the final stone completes no five", outcome)
	}
	b.Unmake()
	tag := server.WonByTag(b, winner)
	return &tag, nil
}
