package tourney

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrDriveLive = errors.New("tourney: the run's drive is still live")

type Manager struct {
	conductor *Conductor
	store     *Store

	mu     sync.Mutex
	drives map[int64]*driveState
}

type driveState struct {
	done chan struct{}
	err  error
}

func NewManager(store *Store, source MatchSource) *Manager {
	return &Manager{
		conductor: NewConductor(source),
		store:     store,
		drives:    make(map[int64]*driveState),
	}
}

func (m *Manager) StartRun(ctx context.Context, spec RunSpec, parallel int) (Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	run, tiers, err := m.conductor.startRun(ctx, m.store, spec.Roster, spec.TCIdx, spec.BOLen, spec.StartRating, parallel, "ui", spec.Variant)
	if err != nil {
		return Run{}, err
	}
	st := &driveState{done: make(chan struct{})}
	m.drives[run.ID] = st
	go func() {
		_, err := m.conductor.drive(context.Background(), m.store, run, spec.Roster, tiers, parallel)
		m.mu.Lock()
		st.err = err
		close(st.done)
		m.mu.Unlock()
	}()
	return run, nil
}

type Detail struct {
	Run     Run
	Roster  []Participant
	Series  []Series
	Board   []Standings
	Running bool
	Failure error
}

func (m *Manager) Detail(ctx context.Context, runID int64) (Detail, error) {
	running, failure := m.driveState(runID)
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
	if run.Status != RunStateOngoing {
		running = false
	}
	return Detail{
		Run: run, Roster: roster, Series: series, Board: board,
		Running: running, Failure: failure,
	}, nil
}

func (m *Manager) Runs(ctx context.Context) ([]Run, error) {
	return m.store.Runs(ctx)
}

func (m *Manager) CloseStalled(ctx context.Context, runID int64) error {
	run, err := m.store.Run(ctx, runID)
	if err != nil {
		return err
	}
	if run.Status != RunStateOngoing {
		return fmt.Errorf("tourney: run %d is already %s", runID, run.Status)
	}
	if running, _ := m.driveState(runID); running {
		return fmt.Errorf("tourney: run %d: %w", runID, ErrDriveLive)
	}
	return m.store.CloseStalledRun(ctx, runID, time.Now().Unix())
}

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
	if run.Status != RunStateOngoing {
		return Detail{}, false, nil
	}
	series, err := m.store.SeriesAll(ctx, ongoing)
	if err != nil {
		return Detail{}, false, err
	}
	return Detail{Run: run, Series: series, Running: true}, true, nil
}

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
