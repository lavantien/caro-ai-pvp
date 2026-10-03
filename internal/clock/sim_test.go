package clock

import (
	"fmt"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Full-game adversarial simulations against the public clock API. Every
// scenario plays a whole side for the full-board bound of moves under one
// seeded cost model and tries to break the never-flag invariant or the
// grant bounds over horizons far past the law's 30-move anchor.

const (
	simMovesPerSide = 128
	simSeedBase     = 0x5EEDF00D
	simSeedHi       = 0xC10CCAFE
)

// simCost charges one move given the granted budget. The move index starts
// at 1; the rng is the game's own seeded stream.
type simCost func(rng *rand.Rand, move int, budget time.Duration) time.Duration

type simGame struct {
	label   string
	tcIdx   int
	seedLo  uint64
	noHoard bool
	rng     *rand.Rand
	cost    simCost
}

func simGames() []simGame {
	var games []simGame
	add := func(label string, tcIdx int, noHoard bool, cost simCost) {
		seedLo := simSeedBase + uint64(len(games))
		games = append(games, simGame{
			label:   label,
			tcIdx:   tcIdx,
			seedLo:  seedLo,
			noHoard: noHoard,
			rng:     rand.New(rand.NewPCG(seedLo, simSeedHi)),
			cost:    cost,
		})
	}
	flatOvershootsMs := []float64{1, 10, 50, 100, config.SearchSafetyMarginMs}
	for tcIdx := range config.TimeControls {
		add("exact", tcIdx, true, func(_ *rand.Rand, _ int, budget time.Duration) time.Duration {
			return budget
		})
		for _, ovMs := range flatOvershootsMs {
			overshoot := time.Duration(ovMs) * time.Millisecond
			add(fmt.Sprintf("overshoot-flat/%gms", ovMs), tcIdx, ovMs <= 10, func(_ *rand.Rand, _ int, budget time.Duration) time.Duration {
				return budget + overshoot
			})
		}
		add("overshoot-fraction", tcIdx, false, func(rng *rand.Rand, _ int, budget time.Duration) time.Duration {
			return time.Duration(float64(budget) * (1 + 0.5*rng.Float64()))
		})
		add("spiky", tcIdx, false, func(_ *rand.Rand, move int, budget time.Duration) time.Duration {
			if move%2 == 1 {
				return 0
			}
			return budget + 50*time.Millisecond
		})
		add("random", tcIdx, false, func(rng *rand.Rand, _ int, budget time.Duration) time.Duration {
			return time.Duration(3 * float64(budget) * rng.Float64())
		})
		add("adversarial-max", tcIdx, false, func(_ *rand.Rand, _ int, budget time.Duration) time.Duration {
			return budget + time.Duration(config.SearchSafetyMarginMs-1)*time.Millisecond
		})
	}
	return games
}

func TestSimFullGameInvariants(t *testing.T) {
	minDur := time.Duration(config.SearchMinMoveTimeMs) * time.Millisecond
	marginNs := int64(config.SearchSafetyMarginMs) * int64(time.Millisecond)
	for _, g := range simGames() {
		t.Run(fmt.Sprintf("%s/tc%d", g.label, g.tcIdx), func(t *testing.T) {
			c := NewGameClock(g.tcIdx)
			initial := time.Duration(config.TimeControls[g.tcIdx].InitialMin) * time.Minute
			var budgetSum time.Duration
			for move := 1; move <= simMovesPerSide; move++ {
				budget := c.Budget()
				remAtGrant := c.Remaining()
				c.Commit(g.cost(g.rng, move, budget))
				budgetSum += budget
				if rem := c.Remaining(); rem < 0 {
					t.Fatalf("game %s seed %#x/%#x move %d: remaining %v went negative", g.label, g.seedLo, simSeedHi, move, rem)
				}
				if budget < minDur {
					t.Fatalf("game %s seed %#x/%#x move %d: budget %v below floor %v", g.label, g.seedLo, simSeedHi, move, budget, minDur)
				}
				// Independent ceiling recompute in exact integer nanoseconds
				// from the public remaining at grant time. A float ns-ms-ns
				// round trip rounds the bound down by 1ns on near-integer
				// remainings and would fake a violation where none exists.
				hiNs := int64(remAtGrant) - marginNs
				if lo := int64(minDur); hiNs < lo {
					hiNs = lo
				}
				if got := int64(budget); got > hiNs {
					t.Fatalf("game %s seed %#x/%#x move %d: budget %dns above independent ceiling %dns (remaining at grant %v)", g.label, g.seedLo, simSeedHi, move, got, hiNs, remAtGrant)
				}
				if got := c.Moves(); got != move {
					t.Fatalf("game %s move %d: moves counter %d", g.label, move, got)
				}
			}
			t.Logf("final remaining %v, average budget %v", c.Remaining(), budgetSum/simMovesPerSide)
			if g.noHoard && c.Remaining() >= initial {
				t.Fatalf("game %s: clock hoarded time, final remaining %v >= initial %v", g.label, c.Remaining(), initial)
			}
		})
	}
}

func TestSimPollCountDoesNotChangeCommitCharge(t *testing.T) {
	for tcIdx := range config.TimeControls {
		for _, k := range []int{1, 2, 3, 5, 8, 13} {
			for _, elapsed := range []time.Duration{0, time.Millisecond, 10 * time.Millisecond, 100 * time.Millisecond, time.Second, time.Hour} {
				ref := NewGameClock(tcIdx)
				ref.Commit(time.Millisecond)
				want := ref.Budget()
				ref.Commit(elapsed)

				pol := NewGameClock(tcIdx)
				pol.Commit(time.Millisecond)
				for i := 0; i < k; i++ {
					if got := pol.Budget(); got != want {
						t.Fatalf("tc %d k %d poll %d: grant drifted, %v != %v", tcIdx, k, i, got, want)
					}
				}
				pol.Commit(elapsed)
				if pol.Remaining() != ref.Remaining() {
					t.Fatalf("tc %d k %d elapsed %v: remaining after %d polls = %v, k=1 gives %v", tcIdx, k, elapsed, k, pol.Remaining(), ref.Remaining())
				}
				if pol.Budget() != ref.Budget() {
					t.Fatalf("tc %d k %d elapsed %v: next budget diverged, %v != %v", tcIdx, k, elapsed, pol.Budget(), ref.Budget())
				}
			}
		}
	}
}

func TestSimInterleavedPollingWholeGame(t *testing.T) {
	for tcIdx := range config.TimeControls {
		ref := NewGameClock(tcIdx)
		pol := NewGameClock(tcIdx)
		rng := rand.New(rand.NewPCG(simSeedBase+0xB0A0, simSeedHi+uint64(tcIdx)))
		for move := 1; move <= simMovesPerSide; move++ {
			want := ref.Budget()
			for i := 0; i < 1+rng.IntN(7); i++ {
				if got := pol.Budget(); got != want {
					t.Fatalf("tc %d move %d poll %d: over-polled grant %v != %v", tcIdx, move, i, got, want)
				}
			}
			elapsed := time.Duration(float64(want) * (1 + 0.5*rng.Float64()))
			ref.Commit(elapsed)
			pol.Commit(elapsed)
			if pol.Remaining() != ref.Remaining() {
				t.Fatalf("tc %d move %d: over-polled remaining %v, single-poll %v", tcIdx, move, pol.Remaining(), ref.Remaining())
			}
			if pol.Moves() != ref.Moves() {
				t.Fatalf("tc %d move %d: over-polled moves %d, single-poll %d", tcIdx, move, pol.Moves(), ref.Moves())
			}
		}
	}
}
