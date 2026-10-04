package tourney

// The M7 UI's service over the conductor. The conductor's Run is synchronous
// and headless (the CLI drivers block on it); the setup page needs the run id
// the moment the form posts and progress while the games play out. The
// manager bridges the two without touching the conductor: it persists with
// startRun on the caller's context, hands the id back, and drives the
// persisted run on a background goroutine under its own context, because a
// page request's context dies with the redirect response. Progress itself is
// never held in memory: Detail reads the store the conductor persists to, so
// the run page sees exactly what landed, and only the terminal drive error
// rides the manager's table.

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrDriveLive refuses closing a run this manager still drives: the drive's
// own FinishRun, not a hand close, owns the row.
var ErrDriveLive = errors.New("tourney: the run's drive is still live")

// Manager starts tournament runs in the background and reads their state.
type Manager struct {
	conductor *Conductor
	store     *Store

	mu     sync.Mutex
	drives map[int64]*driveState
}

// driveState tracks one background drive: done closes after the drive
// returned, err holds its terminal error, written before done closes.
type driveState struct {
	done chan struct{}
	err  error
}

// NewManager wires a manager over the tourney store and the match surface
// every series of its runs rides.
func NewManager(store *Store, source MatchSource) *Manager {
	return &Manager{
		conductor: NewConductor(source),
		store:     store,
		drives:    make(map[int64]*driveState),
	}
}

// StartRun validates and persists one tournament synchronously, then drives
// it in the background. The returned Run exists the moment StartRun returns,
// so a page can redirect onto the run's live board before the first series
// starts. The manager lock spans the persisting half: two in-process starts
// serialize on it, so the second reads the first's ongoing row through the
// run gate and refuses instead of oversubscribing the machine; the drive
// itself runs outside the lock. ctx bounds only the persisting half; the
// drive runs under the manager's own context and dies with the process (a
// graceful shutdown retires the rooms, the streams fail, and the drive
// aborts on its own).
func (m *Manager) StartRun(ctx context.Context, spec RunSpec, parallel int) (Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, tiers, err := m.conductor.startRun(ctx, m.store, spec.Roster, spec.TCIdx, spec.BOLen, spec.StartRating, parallel)
	if err != nil {
		return Run{}, err
	}
	st := &driveState{done: make(chan struct{})}
	m.drives[run.ID] = st
	go func() {
		_, err := m.conductor.drive(context.Background(), m.store, run, spec.Roster, tiers, parallel, "ui")
		// The done channel closes under the same lock the error write
		// takes: a window with err set but done open lets OngoingRun's
		// scan report this finished drive as the live one, and a StartRun
		// landing there leaves two open drives whose map-order pick is
		// random. driveState's done check is a non-blocking select, so
		// closing under the lock blocks nobody.
		m.mu.Lock()
		st.err = err
		close(st.done)
		m.mu.Unlock()
	}()
	return run, nil
}

// Detail is one run's whole read for a page: the header, the roster, the
// series lines with their settled aggregates, the live leaderboard, and the
// drive state. Running is true while this manager still drives the run;
// Failure is the terminal drive error of a finished drive (a run this
// process never drove reads back unfailed and not running).
type Detail struct {
	Run     Run
	Roster  []Participant
	Series  []Series
	Board   []Standings
	Running bool
	Failure error
}

// Detail reads one run's render state; an unknown run maps onto
// server.ErrNotFound.
func (m *Manager) Detail(ctx context.Context, runID int64) (Detail, error) {
	run, err := m.store.Run(ctx, runID)
	if err != nil {
		return Detail{}, err
	}
	roster, err := m.store.Roster(ctx, runID)
	if err != nil {
		return Detail{}, err
	}
	series, err := m.store.SeriesAll(ctx, runID)
	if err != nil {
		return Detail{}, err
	}
	board, err := m.store.Leaderboard(ctx, runID)
	if err != nil {
		return Detail{}, err
	}
	running, failure := m.driveState(runID)
	return Detail{
		Run: run, Roster: roster, Series: series, Board: board,
		Running: running, Failure: failure,
	}, nil
}

// Runs lists run headers newest first, the setup page's past-runs section.
func (m *Manager) Runs(ctx context.Context) ([]Run, error) {
	return m.store.Runs(ctx)
}

// CloseStalled closes a stalled run's row: an ongoing run no live drive of
// this manager owns (a previous process's crash left it, the run page
// already renders it stalled). A live drive refuses with ErrDriveLive, an
// unknown run with server.ErrNotFound, and an already-closed run with its
// own refusal. The close writes no standings snapshot: the live leaderboard
// keeps deriving from the games, the snapshot stays a clean-finish artifact.
func (m *Manager) CloseStalled(ctx context.Context, runID int64) error {
	if running, _ := m.driveState(runID); running {
		return fmt.Errorf("tourney: run %d: %w", runID, ErrDriveLive)
	}
	run, err := m.store.Run(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != RunStateOngoing {
		return fmt.Errorf("tourney: run %d is already %s", runID, run.Status)
	}
	return m.store.CloseStalledRun(ctx, runID, time.Now().Unix())
}

// OngoingRun reads the run this manager currently drives with the banner's
// own slice of detail: the header and the series lines, no roster and no
// leaderboard fold. The read rides every home poll of every viewer while a
// run is live, so it touches the two tables the settled count needs and
// nothing else. ok is false when no drive is live; with done closed under
// the lock, an open drive is a driven run and the run gate guarantees at
// most one.
func (m *Manager) OngoingRun(ctx context.Context) (Detail, bool, error) {
	m.mu.Lock()
	var ongoing int64
	found := false
	for id, st := range m.drives {
		select {
		case <-st.done:
		default:
			ongoing, found = id, true
		}
	}
	m.mu.Unlock()
	if !found {
		return Detail{}, false, nil
	}
	run, err := m.store.Run(ctx, ongoing)
	if err != nil {
		return Detail{}, false, err
	}
	series, err := m.store.SeriesAll(ctx, ongoing)
	if err != nil {
		return Detail{}, false, err
	}
	return Detail{Run: run, Series: series, Running: true}, true, nil
}

// driveState reads one run's tracked drive under the manager lock.
func (m *Manager) driveState(runID int64) (running bool, failure error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	st, ok := m.drives[runID]
	if !ok {
		return false, nil
	}
	select {
	case <-st.done:
		return false, st.err
	default:
		return true, nil
	}
}
