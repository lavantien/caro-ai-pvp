package main

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
	if code := run([]string{"tourney", "--db", t.TempDir(), "smoke10"}); code != 1 {
		t.Errorf("unopenable store exit = %d, want 1", code)
	}
}

func TestRunTourneyParallelOverBudgetIsRefused(t *testing.T) {
	for _, args := range [][]string{
		{"tourney", "--parallel", "9", "--db", filepath.Join(t.TempDir(), "caro.sqlite"), "smoke10"},
		{"tourney", "smoke10", "--parallel", "9", "--db", filepath.Join(t.TempDir(), "caro.sqlite")},
	} {
		if code := run(args); code != 1 {
			t.Errorf("%v: exit = %d, want 1", args, code)
		}
	}
}

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
		config.SeriesBO3, config.TournamentStartRating, roster, "test", ""); err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if code := run([]string{"tourney", "--db", db, "smoke10"}); code != 1 {
		t.Errorf("driver against an ongoing run = exit %d, want 1", code)
	}
}

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
		config.SeriesBO3, config.TournamentStartRating, roster, "test", "")
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

func TestRunTourneyResumeUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want int
	}{
		{"resume without id", []string{"tourney", "resume"}, 2},
		{"resume two ids", []string{"tourney", "resume", "1", "2"}, 2},
		{"resume non-numeric id", []string{"tourney", "resume", "soon"}, 2},
		{"resume zero id", []string{"tourney", "resume", "0"}, 2},
		{"resume unknown flag", []string{"tourney", "resume", "-nope"}, 2},
	}
	for _, tc := range cases {
		if code := run(tc.args); code != tc.want {
			t.Errorf("%s: exit = %d, want %d", tc.name, code, tc.want)
		}
	}
}

func TestRunTourneyResumeRefusals(t *testing.T) {
	db := filepath.Join(t.TempDir(), "caro.sqlite")
	srv, err := server.Open(db)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	roster := []tourney.Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "medium-1", Tier: config.TierMedium.Name},
	}
	finished, err := tourney.NewStore(srv).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, roster, "test", "")
	if err != nil {
		t.Fatalf("plant finished run: %v", err)
	}
	if err := tourney.NewStore(srv).CloseStalledRun(context.Background(), finished.ID, 1); err != nil {
		t.Fatalf("finish the planted run: %v", err)
	}
	ongoing, err := tourney.NewStore(srv).CreateRun(context.Background(), 1,
		config.SeriesBO3, config.TournamentStartRating, roster, "test", "")
	if err != nil {
		t.Fatalf("plant ongoing run: %v", err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if code := run([]string{"tourney", "resume", "999", "--db", db}); code != 1 {
		t.Errorf("resume unknown run = exit %d, want 1", code)
	}
	if code := run([]string{"tourney", "resume", strconv.FormatInt(finished.ID, 10), "--db", db}); code != 1 {
		t.Errorf("resume finished run = exit %d, want 1", code)
	}
	if code := run([]string{"tourney", "resume", strconv.FormatInt(ongoing.ID, 10),
		"--parallel", "9", "--db", db}); code != 1 {
		t.Errorf("resume over budget = exit %d, want 1", code)
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

type failWriter struct{}

func (failWriter) Write([]byte) (int, error) { return 0, errors.New("sink is down") }

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
