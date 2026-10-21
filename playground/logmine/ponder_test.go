package main

import (
	"strings"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

func ponderFixture() []moveRow {
	mk := func(tier string, adopted bool, depth int, t float64, moveNo int) moveRow {
		tag := ""
		if adopted {
			tag = "[PONDER]"
		}
		return moveRow{
			tier: tier, tc: "2+1", depth: depth, nodes: 1_000_000, nps: 2_000_000,
			tag: tag, moveNo: moveNo, scoreTxt: "+100",
			elapsedNS: int64(t * 1e9), allocNS: 3_000_000_000,
			srcFile: "a.txt", gameID: 1, pvLen: 6,
		}
	}
	return []moveRow{
		mk("master", true, 10, 0.01, 2),
		mk("hard", false, 9, 2.50, 3),
		mk("master", false, 8, 2.80, 4),
		mk("master", false, 12, 0.20, 5),
		mk("easy", false, 7, 2.00, 6),
	}
}

func TestPonderCapableTiers(t *testing.T) {
	capable := ponderCapableTiers()
	if capable["easy"] {
		t.Error("easy counted as ponder capable")
	}
	for _, want := range []string{"medium", "hard", "master", "grandmaster"} {
		if !capable[want] {
			t.Errorf("tier %s missing from capable set: %v", want, capable)
		}
	}
}

func TestPonderStatsAdoptionAndClauses(t *testing.T) {
	stats := ponderStats(ponderFixture())
	m, ok := stats[ponderKey{tier: "master", tc: "2+1"}]
	if !ok {
		t.Fatal("master key missing")
	}
	if m.opportunities != 3 || m.adopted != 1 {
		t.Errorf("master opportunities %d adopted %d, want 3 and 1", m.opportunities, m.adopted)
	}
	if m.adoptT.med != 0.01 {
		t.Errorf("adopt t med = %v, want 0.01", m.adoptT.med)
	}
	if m.untaggedDepth.med != 10 {
		t.Errorf("untagged depth med = %v, want 10", m.untaggedDepth.med)
	}
	if m.clause1Total != 1 || m.clause1Pass != 1 {
		t.Errorf("clause1 = %d/%d, want 1/1: adopted depth 10 vs untagged median 10", m.clause1Pass, m.clause1Total)
	}
	if _, ok := stats[ponderKey{tier: "hard", tc: "2+1"}]; ok {
		t.Error("hard present, want absent: no [PONDER] answer from hard in the fixture")
	}
	if _, ok := stats[ponderKey{tier: "easy", tc: "2+1"}]; ok {
		t.Error("easy present, want excluded as non capable")
	}
}

func TestPonderReportRenders(t *testing.T) {
	stats := ponderStats(ponderFixture())
	out := renderPonderReport("logs/tourny", 1111, stats)
	for _, want := range []string{
		"# ponder adoption: logs/tourny", "definition: opportunities are all moves by a ponder-capable tier",
		"| master | 2+1 | 3 | 1 | 33.3% |",
		"clause 1", "clause 2 (best move stable over the last 3 completed iterations)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q", want)
		}
	}
	if len(config.TimeControls) == 0 {
		t.Error("config sanity")
	}
}
