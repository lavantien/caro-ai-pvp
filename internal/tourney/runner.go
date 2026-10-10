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

type MatchSource interface {
	StartSeries(host, guest *config.Tier, hostName, guestName string, tcIdx, boLen int) (SeriesStream, error)
}
type SeriesStream interface {
	Events() <-chan server.Event
	TruthMoves() []rules.Move
	Err() error
	Close()
}
type RoomSource struct{ RM *server.RoomManager }

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
func (s RoomSource) LiveCoreBookings() int { return s.RM.LiveCoreBookings() }

type coreLedgerSource interface {
	LiveCoreBookings() int
}
type roomStream struct {
	room *server.Room
	sub  *server.Subscription
}

func (s roomStream) Events() <-chan server.Event { return s.sub.Events() }
func (s roomStream) Err() error                  { return s.sub.Err() }
func (s roomStream) TruthMoves() []rules.Move    { return s.room.LastGameMoves() }
func (s roomStream) Close() {
	s.sub.Unsubscribe()
	s.room.Close()
}

var ErrRunInProgress = errors.New("tourney: another run is in progress")

type RunInProgressError struct{ RunID int64 }

func (e *RunInProgressError) Error() string {
	return fmt.Sprintf("tourney: run %d is still ongoing, one run holds the machine at a time", e.RunID)
}
func (e *RunInProgressError) Unwrap() error { return ErrRunInProgress }

type SeriesResult struct {
	PairingSlot   int
	RedFirst      Participant
	BlueFirst     Participant
	WinnerSlot    *int
	RedFirstWins  int
	BlueFirstWins int
}
type RunResult struct {
	Run    Run
	Series []SeriesResult
	Board  []Standings
}
type Conductor struct {
	source MatchSource
}

func NewConductor(source MatchSource) *Conductor {
	return &Conductor{source: source}
}
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
func (c *Conductor) Run(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int, label string) (RunResult, error) {
	run, tiers, err := c.startRun(ctx, store, roster, tcIdx, boLen, startRating, parallel, label)
	if err != nil {
		return RunResult{}, err
	}
	return c.drive(ctx, store, run, roster, tiers, parallel)
}
func (c *Conductor) startRun(ctx context.Context, store *Store, roster []Participant,
	tcIdx, boLen, startRating, parallel int, label string) (Run, []*config.Tier, error) {
	tiers, err := tiersOfRoster(roster)
	if err != nil {
		return Run{}, nil, err
	}
	if err := checkCoreBudget(parallel, tiers); err != nil {
		return Run{}, nil, err
	}
	if src, ok := c.source.(coreLedgerSource); ok {
		if held := src.LiveCoreBookings(); held > 0 {
			return Run{}, nil, fmt.Errorf("tourney: %d live-search cores are held by open rooms, close them before starting a run", held)
		}
	}
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
	if err := store.ClaimDrive(ctx, run.ID, time.Now().Unix()); err != nil {
		return RunResult{}, err
	}
	finished := false
	defer func() {
		if !finished {
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
	for slot := range pairs {
		if schedule[slot].FinishedAt == nil {
			if err := store.ResetSeries(ctx, schedule[slot].ID); err != nil {
				return RunResult{}, err
			}
		}
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sem := make(chan struct{}, parallel)
	var wg sync.WaitGroup
	var once sync.Once
	var failure error
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
	if serr := logs.WriteRunSummary(run, roster, res.Board); serr != nil {
		return res, serr
	}
	if err := store.FinishRun(ctx, run.ID, time.Now().Unix()); err != nil {
		return res, fmt.Errorf("tourney: finish run %d: %w", run.ID, err)
	}
	finished = true
	return res, nil
}
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
func checkCoreBudget(parallel int, tiers []*config.Tier) error {
	if parallel < 1 {
		return fmt.Errorf("tourney: parallel %d, want at least 1", parallel)
	}
	worst := 0
	for _, t := range tiers {
		worst = max(worst, t.Cores)
	}
	for i, a := range tiers {
		for _, b := range tiers[i+1:] {
			worst = max(worst, config.RoomCores(*a, *b))
		}
	}
	if need := parallel * worst; need > config.MachineCores {
		return fmt.Errorf("tourney: %d parallel rooms at %d live-search cores each need %d cores, machine budget is %d",
			parallel, worst, need, config.MachineCores)
	}
	return nil
}
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
