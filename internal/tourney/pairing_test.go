package tourney

import (
	"fmt"
	"slices"
	"testing"
)

// roster builds the canonical test roster: n participants whose slots equal
// their positions, the contract CreateRun enforces.
func roster(n int) []Participant {
	out := make([]Participant, n)
	for i := range out {
		out[i] = Participant{Slot: i, Name: fmt.Sprintf("bot-%d", i), Tier: "hard"}
	}
	return out
}

// pairs projects a schedule onto its seat slots.
func pairs(ps []Pairing) [][2]int {
	out := make([][2]int, len(ps))
	for i, p := range ps {
		out[i] = [2]int{p.RedFirst.Slot, p.BlueFirst.Slot}
	}
	return out
}

func TestPairingsPinsN2(t *testing.T) {
	got := pairs(Pairings(roster(2)))
	want := [][2]int{{0, 1}, {1, 0}}
	if !slices.Equal(got, want) {
		t.Fatalf("n=2 pairings = %v, want %v", got, want)
	}
}

// TestPairingsPinsN6 pins the Implication 2.4 roster shape: 6 bots, 2 per
// tier, 30 series in the mirrored twice-pair layout.
func TestPairingsPinsN6(t *testing.T) {
	got := pairs(Pairings(roster(6)))
	if len(got) != 30 {
		t.Fatalf("n=6 schedule holds %d series, want n*(n-1) = 30", len(got))
	}
	if got[0] != [2]int{0, 1} || got[14] != [2]int{4, 5} {
		t.Errorf("first half runs %v..%v, want the lexicographic (0,1)..(4,5)", got[0], got[14])
	}
	if got[15] != [2]int{5, 4} || got[29] != [2]int{1, 0} {
		t.Errorf("second half runs %v..%v, want the mirrored swaps (5,4)..(1,0)", got[15], got[29])
	}
}

func TestPairingsProperties(t *testing.T) {
	for n := 1; n <= 8; n++ {
		got := pairs(Pairings(roster(n)))
		if len(got) != n*(n-1) {
			t.Errorf("n=%d: %d pairings, want n*(n-1) = %d", n, len(got), n*(n-1))
		}
		seen := make(map[[2]int]int, len(got))
		for _, p := range got {
			seen[p]++
			if p[0] == p[1] {
				t.Errorf("n=%d: self-pairing %v", n, p)
			}
		}
		for a := 0; a < n; a++ {
			for b := 0; b < n; b++ {
				want := 1
				if a == b {
					want = 0
				}
				if seen[[2]int{a, b}] != want {
					t.Errorf("n=%d: ordered pair (%d,%d) appears %d times, want %d", n, a, b, seen[[2]int{a, b}], want)
				}
			}
		}
		if again := pairs(Pairings(roster(n))); !slices.Equal(got, again) {
			t.Errorf("n=%d: schedule differs across calls", n)
		}
		for k := range got {
			last := len(got) - 1 - k
			if got[last] != [2]int{got[k][1], got[k][0]} {
				t.Errorf("n=%d: position %d from the end is %v, want the color-swap of %v", n, k, got[last], got[k])
				break
			}
		}
	}
}
