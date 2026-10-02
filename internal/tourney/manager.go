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
	"sync"
)

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
// starts. ctx bounds only the persisting half; the drive runs under the
// manager's own context and dies with the process (a graceful shutdown
// retires the rooms, the streams fail, and the drive aborts on its own).
func (m *Manager) StartRun(ctx context.Context, spec RunSpec, parallel int) (Run, error) {
	run, tiers, err := m.conductor.startRun(ctx, m.store, spec.Roster, spec.TCIdx, spec.BOLen, spec.StartRating, parallel)
	if err != nil {
		return Run{}, err
	}
	st := &driveState{done: make(chan struct{})}
	m.mu.Lock()
	m.drives[run.ID] = st
	m.mu.Unlock()
	go func() {
		_, err := m.conductor.drive(context.Background(), m.store, run, spec.Roster, tiers, parallel)
		m.mu.Lock()
		st.err = err
		m.mu.Unlock()
		close(st.done)
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
