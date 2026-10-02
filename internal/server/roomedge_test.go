package server

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Join and grid edges of the room layer: the mid-retirement window where the
// room is over but still keyed, the persistence failure that must leave the
// guest seat open, and the grid line that skips a retired room.

// TestJoinAndListDuringRetirementWindow pins the state a concurrent client
// can observe between retire's over flip and the manager's map delete: join
// answers room_closed and the grid omits the room, while live neighbors stay
// listed.
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

	// Room.Close retires without dropping the manager entry, holding the
	// room in exactly the mid-retirement state.
	retiring.Close()

	if err := s.rm.Join(retiring.ID(), bob.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("join a retired-but-keyed room = %v, want ErrRoomClosed", err)
	}
	rooms := s.rm.List()
	if len(rooms) != 1 || rooms[0].ID != live.ID() {
		t.Fatalf("grid = %+v, want only the live room %s", rooms, live.ID())
	}
}

// TestJoinPersistenceFailureLeavesSeatOpen pins join's atomicity: when the
// pairing row cannot persist, the join refuses and the guest seat stays
// open, so no game can reference a series that was never written.
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

// TestRoomListTiesBrokenByID pins the grid's second sort key: two rooms
// stamped the same creation instant list in id order, so the grid order is
// total whatever the clock resolution.
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
