package main

import (
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func redWinsGame(t *testing.T) parsedGame {
	t.Helper()
	names := []string{"H8", "A1", "H9", "B1", "H10", "C1", "H11", "D1", "H12"}
	moves := make([]gameMove, 0, len(names))
	for i, name := range names {
		cell, err := rules.ParseCell(name)
		if err != nil {
			t.Fatalf("cell %s: %v", name, err)
		}
		color := rules.Red
		if i%2 == 1 {
			color = rules.Blue
		}
		moves = append(moves, gameMove{Color: color, Cell: cell})
	}
	return parsedGame{Red: seat{Name: "r"}, Blue: seat{Name: "b"}, Outcome: outcomeRedWin, Moves: moves}
}

func TestReconstructRedWinGame(t *testing.T) {
	g := redWinsGame(t)
	samples, err := reconstructGame(g, "s1")
	if err != nil {
		t.Fatalf("reconstruct: %v", err)
	}
	if len(samples) != len(g.Moves) {
		t.Fatalf("samples = %d, want %d", len(samples), len(g.Moves))
	}
	for i, s := range samples {
		wantY := 0.0
		if g.Moves[i].Color == rules.Red {
			wantY = 1.0
		}
		if s.Y != wantY {
			t.Fatalf("sample %d y = %v, want %v", i, s.Y, wantY)
		}
		if s.Series != "s1" {
			t.Fatalf("sample %d series = %q, want s1", i, s.Series)
		}
	}
	for c, v := range samples[0].F {
		if v != 0 {
			t.Fatalf("empty board class %d count = %d, want 0", c+1, v)
		}
	}
	for c, v := range samples[2].F {
		if v != 0 {
			t.Fatalf("lone scattered stones class %d count = %d, want 0", c+1, v)
		}
	}
	if samples[3].F[0] >= 0 {
		t.Fatalf("blue mover facing a red open two: class 1 diff = %d, want negative", samples[3].F[0])
	}
	if samples[8].F[4] <= 0 {
		t.Fatalf("red mover with an open four: class 5 diff = %d, want positive", samples[8].F[4])
	}
}

func TestReconstructIntegrityRejections(t *testing.T) {
	base := redWinsGame(t)
	corruptOutcome := base
	corruptOutcome.Outcome = outcomeBlueWin
	fakeDraw := base
	fakeDraw.Outcome = outcomeDraw
	colorFlip := base
	colorFlip.Moves = append([]gameMove{}, base.Moves...)
	colorFlip.Moves[0].Color = rules.Blue
	dupCell := base
	dupCell.Moves = append([]gameMove{}, base.Moves...)
	dupCell.Moves[8].Cell = dupCell.Moves[6].Cell
	cases := []struct {
		name string
		game parsedGame
		want string
	}{
		{"wrong winner color", corruptOutcome, "winning line"},
		{"drawn but board not full", fakeDraw, "not full"},
		{"first move not red", colorFlip, "alternation"},
		{"move on occupied cell", dupCell, "occupied"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := reconstructGame(tc.game, "s1")
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want substring %q", err, tc.want)
			}
		})
	}
}
