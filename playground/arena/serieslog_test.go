package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTempSeries(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestParseSeriesLogFullShape(t *testing.T) {
	body := strings.Join([]string{
		"run 7 series 12",
		"pairing master one (master) vs easy two (easy)",
		"tc 3+2 bo3",
		"M1, Red, H8, d=11, n=42.35m, nps=6.05m, ebf=4.9, tt=8%, hf=42%, fh1=91%, s=-300, thr=6, t=7.00, alloc=7.00, pv=H8 H6",
		"game 1: master one (red) vs easy two (blue), drawn at 256 moves",
		"game 2: master one (blue) beat easy two (red), 44 moves, won by 4",
		"game 3: master one (red) beat easy two (blue), 75 moves, won by open 4",
		"series master one 2-0",
		"",
	}, "\n")
	sl, err := parseSeriesLog(writeTempSeries(t, "run7_s12.txt", body))
	if err != nil {
		t.Fatalf("parseSeriesLog: %v", err)
	}
	if sl.redFirst != "master one" || sl.blueFirst != "easy two" {
		t.Errorf("pairing = %q vs %q", sl.redFirst, sl.blueFirst)
	}
	want := []Result{Draw, Win, Win}
	if len(sl.games) != len(want) {
		t.Fatalf("games = %v, want %v", sl.games, want)
	}
	for i, r := range want {
		if sl.games[i] != r {
			t.Errorf("game %d = %d, want %d", i+1, sl.games[i], r)
		}
	}
	if !sl.hasVerdict || sl.verdictWinner != "master one" || sl.firstWins != 2 || sl.secondWins != 0 {
		t.Errorf("verdict = (%q, %d, %d, %v)", sl.verdictWinner, sl.firstWins, sl.secondWins, sl.hasVerdict)
	}
}

func TestParseSeriesLogColorFlipResults(t *testing.T) {
	body := strings.Join([]string{
		"pairing a (easy) vs b (easy)",
		"game 1: b (blue) beat a (red), 48 moves, won by 4",
		"game 2: a (blue) beat b (red), 45 moves, won by 4",
		"game 3: a (red) beat b (blue), 12 moves, won by double four",
		"series a 2-1",
		"",
	}, "\n")
	sl, err := parseSeriesLog(writeTempSeries(t, "s.txt", body))
	if err != nil {
		t.Fatalf("parseSeriesLog: %v", err)
	}
	want := []Result{Loss, Win, Win}
	for i, r := range want {
		if sl.games[i] != r {
			t.Errorf("game %d = %d, want %d", i+1, sl.games[i], r)
		}
	}
	pt := replaySeriesPtnml(sl)
	if got := pt.Summary(); got != "0-0-1-0-0 over 1 pairs, 1 unpaired" {
		t.Errorf("replay Summary = %q", got)
	}
}

func TestParseSeriesLogUnfinishedSeries(t *testing.T) {
	body := strings.Join([]string{
		"pairing a (easy) vs b (easy)",
		"game 1: a (red) beat b (blue), 20 moves",
		"",
	}, "\n")
	sl, err := parseSeriesLog(writeTempSeries(t, "s.txt", body))
	if err != nil {
		t.Fatalf("parseSeriesLog: %v", err)
	}
	if sl.hasVerdict {
		t.Errorf("unfinished series reported a verdict")
	}
	if len(sl.games) != 1 || sl.games[0] != Win {
		t.Errorf("games = %v, want one win", sl.games)
	}
	pt := replaySeriesPtnml(sl)
	if pt.pairs != 0 || pt.unpaired != 1 || pt.HalfPoints() != 2 {
		t.Errorf("replay of one win = pairs %d unpaired %d half %d", pt.pairs, pt.unpaired, pt.HalfPoints())
	}
}

func TestParseSeriesLogErrors(t *testing.T) {
	cases := []struct {
		name, body, wantErr string
	}{
		{"no pairing", "game 1: a (red) beat b (blue), 9 moves\n", "no pairing line"},
		{"second pairing", "pairing a (easy) vs b (easy)\npairing c (easy) vs d (easy)\n", "second pairing line"},
		{"bad pairing", "pairing a easy vs b easy\n", "unparsable pairing line"},
		{"game out of order", "pairing a (easy) vs b (easy)\ngame 2: a (red) beat b (blue), 9 moves\n", "out of order"},
		{"game no outcome", "pairing a (easy) vs b (easy)\ngame 1: a (red) vs b (blue), 9 moves\n", "no outcome"},
		{"bad game number", "pairing a (easy) vs b (easy)\ngame x: a (red) beat b (blue), 9 moves\n", "unparsable"},
		{"bad verdict", "pairing a (easy) vs b (easy)\nseries a 2\n", "unparsable series line"},
		{"color alternation broken", "pairing a (easy) vs b (easy)\ngame 1: a (red) beat b (blue), 9 moves\ngame 2: b (blue) beat a (red), 9 moves\n", "color alternation"},
		{"first game red seat wrong", "pairing a (easy) vs b (easy)\ngame 1: b (red) beat a (blue), 9 moves\n", "color alternation"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := parseSeriesLog(writeTempSeries(t, "s.txt", c.body))
			if err == nil || !strings.Contains(err.Error(), c.wantErr) {
				t.Errorf("err = %v, want it to contain %q", err, c.wantErr)
			}
		})
	}
}

func TestVerdictLineCrossCheckLaw(t *testing.T) {
	body := strings.Join([]string{
		"pairing a (easy) vs b (easy)",
		"game 1: a (red) vs b (blue), drawn at 256 moves",
		"game 2: a (blue) beat b (red), 44 moves",
		"game 3: b (blue) beat a (red), 84 moves",
		"series drawn 1-1",
		"",
	}, "\n")
	sl, err := parseSeriesLog(writeTempSeries(t, "s.txt", body))
	if err != nil {
		t.Fatalf("parseSeriesLog: %v", err)
	}
	wins, draws, losses := sl.winDrawLoss()
	if wins != sl.firstWins || losses != sl.secondWins {
		t.Errorf("parsed %d-%d vs verdict %d-%d", wins, losses, sl.firstWins, sl.secondWins)
	}
	if draws != 1 {
		t.Errorf("draws = %d, want 1", draws)
	}
}

func TestMainCheckoutRootRejectsPlainGitDir(t *testing.T) {
	dir := t.TempDir()
	if got := mainCheckoutRoot(dir); got != "" {
		t.Errorf("mainCheckoutRoot on a dir without .git = %q, want empty", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o700); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if got := mainCheckoutRoot(dir); got != "" {
		t.Errorf("mainCheckoutRoot on a plain .git dir = %q, want empty", got)
	}
}
