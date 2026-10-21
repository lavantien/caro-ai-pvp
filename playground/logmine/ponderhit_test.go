package main

import (
	"strings"
	"testing"
)

func hitFixture() []moveRow {
	mk := func(tier string, moveNo int, cell string, pv string, tag string) moveRow {
		return moveRow{
			tier: tier, tc: "3+2", moveNo: moveNo, cell: cell, pvCells: strings.Fields(pv),
			pvLen: pvWordCount(pv), tag: tag, srcFile: "h.txt", gameID: 1,
			allocNS: 3e9, elapsedNS: 2e9, depth: 9, nodes: 1_000_000, nps: 1_000_000,
		}
	}
	return []moveRow{
		mk("master", 1, "H8", "H8 H9 G7 F6", ""),
		mk("hard", 2, "H9", "H9 H8", ""),
		mk("master", 3, "G7", "G7 F6", "[PONDER]"),
		mk("hard", 4, "F6", "F6 G7", ""),
		mk("master", 5, "H8", "H8 G7 F6 E5", ""),
		mk("hard", 6, "E5", "E5 F4", ""),
		mk("master", 7, "F6", "F6 E5", ""),
		mk("easy", 8, "E5", "E5 F4", ""),
	}
}

func TestHitStats(t *testing.T) {
	agg := hitStats(hitFixture())
	m, ok := agg[hitKey{tier: "master", tc: "3+2"}]
	if !ok {
		t.Fatal("master key missing")
	}
	if m.predictions != 4 {
		t.Errorf("master predictions = %d, want 4", m.predictions)
	}
	if m.hits != 3 {
		t.Errorf("master hits = %d, want 3: moves 1, 3, 7 predict moves 2, 4, 8", m.hits)
	}
	if m.adoptGivenHit != 1 {
		t.Errorf("adoptGivenHit = %d, want 1: move 3 is [PONDER], the move 7 hit has no move 9", m.adoptGivenHit)
	}
	if m.missDist["pv3"] != 0 || m.missDist["pv4"] != 1 || m.missDist["pv5+"] != 0 || m.missDist["not-in-pv"] != 0 {
		t.Errorf("master miss distance = %v, want only pv4 1", m.missDist)
	}
	if _, ok := agg[hitKey{tier: "easy", tc: "3+2"}]; ok {
		t.Error("easy present, want excluded")
	}
}

func TestHitReportRenders(t *testing.T) {
	agg := hitStats(hitFixture())
	out := renderHitReport("logs/tourny", 1111, agg)
	for _, want := range []string{
		"# ponder hit rate: logs/tourny", "compare pv[1] of turn n (the predicted reply) against the played cell of turn n+1",
		"| master | 3+2 | 4 | 3 | 75.0% | 1 | 33.3% |",
		"not-in-pv",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
}
