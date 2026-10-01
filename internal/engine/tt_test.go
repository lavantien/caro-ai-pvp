package engine

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestTTSlotIsTwoWords(t *testing.T) {
	tb := newTT(ttEntryBytes)
	if len(tb.entries) != 2 {
		t.Errorf("one slot uses %d words, want 2 (%d bytes)", len(tb.entries), ttEntryBytes)
	}
}

func TestTTInitSizes(t *testing.T) {
	cases := [...]struct {
		bytes int64
		slots int
	}{
		{0, 0},
		{15, 0},
		{16, 1},
		{1 << 20, 1 << 16},
		{1<<20 + 3*ttEntryBytes, 1 << 16},
	}
	for _, c := range cases {
		if tb := newTT(c.bytes); len(tb.entries) != 2*c.slots {
			t.Errorf("newTT(%d) slots = %d, want %d", c.bytes, len(tb.entries)/2, c.slots)
		}
	}
}

func TestTTRoundTripBounds(t *testing.T) {
	tb := newTT(1 << 12)
	hash := uint64(0xDEADBEEF)
	quiet := 250
	tb.store(hash, quiet, 42, 8, ttBoundExact, 3)
	score, move, cutoff := tb.probe(hash, 8, -1000, 1000, 3)
	if !cutoff || score != quiet || move != 42 {
		t.Fatalf("exact round trip: %d %d %v", score, move, cutoff)
	}
	if mv := tb.move(hash); mv != 42 {
		t.Errorf("tt move = %d, want 42", mv)
	}

	tb.store(hash^1, 100, 7, 6, ttBoundLower, 2)
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

	tb.store(hash^2, -900, 9, 4, ttBoundUpper, 6)
	if _, _, cutoff := tb.probe(hash^2, 4, -800, 0, 6); !cutoff {
		t.Error("upper bound at alpha must cut")
	}
	if _, _, cutoff := tb.probe(hash^2, 4, -1000, -850, 6); cutoff {
		t.Error("upper bound above alpha must not cut")
	}
}

func TestTTWrongKeyNeverCuts(t *testing.T) {
	tb := newTT(16)
	tb.store(0x1234, 500, 3, 4, ttBoundExact, 0)
	if _, _, cutoff := tb.probe(0x1234&tb.mask, 4, -1000, 1000, 0); cutoff {
		t.Error("index collision without key match must not cut")
	}
}

func TestTTMateScorePlyAdjustment(t *testing.T) {
	tb := newTT(1 << 10)
	mate := mateWin(6)
	tb.store(777, mate, 5, 10, ttBoundExact, 7)
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
	tb.store(778, mated, 6, 9, ttBoundExact, 1)
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
	tb := newTT(1 << 10)
	tb.store(999, 10, 1, 200, ttBoundExact, 0)
	if d := int(uint8(tb.entries[2*(999&tb.mask)] >> 48)); d != ttMaxDepth {
		t.Errorf("stored depth = %d, want clamp %d", d, ttMaxDepth)
	}
}

func TestTTDepthPreferredWithinGeneration(t *testing.T) {
	tb := newTT(1 << 10)
	tb.gen = 4
	tb.store(1000, 10, 1, 12, ttBoundExact, 0)
	tb.store(1000, 20, 2, 5, ttBoundExact, 0)
	if mv := tb.move(1000); mv != 1 {
		t.Errorf("shallow store must not replace deeper entry, move = %d", mv)
	}
	tb.store(1000, 30, 3, 12, ttBoundExact, 0)
	if mv := tb.move(1000); mv != 3 {
		t.Errorf("equal depth store must replace, move = %d", mv)
	}
}

func TestTTGenerationReplacement(t *testing.T) {
	tb := newTT(1 << 10)
	tb.gen = 4
	tb.store(1000, 10, 1, 20, ttBoundExact, 0)
	tb.gen = 5
	tb.store(1000, 20, 2, 1, ttBoundExact, 0)
	if mv := tb.move(1000); mv != 2 {
		t.Errorf("older generation entry must be replaced regardless of depth, move = %d", mv)
	}
}

func TestTTGenerationBumpWraps(t *testing.T) {
	tb := newTT(16)
	tb.gen = 63
	tb.bumpGen()
	if tb.gen != 1 {
		t.Errorf("generation after wrap = %d, want 1", tb.gen)
	}
	tb.bumpGen()
	if tb.gen != 2 {
		t.Errorf("generation = %d, want 2", tb.gen)
	}
}

func TestTTHashZeroNeverStores(t *testing.T) {
	tb := newTT(16)
	tb.store(0, 999, 1, 5, ttBoundExact, 0)
	if tb.entries[0] != 0 || tb.entries[1] != 0 {
		t.Errorf("zero key wrote an entry: data %#x sig %#x", tb.entries[0], tb.entries[1])
	}
}

func TestTTHashFullPermille(t *testing.T) {
	tb := newTT(2 * ttEntryBytes)
	tb.gen = 1
	tb.store(11, 0, 1, 3, ttBoundExact, 0)
	if got := tb.hashFullPermille(); got != 500 {
		t.Errorf("hash full = %d, want 500", got)
	}
	tb.store(11+1, 0, 2, 3, ttBoundExact, 0)
	if got := tb.hashFullPermille(); got != 1000 {
		t.Errorf("hash full = %d, want 1000", got)
	}
	tb.gen = 2
	if got := tb.hashFullPermille(); got != 0 {
		t.Errorf("stale generation hash full = %d, want 0", got)
	}
}

func TestTTDisabledIsInert(t *testing.T) {
	tb := newTT(0)
	if tb.enabled() {
		t.Fatal("zero byte table must disable")
	}
	tb.store(5, 5, 5, 5, ttBoundExact, 0)
	if score, move, cutoff := tb.probe(5, 5, 0, 10, 0); cutoff || score != 0 || move != -1 {
		t.Errorf("disabled probe = %d %d %v", score, move, cutoff)
	}
	if mv := tb.move(5); mv != moveNone {
		t.Errorf("disabled move = %v", mv)
	}
	if got := tb.hashFullPermille(); got != 0 {
		t.Errorf("disabled hash full = %d, want 0", got)
	}
}

// TestTTLocklessConcurrentAccess is the torn-read property of the lockless
// layout under contention: writers only ever store (hash, score, move)
// triples derived from the hash, so any probe that hits must return exactly
// that triple. A torn word pair shows up as a miss, never as a mismatched
// hit, and the race detector watches every word access.
func TestTTLocklessConcurrentAccess(t *testing.T) {
	tb := newTT(1 << 12)
	tb.gen = 1
	hashes := []uint64{1 << 40, 1<<40 + 7, 1 << 41, 1<<41 + 9}
	moveOf := func(h uint64) rules.Move { return rules.Move(h%251 + 1) }
	scoreOf := func(h uint64) int { return int(int32(h)) }
	var wg sync.WaitGroup
	for w := range 8 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := range 4000 {
				h := hashes[i%len(hashes)]
				if w%2 == 0 {
					tb.store(h, scoreOf(h), moveOf(h), 1+i%3, ttBoundExact, 0)
				} else if s, m, ok := tb.probe(h, 1, -config.EvalMateMax, config.EvalMateMax, 0); ok {
					if s != scoreOf(h) || m != int(moveOf(h)) {
						panic("torn entry surfaced as a hit")
					}
				}
			}
		}(w)
	}
	wg.Wait()
	if tb.hashFullPermille() == 0 {
		t.Error("contended writes left the sampled slots empty")
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
