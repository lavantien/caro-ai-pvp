package main

// The tourney subcommand's argument surface and result rendering: the
// drivers never run here (real engines belong to the conductor tests), so
// these cover the flag validation, the driver table, and printRunResult.

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

func TestRunTourneyArgumentValidation(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"no driver", []string{"tourney"}, 2},
		{"unknown driver", []string{"tourney", "bogus"}, 2},
		{"two drivers", []string{"tourney", "smoke10", "full"}, 2},
		{"bad flag", []string{"tourney", "-nope", "smoke10"}, 2},
	}
	for _, tc := range cases {
		if code := run(tc.args); code != tc.want {
			t.Errorf("%s: exit = %d, want %d", tc.name, code, tc.want)
		}
	}
}

func TestRunTourneyRejectsUnopenableStore(t *testing.T) {
	// A directory is not a database file: Open fails before any engine
	// starts, proving the wiring reaches the store.
	if code := run([]string{"tourney", "--db", t.TempDir(), "smoke10"}); code != 1 {
		t.Errorf("unopenable store exit = %d, want 1", code)
	}
}

func TestRunTourneyParallelOverBudgetIsRefused(t *testing.T) {
	// The core-budget refusal fires before any room exists, so this runs no
	// engine and still proves the conductor wiring end to end. Both driver
	// positions (ahead of and behind the flags) must reach it.
	for _, args := range [][]string{
		{"tourney", "--parallel", "9", "--db", filepath.Join(t.TempDir(), "caro.sqlite"), "smoke10"},
		{"tourney", "smoke10", "--parallel", "9", "--db", filepath.Join(t.TempDir(), "caro.sqlite")},
	} {
		if code := run(args); code != 1 {
			t.Errorf("%v: exit = %d, want 1", args, code)
		}
	}
}

// TestRunTourneyRefusesWhileRunOngoing pins the run gate's CLI arm: with an
// ongoing row in the db (a previous process's crash), the driver refuses
// before any engine starts.
func TestRunTourneyRefusesWhileRunOngoing(t *testing.T) {
	db := filepath.Join(t.TempDir(), "caro.sqlite")
	srv, err := server.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	roster := []tourney.Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
	if _, err := tourney.NewStore(srv).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, roster); err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if code := run([]string{"tourney", "--db", db, "smoke10"}); code != 1 {
		t.Errorf("driver against an ongoing run = exit %d, want 1", code)
	}
}

// TestRunTourneyCloseUsage pins the recovery arm's argument surface: the id
// is exactly one positive integer, whichever side of the flags it sits on.
func TestRunTourneyCloseUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"close without id", []string{"tourney", "close"}, 2},
		{"close two ids", []string{"tourney", "close", "1", "2"}, 2},
		{"close non-numeric id", []string{"tourney", "close", "soon"}, 2},
		{"close zero id", []string{"tourney", "close", "0"}, 2},
		{"close unknown flag", []string{"tourney", "close", "-nope"}, 2},
	}
	for _, tc := range cases {
		if code := run(tc.args); code != tc.want {
			t.Errorf("%s: exit = %d, want %d", tc.name, code, tc.want)
		}
	}
}

// TestRunTourneyCloseStalledRun drives the recovery arm against a planted
// ongoing row, the state a killed run leaves behind: the first close exits 0,
// a second refuses on the already-finished row, an unknown id refuses, and an
// unopenable store exits 1 like the drivers.
func TestRunTourneyCloseStalledRun(t *testing.T) {
	db := filepath.Join(t.TempDir(), "caro.sqlite")
	srv, err := server.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	roster := []tourney.Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
	planted, err := tourney.NewStore(srv).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, roster)
	if err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}
	id := strconv.FormatInt(planted.ID, 10)

	if code := run([]string{"tourney", "close", id, "--db", db}); code != 0 {
		t.Errorf("close stalled run = exit %d, want 0", code)
	}
	if code := run([]string{"tourney", "--db", db, "close", id}); code != 1 {
		t.Errorf("second close on finished row = exit %d, want 1", code)
	}
	if code := run([]string{"tourney", "close", "999", "--db", db}); code != 1 {
		t.Errorf("close unknown run = exit %d, want 1", code)
	}
	if code := run([]string{"tourney", "close", "1", "--db", t.TempDir()}); code != 1 {
		t.Errorf("close on unopenable store = exit %d, want 1", code)
	}
}

func TestPrintRunResult(t *testing.T) {
	roster := []tourney.Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
	winner := 0
	res := tourney.RunResult{
		Series: []tourney.SeriesResult{
			{PairingSlot: 0, RedFirst: roster[0], BlueFirst: roster[1],
				WinnerSlot: &winner, RedFirstWins: 2, BlueFirstWins: 1},
			{PairingSlot: 1, RedFirst: roster[1], BlueFirst: roster[0],
				WinnerSlot: nil, RedFirstWins: 1, BlueFirstWins: 1},
		},
		Board: []tourney.Standings{
			{Slot: 0, Rating: 1030, Wins: 2, Losses: 1, SeriesWon: 1, GamesPlayed: 3},
			{Slot: 1, Rating: 970, Wins: 1, Losses: 2, GamesPlayed: 3},
		},
	}
	var buf bytes.Buffer
	if err := printRunResult(&buf, roster, res); err != nil {
		t.Fatalf("print: %v", err)
	}
	out := buf.String()
	// tabwriter pads the table cells, so assert on the tab-free text.
	for _, want := range []string{
		"series  0: easy-1 (red-first) 2 - 1 medium-1, winner easy-1",
		"series  1: medium-1 (red-first) 1 - 1 easy-1, winner drawn",
		"participant", "rating", "easy-1", "1030", "medium-1", "970",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}

// failWriter rejects every write: the sink-down fault behind the renderer's
// error wraps.
type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("sink is down") }

// TestPrintRunResultSurfacesWriteFaults pins the renderer's error arms: a
// failing series-line write returns at once, and a leaderboard that only
// fails when the tabwriter flushes surfaces the flush wrap.
func TestPrintRunResultSurfacesWriteFaults(t *testing.T) {
	roster := []tourney.Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
	withLine := tourney.RunResult{Series: []tourney.SeriesResult{
		{PairingSlot: 0, RedFirst: roster[0], BlueFirst: roster[1]},
	}}
	if err := printRunResult(failWriter{}, roster, withLine); err == nil ||
		!strings.Contains(err.Error(), "print series line") {
		t.Errorf("series-line write fault = %v, want the print wrap", err)
	}

	boardOnly := tourney.RunResult{Board: []tourney.Standings{{Slot: 0, Rating: 1000}}}
	if err := printRunResult(failWriter{}, roster, boardOnly); err == nil ||
		!strings.Contains(err.Error(), "flush leaderboard") {
		t.Errorf("flush fault = %v, want the flush wrap", err)
	}
}
