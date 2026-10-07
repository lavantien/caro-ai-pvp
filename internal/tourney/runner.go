package tourney

import (
	"context"
	"errors"
	"fmt"
	"slices"
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
	StartSeries(host, guest *config.Tier, hostName, guestName string, tcIdx, boLen int) (SeriesStream, error)
}

// SeriesStream is one live bot-vs-bot series from the conductor's view: the
// ordered event stream ending at the series event, the room-authoritative
// move list behind it, plus retirement.
type SeriesStream interface {
	// Events delivers the room's events in publish order. The channel
	// closes when the subscription ends; a close before the series event is
	// a missed-event gap and fails the run.
	Events() <-chan server.Event
	// TruthMoves returns the room-authoritative move list of the game that
	// just ended: the stones the room itself applied, in play order. The
	// runner reconciles its delivered accumulation against it at every
	// game end, because a lost even-length prefix replays clean.
	TruthMoves() []rules.Move
	// Err explains why Events ended: server.ErrSlowConsumer after an
	// eviction, server.ErrHubClosed after a hub close.
	Err() error
	// Close retires the series room and joins its resources. Idempotent.
	Close()
}

// RoomSource adapts the room manager onto MatchSource. CreateBotVsBot
// returns with game 1 already live, so the subscription registers on the
// very next statement (the SSE handler's subscribe-before-liveness
// discipline). The residual window inside the create is closed by the
// per-game truth reconciliation, not the replay validation: an even-length
// lost prefix (one red and one blue leading move) preserves replay parity,
// so a truncated stream replays clean whenever the dropped stones sit
// outside the winning five; comparing the delivered list against the
// room's own record fails the series before anything persists.
type RoomSource struct{ RM *server.RoomManager }

// StartSeries opens the bot-vs-bot room and subscribes before returning the
// stream, then rechecks liveness exactly like the SSE handler: a room that
// already retired published its terminal event before this subscription
// registered, so waiting would park forever. That is a missed-event gap and
// a hard error, never a silent partial record. A failed subscribe or the
// liveness rejection retires the room it opened.
func (s RoomSource) StartSeries(host, guest *config.Tier, hostName, guestName string, tcIdx, boLen int) (SeriesStream, error) {
	room, err := s.RM.CreateBotVsBot(host, hostName, guest, guestName, tcIdx, boLen)
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

// LiveCoreBookings exposes the room manager's core ledger, the conductor's
// casual-room gate at startRun: the production source rides the manager, so
// it carries the capability, while scripted sources hold no rooms and no
// ledger.
func (s RoomSource) LiveCoreBookings() int { return s.RM.LiveCoreBookings() }

// coreLedgerSource is the optional MatchSource capability the start gate
// reads when the source rides the room manager.
type coreLedgerSource interface {
	LiveCoreBookings() int
}

// roomStream is one live room's subscription wrapper.
type roomStream struct {
	room *server.Room
	sub  *server.Subscription
}

func (s roomStream) Events() <-chan server.Event { return s.sub.Events() }
func (s roomStream) Err() error                  { return s.sub.Err() }

// TruthMoves is the room's own record of the game that just ended.
func (s roomStream) TruthMoves() []rules.Move { return s.room.LastGameMoves() }

// Close drops the subscription first, then retires the room and joins its
// bot worker, so no engine instance outlives the series.
func (s roomStream) Close() {
	s.sub.Unsubscribe()
	s.room.Close()
}

// ErrRunInProgress refuses a second concurrent run: MachineCores budgets the
// machine as a whole, so the first ongoing run holds the start gate until it
// finishes or its stalled row is closed by hand.
var ErrRunInProgress = errors.New("tourney: another run is in progress")

// RunInProgressError names the ongoing run holding the gate; it matches
// ErrRunInProgress under errors.Is while carrying the blocking run id for the
// surfaces that render it.
type RunInProgressError struct{ RunID int64 }

func (e *RunInProgressError) Error() string {
	return fmt.Sprintf("tourney: run %d is still ongoing, one run holds the machine at a time", e.RunID)
}

func (e *RunInProgressError) Unwrap() error { return ErrRunInProgress }

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

// Resume continues the machine's one ongoing run from its own persisted
// state: the settled series stand as evidence and are never replayed, every
// interrupted series is scrubbed to its created state and replayed, and the
// run finishes with its standings snapshot and summary in the folder the
// run's own label names. Every termination shape lands here the same way:
// a cancelled drive (SIGINT), a crashed process, or a machine cut. A
// finished or unknown run refuses, and so does a resume while another run's
// row holds the machine-wide gate.
func (c *Conductor) Resume(ctx context.Context, store *Store, runID int64, parallel int) (RunResult, error) {
	run, err := store.Run(ctx, runID)
	if err != nil {
		return RunResult{}, err
	}
	if run.Status != RunStateOngoing {
		return RunResult{}, fmt.Errorf("tourney: run %d is already %s", runID, run.Status)
	}
	if run.Label == config.TournamentLegacyLabel {
		return RunResult{}, fmt.Errorf("tourney: run %d predates persisted labels, its log folder is unknowable: close it and start a fresh run", runID)
	}
	if id, held, err := store.OngoingRunID(ctx); err != nil {
		return RunResult{}, err
	} else if held && id != runID {
		return RunResult{}, &RunInProgressError{RunID: id}
	}
	roster, err := store.Roster(ctx, runID)
	if err != nil {
		return RunResult{}, err
	}
	tiers, err := tiersOfRoster(roster)
	if err != nil {
		return RunResult{}, err
	}
	if err := checkCoreBudget(parallel, tiers); err != nil {
		return RunResult{}, err
	}
	return c.drive(ctx, store, run, roster, tiers, parallel)
}

// Run executes one tournament. The roster's tiers resolve through the config
// table and the worst-case live-search demand (parallel rooms, one search
// at a time each, at the roster's largest tier core count) must fit
// config.MachineCores; both refuse before anything persists. The label names
// the run's own log folder under config.TournamentLogRoot. The schedule
// lands in CreateRun, every pairing runs under the semaphore, and the run
// finishes with the standings snapshot and the summary file.
func (c *Conductor) Run(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int, label string) (RunResult, error) {

	run, tiers, err := c.startRun(ctx, store, roster, tcIdx, boLen, startRating, parallel, label)
	if err != nil {
		return RunResult{}, err
	}
	return c.drive(ctx, store, run, roster, tiers, parallel)
}

// startRun is the synchronous half of a run: resolve the roster tiers, refuse
// a core budget the machine cannot book, refuse while any run row is still
// ongoing (the machine-wide run gate), refuse while live rooms hold the
// machine's core ledger (any booking at startRun time is casual, tournament
// rooms cannot predate their run), and persist the run. It returns
// once the run row exists, before any series starts, so the UI manager can
// hand the id out for a redirect. Split from Run as the M7 service seam; the
// conductor's behavior is unchanged.
func (c *Conductor) startRun(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int, label string) (Run, []*config.Tier, error) {

	tiers, err := tiersOfRoster(roster)
	if err != nil {
		return Run{}, nil, err
	}
	if err := checkCoreBudget(parallel, tiers); err != nil {
		return Run{}, nil, err
	}
	// The casual-room gate: MachineCores budgets the machine as a whole, so
	// open bot rooms outside the run hold cores the run's own rooms would
	// oversubscribe. Read only when the source rides the room manager; a
	// source without the capability holds no rooms.
	if src, ok := c.source.(coreLedgerSource); ok {
		if held := src.LiveCoreBookings(); held > 0 {
			return Run{}, nil, fmt.Errorf("tourney: %d live-search cores are held by open rooms, close them before starting a run", held)
		}
	}
	// The machine-wide run gate: checkCoreBudget books against the whole
	// machine per run, so two concurrent runs would silently double the
	// booking. The gate reads the store immediately before the persist;
	// two PROCESSES starting at the same instant can still both pass it (the
	// status CHECK admits any number of ongoing rows and no schema change
	// may fence that here), while two starts inside one process serialize
	// on the manager's mutex and the second reads the first's row.
	if id, held, err := store.OngoingRunID(ctx); err != nil {
		return Run{}, nil, err
	} else if held {
		return Run{}, nil, &RunInProgressError{RunID: id}
	}
	run, err := store.CreateRun(ctx, tcIdx, boLen, startRating, roster, label)
	if err != nil {
		return Run{}, nil, err
	}
	return run, tiers, nil
}

// drive executes one already-persisted run: read back the schedule and
// cross-check it against the pairing plan, claim the run's drive lease,
// run every pairing under the semaphore, then close out leaderboard-first
// and summary-first so every crash window in the close is idempotent. The
// run's own persisted label names its log folder. Settled series (an
// earlier drive's record, the resume shape) reconstruct their line from
// the row and never
// replay; every unsettled row is scrubbed to its created state first, so an
// interrupted series replays from idx 0 while a fresh run's bare rows make
// the scrub a no-op. The first series error aborts the run: pending pairings
// stop at the semaphore, live streams retire through their Close, the run
// row stays ongoing for the post-mortem, and the error surfaces. Cancelling
// ctx is the same abort with the context's error.
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

	logs := NewLogs(RunDirName(run.Label, time.Unix(run.CreatedAt, 0)))
	// The drive lease, claimed before any mutation: one live drive per run
	// across processes. A fresh claim refuses every other drive and a dead
	// process's claim has gone stale, so a takeover (this resume) proceeds;
	// the scrub below and every append ride the claim from here on.
	if err := store.ClaimDrive(ctx, run.ID, time.Now().Unix()); err != nil {
		return RunResult{}, err
	}
	finished := false
	defer func() {
		if !finished {
			// The drive ends without finishing the run (an abort or a
			// close-out fault): drop the claim so the next resume does not
			// wait out the stale window. The release rides a background
			// context because the abort shapes cancel the caller's, and a
			// failed release only costs the stale window, never correctness.
			_ = store.ReleaseDrive(context.Background(), run.ID)
		}
	}()
	res := RunResult{Run: run, Series: make([]SeriesResult, len(pairs))}
	for slot := range pairs {
		line := SeriesResult{
			PairingSlot: slot, RedFirst: pairs[slot].RedFirst, BlueFirst: pairs[slot].BlueFirst,
		}
		if schedule[slot].FinishedAt != nil {
			line.WinnerSlot = schedule[slot].WinnerSlot
			line.RedFirstWins = schedule[slot].RedFirstWins
			line.BlueFirstWins = schedule[slot].BlueFirstWins
		}
		res.Series[slot] = line
	}
	// The interrupted drive's in-flight series hold partial games: scrub
	// every unsettled row to its created state so each replay starts from
	// idx 0 (a fresh run's rows are already bare, the scrub is their no-op).
	for slot := range pairs {
		if schedule[slot].FinishedAt == nil {
			if err := store.ResetSeries(ctx, schedule[slot].ID); err != nil {
				return RunResult{}, err
			}
		}
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	// The dispatch loop takes the semaphore slot itself before spawning, so
	// pairings start in schedule order whenever slots free in order (under
	// parallel 1 the run is fully serial and deterministic) while parallel
	// slots keep overlapping freely. Settled series skip the loop whole:
	// their record is evidence, never a replay.
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	var once sync.Once
	var failure error
	// The lease heartbeat: re-stamp the claim every beat so resumes and
	// closes keep refusing it. A beat error is a lost lease and fails the
	// run through the same once/cancel path a series failure rides; a beat
	// that finds the claim gone (the run finishing underneath) just stops.
	// The beater joins through beaterDone before failure is read, so its
	// write is ordered against the switch below like any series goroutine's.
	beat := time.NewTicker(time.Duration(config.TournamentDriveBeatSec) * time.Second)
	defer beat.Stop()
	beaterDone := make(chan struct{})
	go func() {
		defer close(beaterDone)
		for {
			select {
			case <-beat.C:
				alive, berr := store.BeatDrive(context.Background(), run.ID, time.Now().Unix())
				if berr != nil {
					once.Do(func() {
						failure = fmt.Errorf("tourney: run %d lost its drive lease: %w", run.ID, berr)
						cancel()
					})
					return
				}
				if !alive {
					return
				}
			case <-runCtx.Done():
				return
			}
		}
	}()
dispatch:
	for slot := range pairs {
		if schedule[slot].FinishedAt != nil {
			continue
		}
		select {
		case sem <- struct{}{}:
			// A cancelled run must not dispatch: the slot freeing after
			// an abort leaves both select arms ready and Go picks at
			// random, so the cancel is rechecked after the acquire.
			if runCtx.Err() != nil {
				<-sem
				break dispatch
			}
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
	// Join the beater before reading failure: cancel after the series drain
	// ends its select, and its own lost-lease exit is a plain return. The
	// deferred cancel stays as the idempotent safety net.
	cancel()
	<-beaterDone
	closeErr := logs.Close()
	switch {
	case failure != nil:
		return res, errors.Join(failure, closeErr)
	case ctx.Err() != nil:
		return res, errors.Join(fmt.Errorf("tourney: run %d aborted: %w", run.ID, ctx.Err()), closeErr)
	case closeErr != nil:
		return res, closeErr
	}
	res.Board, err = store.Leaderboard(ctx, run.ID)
	if err != nil {
		return res, err
	}
	// The summary lands before the row flips: a crash anywhere in the close
	// leaves an ongoing row whose every series is settled, and the resume's
	// zero-dispatch path rewrites the same summary and finishes, so no crash
	// window can strand a finished run without its summary.
	if serr := logs.WriteRunSummary(run, roster, res.Board); serr != nil {
		return res, serr
	}
	if err := store.FinishRun(ctx, run.ID, time.Now().Unix()); err != nil {
		return res, fmt.Errorf("tourney: finish run %d: %w", run.ID, err)
	}
	finished = true
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
// delivered move list that diverges from the room's own record, a move
// list that does not replay onto the verdict, a stream that ends before
// the series event, a verdict that disagrees with the billed line) fails
// the series and with it the run.
func (c *Conductor) series(ctx context.Context, store *Store, logs *Logs, run Run,
	row Series, pair Pairing, tiers []*config.Tier) (line SeriesResult, err error) {

	line = SeriesResult{PairingSlot: row.PairingSlot, RedFirst: pair.RedFirst, BlueFirst: pair.BlueFirst}
	if err = logs.WriteSeriesHeader(run.ID, row.ID, run.TCIdx, run.BOLen, pair.RedFirst, pair.BlueFirst); err != nil {
		return line, err
	}
	defer func() {
		err = errors.Join(err, logs.CloseSeries(run.ID, row.ID))
	}()

	stream, serr := c.source.StartSeries(tiers[pair.RedFirst.Slot], tiers[pair.BlueFirst.Slot],
		pair.RedFirst.Name, pair.BlueFirst.Name, run.TCIdx, run.BOLen)
	if serr != nil {
		return line, serr
	}
	defer stream.Close()

	// Per-series state: the current game's moves, the game index, and the
	// red seat fold. Game 1 seats the host (the red-first participant) on
	// red per the room contract; after that the benchmark alternation folds
	// red to the other participant after every game, mirroring the room's
	// own rotation, and the verdict cross-check below fails the run if the
	// two ever disagree.
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
			// The truth reconciliation: the room's own list is the record's
			// authority, compared before anything persists. The replay net
			// below is the second check, not the first: an even-length lost
			// prefix preserves parity and replays clean whenever the dropped
			// stones sit outside the winning five, so a delivery gap of that
			// shape would otherwise persist a silently corrupted game.
			truth := stream.TruthMoves()
			if len(truth) != len(moves) || !slices.Equal(truth, moves) {
				return line, fmt.Errorf("game %d delivered %d moves against the room's %d, the stream dropped or corrupted stones",
					games+1, len(moves), len(truth))
			}
			wonBy, rerr := replayGame(truth, outcome)
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
				WonBy: wonBy, FullTurns: len(truth) / 2, Moves: truth,
			}); aerr != nil {
				return line, fmt.Errorf("persist game %d: %w", games+1, aerr)
			}
			// The physical trace: one summary line per finished game beside
			// the M-lines, so a series log reads as evidence without the db.
			redName, blueName := pair.RedFirst.Name, pair.BlueFirst.Name
			if !redIsRedFirst {
				redName, blueName = blueName, redName
			}
			if lerr := logs.WriteSeriesLine(run.ID, row.ID,
				gameSummary(games+1, redName, blueName, outcome.String(), len(truth), wonBy)); lerr != nil {
				return line, lerr
			}
			switch outcome {
			case server.RedWins:
				if redIsRedFirst {
					line.RedFirstWins++
				} else {
					line.BlueFirstWins++
				}
			case server.BlueWins:
				if redIsRedFirst {
					line.BlueFirstWins++
				} else {
					line.RedFirstWins++
				}
			}
			redIsRedFirst = !redIsRedFirst
			moves = nil
			games++
		case server.EventKindSeries:
			return c.closeSeries(logs, run, row, line, games, ev.Payload)
		default:
			return line, fmt.Errorf("event kind %q is not part of the room contract", ev.Kind)
		}
	}
}

// gameSummary renders one finished game's trace line, names never bare
// colors: with the benchmark alternation a color says nothing about who
// played it. The outcome rides its persisted string form. Single-sourced so
// the conductor's writer and the tests' asserts cannot drift apart.
func gameSummary(gameNo int, redName, blueName, outcome string, stones int, wonBy *string) string {
	var line string
	switch outcome {
	case server.RedWins.String():
		line = fmt.Sprintf("game %d: %s (red) beat %s (blue), %d moves", gameNo, redName, blueName, stones)
	case server.BlueWins.String():
		line = fmt.Sprintf("game %d: %s (blue) beat %s (red), %d moves", gameNo, blueName, redName, stones)
	default:
		line = fmt.Sprintf("game %d: %s (red) vs %s (blue), drawn at %d moves", gameNo, redName, blueName, stones)
	}
	if wonBy != nil {
		line += ", won by " + *wonBy
	}
	return line
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
	// The verdict line names the winner: host and guest are room concepts,
	// and with red alternating every game they tell the reader nothing.
	verdictName := "drawn"
	if winner != nil {
		verdictName = line.BlueFirst.Name
		if *winner == line.RedFirst.Slot {
			verdictName = line.RedFirst.Name
		}
	}
	if lerr := logs.WriteSeriesLine(run.ID, row.ID,
		fmt.Sprintf("series %s %d-%d", verdictName, line.RedFirstWins, line.BlueFirstWins)); lerr != nil {
		return line, lerr
	}
	line.WinnerSlot = winner
	return line, nil
}

// replayGame validates one finished game's move list and derives the won-by
// tag. The replay is the conductor's second net, behind the truth
// reconciliation: it replays the stones onto a fresh board and demands the
// terminal predicate (the winner's exact five on the final stone, or a full
// board for a draw), catching whatever corruption keeps list lengths and
// elements aligned. The tag reuses server.WonByTag on the position at move
// n-1, the same classification the room's completion path writes for PvP
// games.
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
