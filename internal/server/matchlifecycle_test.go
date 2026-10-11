package server

import (
	"bytes"
	"errors"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestReadyRefusedBeforeJoinAndAfterClose(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")

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

	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit open room: %v", err)
	}
	if err := r.Ready(alice.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("ready after close = %v, want ErrRoomClosed", err)
	}
}

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

func TestGuestWonSeriesPersistsGuestWinner(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)

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

func TestInMemoryRoomSkipsGamePersistence(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	r.mu.Lock()
	r.seriesID = 0
	r.mu.Unlock()

	playScript(t, r, alice.ID, bob.ID, hostWinsRed)

	var games, ratings int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&games); err != nil {
		t.Fatalf("count games: %v", err)
	}
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM rating_events`).Scan(&ratings); err != nil {
		t.Fatalf("count rating events: %v", err)
	}
	if games != 0 || ratings != 0 {
		t.Errorf("disk after the in-memory game = %d games %d ratings, want zero of both", games, ratings)
	}
	r.mu.Lock()
	state := r.series.State()
	r.mu.Unlock()
	if state != SeriesInGame {
		t.Errorf("series state after game 1 = %v, want in-game", state)
	}
	if _, ok := r.Info(); !ok {
		t.Error("room retired after an in-memory game, want live")
	}
}

func TestFirstPersistedGameIndexesSeriesFromZero(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)

	var idx int
	if err := s.store.db.QueryRow(`SELECT idx_in_series FROM games WHERE series_id = ?`, r.SeriesID()).Scan(&idx); err != nil {
		t.Fatalf("read game index: %v", err)
	}
	if idx != 0 {
		t.Errorf("game 1 idx_in_series = %d, want 0", idx)
	}
}

func TestForfeitMarksRoomFinished(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})

	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
	r.mu.Lock()
	finished, over := r.finished, r.over
	r.mu.Unlock()
	if !finished || !over {
		t.Errorf("room flags after the forfeit = finished %t over %t, want both true", finished, over)
	}
}

func TestForfeitOnInMemoryRoomSkipsPersistence(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, []string{"D4", "P16"})
	r.mu.Lock()
	r.seriesID = 0
	r.mu.Unlock()

	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit an in-memory room: %v", err)
	}
	var games int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM games`).Scan(&games); err != nil {
		t.Fatalf("count games: %v", err)
	}
	if games != 0 {
		t.Errorf("games after the in-memory forfeit = %d, want 0", games)
	}
	if _, ok := r.Info(); ok {
		t.Error("room still live after the forfeit, want retired")
	}
}

func TestForfeitMidGameBillsExactFullTurns(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	live := hostWinsRed[:5]
	playScript(t, r, alice.ID, bob.ID, live)
	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}

	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(rows) != config.SeriesBO3 {
		t.Fatalf("history = %d rows err %v, want the swept %d", len(rows), err, config.SeriesBO3)
	}
	liveRow := rows[len(rows)-1]
	if liveRow.FullTurns != len(live)/2 {
		t.Errorf("live game full turns = %d, want %d", liveRow.FullTurns, len(live)/2)
	}
	if want := EncodeMoves(nil, movesOf(t, live)); string(liveRow.Moves) != string(want) {
		t.Errorf("live game blob = %v, want %v", liveRow.Moves, want)
	}
}

func TestForfeitSweepNumbersGamesFromZero(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}

	rows, err := s.store.db.Query(`SELECT idx_in_series FROM games WHERE series_id = ? ORDER BY id`, r.SeriesID())
	if err != nil {
		t.Fatalf("read game indexes: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var got []int
	for rows.Next() {
		var idx int
		if err := rows.Scan(&idx); err != nil {
			t.Fatalf("scan game index: %v", err)
		}
		got = append(got, idx)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate game indexes: %v", err)
	}
	if len(got) != config.SeriesBO3 {
		t.Fatalf("game indexes = %v, want %d entries", got, config.SeriesBO3)
	}
	for i, idx := range got {
		if idx != i {
			t.Errorf("game %d idx_in_series = %d, want %d", i+1, idx, i)
		}
	}
}

func TestLogBotCompletionReportsOnlyFailures(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})

	logBotCompletion(nil)
	if buf.Len() != 0 {
		t.Errorf("nil completion logged %q, want silence", buf.String())
	}
	boom := errors.New("bot completion probe failure")
	logBotCompletion(boom)
	if out := buf.String(); !strings.Contains(out, "server: bot game completion persistence failed: "+boom.Error()) {
		t.Errorf("failed completion logged %q, want the persistence failure line", out)
	}
}
