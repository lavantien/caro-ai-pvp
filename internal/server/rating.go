package server

import (
	"math"

	"github.com/lavantien/caro-ai-pvp/internal/config"
)

// Outcome is one game result from the red/blue color perspective. Draw is the
// zero value so an unset outcome can never award a phantom win.
type Outcome int

const (
	Draw Outcome = iota
	RedWins
	BlueWins
)

func (o Outcome) String() string {
	switch o {
	case Draw:
		return "draw"
	case RedWins:
		return "red"
	case BlueWins:
		return "blue"
	default:
		return "unknown"
	}
}

// RatingDeltas applies the per-match rating law of first-cause.md Scenario 1:
// K = 10^((R_loser-R_winner)/D) where D is RatingDivisorFull when the
// winner's pre-match rating is at or below the loser's (an upset, equality
// included), otherwise the decay band min(RatingDecayMax,
// max(RatingDecayMin, RatingDecayBase+RatingDecaySlope*R_loser)) built from
// the loser's pre-match rating. The winner gains RatingDelta*K rounded with
// math.Round, the loser loses the same amount, a draw moves nothing. Ratings
// start at RatingStart and may go negative. It panics on an Outcome outside
// the enum; callers feed outcomes produced by this package.
func RatingDeltas(rRed, rBlue int, outcome Outcome) (dRed, dBlue, afterRed, afterBlue int) {
	switch outcome {
	case Draw:
		return 0, 0, rRed, rBlue
	case RedWins:
		d := ratingDelta(rRed, rBlue)
		return d, -d, rRed + d, rBlue - d
	case BlueWins:
		d := ratingDelta(rBlue, rRed)
		return -d, d, rRed - d, rBlue + d
	default:
		panic("server: invalid Outcome " + outcome.String())
	}
}

func ratingDelta(rWinner, rLoser int) int {
	return int(math.Round(config.RatingDelta * ratingK(rWinner, rLoser)))
}

func ratingK(rWinner, rLoser int) float64 {
	d := float64(config.RatingDivisorFull)
	if rWinner > rLoser {
		band := float64(config.RatingDecayBase) + config.RatingDecaySlope*float64(rLoser)
		d = math.Min(math.Max(band, config.RatingDecayMin), config.RatingDecayMax)
	}
	return math.Pow(10, float64(rLoser-rWinner)/d)
}
