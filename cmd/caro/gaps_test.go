package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

func dirAsStore(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "asdir")
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunTourneyResumeRejectsUnopenableStore(t *testing.T) {
	if code := run([]string{"tourney", "resume", "1", "--db", dirAsStore(t)}); code != 1 {
		t.Fatalf("resume against an unopenable store = %d, want 1", code)
	}
}

func TestRunMigrateArgumentAndStoreErrors(t *testing.T) {
	if code := run([]string{"migrate", "-nope"}); code != 2 {
		t.Fatalf("migrate with a bad flag = %d, want 2", code)
	}
	if code := run([]string{"migrate", "--db", dirAsStore(t)}); code != 1 {
		t.Fatalf("migrate against an unopenable store = %d, want 1", code)
	}
}

func TestLeaderNameMatchesBoardLeader(t *testing.T) {
	roster := []tourney.Participant{{Slot: 0, Name: "red"}, {Slot: 1, Name: "blue"}}
	if got := leaderName(tourney.Detail{Roster: roster}); got != "" {
		t.Fatalf("leaderName without a board = %q, want empty", got)
	}
	odd := tourney.Detail{Roster: roster, Board: []tourney.Standings{{Slot: 7, Rating: 10}}}
	if got := leaderName(odd); got != "" {
		t.Fatalf("leaderName with an off-roster slot = %q, want empty", got)
	}
	led := tourney.Detail{Roster: roster, Board: []tourney.Standings{{Slot: 1, Rating: 12}, {Slot: 0, Rating: 8}}}
	if got := leaderName(led); got != "blue" {
		t.Fatalf("leaderName = %q, want blue", got)
	}
}
