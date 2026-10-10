package tourney

import (
	"fmt"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

type RunSpec struct {
	Roster      []Participant
	TCIdx       int
	BOLen       int
	StartRating int
}

func twoPerTierRoster() []Participant {
	def := config.DefaultRoster()
	out := make([]Participant, len(def))
	for i, seat := range def {
		out[i] = Participant{Slot: i, Name: seat.Name, Tier: seat.Tier}
	}
	return out
}

func mustTC(initialMin, incrementSec int) int {
	idx, ok := config.TCIndex(initialMin, incrementSec)
	if !ok {
		panic(fmt.Sprintf("tourney: spec time control %d+%d is not configured", initialMin, incrementSec))
	}
	return idx
}

func SmokeRoster32() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(3, 2),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

func SmokeRoster10() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

func FullRoster24() RunSpec {
	return RunSpec{
		Roster: twoPerTierRoster(), TCIdx: mustTC(2, 1),
		BOLen: config.SeriesBO3, StartRating: config.TournamentStartRating,
	}
}

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

func PonderProbe() RunSpec {
	return RunSpec{
		Roster: []Participant{
			{Slot: 0, Name: "master-1", Tier: config.TierMaster.Name},
			{Slot: 1, Name: "master-2", Tier: config.TierMaster.Name},
		},
		TCIdx: mustTC(1, 0),
		BOLen: config.SeriesBO11, StartRating: config.TournamentStartRating,
	}
}
