package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSeries(t *testing.T, dir, name, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestFoldSeriesByOpponentBothColors(t *testing.T) {
	dir := t.TempDir()
	writeSeries(t, dir, "run1_s01_a-vs-b.txt", "run 1 series 1\npairing seat-a (hard) vs seat-b (master)\ntc 2+1 bo3\ngame 1: seat-a (red) beat seat-b (blue), 30 moves\ngame 2: seat-b (red) beat seat-a (blue), 30 moves\ngame 3: seat-a (red) beat seat-b (blue), 30 moves\nseries seat-a 2-1\n")
	writeSeries(t, dir, "run1_s02_b-vs-a.txt", "run 1 series 2\npairing seat-b (master) vs seat-a (hard)\ntc 2+1 bo3\ngame 1: seat-b (red) beat seat-a (blue), 30 moves\ngame 2: seat-a (red) beat seat-b (blue), 30 moves\ngame 3: seat-b (red) beat seat-a (blue), 30 moves\nseries seat-b 2-1\n")
	a, err := loadArm("probe", dir)
	if err != nil {
		t.Fatal(err)
	}
	sa, sb := a.seats["seat-a"], a.seats["seat-b"]
	if sa.seriesW != 1 || sa.seriesL != 1 || sb.seriesW != 1 || sb.seriesL != 1 {
		t.Fatalf("standings: a %d-%d b %d-%d", sa.seriesW, sa.seriesL, sb.seriesW, sb.seriesL)
	}
	if got := sa.seriesBy["seat-b"]; got != (wld{w: 1, l: 1}) {
		t.Errorf("seat-a seriesBy seat-b: got %v", got)
	}
	if got := sb.seriesBy["seat-a"]; got != (wld{w: 1, l: 1}) {
		t.Errorf("seat-b seriesBy seat-a: got %v", got)
	}
	if got := sa.seriesBy["seat-a"]; got != (wld{}) {
		t.Errorf("seat-a seriesBy itself: got %v", got)
	}
	if got := sb.seriesBy["seat-b"]; got != (wld{}) {
		t.Errorf("seat-b seriesBy itself: got %v", got)
	}
	if got := sa.vsSeat["seat-b"]; got != (wld{w: 3, l: 3}) {
		t.Errorf("seat-a vsSeat seat-b: got %v", got)
	}
	if got := sa.red; got != (wld{w: 3, l: 0}) {
		t.Errorf("seat-a red record: got %v", got)
	}
	if got := sa.blue; got != (wld{w: 0, l: 3}) {
		t.Errorf("seat-a blue record: got %v", got)
	}
	if got := sa.vsTier["master"]; got != (wld{w: 3, l: 3}) {
		t.Errorf("seat-a vsTier medium: got %v", got)
	}
}

func TestFoldFinalScoresPerSeat(t *testing.T) {
	dir := t.TempDir()
	writeSeries(t, dir, "run1_s01_a-vs-b.txt", "run 1 series 1\npairing seat-a (medium) vs seat-b (medium)\ntc 1+0 bo1\nM1, Red, H8, d=9, n=2.86m, nps=1.43m, ebf=5.1, tt=8%, hf=42%, fh1=89%, s=-200, thr=1, t=2.00, alloc=2.00, pv=H8\nM2, Blue, H9, d=9, n=2.86m, nps=1.43m, ebf=5.1, tt=8%, hf=42%, fh1=89%, s=+200, thr=1, t=2.00, alloc=2.00, pv=H9\ngame 1: seat-a (red) beat seat-b (blue), 2 moves, won by 4\nseries seat-a 1-0\n")
	a, err := loadArm("probe", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.games) != 1 {
		t.Fatalf("games: %d", len(a.games))
	}
	g := a.games[0]
	if g.redLast != -200 || g.blueLast != 200 {
		t.Errorf("final scores: red %d blue %d", g.redLast, g.blueLast)
	}
	if loser, tier, ok := g.loser(); !ok || loser != "seat-b" || tier != "medium" {
		t.Errorf("loser: %s %s %v", loser, tier, ok)
	}
	if g.lastScoreOf("seat-b") != 200 {
		t.Errorf("seat-b final score: %d", g.lastScoreOf("seat-b"))
	}
	sa := a.seats["seat-a"]
	if sa.tel.lines != 1 || sa.phase[0].lines != 1 || sa.phase[1].lines != 0 {
		t.Errorf("telemetry lines: total %d early %d mid %d", sa.tel.lines, sa.phase[0].lines, sa.phase[1].lines)
	}
}
