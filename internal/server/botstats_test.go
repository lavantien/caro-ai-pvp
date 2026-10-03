package server

// The Implication 1.5 recording law of player-facing bot matches: every
// series and game persists with one game_stats row per bot move (the
// rendered M-line, byte-equal to the published hub payload), the W-L-D and
// level record updates off those rows, and no rating ever moves. Tournament
// bot-vs-bot rooms stay outside the player tables entirely.

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// botLoseScripts is the per-game scripted-bot sweep of a human-won bo3: game
// 1 has the bot on blue scattering while the human closes the D file (the
// hostWinsRed pairing), game 2 rotates red to the losing bot (the
// guestRedLosesToBlue pairing with the seats swapped).
var botLoseScripts = [][]string{
	{"P16", "P12", "P8", "N16", "M4"},
	{"P16", "H8", "P12", "P8", "N16"},
}

// humanWinSides is the human's half of each game of botLoseScripts: red's
// stones of game 1, blue's stones of game 2.
var humanWinSides = [][]string{
	{"D4", "H8", "D5", "D6", "D7", "D3"},
	{"D4", "D5", "D6", "D7", "D3"},
}

// injectPerGameBot swaps the room's engine factory for scripted bots, one
// fresh script per game in creation order. Runs under the room lock like
// every engine factory swap.
func injectPerGameBot(t *testing.T, r *Room, scripts [][]string) {
	t.Helper()
	parsed := make([][]rules.Move, 0, len(scripts))
	for _, names := range scripts {
		parsed = append(parsed, movesOf(t, names))
	}
	next := 0
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher {
		b := &scriptedBot{script: append([]rules.Move(nil), parsed[next%len(parsed)]...), stats: fakeStats()}
		next++
		return b
	}
	r.mu.Unlock()
}

// driveHumanWonBotSweep plays the scripted bo3 where the human takes both
// games, waiting for the worker's reply before each human stone, and returns
// the drained hub stream.
func driveHumanWonBotSweep(t *testing.T, r *Room, alice User, sub *Subscription) []string {
	t.Helper()
	for game, side := range humanWinSides {
		for _, name := range side {
			waitFor(t, func() bool { return humanTurn(r, alice.ID) })
			if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
				t.Fatalf("game %d human move %s: %v", game+1, name, err)
			}
		}
	}
	waitFor(t, func() bool {
		_, ok := r.Info()
		return !ok
	})
	return drainEvents(sub)
}

// gameStatRows reads one game's persisted stat lines in move order.
func gameStatRows(t *testing.T, s *Store, gameID int64) []GameStat {
	t.Helper()
	rows, err := s.db.Query(
		`SELECT move_no, line FROM game_stats WHERE game_id = ? ORDER BY move_no`, gameID)
	if err != nil {
		t.Fatalf("list game stats %d: %v", gameID, err)
	}
	defer func() { _ = rows.Close() }()
	var out []GameStat
	for rows.Next() {
		var g GameStat
		if err := rows.Scan(&g.MoveNo, &g.Line); err != nil {
			t.Fatalf("scan game stat %d: %v", gameID, err)
		}
		out = append(out, g)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list game stats %d: %v", gameID, err)
	}
	return out
}

// onlyGameID reads a series' single game id, the store-level tests' one-row
// shape.
func onlyGameID(t *testing.T, s *Store, seriesID int64) int64 {
	t.Helper()
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM games WHERE series_id = ?`, seriesID).Scan(&id); err != nil {
		t.Fatalf("read game id of series %d: %v", seriesID, err)
	}
	return id
}

// mLinesOf splits a drained hub stream into the per-game published M-line
// payloads, in order, dropping everything else. A forfeited live game ends
// with no gameend event, so the trailing group flushes too.
func mLinesOf(events []string) [][]string {
	var out [][]string
	var cur []string
	for _, e := range events {
		switch {
		case strings.HasPrefix(e, EventKindMLine+" "):
			cur = append(cur, strings.TrimPrefix(e, EventKindMLine+" "))
		case e == EventKindGameEnd+" "+OutcomeRed || e == EventKindGameEnd+" "+OutcomeBlue || e == EventKindGameEnd+" "+OutcomeDraw:
			out = append(out, cur)
			cur = nil
		}
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}
	return out
}

// TestBotSeriesRecordsGamesStatsAndHistory drives a human-won bo3 against a
// scripted easy bot and pins the whole recording law: the series row and
// both games persist, each game carries one stat row per bot move whose line
// is byte-equal to the published M-line payload at the matching move number,
// the W-L-D and level record updates off the rows, history renders the
// opponent as "AI <tier>", and not one rating event lands.
func TestBotSeriesRecordsGamesStatsAndHistory(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	botID, err := s.store.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	injectPerGameBot(t, r, botLoseScripts)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	events := driveHumanWonBotSweep(t, r, alice, sub)

	// The pairing row finished with the human winner, both seats recorded.
	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != alice.ID ||
		sr.RedUser != alice.ID || sr.BlueUser != botID {
		t.Errorf("series row = %+v, want finished host over the bot seat %d", sr, botID)
	}

	// One stat row per bot move, byte-equal to the published payload, at the
	// bot's own move numbers (even plies when the bot holds blue, odd when
	// red, both counted from the game's first stone).
	published := mLinesOf(events)
	if len(published) != 2 {
		t.Fatalf("published mline games = %d, want 2: %v", len(published), events)
	}
	ids := gameIDs(t, s)
	if len(ids) != 2 {
		t.Fatalf("persisted games = %d, want 2", len(ids))
	}
	wantNos := [][]int{{2, 4, 6, 8, 10}, {1, 3, 5, 7, 9}}
	for gi, id := range ids {
		rows := gameStatRows(t, s.store, id)
		if len(rows) != len(published[gi]) {
			t.Fatalf("game %d stat rows = %d, want %d (one per bot move)", gi+1, len(rows), len(published[gi]))
		}
		for i, row := range rows {
			if row.Line != published[gi][i] {
				t.Errorf("game %d stat %d = %q, want the published payload %q", gi+1, i, row.Line, published[gi][i])
			}
			if row.MoveNo != wantNos[gi][i] {
				t.Errorf("game %d stat %d move_no = %d, want %d", gi+1, i, row.MoveNo, wantNos[gi][i])
			}
		}
	}

	// Ratings never move: no event, the chain tail stays at the start value.
	if hist, err := s.store.RatingHistoryByUser(alice.ID); err != nil || len(hist) != 0 {
		t.Errorf("rating events after the bot series = %d err %v, want none", len(hist), err)
	}
	if got, err := currentRating(s.store, alice.ID); err != nil || got != config.RatingStart {
		t.Errorf("rating after the bot series = %d err %v, want the unmoved %d", got, err, config.RatingStart)
	}

	// The W-L-D and level record updates off the recorded rows: 2 wins, one
	// series taken.
	st, err := s.store.UserStats(alice.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st != (UserStats{Wins: 2, Losses: 0, Draws: 0, SeriesWon: 1}) {
		t.Errorf("stats after the bot series = %+v, want 2-0-0 with 1 series won", st)
	}

	// History lists both games with the tier as the opponent.
	rows, err := s.store.MatchHistory(alice.ID)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("history rows = %d, want 2", len(rows))
	}
	g2, g1 := rows[0], rows[1]
	if g1.Red != "alice" || g1.Blue != config.BotAccountName(0) || g1.RedWins != 1 || g1.BlueWins != 0 {
		t.Errorf("game 1 row = %s vs %s %d-%d, want alice over %s at 1-0",
			g1.Red, g1.Blue, g1.RedWins, g1.BlueWins, config.BotAccountName(0))
	}
	if g2.Red != config.BotAccountName(0) || g2.Blue != "alice" || g2.RedWins != 0 || g2.BlueWins != 2 {
		t.Errorf("game 2 row = %s vs %s %d-%d, want %s over alice at 0-2",
			g2.Red, g2.Blue, g2.RedWins, g2.BlueWins, config.BotAccountName(0))
	}
}

// TestForfeitVsBotBooksSweptLossesWithoutRating pins the forfeit billing of
// a bot series: every remaining game lands as the quitter's loss in the
// record, the live game keeps its partial moves and the bot log emitted so
// far, the series finishes with the bot seat as winner, and no rating moves.
func TestForfeitVsBotBooksSweptLossesWithoutRating(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	botID, err := s.store.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	injectPerGameBot(t, r, botLoseScripts)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}

	// Two full rounds land (the human waits for the bot's second reply so
	// both M-lines are in), then the human walks mid game 1.
	for _, name := range []string{"D4", "H8"} {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("human move %s: %v", name, err)
		}
	}
	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit vs bot: %v", err)
	}

	sr, err := s.store.SeriesByID(r.SeriesID())
	if err != nil {
		t.Fatalf("series row: %v", err)
	}
	if sr.State != SeriesStateFinished || sr.Winner == nil || *sr.Winner != botID {
		t.Errorf("series row = %+v, want finished with the bot seat %d as winner", sr, botID)
	}
	ids := gameIDs(t, s)
	if len(ids) != 3 {
		t.Fatalf("persisted games = %d, want the swept bo3", len(ids))
	}
	live := gameStatRows(t, s.store, ids[0])
	published := mLinesOf(drainEvents(sub))
	if len(published) != 1 || len(published[0]) != 2 {
		t.Fatalf("published mlines = %v, want the 2 live lines", published)
	}
	if len(live) != len(published[0]) {
		t.Fatalf("live game stat rows = %d, want the %d emitted bot lines", len(live), len(published[0]))
	}
	for i, row := range live {
		if row.Line != published[0][i] || row.MoveNo != 2*(i+1) {
			t.Errorf("live stat %d = %d %q, want %d and the published payload", i, row.MoveNo, row.Line, 2*(i+1))
		}
	}
	for _, id := range ids[1:] {
		if rows := gameStatRows(t, s.store, id); len(rows) != 0 {
			t.Errorf("never-started game %d stat rows = %d, want 0", id, len(rows))
		}
	}

	st, err := s.store.UserStats(alice.ID)
	if err != nil {
		t.Fatalf("stats: %v", err)
	}
	if st != (UserStats{Wins: 0, Losses: 3, Draws: 0, SeriesWon: 0}) {
		t.Errorf("stats after the forfeit = %+v, want 0-3-0 with no series won", st)
	}
	if hist, err := s.store.RatingHistoryByUser(alice.ID); err != nil || len(hist) != 0 {
		t.Errorf("rating events after the forfeit = %d err %v, want none", len(hist), err)
	}
}

// TestApplyCompletionBotSeatSkipsRatingPair is the store-level pin of the
// ledger law: a decisive game against a reserved bot seat lands whole, stat
// lines included, without any rating event.
func TestApplyCompletionBotSeatSkipsRatingPair(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed bot series: %v", err)
	}

	want := []GameStat{{MoveNo: 2, Line: "M2, Blue, P16, d=9"}}
	if err := s.ApplyCompletion(context.Background(), Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeBlue, Moves: []byte{1, 0}, FullTurns: 1, StatLines: want,
		}},
	}); err != nil {
		t.Fatalf("apply bot completion: %v", err)
	}
	if games, ratings := completionFootprint(t, s, sr.ID); games != 1 || ratings != 0 {
		t.Errorf("bot completion = %d games %d ratings, want 1 and 0", games, ratings)
	}
	if rows := gameStatRows(t, s, onlyGameID(t, s, sr.ID)); len(rows) != 1 || rows[0] != want[0] {
		t.Errorf("bot game stats = %+v, want the one carried line %+v", rows, want[0])
	}
	if got, err := currentRating(s, alice.ID); err != nil || got != config.RatingStart {
		t.Errorf("rating after the bot game = %d err %v, want the unmoved %d", got, err, config.RatingStart)
	}
}

// TestApplyCompletionStatFailureRollsBackTheUnit pins the stat record's
// atomicity slice: a game_stats insert failing mid-unit leaves no game row,
// no stat row, and the series untouched, exactly like a failed rating insert
// already does.
func TestApplyCompletionStatFailureRollsBackTheUnit(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	alice := seedUser(t, s, "alice")
	botID, err := s.BotAccountID(config.TierEasy)
	if err != nil {
		t.Fatalf("resolve bot seat: %v", err)
	}
	sr, err := s.CreateSeries(context.Background(), 0, config.SeriesBO3, alice.ID, botID)
	if err != nil {
		t.Fatalf("seed bot series: %v", err)
	}
	if _, err := s.db.Exec(
		`CREATE TRIGGER boom_stat BEFORE INSERT ON game_stats
		BEGIN SELECT RAISE(ABORT, 'injected stat failure'); END`); err != nil {
		t.Fatalf("inject stat abort: %v", err)
	}

	unit := Completion{
		Games: []Game{{
			SeriesID: sr.ID, IdxInSeries: 0, RedUser: alice.ID, BlueUser: botID,
			Outcome: OutcomeBlue, Moves: []byte{1, 0}, FullTurns: 1,
			StatLines: []GameStat{{MoveNo: 2, Line: "M2, Blue, P16"}},
		}},
		Finish: &SeriesFinish{SeriesID: sr.ID, Winner: &botID, FinishedAt: 1_700_000_000},
	}
	err = s.ApplyCompletion(context.Background(), unit)
	if err == nil || !strings.Contains(err.Error(), "injected stat failure") {
		t.Fatalf("apply over the aborted stat insert = %v, want the injected failure", err)
	}
	if games, ratings := completionFootprint(t, s, sr.ID); games != 0 || ratings != 0 {
		t.Errorf("disk after the rejected unit = %d games %d ratings, want zero of both", games, ratings)
	}
	if got, err := s.SeriesByID(sr.ID); err != nil || got.State != SeriesStateOngoing {
		t.Errorf("series after the rejected unit = %+v err %v, want untouched ongoing", got, err)
	}

	// With the fault gone the same unit lands whole.
	if _, err := s.db.Exec(`DROP TRIGGER boom_stat`); err != nil {
		t.Fatalf("drop stat abort: %v", err)
	}
	if err := s.ApplyCompletion(context.Background(), unit); err != nil {
		t.Fatalf("apply after the fault dropped: %v", err)
	}
	if games, ratings := completionFootprint(t, s, sr.ID); games != 1 || ratings != 0 {
		t.Errorf("recovered disk = %d games %d ratings, want 1 and 0", games, ratings)
	}
	if got, err := s.SeriesByID(sr.ID); err != nil || got.State != SeriesStateFinished {
		t.Errorf("recovered series = %+v err %v, want finished", got, err)
	}
}

// TestPlaybackBotGameOpensForTheHumanSeat pins the playback gate and render
// of a recorded bot game: the human participant opens it and sees "AI <tier>"
// on the bot seat, strangers and anonymous visitors get the 404.
func TestPlaybackBotGameOpensForTheHumanSeat(t *testing.T) {
	s := newStack(t)
	srv := newPageServer(t, s)
	alice, r := botRoom(t, s, config.TierEasy)
	injectPerGameBot(t, r, botLoseScripts)
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer sub.Unsubscribe()
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	driveHumanWonBotSweep(t, r, alice, sub)

	ids := gameIDs(t, s)
	if len(ids) != 2 {
		t.Fatalf("persisted games = %d, want 2", len(ids))
	}
	ta := mintSession(t, s.store, alice)
	status, body := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), ta)
	if status != http.StatusOK {
		t.Fatalf("human playback status = %d, want 200 (body %s)", status, body)
	}
	// Game 1 seated the bot on blue: the page names the seats through the
	// same users rows history reads.
	if !strings.Contains(body, "red alice vs blue "+config.BotAccountName(0)) {
		t.Errorf("playback body misses the bot seat name %q (body %s)", config.BotAccountName(0), body)
	}
	carol := seedUser(t, s.store, "carol")
	for name, tok := range map[string]string{"stranger": mintSession(t, s.store, carol), "anonymous": ""} {
		if status, _ := getRoomPage(t, srv, "/rooms/history/"+fmt.Sprint(ids[0]), tok); status != http.StatusNotFound {
			t.Errorf("%s playback status = %d, want 404", name, status)
		}
	}
}

// TestBotAccountResolution pins the seat resolver: every config tier
// resolves to a distinct reserved row, an unknown tier value fails loudly,
// and a real account that squatted a seat's name never resolves as the seat.
func TestBotAccountResolution(t *testing.T) {
	s := mustOpen(t, dbPath(t))
	defer func() { _ = s.Close() }()
	seen := map[int64]bool{}
	for i := range config.Tiers {
		id, err := s.BotAccountID(config.Tiers[i])
		if err != nil {
			t.Fatalf("resolve tier %s: %v", config.Tiers[i].Name, err)
		}
		if id <= 0 || seen[id] {
			t.Errorf("tier %s seat id = %d dup %t, want a distinct positive id", config.Tiers[i].Name, id, seen[id])
		}
		seen[id] = true
	}
	if _, err := s.BotAccountID(config.Tier{Name: "bogus"}); !errors.Is(err, ErrUnknownTier) {
		t.Errorf("resolve unknown tier = %v, want ErrUnknownTier", err)
	}

	// A pre-v4 squatter on the seat's name keeps the seed's insert out, so
	// the resolver fails loudly instead of seating the human's row.
	path := dbPath(t)
	squatted := mustOpen(t, path)
	if _, err := squatted.db.Exec(`DELETE FROM schema_version WHERE version = 4`); err != nil {
		t.Fatalf("roll ledger back to v3: %v", err)
	}
	if _, err := squatted.db.Exec(`DELETE FROM users WHERE length(salt) = 0`); err != nil {
		t.Fatalf("drop the seeded seats: %v", err)
	}
	if _, err := squatted.db.Exec(
		"INSERT INTO users (username, argon2_time, argon2_memory, argon2_parallelism, salt, hash) VALUES (?, ?, ?, ?, ?, ?)",
		config.BotAccountName(0), config.Argon2Time, config.Argon2MemoryKiB, config.Argon2Parallelism, []byte("s"), []byte("h"),
	); err != nil {
		t.Fatalf("seed the squatter: %v", err)
	}
	if err := squatted.Close(); err != nil {
		t.Fatalf("close squatted db: %v", err)
	}
	again := mustOpen(t, path)
	defer func() { _ = again.Close() }()
	if _, err := again.BotAccountID(config.TierEasy); !errors.Is(err, ErrNotFound) {
		t.Errorf("resolve over a squatted name = %v, want ErrNotFound", err)
	}
	if _, err := again.BotAccountID(config.TierHard); err != nil {
		t.Errorf("resolve the unsquatted hard seat = %v, want nil", err)
	}
}
