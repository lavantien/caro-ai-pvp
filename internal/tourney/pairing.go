// Package tourney is the pure core of the M7 tournament conductor of
// first-cause.md Scenario 2: the twice-pair round robin schedule, the
// tournament rating space over the landed per-match law, schema v3
// persistence, and the per-series txt run logs. It drives no rooms: the
// conductor feeds finished games in and reads the leaderboard out.
package tourney

// Participant is one roster entry: a slot indexing the run's roster plus
// the display identity of Scenario 2's setup screen.
type Participant struct {
	Slot int
	Name string
	Tier string
}

// Pairing is one scheduled series: RedFirst is the participant hosting
// game 1 with red, BlueFirst the other.
type Pairing struct {
	RedFirst  Participant
	BlueFirst Participant
}

// Pairings lays out the twice-pair round robin: every unordered pair meets
// exactly twice, once with each side red-first, n*(n-1) series for n
// participants. The schedule is the lexicographic (a, b) first half
// followed by its color-swaps in reverse, so the run mirrors itself:
// position k from the end is the swap of position k.
func Pairings(roster []Participant) []Pairing {
	n := len(roster)
	out := make([]Pairing, 0, n*(n-1))
	for a := 0; a < n; a++ {
		for b := a + 1; b < n; b++ {
			out = append(out, Pairing{RedFirst: roster[a], BlueFirst: roster[b]})
		}
	}
	for k := len(out) - 1; k >= 0; k-- {
		p := out[k]
		out = append(out, Pairing{RedFirst: p.BlueFirst, BlueFirst: p.RedFirst})
	}
	return out
}
