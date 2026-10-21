package server

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func TestJoinAndListDuringRetirementWindow(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	retiring, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create retiring: %v", err)
	}
	live, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create live: %v", err)
	}

	retiring.Close()

	if err := s.rm.Join(retiring.ID(), bob.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("join a retired-but-keyed room = %v, want ErrRoomClosed", err)
	}
	rooms := s.rm.List()
	if len(rooms) != 1 || rooms[0].ID != live.ID() {
		t.Fatalf("grid = %+v, want only the live room %s", rooms, live.ID())
	}
}

func TestJoinPersistenceFailureLeavesSeatOpen(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bob := seedUser(t, s.store, "bob")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := s.store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	err = s.rm.Join(r.ID(), bob.ID)
	if err == nil || errors.Is(err, ErrRoomClosed) || errors.Is(err, ErrRoomFull) {
		t.Fatalf("join over a dead store = %v, want the create-series failure", err)
	}
	if !strings.Contains(err.Error(), "create series") {
		t.Errorf("join error = %v, want the create-series failure to surface", err)
	}

	info, ok := r.Info()
	if !ok {
		t.Fatal("room retired by a failed join, want it live")
	}
	if info.GuestUserID != 0 || info.State != SeriesCreated {
		t.Errorf("room after the failed join = %+v, want the open seat and created state", info)
	}
}

func TestRoomListTiesBrokenByID(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	first, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create first: %v", err)
	}
	second, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create second: %v", err)
	}
	same := time.Unix(1_700_000_000, 0).UTC()
	for _, r := range []*Room{first, second} {
		r.mu.Lock()
		r.createdAt = same
		r.mu.Unlock()
	}

	rooms := s.rm.List()
	if len(rooms) != 2 {
		t.Fatalf("grid = %d rooms, want 2", len(rooms))
	}
	lower, higher := first.ID(), second.ID()
	if strings.Compare(higher, lower) < 0 {
		lower, higher = higher, lower
	}
	if rooms[0].ID != lower || rooms[1].ID != higher {
		t.Errorf("same-instant order = [%s %s], want the id order [%s %s]",
			rooms[0].ID, rooms[1].ID, lower, higher)
	}
}

func TestPlayMoveAfterCloseRefused(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})

	r.Close()
	if err := r.PlayMove(alice.ID, mustCellT(t, "H8")); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("play after close = %v, want ErrRoomClosed", err)
	}
}

func TestPlayMoveZeroUserOnGuestlessRoomRefused(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.PlayMove(0, mustCellT(t, "D4")); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("play as user 0 on a guestless room = %v, want ErrNotParticipant", err)
	}
}

func TestForfeitAfterCloseRefused(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})

	r.Close()
	if err := r.Forfeit(alice.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("forfeit after close = %v, want ErrRoomClosed", err)
	}
}

func TestForfeitZeroUserOnGuestlessRoomRefused(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.Forfeit(0); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("forfeit as user 0 on a guestless room = %v, want ErrNotParticipant", err)
	}
	if _, ok := r.Info(); !ok {
		t.Error("room retired by a refused forfeit, want untouched")
	}
}

func injectSeriesAbort(t *testing.T, s *stack) func() {
	t.Helper()
	if _, err := s.store.db.Exec(
		`CREATE TRIGGER boom_series BEFORE INSERT ON series
		BEGIN SELECT RAISE(ABORT, 'injected series failure'); END`); err != nil {
		t.Fatalf("inject series abort: %v", err)
	}
	return func() {
		if _, err := s.store.db.Exec(`DROP TRIGGER boom_series`); err != nil {
			t.Fatalf("drop series abort: %v", err)
		}
	}
}

func TestCreateVsBotFailureLeavesLedgerClean(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	bot := config.Tiers[1]
	drop := injectSeriesAbort(t, s)
	defer drop()

	_, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &bot)
	if err == nil || !strings.Contains(err.Error(), "injected series failure") {
		t.Fatalf("create over the aborted series write = %v, want the injected failure", err)
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the failed create = %d, want 0", got)
	}
	if got := s.rm.adm.TTFree(); got != config.MachineTTBudgetBytes {
		t.Errorf("ttFree after the failed create = %d, want the full budget", got)
	}
	if _, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &bot); err == nil || !strings.Contains(err.Error(), "injected series failure") {
		t.Errorf("retry after the failed create = %v, want the injected failure again, not a busy machine", err)
	}
	if got := s.rm.LiveCoreBookings(); got != 0 {
		t.Errorf("cores after the retry = %d, want 0", got)
	}
}
