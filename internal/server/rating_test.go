package server

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Expected integers are hand-derived from the spec law K = 10^((R_loser -
// R_winner)/D) with D = RatingDivisorFull on an upset and the loser-band
// clamp otherwise, then cross-checked against an independent scratch run of
// the raw formula. Rounding margins stay far from the .5 boundary.
func TestRatingDeltasBandClosedForm(t *testing.T) {
	cases := []struct {
		name    string
		rRed    int
		rBlue   int
		outcome Outcome
		dRed    int
	}{
		{"favorite gap 300, loser 0", 300, 0, RedWins, 8},
		{"favorite gap 300, loser 200", 500, 200, RedWins, 15},
		{"favorite gap 300, loser 400", 700, 400, RedWins, 19},
		{"favorite gap 300, loser 600", 900, 600, RedWins, 21},
		{"favorite gap 300, loser 800", 1100, 800, RedWins, 23},
		{"favorite gap 300, loser 1001 clamps D to 3000", 1301, 1001, RedWins, 24},
		{"favorite gap 300, loser 2000 clamps D to 3000", 2300, 2000, RedWins, 24},
		{"blue favorite winner mirrors the band", 200, 300, BlueWins, -24},
	}
	for _, c := range cases {
		dRed, dBlue, afterRed, afterBlue := RatingDeltas(c.rRed, c.rBlue, c.outcome)
		if dRed != c.dRed || dBlue != -c.dRed {
			t.Errorf("%s: deltas = %d/%d, want +-%d", c.name, dRed, dBlue, c.dRed)
		}
		if afterRed != c.rRed+c.dRed || afterBlue != c.rBlue-c.dRed {
			t.Errorf("%s: afters = %d/%d, want %d/%d",
				c.name, afterRed, afterBlue, c.rRed+c.dRed, c.rBlue-c.dRed)
		}
	}
}

func TestRatingDeltasBranchSelection(t *testing.T) {
	cases := []struct {
		name  string
		rRed  int
		rBlue int
		dRed  int
	}{
		{"upset gap 100 at loser 200 uses the full 3000 divisor", 100, 200, 32},
		{"favorite gap 100 at loser 200 uses the 1000 band", 300, 200, 24},
		{"upset gap 1500", 500, 2000, 95},
	}
	for _, c := range cases {
		dRed, _, _, _ := RatingDeltas(c.rRed, c.rBlue, RedWins)
		if dRed != c.dRed {
			t.Errorf("%s: dRed = %d, want %d", c.name, dRed, c.dRed)
		}
	}
	// Equal ratings: the exponent is 0, so K is 1 and the deltas are +-30 no
	// matter which divisor the <= branch picks; the branch itself is output
	// invisible at equality, the outcome is pinned anyway.
	for _, r := range []int{0, 200, 1001, -50} {
		dRed, dBlue, afterRed, afterBlue := RatingDeltas(r, r, RedWins)
		if dRed != config.RatingDelta || dBlue != -config.RatingDelta {
			t.Errorf("equal %d: deltas = %d/%d, want +-%d", r, dRed, dBlue, config.RatingDelta)
		}
		if afterRed != r+config.RatingDelta || afterBlue != r-config.RatingDelta {
			t.Errorf("equal %d: afters = %d/%d", r, afterRed, afterBlue)
		}
	}
}

func TestRatingDeltasDraw(t *testing.T) {
	for _, c := range [][2]int{{0, 0}, {100, -50}, {-1000, 2000}, {999, 1000}} {
		dRed, dBlue, afterRed, afterBlue := RatingDeltas(c[0], c[1], Draw)
		if dRed != 0 || dBlue != 0 || afterRed != c[0] || afterBlue != c[1] {
			t.Errorf("draw %v: got %d/%d/%d/%d, want 0/0/%d/%d",
				c, dRed, dBlue, afterRed, afterBlue, c[0], c[1])
		}
	}
}

func TestRatingDeltasSideSwap(t *testing.T) {
	for _, c := range [][2]int{{100, 200}, {200, 100}, {0, 1001}, {-500, 500}, {1234, 5}} {
		dRed, dBlue, afterRed, afterBlue := RatingDeltas(c[0], c[1], RedWins)
		revRed, revBlue, revAfterRed, revAfterBlue := RatingDeltas(c[1], c[0], BlueWins)
		if dRed != revBlue || dBlue != revRed {
			t.Errorf("swap %v: winner delta moved from %d to %d", c, dRed, revBlue)
		}
		if afterRed != revAfterBlue || afterBlue != revAfterRed {
			t.Errorf("swap %v: afters %d/%d vs %d/%d", c, afterRed, afterBlue, revAfterRed, revAfterBlue)
		}
	}
}

// An exact .5 product is unreachable: 30*10^(g/D) is irrational for every
// integer gap g != 0, so the pin is the half crossing itself. On the upset
// branch 30*10^(21/3000) = 30.487... rounds down while gap 22 lands on
// 30.510... and rounds up.
func TestRatingDeltasRoundHalfCrossing(t *testing.T) {
	d21, _, _, _ := RatingDeltas(0, 21, RedWins)
	if d21 != 30 {
		t.Errorf("upset gap 21: dRed = %d, want 30 (30.487 rounds down)", d21)
	}
	d22, dBlue22, afterRed22, afterBlue22 := RatingDeltas(0, 22, RedWins)
	if d22 != 31 || dBlue22 != -31 || afterRed22 != 31 || afterBlue22 != -9 {
		t.Errorf("upset gap 22: got %d/%d/%d/%d, want 31/-31/31/-9 (30.510 rounds up)",
			d22, dBlue22, afterRed22, afterBlue22)
	}
}

func TestRatingLawProperties(t *testing.T) {
	grid := []int{-2000, -500, -1, 0, 1, 199, 200, 201, 400, 601, 800, 1000, 1001, 1500, 3000}
	for _, rWinner := range grid {
		for _, rLoser := range grid {
			dRed, dBlue, afterRed, afterBlue := RatingDeltas(rWinner, rLoser, RedWins)
			if dRed+dBlue != 0 {
				t.Fatalf("zero-sum broken at %d/%d: %d+%d != 0", rWinner, rLoser, dRed, dBlue)
			}
			if afterRed != rWinner+dRed || afterBlue != rLoser+dBlue {
				t.Fatalf("afters not pre+delta at %d/%d", rWinner, rLoser)
			}
			if rWinner <= rLoser && dRed < config.RatingDelta {
				t.Fatalf("upset winner %d over loser %d gained %d, under %d",
					rWinner, rLoser, dRed, config.RatingDelta)
			}
			if rWinner > rLoser && dRed > config.RatingDelta {
				t.Fatalf("favorite winner %d over loser %d gained %d, over %d",
					rWinner, rLoser, dRed, config.RatingDelta)
			}
		}
	}
	for _, rLoser := range grid {
		gaps := []int{1, 2, 3, 4, 5, 10, 21, 22, 23, 50}
		for _, g := range gaps {
			up, _, _, _ := RatingDeltas(rLoser-g, rLoser, RedWins)
			fav, _, _, _ := RatingDeltas(rLoser+g, rLoser, RedWins)
			if up < fav {
				t.Fatalf("loser %d gap %d: upset %d under favorite %d", rLoser, g, up, fav)
			}
			if g >= 22 && up <= fav {
				t.Fatalf("loser %d gap %d: upset %d not strictly over favorite %d", rLoser, g, up, fav)
			}
		}
	}
}

func TestRatingDeltasInvalidOutcomePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("RatingDeltas with Outcome(42) must panic")
		}
	}()
	RatingDeltas(100, 100, Outcome(42))
}

func TestRatingOutcomeString(t *testing.T) {
	for _, c := range []struct {
		o    Outcome
		want string
	}{{RedWins, "red"}, {BlueWins, "blue"}, {Draw, "draw"}, {Outcome(9), "unknown"}} {
		if got := c.o.String(); got != c.want {
			t.Errorf("Outcome(%d).String() = %q, want %q", int(c.o), got, c.want)
		}
	}
}
