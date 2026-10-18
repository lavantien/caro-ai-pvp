package main

import (
	"os"
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

const fixtureOrigin = "synthetic series fixture modeled on the logs/tourny run26 series log format, hand written 2026-10-19, no game captured"

const fixturePath = "testdata/series-basic.txt"

func loadFixtureLines(t *testing.T) []string {
	t.Helper()
	data, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

func TestParseSeriesFixture(t *testing.T) {
	rec, err := parseSeriesText("fixture", loadFixtureLines(t))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rec.Run != 26 || rec.Series != 491 {
		t.Fatalf("header run/series = %d/%d, want 26/491", rec.Run, rec.Series)
	}
	if rec.Red.Name != "master-1" || rec.Red.Tier != "master" {
		t.Fatalf("red seat = %+v, want master-1 (master)", rec.Red)
	}
	if rec.Blue.Name != "grandmaster-1" || rec.Blue.Tier != "grandmaster" {
		t.Fatalf("blue seat = %+v, want grandmaster-1 (grandmaster)", rec.Blue)
	}
	if rec.TC != "3+2 bo3" {
		t.Fatalf("tc = %q, want %q", rec.TC, "3+2 bo3")
	}
	if len(rec.Games) != 3 {
		t.Fatalf("games = %d, want 3", len(rec.Games))
	}
	wantOutcome := []gameOutcome{outcomeRedWin, outcomeBlueWin, outcomeDraw}
	for i, want := range wantOutcome {
		if rec.Games[i].Outcome != want {
			t.Fatalf("game %d outcome = %d, want %d", i+1, rec.Games[i].Outcome, want)
		}
	}
	wantRed := []string{"master-1", "grandmaster-1", "master-1"}
	wantBlue := []string{"grandmaster-1", "master-1", "grandmaster-1"}
	for i := range rec.Games {
		if rec.Games[i].Red.Name != wantRed[i] || rec.Games[i].Blue.Name != wantBlue[i] {
			t.Fatalf("game %d seats %s vs %s, want %s vs %s", i+1,
				rec.Games[i].Red.Name, rec.Games[i].Blue.Name, wantRed[i], wantBlue[i])
		}
	}
	if len(rec.Games[0].Moves) != 5 {
		t.Fatalf("game 1 moves = %d, want 5", len(rec.Games[0].Moves))
	}
	wantColors := []rules.Color{rules.Red, rules.Blue, rules.Red, rules.Blue, rules.Red}
	for i, m := range rec.Games[0].Moves {
		if m.Color != wantColors[i] {
			t.Fatalf("game 1 move %d color = %d, want %d", i+1, m.Color, wantColors[i])
		}
	}
	e8, err := rules.ParseCell("E8")
	if err != nil {
		t.Fatalf("E8: %v", err)
	}
	if rec.Games[0].Moves[2].Cell != e8 {
		t.Fatalf("game 1 move 3 cell = %d, want E8 (%d)", rec.Games[0].Moves[2].Cell, e8)
	}
	f9, _ := rules.ParseCell("F9")
	f10, _ := rules.ParseCell("F10")
	if rec.Games[0].Moves[3].Cell != f9 || rec.Games[0].Moves[4].Cell != f10 {
		t.Fatal("tagged M-lines ([BOOK], [PONDER]) must still parse their move field")
	}
	if len(rec.Games[1].Moves) != 4 || len(rec.Games[2].Moves) != 2 {
		t.Fatalf("game move counts = %d/%d, want 4/2", len(rec.Games[1].Moves), len(rec.Games[2].Moves))
	}
}

func TestParseMalformedInputs(t *testing.T) {
	base := loadFixtureLines(t)
	cases := []struct {
		name string
		mut  func([]string) []string
		want string
	}{
		{"no pairing", func(l []string) []string {
			return []string{l[0], l[2], l[3]}
		}, "pairing"},
		{"second pairing", func(l []string) []string {
			return append(append([]string{}, l...), l[1])
		}, "second pairing line"},
		{"game before pairing", func(l []string) []string {
			return []string{l[0], l[6], l[1]}
		}, "before the pairing line"},
		{"bad color", func(l []string) []string {
			out := append([]string{}, l...)
			out[3] = strings.Replace(out[3], "Red", "Green", 1)
			return out
		}, "color"},
		{"bad cell", func(l []string) []string {
			out := append([]string{}, l...)
			out[4] = strings.Replace(out[4], "F8", "Q8", 1)
			return out
		}, "cell"},
		{"short M-line", func(l []string) []string {
			out := append([]string{}, l...)
			out[3] = "M1, Red"
			return out
		}, "M-line"},
		{"outcome body unknown", func(l []string) []string {
			out := append([]string{}, l...)
			out[6] = "game 1: master-1 (red) crushed grandmaster-1 (blue), 5 moves"
			return out
		}, "outcome"},
		{"game number skipped", func(l []string) []string {
			out := append([]string{}, l...)
			out[13] = strings.Replace(out[13], "game 2", "game 3", 1)
			return out
		}, "out of order"},
		{"seat alternation broken", func(l []string) []string {
			out := append([]string{}, l...)
			out[13] = "game 2: master-1 (red) beat grandmaster-1 (blue), 4 moves, won by 4"
			return out
		}, "color alternation"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSeriesText("fixture", tc.mut(base))
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want substring %q", err, tc.want)
			}
		})
	}
}
