package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestEvalKillNearEdgeSearchHoldsPinnedResult(t *testing.T) {
	b := rules.NewBoard()
	for i, name := range [...]string{"C3", "C4", "D3", "E5"} {
		b.Side = rules.Color(i % 2)
		b.Make(mustCell(t, name))
	}
	e := New(testTTBytes)
	mv, stats := e.SearchDepth(b, cappedNodeBudget(1_000_000), 4)
	if mv != 36 || stats.Score != -250 {
		t.Fatalf("near-edge board depth 4 = move %d score %d, want the clean-engine pin (36, -250) recorded 2026-11-08", mv, stats.Score)
	}
}
