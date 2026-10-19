package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

func cellT(t *testing.T, name string) rules.Move {
	t.Helper()
	c, err := rules.ParseCell(name)
	if err != nil {
		t.Fatalf("cell %s: %v", name, err)
	}
	return rules.Move(c)
}

func redWinMoves(t *testing.T) []rules.Move {
	t.Helper()
	return []rules.Move{cellT(t, "D4"), cellT(t, "P16"), cellT(t, "D5"), cellT(t, "H8"), cellT(t, "D6"), cellT(t, "P12"), cellT(t, "D7"), cellT(t, "P8"), cellT(t, "D8")}
}

func blueWinMoves(t *testing.T) []rules.Move {
	t.Helper()
	return []rules.Move{cellT(t, "P16"), cellT(t, "D4"), cellT(t, "H8"), cellT(t, "D5"), cellT(t, "P12"), cellT(t, "D6"), cellT(t, "P8"), cellT(t, "D7"), cellT(t, "N16"), cellT(t, "D8")}
}

func buildFixtureDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.db")
	srv, err := server.Open(path)
	if err != nil {
		t.Fatalf("open server store: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	ts := tourney.NewStore(srv)
	ctx := context.Background()
	roster := []tourney.Participant{
		{Slot: 0, Name: "master-a", Tier: "master"},
		{Slot: 1, Name: "master-b", Tier: "master"},
		{Slot: 2, Name: "easy-a", Tier: "easy"},
	}
	run, err := ts.CreateRun(ctx, 3, config.SeriesBO3, 1500, roster, "evalfit-fixture")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if err := ts.ClaimDrive(ctx, run.ID, time.Now().Unix()); err != nil {
		t.Fatalf("claim drive: %v", err)
	}
	series, err := ts.Schedule(ctx, run.ID)
	if err != nil {
		t.Fatalf("schedule: %v", err)
	}
	wantSeries := len(roster) * (len(roster) - 1)
	if len(series) != wantSeries {
		t.Fatalf("series = %d, want the n*(n-1) round robin of run %d", len(series), run.ID)
	}
	appended := 0
	for _, s := range series {
		seats := [][2]int{{s.RedFirstSlot, s.BlueFirstSlot}, {s.BlueFirstSlot, s.RedFirstSlot}}
		for i, seatPair := range seats {
			g := tourney.Game{
				RunID:       run.ID,
				SeriesID:    s.ID,
				IdxInSeries: i,
				RedSlot:     seatPair[0],
				BlueSlot:    seatPair[1],
				FullTurns:   9,
			}
			if i == 0 {
				g.Outcome, g.Moves = server.RedWins, redWinMoves(t)
			} else {
				g.Outcome, g.Moves = server.BlueWins, blueWinMoves(t)
			}
			g.FullTurns = len(g.Moves)
			if _, err := ts.AppendGame(ctx, g); err != nil {
				t.Fatalf("append game %d of series %d: %v", i+1, s.ID, err)
			}
			appended++
		}
	}
	if appended == 0 {
		t.Fatal("fixture appended no games")
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close server store: %v", err)
	}
	return path
}

func TestLoadDBGamesFiltersTiersAndDecodesMoves(t *testing.T) {
	path := buildFixtureDB(t)
	masterOnly := map[string]bool{"master": true}
	load, err := loadDBGames(path, masterOnly)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(load.Series) == 0 {
		t.Fatal("no master-only series loaded")
	}
	games := 0
	for _, s := range load.Series {
		for _, g := range s.Games {
			games++
			if g.Red.Tier != "master" || g.Blue.Tier != "master" {
				t.Fatalf("game seats %s/%s leaked a non-master tier", g.Red.Tier, g.Blue.Tier)
			}
			if g.Moves[0].Color != rules.Red {
				t.Fatalf("first move color = %v, want red", g.Moves[0].Color)
			}
			if g.Moves[1].Color != rules.Blue {
				t.Fatalf("second move color = %v, want blue", g.Moves[1].Color)
			}
			if g.Moves[0].Cell == g.Moves[1].Cell {
				t.Fatal("decoded cells collapsed")
			}
		}
	}
	if games < 2 {
		t.Fatalf("master-only games = %d, want at least the 2 of one pairing", games)
	}
	all, err := loadDBGames(path, nil)
	if err != nil {
		t.Fatalf("load unfiltered: %v", err)
	}
	allGames := 0
	for _, s := range all.Series {
		allGames += len(s.Games)
	}
	if allGames <= games {
		t.Fatalf("unfiltered games %d must exceed the master-only %d", allGames, games)
	}
}

func TestLoadDBGamesSeriesKeysAreStable(t *testing.T) {
	path := buildFixtureDB(t)
	load, err := loadDBGames(path, nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, s := range load.Series {
		if s.Key == "" {
			t.Fatal("series key empty")
		}
	}
	if load.Series[0].Key == load.Series[len(load.Series)-1].Key && len(load.Series) > 1 {
		t.Fatal("distinct series share a key")
	}
}

func TestDecodeMovesRejectsCorruptBlobs(t *testing.T) {
	if _, err := decodeMoves([]byte{1, 2, 3}, 7); err == nil {
		t.Fatal("odd-length blob accepted")
	}
	if _, err := decodeMoves([]byte{0x00, 0x01}, 7); err == nil {
		t.Fatal("out-of-board cell accepted")
	}
	moves, err := decodeMoves([]byte{0x77, 0x00, 0x57, 0x00}, 7)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(moves) != 2 || moves[0].Cell != rules.Cell(119) || moves[1].Cell != rules.Cell(87) {
		t.Fatalf("moves = %v, want cells 119 then 87", moves)
	}
}

func TestOutcomeOfColumnRejectsJunk(t *testing.T) {
	for _, oc := range []string{"red", "blue", "draw"} {
		if _, err := outcomeOfColumn(oc, 1); err != nil {
			t.Fatalf("outcome %q: %v", oc, err)
		}
	}
	if _, err := outcomeOfColumn("walkover", 1); err == nil {
		t.Fatal("junk outcome accepted")
	}
}
