package main

import (
	"strings"
	"testing"
)

func runawayFixture() []moveRow {
	mk := func(depth int, moveNo int, score string, pvLen int, tc string, ebf int) moveRow {
		return moveRow{
			tier: "easy", tc: tc, depth: depth, nodes: 4_000_000, nps: 1_500_000,
			ebfMilli: ebf, moveNo: moveNo, scoreTxt: score, pvLen: pvLen,
			elapsedNS: 12_000_000_000, allocNS: 12_000_000_000,
			srcFile: "r1_s1_easy-1-vs-easy-2.txt", gameID: 1,
		}
	}
	return []moveRow{
		mk(64, 41, "-M4", 4, "1+0", 32000),
		mk(64, 42, "M3", 3, "1+0", 32000),
		mk(44, 39, "+1200", 6, "1+0", 12000),
		mk(9, 12, "-200", 8, "1+0", 8000),
	}
}

func TestRunawayStats(t *testing.T) {
	stats := mineRunaway(runawayFixture())
	if stats.total != 3 {
		t.Fatalf("total = %d, want 3", stats.total)
	}
	if stats.atCap != 2 {
		t.Errorf("atCap = %d, want 2", stats.atCap)
	}
	if stats.mate != 2 || stats.nearMate != 1 || stats.normal != 0 {
		t.Errorf("score classes = mate %d near %d normal %d, want 2 1 0", stats.mate, stats.nearMate, stats.normal)
	}
	if stats.moveNo.med != 41 {
		t.Errorf("moveNo med = %v, want 41", stats.moveNo.med)
	}
	if len(stats.examples) != 3 {
		t.Errorf("examples = %d, want 3", len(stats.examples))
	}
	if stats.verdict != "cap-bound mate ladder on sparse tails" {
		t.Errorf("verdict = %q", stats.verdict)
	}
}

func TestRunawayReportRenders(t *testing.T) {
	stats := mineRunaway(runawayFixture())
	out := renderRunawayReport("logs/tourny", 1111, stats)
	for _, want := range []string{
		"# easy tier depth runaway: logs/tourny", "depth >= 30", "SearchMaxPly 64",
		"mate ladder", "| 3 | 2 | 2 | 1 | 0 | 0 |", "verdict: cap-bound mate ladder on sparse tails",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
}

func TestRunawayZeroScoreBranch(t *testing.T) {
	rows := []moveRow{
		{tier: "easy", tc: "2+1", depth: 64, moveNo: 250, scoreTxt: "+0", pvLen: 3,
			ebfMilli: 2000, nodes: 1490, elapsedNS: 12e9, allocNS: 12e9, srcFile: "z.txt", gameID: 2},
		{tier: "easy", tc: "2+1", depth: 64, moveNo: 252, scoreTxt: "-0", pvLen: 2,
			ebfMilli: 2000, nodes: 900, elapsedNS: 12e9, allocNS: 12e9, srcFile: "z.txt", gameID: 2},
	}
	s := mineRunaway(rows)
	if s.total != 2 || s.atCap != 2 || s.zero != 2 {
		t.Fatalf("stats = %+v", s)
	}
	if s.verdict != "cap-bound ladder on collapsed branching with draw scores, not a mate ladder" {
		t.Errorf("verdict = %q", s.verdict)
	}
}
