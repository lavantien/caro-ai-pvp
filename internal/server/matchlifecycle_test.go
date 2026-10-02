package server

import (
	"errors"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Handshake and series-resolution edges of the match driver: the refused
// ready calls, the pre-handshake clock read, and the two series-winner
// resolutions the fix waves left unexercised, the drawn series and the guest
// sweep.

func TestReadyRefusedBeforeJoinAndAfterClose(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")

	// No opponent seated, no series formed: the handshake cannot start.
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := r.Ready(alice.ID); !errors.Is(err, ErrNotReady) {
		t.Errorf("ready before join = %v, want ErrNotReady", err)
	}
	if rem := r.ClockRemaining(rules.Red); rem != 0 {
		t.Errorf("red clock before the handshake = %s, want the documented 0", rem)
	}
	if rem := r.ClockRemaining(rules.Blue); rem != 0 {
		t.Errorf("blue clock before the handshake = %s, want the documented 0", rem)
	}

	// A retired room refuses everything, the handshake included.
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit open room: %v", err)
	}
	if err := r.Ready(alice.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("ready after close = %v, want ErrRoomClosed", err)
	}
}

// TestDrawnSeriesPersistsNilWinner closes a bo3 on three drawn games: the
// schedule runs out level, the series winner resolves to the nil
// representation callers persist, and the finished row keeps winner NULL
// while the games and the level stats land.
func TestDrawnSeriesPersistsNilWinner(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)

	for game := 1; game <= config.SeriesBO3; game++ {
		playScript(t, r, alice.ID, bob.ID, drawMoves(t))
	}
	if _, ok := r.Info(); ok {
		t.Fatal("room still live after the drawn-out schedule, want retired")
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished {
		t.Errorf("series state = %s, want finished", sr.State)
	}
	if sr.Winner != nil {
		t.Errorf("drawn series winner = %d, want NULL", *sr.Winner)
	}
	if sr.FinishedAt == nil {
		t.Error("drawn series finished_at = NULL, want stamped")
	}

	// Three drawn games persisted, each with its full blob, and the level
	// and draw totals read them back.
	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(rows) != config.SeriesBO3 {
		t.Fatalf("history = %d rows err %v, want the %d draws", len(rows), err, config.SeriesBO3)
	}
	for i, row := range rows {
		if row.WonBy != nil {
			t.Errorf("drawn game %d won_by = %q, want nil", i+1, *row.WonBy)
		}
		if len(row.Moves) != 2*config.BoardCells {
			t.Errorf("drawn game %d blob = %d bytes, want the full fill", i+1, len(row.Moves))
		}
	}
	for _, u := range []User{alice, bob} {
		st, err := s.store.UserStats(u.ID)
		if err != nil {
			t.Fatalf("stats %s: %v", u.Username, err)
		}
		if st.Wins != 0 || st.Losses != 0 || st.Draws != config.SeriesBO3 || st.SeriesWon != 0 {
			t.Errorf("%s stats = %+v, want 0-0-%d draws and no level", u.Username, st, config.SeriesBO3)
		}
		if hist, err := s.store.RatingHistoryByUser(u.ID); err != nil || len(hist) != 0 {
			t.Errorf("%s rating events = %d err %v, want none on all-draw games", u.Username, len(hist), err)
		}
	}
}

// TestGuestWonSeriesPersistsGuestWinner covers the other side of the
// resolution: a guest sweep maps the machine's SideGuest onto the guest's
// user id in the persisted finish.
func TestGuestWonSeriesPersistsGuestWinner(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)

	// The host holds red in both games and loses both as red: the guest
	// sweeps 2-0 without ever taking the red seat.
	playScript(t, r, alice.ID, bob.ID, guestRedLosesToBlue)
	playScript(t, r, alice.ID, bob.ID, guestRedLosesToBlue)
	if _, ok := r.Info(); ok {
		t.Fatal("room still live after the guest sweep, want retired")
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != bob.ID {
		t.Errorf("series row = %+v, want finished with the guest winner %d", sr, bob.ID)
	}

	hostWins, guestWins := 0, 0
	if hostWins, guestWins = r.series.Score(); guestWins != 2 || hostWins != 0 {
		t.Errorf("final line = %d-%d, want the guest sweep 2-0", hostWins, guestWins)
	}
	if st, err := s.store.UserStats(bob.ID); err != nil || st.SeriesWon != 1 {
		t.Errorf("guest level = %+v err %v, want one series won", st, err)
	}
}
