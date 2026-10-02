package main

// The tourney subcommand's argument surface and result rendering: the
// drivers never run here (real engines belong to the conductor tests), so
// these cover the flag validation, the driver table, and printRunResult.

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
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
	// engine and still proves the conductor wiring end to end.
	if code := run([]string{"tourney", "--parallel", "9", "--db",
		filepath.Join(t.TempDir(), "caro.sqlite"), "smoke10"}); code != 1 {
		t.Errorf("over-budget parallelism exit = %d, want 1", code)
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
		"participant", "rating", "easy-1", "1030", "medium-1", "970",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output misses %q:\n%s", want, out)
		}
	}
}
