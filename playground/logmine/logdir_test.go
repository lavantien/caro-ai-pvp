package main

import (
	"os"
	"path/filepath"
	"testing"
)

func writeSeriesLog(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

const sampleSeriesA = `run 1 series 1
pairing master-1 (master) vs hard-2 (hard)
tc 3+2 bo3
M1, Red, H8, d=0, n=0, nps=0, ebf=0.0, tt=0%, hf=0%, fh1=0%, s=0, thr=8, t=0.00, alloc=2.00, [BOOK], pv=H8
M2, Blue, H6, d=14, n=6.25m, nps=2.5m, ebf=2.1, tt=38%, hf=45%, fh1=93%, s=+150, thr=4, t=2.50, alloc=2.50, pv=H6 H5 G6
game 1: master-1 (red) beat hard-2 (blue), 3 moves, won by 4
M1, Blue, H8, d=13, n=5.00m, nps=2.0m, ebf=2.0, tt=30%, hf=40%, fh1=90%, s=-100, thr=4, t=2.00, alloc=2.00, pv=H8 G7
game 2: master-1 (blue) beat hard-2 (red), 1 moves, won by 4
series master-1 2-0
`

const sampleSeriesB = `run 1 series 2
pairing easy-1 (easy) vs easy-2 (easy)
tc 1+0 bo3
M1, Red, H8, d=9, n=3.17m, nps=1.59m, ebf=5.2, tt=0%, hf=0%, fh1=90%, s=-200, thr=1, t=2.00, alloc=2.00, pv=H8 H6
M2, Blue, H6, d=8, n=3.13m, nps=1.57m, ebf=6.4, tt=0%, hf=0%, fh1=89%, s=+200, thr=1, t=2.00, alloc=2.00, pv=H6 H5
game 1: easy-2 (blue) beat easy-1 (red), 2 moves, won by 4
M1, Red, H5, d=7, n=3.18m, nps=1.59m, ebf=8.4, tt=0%, hf=0%, fh1=96%, s=-300, thr=1, t=2.00, alloc=2.00, pv=H5 F6
M2, Blue, F6, d=7, n=3.21m, nps=1.61m, ebf=8.4, tt=0%, hf=0%, fh1=92%, s=+2400, thr=1, t=0.05, alloc=2.00, pv=F6 E5
game 2: easy-1 (red) vs easy-2 (blue), drawn at 2 moves
series easy-1 1-1
garbage line
`

func TestParseSeriesLogAttributesTiersAndColors(t *testing.T) {
	s, err := parseSeriesLog([]byte(sampleSeriesA))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if s.tc != "3+2" || s.firstTier != "master" || s.secondTier != "hard" {
		t.Errorf("header = %+v", s)
	}
	if len(s.moves) != 3 {
		t.Fatalf("moves = %d, want 3", len(s.moves))
	}
	if s.moves[0].tier != "master" || s.moves[0].tag != "[BOOK]" || s.moves[0].tc != "3+2" {
		t.Errorf("move 0 = %+v", s.moves[0])
	}
	if s.moves[1].tier != "hard" || s.moves[1].depth != 14 {
		t.Errorf("move 1 = %+v", s.moves[1])
	}
	if s.moves[2].tier != "master" || s.moves[2].depth != 13 {
		t.Errorf("move 2 = %+v: game 2 has master on blue", s.moves[2])
	}
	if s.firstWins != 2 || s.secondWins != 0 {
		t.Errorf("verdict = %d-%d, want 2-0", s.firstWins, s.secondWins)
	}
	want := map[outcomeKey]int{
		{tc: "3+2", redTier: "master", blueTier: "hard", outcome: "red"}:  1,
		{tc: "3+2", redTier: "hard", blueTier: "master", outcome: "blue"}: 1,
	}
	if len(s.outcomes) != len(want) {
		t.Fatalf("outcomes = %+v", s.outcomes)
	}
	for _, o := range s.outcomes {
		if want[o.key] != o.count {
			t.Errorf("outcome %+v = %d, want %d", o.key, o.count, want[o.key])
		}
	}
}

func TestParseSeriesLogDrawAndRejects(t *testing.T) {
	s, err := parseSeriesLog([]byte(sampleSeriesB))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	drawSeen := false
	for _, o := range s.outcomes {
		if o.key.outcome == "draw" && o.key.redTier == "easy" {
			drawSeen = true
		}
	}
	if !drawSeen {
		t.Errorf("draw outcome missing: %+v", s.outcomes)
	}
	if s.rejects != 1 {
		t.Errorf("rejects = %d, want 1", s.rejects)
	}
	if s.firstWins != 1 || s.secondWins != 1 {
		t.Errorf("verdict = %d-%d, want 1-1", s.firstWins, s.secondWins)
	}
}

func TestMineLogDir(t *testing.T) {
	dir := t.TempDir()
	writeSeriesLog(t, dir, "run1_s1_a.txt", sampleSeriesA)
	writeSeriesLog(t, dir, "run1_s2_b.txt", sampleSeriesB)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeSeriesLog(t, filepath.Join(dir, "nested"), "run2_s1_c.txt", sampleSeriesA)
	m, err := mineLogDir(dir)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if m.files != 3 {
		t.Errorf("files = %d, want 3", m.files)
	}
	if len(m.tournament) != 10 {
		t.Fatalf("moves = %d, want 10", len(m.tournament))
	}
	tiers := map[string]int{}
	for _, r := range m.tournament {
		tiers[r.tier]++
	}
	if tiers["master"] != 4 || tiers["hard"] != 2 || tiers["easy"] != 4 {
		t.Errorf("tier counts = %v", tiers)
	}
	if m.rejects != 1 {
		t.Errorf("rejects = %d, want 1", m.rejects)
	}
	if len(m.firstWins) != 3 || len(m.outcomes) != 4 {
		t.Errorf("firstWins %d outcomes %d, want 3 and 4", len(m.firstWins), len(m.outcomes))
	}
}
