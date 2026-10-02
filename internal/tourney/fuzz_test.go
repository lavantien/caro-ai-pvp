package tourney

import (
	"slices"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// FuzzPairings holds the schedule invariants over arbitrary sizes: every
// roster from 1 to 8 participants yields exactly n*(n-1) series, every
// ordered pair of distinct slots exactly once, none twice, and the mirrored
// layout keeps position k from the end the color-swap of position k.
func FuzzPairings(f *testing.F) {
	for _, n := range []int{1, 2, 3, 6, 8} {
		f.Add(n)
	}
	f.Fuzz(func(t *testing.T, n int) {
		n = 1 + ((n%8)+8)%8
		got := pairs(Pairings(roster(n)))
		if len(got) != n*(n-1) {
			t.Fatalf("n=%d: %d pairings, want %d", n, len(got), n*(n-1))
		}
		seen := make(map[[2]int]int, len(got))
		for _, p := range got {
			seen[p]++
			if p[0] == p[1] {
				t.Fatalf("n=%d: self-pairing %v", n, p)
			}
			if p[0] < 0 || p[0] >= n || p[1] < 0 || p[1] >= n {
				t.Fatalf("n=%d: pairing %v outside the roster", n, p)
			}
		}
		for a := range n {
			for b := range n {
				want := 1
				if a == b {
					want = 0
				}
				if seen[[2]int{a, b}] != want {
					t.Fatalf("n=%d: ordered pair (%d,%d) appears %d times, want %d", n, a, b, seen[[2]int{a, b}], want)
				}
			}
		}
		if !slices.Equal(got, pairs(Pairings(roster(n)))) {
			t.Fatalf("n=%d: schedule differs across calls", n)
		}
		for k := range got {
			last := len(got) - 1 - k
			if got[last] != [2]int{got[k][1], got[k][0]} {
				t.Fatalf("n=%d: position %d from the end is %v, want the swap of %v", n, k, got[last], got[k])
			}
		}
	})
}

// FuzzFoldZeroSum throws arbitrary outcome streams at the fold: the chain
// must equal direct server.RatingDeltas calls, the field stays zero-sum
// around the start, and the game tallies count every game on both sides.
func FuzzFoldZeroSum(f *testing.F) {
	f.Add([]byte{0, 1, 2, 0, 1, 2})
	f.Add([]byte{})
	f.Add([]byte{1, 1, 1, 1})
	f.Fuzz(func(t *testing.T, data []byte) {
		const start = 1000
		ratings := [3]int{start, start, start}
		games := make([]foldedGame, 0, len(data))
		for _, b := range data {
			red := int(b) % 3
			blue := (red + 1 + int(b>>4)%2) % 3
			g := foldedGame{RedSlot: red, BlueSlot: blue, Outcome: server.Outcome(int(b) % 3)}
			_, _, afterRed, afterBlue := server.RatingDeltas(ratings[g.RedSlot], ratings[g.BlueSlot], g.Outcome)
			ratings[g.RedSlot], ratings[g.BlueSlot] = afterRed, afterBlue
			games = append(games, g)
		}
		by := foldStandings(start, []int{0, 1, 2}, games, nil)
		sum, played := 0, 0
		for slot := range 3 {
			if by[slot].Rating != ratings[slot] {
				t.Fatalf("slot %d rating = %d, want the law's %d", slot, by[slot].Rating, ratings[slot])
			}
			sum += by[slot].Rating
			played += by[slot].GamesPlayed
		}
		if sum != 3*start {
			t.Fatalf("zero-sum torn by %d games: sum %d", len(games), sum)
		}
		if played != 2*len(games) {
			t.Fatalf("games played = %d, want both sides of %d games", played, len(games))
		}
	})
}
