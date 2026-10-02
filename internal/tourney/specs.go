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
func mustTC(initialSec, incrementSec int) int {
	idx, ok := config.TCIndex(initialSec, incrementSec)
	if !ok {
		panic(fmt.Sprintf("tourney: spec time control %d+%d is not configured", initialSec, incrementSec))
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
