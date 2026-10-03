package engine

import (
	"bytes"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Mutation gate killers for the eval/movegen/bitboard/stats survivor batch.
// Each test pins one observable behavior that a surviving splice breaks;
// equivalent mutants are allowlisted in .mutate-allow with proofs instead.

// TestGenerateEmptyBoardSingleCandidate pins the empty-board branch of
// generate: exactly one candidate, the precomputed center cell. A returned
// count of 2 would make the search play the zeroed moves[ply][1] slot.
func TestGenerateEmptyBoardSingleCandidate(t *testing.T) {
	e := New(0)
	n := e.generate(rules.NewBoard(), 0, moveNone)
	if n != 1 {
		t.Fatalf("empty board generated %d candidates, want 1", n)
	}
	if got := e.moves[0][0]; got != rules.Move(config.SearchEmptyBoardCell) {
		t.Fatalf("empty board move = %d, want center %d", got, config.SearchEmptyBoardCell)
	}
}

// TestRecordCutoffKillerShift pins the killer refresh: a cutoff move that
// already sits in the second slot still rotates to the front, so ordering
// reads it at the killer-1 layer, not killer-2.
func TestRecordCutoffKillerShift(t *testing.T) {
	e := New(0)
	first, second := rules.Move(mustCell(t, "H8")), rules.Move(mustCell(t, "E5"))
	e.killers[3][0], e.killers[3][1] = first, second
	e.recordCutoff(second, rules.Red, 3, 2)
	if e.killers[3][0] != second || e.killers[3][1] != first {
		t.Fatalf("killers after cutoff = [%d, %d], want [%d, %d]",
			e.killers[3][0], e.killers[3][1], second, first)
	}
}

// TestRecordCutoffHistoryDepthSquare pins the history bonus depth*depth and
// the exact aging threshold and halving on overflow.
func TestRecordCutoffHistoryDepthSquare(t *testing.T) {
	e := New(0)
	m := rules.Move(mustCell(t, "H8"))
	e.recordCutoff(m, rules.Red, 0, 5)
	if got := e.history[rules.Red][m]; got != 25 {
		t.Fatalf("history bonus = %d, want depth*depth = 25", got)
	}
}

func TestRecordCutoffHistoryAgingBoundary(t *testing.T) {
	e := New(0)
	m, other := rules.Move(mustCell(t, "H8")), rules.Move(mustCell(t, "A1"))
	e.history[rules.Red][m] = config.SearchHistoryMax - 9
	e.history[rules.Red][other] = 4096
	e.recordCutoff(m, rules.Red, 0, 3)
	if got := e.history[rules.Red][m]; got != config.SearchHistoryMax {
		t.Fatalf("history at exactly max = %d, want %d unaged", got, config.SearchHistoryMax)
	}
	if got := e.history[rules.Red][other]; got != 4096 {
		t.Fatalf("aging fired at exactly max: other entry = %d, want 4096", got)
	}
}

func TestRecordCutoffHistoryAgingHalves(t *testing.T) {
	e := New(0)
	m, x := rules.Move(mustCell(t, "H8")), rules.Move(mustCell(t, "A1"))
	e.history[rules.Red][m] = config.SearchHistoryMax
	e.history[rules.Blue][x] = 4096
	e.recordCutoff(m, rules.Red, 0, 1)
	want := (config.SearchHistoryMax + 1) / 2
	if got := e.history[rules.Red][m]; got != want {
		t.Fatalf("aged history = %d, want halved %d", got, want)
	}
	if got := e.history[rules.Blue][x]; got != 2048 {
		t.Fatalf("aged foreign entry = %d, want halved 2048", got)
	}
}

// TestEBFMilliDegenerateRows pins the guard row and the 1000 milliunit floor
// of the estimate: depth 0 is unknown whatever the node count, the smallest
// admissible node count still estimates, and a sum nothing fits returns lo.
func TestEBFMilliDegenerateRows(t *testing.T) {
	cases := [...]struct {
		nodes uint64
		depth int
		want  int
	}{
		{2, 0, 0},
		{2, 1, 1999},
		{2, 2, 1000},
	}
	for _, c := range cases {
		if got := EBFMilli(c.nodes, c.depth); got != c.want {
			t.Errorf("EBFMilli(%d, %d) = %d, want %d", c.nodes, c.depth, got, c.want)
		}
	}
}

// TestGeoSumSaturationBand pins both sides of the overflow guard around the
// b=2 term ladder: depth 52 stays a real sum, depth 53 saturates.
func TestGeoSumSaturationBand(t *testing.T) {
	cases := [...]struct {
		bMilli int
		depth  int
		want   uint64
	}{
		{2000, 52, 1<<53 - 1},
		{2000, 53, 1 << 62},
	}
	for _, c := range cases {
		if got := geoSum(c.bMilli, c.depth); got != c.want {
			t.Errorf("geoSum(%d, %d) = %d, want %d", c.bMilli, c.depth, got, c.want)
		}
	}
}

// TestAppendPVTruncatesAtArrayEnd pins the bounds guard: a PVLen beyond the
// fixed PV array truncates instead of indexing past the end.
func TestAppendPVTruncatesAtArrayEnd(t *testing.T) {
	var s SearchStats
	s.PVLen = len(s.PV) + 8
	buf := make([]byte, 0, s.PVLen*4)
	buf = s.AppendPV(buf)
	if want := 3*len(s.PV) - 1; len(buf) != want {
		t.Fatalf("truncated pv length = %d, want %d", len(buf), want)
	}
	if got := bytes.Count(buf, []byte("A1")); got != len(s.PV) {
		t.Fatalf("truncated pv holds %d cells, want %d", got, len(s.PV))
	}
}
