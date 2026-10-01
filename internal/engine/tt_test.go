package engine

import (
	"reflect"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestTTEntryIs16Bytes(t *testing.T) {
	if got := reflect.TypeOf(ttEntry{}).Size(); got != ttEntryBytes {
		t.Errorf("ttEntry size = %d, want %d", got, ttEntryBytes)
	}
}

func TestTTInitSizes(t *testing.T) {
	cases := [...]struct {
		bytes   int64
		entries int
	}{
		{0, 0},
		{15, 0},
		{16, 1},
		{1 << 20, 1 << 16},
		{1<<20 + 3*ttEntryBytes, 1 << 16},
	}
	for _, c := range cases {
		var tb ttTable
		tb.init(c.bytes)
		if len(tb.entries) != c.entries {
			t.Errorf("init(%d) entries = %d, want %d", c.bytes, len(tb.entries), c.entries)
		}
	}
}

func TestTTRoundTripBounds(t *testing.T) {
	var tb ttTable
	tb.init(1 << 12)
	hash := uint64(0xDEADBEEF)
	quiet := 250
	tb.store(hash, quiet, 42, 8, ttBoundExact, 3, 5)
	score, move, cutoff := tb.probe(hash, 8, -1000, 1000, 3)
	if !cutoff || score != quiet || move != 42 {
		t.Fatalf("exact round trip: %d %d %v", score, move, cutoff)
	}
	if mv := tb.move(hash); mv != 42 {
		t.Errorf("tt move = %d, want 42", mv)
	}

	tb.store(hash^1, 100, 7, 6, ttBoundLower, 2, 1)
	if _, _, cutoff := tb.probe(hash^1, 6, 0, 100, 2); !cutoff {
		t.Error("lower bound at beta must cut")
	}
	if _, _, cutoff := tb.probe(hash^1, 6, 150, 1000, 2); cutoff {
		t.Error("lower bound below beta must not cut")
	}
	if _, _, cutoff := tb.probe(hash^1, 3, 0, 100, 2); !cutoff {
		t.Error("bound cut must survive probe at shallower depth")
	}
	if _, move, cutoff := tb.probe(hash^1, 9, 0, 1000, 2); cutoff {
		t.Error("deeper probe must not cut")
	} else if move != 7 {
		t.Errorf("deeper probe move = %d, want the ordering move 7", move)
	}

	tb.store(hash^2, -900, 9, 4, ttBoundUpper, 6, 2)
	if _, _, cutoff := tb.probe(hash^2, 4, -800, 0, 6); !cutoff {
		t.Error("upper bound at alpha must cut")
	}
	if _, _, cutoff := tb.probe(hash^2, 4, -1000, -850, 6); cutoff {
		t.Error("upper bound above alpha must not cut")
	}
}

func TestTTWrongKeyNeverCuts(t *testing.T) {
	var tb ttTable
	tb.init(16)
	tb.store(0x1234, 500, 3, 4, ttBoundExact, 0, 1)
	if _, _, cutoff := tb.probe(0x1234&tb.mask, 4, -1000, 1000, 0); cutoff {
		t.Error("index collision without key match must not cut")
	}
}

func TestTTMateScorePlyAdjustment(t *testing.T) {
	var tb ttTable
	tb.init(1 << 10)
	mate := mateWin(6)
	tb.store(777, mate, 5, 10, ttBoundExact, 7, 1)
	score, _, cutoff := tb.probe(777, 10, -config.EvalMateMax, config.EvalMateMax, 2)
	if !cutoff {
		t.Fatal("mate entry must cut")
	}
	if want := mate + (7-2)*config.EvalMateScoreStep; score != want {
		t.Errorf("probe at ply 2 = %d, want root-relative %d", score, want)
	}
	if score%config.EvalMateScoreStep != config.EvalMateMax%config.EvalMateScoreStep {
		t.Errorf("decoded mate %d fell off the mateWin lattice", score)
	}
	if _, _, cutoff := tb.probe(777, 10, -config.EvalMateMax, config.EvalMateMax, 7); !cutoff {
		t.Fatal("mate entry must cut at store ply")
	}
	if score7, _, _ := tb.probe(777, 10, -config.EvalMateMax, config.EvalMateMax, 7); score7 != mate {
		t.Errorf("probe at store ply = %d, want %d", score7, mate)
	}
	mated := -mateWin(1)
	tb.store(778, mated, 6, 9, ttBoundExact, 1, 1)
	if s, _, _ := tb.probe(778, 9, -config.EvalMateMax, config.EvalMateMax, 4); s != mated+3*config.EvalMateScoreStep {
		t.Errorf("mated probe = %d, want %d", s, mated+3*config.EvalMateScoreStep)
	}
}

// Regression for the mate-distance unit defect: probing a mate entry stored
// by an earlier search of the same engine must stay on the mateWin lattice
// and match what a fresh engine reports from the advanced root.
func TestTTCrossSearchMateDistance(t *testing.T) {
	b := mate3Board(t)
	warm := New(testTTBytes)
	dl := NewFixedBudget(time.Second)
	_, first := warm.SearchDepth(b, dl, 5)
	b.Make(rules.Cell(first.PV[0]))
	b.Make(mustCell(t, "I9"))
	_, advanced := warm.SearchDepth(b, dl, 3)
	fresh := New(testTTBytes)
	_, reference := fresh.SearchDepth(b, dl, 3)
	if advanced.Score != reference.Score {
		t.Fatalf("warm TT score %d diverged from fresh %d after re-rooting", advanced.Score, reference.Score)
	}
	if advanced.Score >= evalMateScoreMin || advanced.Score <= -evalMateScoreMin {
		if (advanced.Score-config.EvalMateMax)%config.EvalMateScoreStep != 0 {
			t.Errorf("decoded mate %d fell off the mateWin lattice", advanced.Score)
		}
	}
}

func TestTTStoreClampsDepthToInt8(t *testing.T) {
	var tb ttTable
	tb.init(1 << 10)
	tb.store(999, 10, 1, 200, ttBoundExact, 0, 1)
	if d := int(tb.entries[999&tb.mask].depth); d != ttMaxDepth {
		t.Errorf("stored depth = %d, want clamp %d", d, ttMaxDepth)
	}
}

func TestTTDepthPreferredWithinGeneration(t *testing.T) {
	var tb ttTable
	tb.init(1 << 10)
	tb.store(1000, 10, 1, 12, ttBoundExact, 0, 4)
	tb.store(1000, 20, 2, 5, ttBoundExact, 0, 4)
	if mv := tb.move(1000); mv != 1 {
		t.Errorf("shallow store must not replace deeper entry, move = %d", mv)
	}
	tb.store(1000, 30, 3, 12, ttBoundExact, 0, 4)
	if mv := tb.move(1000); mv != 3 {
		t.Errorf("equal depth store must replace, move = %d", mv)
	}
}

func TestTTGenerationReplacement(t *testing.T) {
	var tb ttTable
	tb.init(1 << 10)
	tb.store(1000, 10, 1, 20, ttBoundExact, 0, 4)
	tb.store(1000, 20, 2, 1, ttBoundExact, 0, 5)
	if mv := tb.move(1000); mv != 2 {
		t.Errorf("older generation entry must be replaced regardless of depth, move = %d", mv)
	}
}

func TestTTHashZeroNeverStores(t *testing.T) {
	var tb ttTable
	tb.init(16)
	tb.store(0, 999, 1, 5, ttBoundExact, 0, 1)
	if e := tb.entries[0]; e.score != 0 || e.depth != 0 || e.move != 0 {
		t.Errorf("zero key wrote an entry: %+v", e)
	}
}

func TestTTHashFullPermille(t *testing.T) {
	var tb ttTable
	tb.init(2 * ttEntryBytes)
	tb.store(11, 0, 1, 3, ttBoundExact, 0, 1)
	if got := tb.hashFullPermille(1); got != 500 {
		t.Errorf("hash full = %d, want 500", got)
	}
	tb.store(11+1, 0, 2, 3, ttBoundExact, 0, 1)
	if got := tb.hashFullPermille(1); got != 1000 {
		t.Errorf("hash full = %d, want 1000", got)
	}
	if got := tb.hashFullPermille(2); got != 0 {
		t.Errorf("stale generation hash full = %d, want 0", got)
	}
}

func TestTTDisabledIsInert(t *testing.T) {
	var tb ttTable
	tb.init(0)
	if tb.enabled() {
		t.Fatal("zero byte table must disable")
	}
	tb.store(5, 5, 5, 5, ttBoundExact, 0, 1)
	if score, move, cutoff := tb.probe(5, 5, 0, 10, 0); cutoff || score != 0 || move != -1 {
		t.Errorf("disabled probe = %d %d %v", score, move, cutoff)
	}
	if mv := tb.move(5); mv != moveNone {
		t.Errorf("disabled move = %v", mv)
	}
	if got := tb.hashFullPermille(1); got != 0 {
		t.Errorf("disabled hash full = %d", got)
	}
}

func TestSearchWorksWithoutTT(t *testing.T) {
	b := midgameBoard(t)
	e := New(0)
	mv, stats := e.Search(b, NewFixedBudget(40*time.Millisecond))
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("no-tt search returned illegal move %d", mv)
	}
	if stats.TTHitPermille != 0 || stats.HashFullPermille != 0 {
		t.Errorf("no-tt tt stats %d %d, want 0 0", stats.TTHitPermille, stats.HashFullPermille)
	}
}
