package server

import (
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// playScript drives alternating moves through PlayMove: names[0] is red's.
// The clock never dips below zero on either side after any move.
func playScript(t *testing.T, r *Room, redUser, blueUser int64, names []string) {
	t.Helper()
	for i, name := range names {
		mover := redUser
		if i%2 == 1 {
			mover = blueUser
		}
		cell, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if err := r.PlayMove(mover, cell); err != nil {
			t.Fatalf("move %d %s by %d: %v", i+1, name, mover, err)
		}
		for _, c := range []rules.Color{rules.Red, rules.Blue} {
			if rem := r.ClockRemaining(c); rem < 0 {
				t.Fatalf("move %d: %s clock at %s, below zero", i+1, sideName(c), rem)
			}
		}
	}
}

// hostWinsRed runs the fastest honest decisive game: red builds an open
// four on the D file while blue stones scatter far away, so the winning
// stone closes an open four. The loser-takes-red rotation is exercised by
// feeding the mirrored script to the next game.
var (
	hostWinsRed         = []string{"D4", "P16", "H8", "P12", "D5", "P8", "D6", "N16", "D7", "M4", "D3"}
	guestRedLosesToBlue = []string{"P16", "D4", "H8", "D5", "P12", "D6", "P8", "D7", "N16", "D3"}
)

// newPvPRoom yields a joined, not-yet-ready bo3 1+0 room between two seeded
// users, host first.
func newPvPRoom(t *testing.T, s *stack) (host, guest User, r *Room) {
	t.Helper()
	host, guest = seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob")
	r, err := s.rm.Create(host.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := s.rm.Join(r.ID(), guest.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	return host, guest, r
}

func readyBoth(t *testing.T, r *Room, host, guest User) {
	t.Helper()
	if err := r.Ready(host.ID); err != nil {
		t.Fatalf("host ready: %v", err)
	}
	if err := r.Ready(guest.ID); err != nil {
		t.Fatalf("guest ready: %v", err)
	}
}

func TestPlayMoveHandshakeAndValidation(t *testing.T) {
	s := newStack(t)
	alice, bob, carol := seedUser(t, s.store, "alice"), seedUser(t, s.store, "bob"), seedUser(t, s.store, "carol")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	d4, _ := rules.ParseCell("D4")

	// Open room: no series yet, and the host cannot move it into existence.
	if err := r.PlayMove(alice.ID, d4); !errors.Is(err, ErrNotReady) {
		t.Errorf("play before join = %v, want ErrNotReady", err)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("host abandons open room: %v", err)
	}
	if _, ok := r.Info(); ok {
		t.Error("abandoned room still listed, want retired")
	}

	r, err = s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("recreate: %v", err)
	}
	if err := s.rm.Join(r.ID(), bob.ID); err != nil {
		t.Fatalf("join: %v", err)
	}
	if err := r.PlayMove(alice.ID, d4); !errors.Is(err, ErrNotReady) {
		t.Errorf("play before both ready = %v, want ErrNotReady", err)
	}
	if err := r.Ready(carol.ID); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("stranger ready = %v, want ErrNotParticipant", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("host ready: %v", err)
	}
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("double ready must be idempotent: %v", err)
	}
	if err := r.PlayMove(alice.ID, d4); !errors.Is(err, ErrNotReady) {
		t.Errorf("play after one ready = %v, want ErrNotReady", err)
	}
	if err := r.Ready(bob.ID); err != nil {
		t.Fatalf("guest ready: %v", err)
	}

	if err := r.PlayMove(carol.ID, d4); !errors.Is(err, ErrNotParticipant) {
		t.Errorf("stranger plays = %v, want ErrNotParticipant", err)
	}
	if err := r.PlayMove(bob.ID, d4); !errors.Is(err, ErrNotYourTurn) {
		t.Errorf("guest moves first = %v, want ErrNotYourTurn", err)
	}
	if err := r.PlayMove(alice.ID, d4); err != nil {
		t.Fatalf("host red first: %v", err)
	}
	p16, _ := rules.ParseCell("P16")
	if err := r.PlayMove(bob.ID, d4); !errors.Is(err, ErrIllegalMove) {
		t.Errorf("occupied cell = %v, want ErrIllegalMove", err)
	}
	if err := r.PlayMove(bob.ID, p16); err != nil {
		t.Fatalf("guest blue: %v", err)
	}
	d5, _ := rules.ParseCell("D5")
	if err := r.PlayMove(alice.ID, d5); !errors.Is(err, ErrIllegalMove) {
		t.Errorf("red second stone inside opening distance = %v, want ErrIllegalMove", err)
	}
}

func TestPvPBO3SeriesDrivenThroughPlayMove(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	readyBoth(t, r, alice, bob)

	// The host takes red first per the spec.
	r.mu.Lock()
	redFirst := r.series.RedUserID()
	r.mu.Unlock()
	if redFirst != alice.ID {
		t.Fatalf("game 1 red = %d, want host %d", redFirst, alice.ID)
	}

	playScript(t, r, alice.ID, bob.ID, hostWinsRed)

	// Decisive game 1: red passes to the loser for game 2.
	r.mu.Lock()
	redSecond, state := r.series.RedUserID(), r.series.State()
	r.mu.Unlock()
	if redSecond != bob.ID {
		t.Fatalf("game 2 red = %d, want loser bob %d", redSecond, bob.ID)
	}
	if state != SeriesInGame {
		t.Fatalf("series state after game 1 = %v, want in-game", state)
	}

	playScript(t, r, bob.ID, alice.ID, guestRedLosesToBlue)

	// Series over 2-0: room retired, off the grid, state machine closed.
	if _, ok := r.Info(); ok {
		t.Error("finished room still listed, want retired")
	}
	if rooms := s.rm.List(); len(rooms) != 0 {
		t.Errorf("grid after finish = %d rooms, want 0", len(rooms))
	}
	d4, _ := rules.ParseCell("D4")
	if err := r.PlayMove(alice.ID, d4); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("play after finish = %v, want ErrRoomClosed", err)
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != alice.ID || sr.FinishedAt == nil {
		t.Errorf("series row = %+v, want finished with host winner", sr)
	}

	// Rating law: zero-sum per decisive game, chained on current ratings.
	// Game 2 seats bob on red, so alice rides the blue deltas.
	d1r, d1b, a1, b1 := RatingDeltas(0, 0, RedWins)
	d2r, d2b, a2, b2 := RatingDeltas(b1, a1, BlueWins)
	assertRatings(t, s, alice, bob, []RatingEvent{
		{Delta: d1r, RatingAfter: a1}, {Delta: d2b, RatingAfter: b2},
	}, []RatingEvent{
		{Delta: d1b, RatingAfter: b1}, {Delta: d2r, RatingAfter: a2},
	})

	// Newest first: game 2 above game 1, with the rotation and the tags.
	// Score lines count the row's players' wins, so alice's 2-0 series reads
	// 1-0 on her red row and 0-2 on bob's red row.
	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("history rows = %d, want 2", len(rows))
	}
	g2, g1 := rows[0], rows[1]
	if g1.Red != "alice" || g1.Blue != "bob" || g1.RedWins != 1 || g1.BlueWins != 0 {
		t.Errorf("game 1 row = %+v, want host red at 1-0", g1)
	}
	if g2.Red != "bob" || g2.Blue != "alice" || g2.RedWins != 0 || g2.BlueWins != 2 {
		t.Errorf("game 2 row = %+v, want guest red at 0-2 by participant", g2)
	}
	for _, row := range []struct {
		m MatchHistoryRow
		n []string
	}{{g1, hostWinsRed}, {g2, guestRedLosesToBlue}} {
		moves, err := DecodeMoves(row.m.Moves)
		if err != nil {
			t.Fatalf("decode moves: %v", err)
		}
		if len(moves) != len(row.n) {
			t.Fatalf("moves blob = %d moves, want %d", len(moves), len(row.n))
		}
		for i, name := range row.n {
			cell, err := rules.ParseCell(name)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}
			if moves[i] != rules.Move(cell) {
				t.Errorf("move %d = %d, want %s", i, moves[i], name)
			}
		}
		if row.m.FullTurns != len(row.n)/2 {
			t.Errorf("full turns = %d, want %d", row.m.FullTurns, len(row.n)/2)
		}
		if row.m.WonBy == nil || *row.m.WonBy != WonByOpenFour {
			t.Errorf("won by = %v, want %q", row.m.WonBy, WonByOpenFour)
		}
	}

	// Hub stream: every stone, each game end, then the series end.
	var kinds []string
	for {
		select {
		case ev := <-sub.Events():
			kinds = append(kinds, ev.Kind+" "+ev.Payload)
		default:
			goto drained
		}
	}
drained:
	want := len(hostWinsRed) + len(guestRedLosesToBlue) + 3 // moves + 2 gameends + series
	if len(kinds) != want {
		t.Fatalf("events = %d, want %d: %v", len(kinds), want, kinds)
	}
	for i, name := range hostWinsRed {
		if kinds[i] != EventKindMove+" "+name {
			t.Errorf("event %d = %q, want move %s", i, kinds[i], name)
		}
	}
	if kinds[len(hostWinsRed)] != EventKindGameEnd+" "+OutcomeRed {
		t.Errorf("game 1 end = %q", kinds[len(hostWinsRed)])
	}
	tail := len(kinds) - 1
	if kinds[tail] != EventKindSeries+" "+SideHost.String() {
		t.Errorf("series end = %q, want host", kinds[tail])
	}
	if kinds[tail-1] != EventKindGameEnd+" "+OutcomeBlue {
		t.Errorf("game 2 end = %q", kinds[tail-1])
	}

	// Clock law on 1+0: banks only drain, never below zero, and the final
	// game's clocks hold one commit per played stone.
	for _, c := range []rules.Color{rules.Red, rules.Blue} {
		cap := time.Duration(config.TimeControls[0].InitialSec) * time.Second
		if rem := r.ClockRemaining(c); rem < 0 || rem > cap {
			t.Errorf("%s clock = %s, want within [0, %d]", sideName(c), rem, cap)
		}
	}
	r.mu.Lock()
	redCommits, blueCommits := r.clock[rules.Red].Moves(), r.clock[rules.Blue].Moves()
	r.mu.Unlock()
	if redCommits != len(guestRedLosesToBlue)/2 || blueCommits != len(guestRedLosesToBlue)/2 {
		t.Errorf("final game clock commits = red %d blue %d, want 5 and 5", redCommits, blueCommits)
	}

	// PvP games carry no bot moves, so the per-move stat table stays empty:
	// its zero rows are the honest record, not a missed write.
	var statRows int
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM game_stats`).Scan(&statRows); err != nil {
		t.Fatalf("count game stats: %v", err)
	}
	if statRows != 0 {
		t.Errorf("game_stats rows after the pvp series = %d, want 0", statRows)
	}
}

func assertRatings(t *testing.T, s *stack, red, blue User, wantRed, wantBlue []RatingEvent) {
	t.Helper()
	for _, tc := range []struct {
		user int64
		want []RatingEvent
	}{{red.ID, wantRed}, {blue.ID, wantBlue}} {
		hist, err := s.store.RatingHistoryByUser(tc.user)
		if err != nil {
			t.Fatalf("history %d: %v", tc.user, err)
		}
		if len(hist) != len(tc.want) {
			t.Fatalf("history %d = %d events, want %d", tc.user, len(hist), len(tc.want))
		}
		for i, w := range tc.want {
			if hist[i].UserID != tc.user || hist[i].Delta != w.Delta || hist[i].RatingAfter != w.RatingAfter {
				t.Errorf("history %d[%d] = %+v, want %+v", tc.user, i, hist[i], w)
			}
		}
	}
}

// drawMoves fills the whole board without either side ever holding five in
// a line: cells are pre-colored by (row+2*col) mod 4, red taking residues 0
// and 1, blue 2 and 3. Every straight window of 5 consecutive cells then
// contains both colors along all four directions, so no five can exist at
// any point of the fill. Red's first two stones are reordered to satisfy
// the opening distance.
func drawMoves(t *testing.T) []string {
	t.Helper()
	var red, blue []string
	for row := range config.BoardSize {
		for col := range config.BoardSize {
			name := string(rune('A'+col)) + strconv.Itoa(row+1)
			if (row+2*col)%4 < 2 {
				red = append(red, name)
			} else {
				blue = append(blue, name)
			}
		}
	}
	red[1], red[len(red)-1] = red[len(red)-1], red[1]
	if len(red) != len(blue) {
		t.Fatalf("draw split = %d red vs %d blue, want equal halves", len(red), len(blue))
	}
	out := make([]string, 0, len(red)+len(blue))
	for i := range red {
		out = append(out, red[i], blue[i])
	}
	return out
}

func TestFullBoardDrawKeepsRedAndPersistsNoTag(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	script := drawMoves(t)
	if len(script) != config.BoardCells {
		t.Fatalf("draw script = %d moves, want %d", len(script), config.BoardCells)
	}
	playScript(t, r, alice.ID, bob.ID, script)

	// Draw: no rating event, no won_by, red keeps the seat, game 2 live.
	r.mu.Lock()
	redAgain, state := r.series.RedUserID(), r.series.State()
	r.mu.Unlock()
	if state != SeriesInGame || redAgain != alice.ID {
		t.Fatalf("after draw: state %v red %d, want in-game with red kept", state, redAgain)
	}
	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(rows) != 1 {
		t.Fatalf("history = %d rows err %v, want 1", len(rows), err)
	}
	if rows[0].WonBy != nil {
		t.Errorf("draw won_by = %q, want nil", *rows[0].WonBy)
	}
	if rows[0].FullTurns != config.BoardCells/2 {
		t.Errorf("draw full turns = %d, want %d", rows[0].FullTurns, config.BoardCells/2)
	}
	if len(rows[0].Moves) != 2*config.BoardCells {
		t.Errorf("draw blob = %d bytes, want %d", len(rows[0].Moves), 2*config.BoardCells)
	}
	for _, uid := range []int64{alice.ID, bob.ID} {
		if hist, err := s.store.RatingHistoryByUser(uid); err != nil || len(hist) != 0 {
			t.Errorf("rating events after draw = %d err %v, want none", len(hist), err)
		}
	}

	// Close the series 2-0 around the draw: win, then win as blue.
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)
	r.mu.Lock()
	redThird := r.series.RedUserID()
	r.mu.Unlock()
	if redThird != bob.ID {
		t.Fatalf("game 3 red = %d, want bob", redThird)
	}
	playScript(t, r, bob.ID, alice.ID, guestRedLosesToBlue)
	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != alice.ID {
		t.Errorf("series row = %+v, want finished host", sr)
	}
	if rows, err = s.store.MatchHistory(alice.ID); err != nil || len(rows) != 3 {
		t.Fatalf("history = %d rows err %v, want 3", len(rows), err)
	}
}

func TestForfeitBeforeFirstStonePersistsZeroLengthBlob(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)

	// Game 1 is live but no stone has landed: the forfeit sweep writes the
	// live game with a zero-length blob, which must satisfy the NOT NULL
	// moves column instead of failing the whole persistence chain.
	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}

	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(rows) != 3 {
		t.Fatalf("history = %d rows err %v, want the swept bo3", len(rows), err)
	}
	for i, row := range rows {
		// The write boundary guarantees a non-nil blob (EncodeMoves), but a
		// zero-length blob scans back as Go nil: only the length is
		// observable after the SQL round trip.
		if len(row.Moves) != 0 {
			t.Errorf("game %d moves = %v, want a zero-length record", 3-i, row.Moves)
		}
		if row.FullTurns != 0 {
			t.Errorf("game %d turns = %d, want 0", 3-i, row.FullTurns)
		}
	}
}

func TestForfeitMidSeriesBillsRemainingGames(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)

	// Three moves into game 2 the quitter walks.
	playScript(t, r, bob.ID, alice.ID, []string{"D4", "P16", "H8"})
	if err := r.Forfeit(bob.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}

	if _, ok := r.Info(); ok {
		t.Error("forfeited room still listed, want retired")
	}
	if err := r.Forfeit(bob.ID); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("double forfeit = %v, want ErrRoomClosed", err)
	}

	r.mu.Lock()
	hostWins, guestWins := r.series.Score()
	r.mu.Unlock()
	if hostWins != 3 || guestWins != 0 {
		t.Fatalf("final line = %d-%d, want 3-0 swept", hostWins, guestWins)
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != alice.ID || sr.FinishedAt == nil {
		t.Errorf("series row = %+v, want finished with host winner", sr)
	}

	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil || len(rows) != 3 {
		t.Fatalf("history = %d rows err %v, want 3", len(rows), err)
	}
	// Newest first: the two synthetic games above the played one.
	wantMoves := [][]byte{
		{}, // game 3 never started
		EncodeMoves(nil, movesOf(t, []string{"D4", "P16", "H8"})),
		EncodeMoves(nil, movesOf(t, hostWinsRed)),
	}
	wantTurns := []int{0, 1, len(hostWinsRed) / 2}
	for i, row := range rows {
		if string(row.Moves) != string(wantMoves[i]) {
			t.Errorf("game %d blob = %v, want %v", 3-i, row.Moves, wantMoves[i])
		}
		if row.FullTurns != wantTurns[i] {
			t.Errorf("game %d turns = %d, want %d", 3-i, row.FullTurns, wantTurns[i])
		}
		if i < 2 && row.WonBy != nil {
			t.Errorf("game %d won_by = %q, want nil: a quit is not a board win", 3-i, *row.WonBy)
		}
	}
	if rows[2].WonBy == nil || *rows[2].WonBy != WonByOpenFour {
		t.Errorf("played game 1 won_by = %v, want its board tag", rows[2].WonBy)
	}
	if rows[2].Red != "alice" || rows[1].Red != "bob" || rows[0].Red != "bob" {
		t.Errorf("rotation in swept rows = %s/%s/%s", rows[2].Red, rows[1].Red, rows[0].Red)
	}
	// Score lines count the row's players' wins, newest first: bob's red
	// rows read 0-3 and 0-2 across the sweep, alice's red row 1-0.
	wantLines := [][2]int{{0, 3}, {0, 2}, {1, 0}}
	for i, row := range rows {
		if row.RedWins != wantLines[i][0] || row.BlueWins != wantLines[i][1] {
			t.Errorf("row %d score = %d-%d, want %d-%d", i, row.RedWins, row.BlueWins, wantLines[i][0], wantLines[i][1])
		}
	}

	// Ratings chain across the sweep, still zero-sum per game. Bob stays on
	// red in every synthetic game, so alice rides the blue deltas.
	d1r, d1b, a1, b1 := RatingDeltas(0, 0, RedWins)
	d2r, d2b, a2, b2 := RatingDeltas(b1, a1, BlueWins)
	d3r, d3b, a3, b3 := RatingDeltas(a2, b2, BlueWins)
	assertRatings(t, s, alice, bob, []RatingEvent{
		{Delta: d1r, RatingAfter: a1}, {Delta: d2b, RatingAfter: b2}, {Delta: d3b, RatingAfter: b3},
	}, []RatingEvent{
		{Delta: d1b, RatingAfter: b1}, {Delta: d2r, RatingAfter: a2}, {Delta: d3r, RatingAfter: a3},
	})
}

// A failed persistence write must surface as the returned error, never as
// a wedged room: the series advances in memory whatever the queue says.
func TestPlayMoveSurvivesWriteQueueClose(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)

	s.wq.Close()
	for i, name := range guestRedLosesToBlue {
		mover := bob.ID
		if i%2 == 1 {
			mover = alice.ID
		}
		err := r.PlayMove(mover, mustCellT(t, name))
		if i < len(guestRedLosesToBlue)-1 && err != nil {
			t.Fatalf("move %d: %v", i+1, err)
		}
		if i == len(guestRedLosesToBlue)-1 && !errors.Is(err, ErrQueueClosed) {
			t.Fatalf("final move = %v, want ErrQueueClosed surfaced", err)
		}
	}
	if _, ok := r.Info(); ok {
		t.Error("room still live after the finished series, want retired")
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); !errors.Is(err, ErrRoomClosed) {
		t.Errorf("play after finish = %v, want ErrRoomClosed", err)
	}
}

// injectRatingAbort raises an SQLite ABORT on every rating_events insert, so
// a completion unit fails mid-way at the exact statement a disk error or a
// busy-timeout expiry would hit. The returned func drops the trigger.
func injectRatingAbort(t *testing.T, s *stack) func() {
	t.Helper()
	if _, err := s.store.db.Exec(
		`CREATE TRIGGER boom_rating BEFORE INSERT ON rating_events
		BEGIN SELECT RAISE(ABORT, 'injected rating failure'); END`); err != nil {
		t.Fatalf("inject rating abort: %v", err)
	}
	return func() {
		if _, err := s.store.db.Exec(`DROP TRIGGER boom_rating`); err != nil {
			t.Fatalf("drop rating abort: %v", err)
		}
	}
}

// diskState snapshots the tables a completion unit touches.
type diskState struct {
	games   int
	ratings int
	series  SeriesRow
}

func readDiskState(t *testing.T, s *stack, seriesID int64) diskState {
	t.Helper()
	var d diskState
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM games WHERE series_id = ?`, seriesID).Scan(&d.games); err != nil {
		t.Fatalf("count games: %v", err)
	}
	if err := s.store.db.QueryRow(`SELECT COUNT(*) FROM rating_events`).Scan(&d.ratings); err != nil {
		t.Fatalf("count rating events: %v", err)
	}
	sr, err := s.store.SeriesByID(seriesID)
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	d.series = sr
	return d
}

// TestGameCompletionUnitAtomicOnFailedStatement pins the all-or-nothing law
// of one finished game: a statement failing mid-unit (here the first rating
// insert) must leave zero game rows, zero rating events, and the series row
// untouched, never a persisted game with a missing or half-applied rating
// pair.
func TestGameCompletionUnitAtomicOnFailedStatement(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)

	drop := injectRatingAbort(t, s)
	names := hostWinsRed
	for i := 0; i < len(names)-1; i++ {
		mover := alice.ID
		if i%2 == 1 {
			mover = bob.ID
		}
		if err := r.PlayMove(mover, mustCellT(t, names[i])); err != nil {
			t.Fatalf("move %d: %v", i+1, err)
		}
	}
	last := len(names) - 1
	if err := r.PlayMove(alice.ID, mustCellT(t, names[last])); err == nil {
		t.Fatal("winning move over an aborted rating insert: want the persistence error surfaced")
	}

	d := readDiskState(t, s, r.SeriesID())
	if d.games != 0 {
		t.Errorf("games after failed unit = %d, want 0 (the game row must not outlive its rating pair)", d.games)
	}
	if d.ratings != 0 {
		t.Errorf("rating events after failed unit = %d, want 0", d.ratings)
	}
	if d.series.State != SeriesStateOngoing || d.series.Winner != nil || d.series.FinishedAt != nil {
		t.Errorf("series row after failed unit = %+v, want untouched ongoing", d.series)
	}

	// The room advances on failure: the in-memory game stays the live truth.
	if _, ok := r.Info(); !ok {
		t.Error("room retired after a failed write, want it live per the advance-on-failure policy")
	}

	// With the fault gone the next completion persists whole again: game 2's
	// unit lands, so the disk holds exactly that game and its pair.
	drop()
	playScript(t, r, bob.ID, alice.ID, guestRedLosesToBlue)
	d = readDiskState(t, s, r.SeriesID())
	if d.games != 1 || d.ratings != 2 {
		t.Errorf("recovered disk = %d games %d ratings, want 1 and 2", d.games, d.ratings)
	}
	if d.series.State != SeriesStateFinished || d.series.Winner == nil || *d.series.Winner != alice.ID {
		t.Errorf("recovered series row = %+v, want finished with host winner", d.series)
	}
}

// TestForfeitCompletionUnitAtomicOnFailedStatement pins the same law on the
// forfeit sweep: an injected failure mid-sweep leaves no partial schedule.
func TestForfeitCompletionUnitAtomicOnFailedStatement(t *testing.T) {
	s := newStack(t)
	alice, bob, r := newPvPRoom(t, s)
	readyBoth(t, r, alice, bob)
	playScript(t, r, alice.ID, bob.ID, hostWinsRed)
	playScript(t, r, bob.ID, alice.ID, []string{"D4", "P16", "H8"})

	drop := injectRatingAbort(t, s)
	defer drop()
	if err := r.Forfeit(bob.ID); err == nil {
		t.Fatal("forfeit over an aborted rating insert: want the persistence error surfaced")
	}

	d := readDiskState(t, s, r.SeriesID())
	if d.games != 1 {
		t.Errorf("games after failed sweep = %d, want 1 (only the honestly played game, no synthetic rows)", d.games)
	}
	if d.ratings != 2 {
		t.Errorf("rating events after failed sweep = %d, want 2 (only the played game's pair)", d.ratings)
	}
	if d.series.State != SeriesStateOngoing || d.series.Winner != nil || d.series.FinishedAt != nil {
		t.Errorf("series row after failed sweep = %+v, want untouched ongoing", d.series)
	}
}

// TestForfeitOnOpenRoomPublishesTerminalEvent pins that every retirement
// path ends spectator streams. An open room (no guest, no series formed)
// still accepts spectators, so its forfeit must publish a terminal event:
// a stream that only ever sees keepalives pins a goroutine and a subscriber
// slot forever.
func TestForfeitOnOpenRoomPublishesTerminalEvent(t *testing.T) {
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()

	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit open room: %v", err)
	}
	if _, ok := r.Info(); ok {
		t.Error("open room still listed after forfeit, want retired")
	}
	select {
	case ev := <-sub.Events():
		// The series kind with the none payload: no series ever formed, so
		// there is no winner, but the stream still ends on a terminal shape
		// the live transport already closes on.
		if ev.Kind != EventKindSeries || ev.Payload != SideNone.String() {
			t.Errorf("terminal event = %s %s, want series none", ev.Kind, ev.Payload)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("open-room forfeit published no terminal event: the spectator stream would keepalive forever")
	}
}

func mustCellT(t *testing.T, name string) rules.Cell {
	t.Helper()
	cell, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return cell
}

func movesOf(t *testing.T, names []string) []rules.Move {
	t.Helper()
	out := make([]rules.Move, 0, len(names))
	for _, name := range names {
		out = append(out, rules.Move(mustCellT(t, name)))
	}
	return out
}

func TestMovesBlobRoundTrip(t *testing.T) {
	moves := movesOf(t, []string{"A1", "P16", "H8", "D3"})
	blob := EncodeMoves(nil, moves)
	if len(blob) != 2*len(moves) {
		t.Fatalf("blob = %d bytes for %d moves, want 2 per move", len(blob), len(moves))
	}
	got, err := DecodeMoves(blob)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != len(moves) {
		t.Fatalf("round trip = %d moves, want %d", len(got), len(moves))
	}
	for i := range moves {
		if got[i] != moves[i] {
			t.Errorf("move %d = %d, want %d", i, got[i], moves[i])
		}
	}
	if _, err := DecodeMoves([]byte{1}); err == nil {
		t.Error("odd blob decoded, want error")
	}
	if _, err := DecodeMoves([]byte{0x01, 0x01}); err == nil {
		t.Error("out-of-board cell decoded, want error")
	}
}

func placeStones(t *testing.T, names map[rules.Color][]string) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, color := range []rules.Color{rules.Red, rules.Blue} {
		for _, name := range names[color] {
			b.Side = color
			b.Make(mustCellT(t, name))
		}
	}
	return b
}

func TestWonByTagClassification(t *testing.T) {
	tests := []struct {
		name  string
		board map[rules.Color][]string
		want  string
	}{
		// D4-D7 with both D3 and D8 empty: two completions, an open four.
		{"open four", map[rules.Color][]string{rules.Red: {"D4", "D5", "D6", "D7"}}, WonByOpenFour},
		// D5-D8 with D9 blocked: only D4 completes, a blockable four.
		{"simple four", map[rules.Color][]string{rules.Red: {"D5", "D6", "D7", "D8"}, rules.Blue: {"D9"}}, WonByFour},
		// Two separately blocked fours: two win-in-1 cells, no open window.
		{"double four", map[rules.Color][]string{
			rules.Red:  {"D5", "D6", "D7", "D8", "L5", "L6", "L7", "L8"},
			rules.Blue: {"D9", "L9"},
		}, WonByDoubleFour},
		// A three beside a simple four: threes are not countable yet, the
		// lone completion decides.
		{"four beside three", map[rules.Color][]string{
			rules.Red:  {"D5", "D6", "D7", "D8", "C3", "C4", "C5"},
			rules.Blue: {"D9"},
		}, WonByFour},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			b := placeStones(t, tc.board)
			b.Side = rules.Blue // the probe must survive any side-to-move
			if got := WonByTag(b, rules.Red); got != tc.want {
				t.Errorf("WonByTag = %q, want %q", got, tc.want)
			}
			if b.Side != rules.Blue {
				t.Error("wonByTag leaked a side-to-move mutation")
			}
		})
	}
}
