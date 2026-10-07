package tourney

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// RunSpec names one headless driver's shape: the roster, the time control,
// the bo length, and the start rating the conductor seeds every participant
// with.
type RunSpec struct {
	Roster      []Participant
	TCIdx       int
	BOLen       int
	StartRating int
}

// twoPerTierRoster builds the config hub's default roster, 2 per tier: the
// smallest twice-pair round robin in which every spec matchup type
// (hard-hard, hard-medium, medium-medium, medium-easy, hard-easy,
// easy-easy) actually meets, once with each instance red-first.
func twoPerTierRoster() []Participant {
	def := config.DefaultRoster()
	out := make([]Participant, len(def))
	for i, seat := range def {
		out[i] = Participant{Slot: i, Name: seat.Name, Tier: seat.Tier}
	}
	return out
}

// mustTC resolves a clock shape onto its table index; the drivers name their
// time controls, never positions.
func mustTC(initialMin, incrementSec int) int {
	idx, ok := config.TCIndex(initialMin, incrementSec)
	if !ok {
		panic(fmt.Sprintf("tourney: spec time control %d+%d is not configured", initialMin, incrementSec))
	}
	return idx
}

// SmokeRoster32 is the Implication 2.2 headless smoke driver: every matchup
// type at the 3+2 time control, bo3.
func SmokeRoster32() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(3, 2),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

// SmokeRoster10 is the Implication 2.3 headless smoke driver: every matchup
// type at the 1+0 time control, bo3.
func SmokeRoster10() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

// FullRoster24 is the Implication 2.4 full run: the 6 bots, 2 per tier,
// twice-pair round robin at 2+1, bo3, seeded at the spec's 1000.
func FullRoster24() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(2, 1),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

// SmokeRoster105 is the 10+5 plumbing smoke: two hard seats meeting twice
// at the fourth time control, bo3, small enough to drive the conductor path
// end to end inside one night window at the slower clock.
func SmokeRoster105() RunSpec {
	return RunSpec{
		Roster: []Participant{
			{Slot: 0, Name: "hard-1", Tier: config.TierHard.Name},
			{Slot: 1, Name: "hard-2", Tier: config.TierHard.Name},
		},
		TCIdx: mustTC(10, 5),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}
