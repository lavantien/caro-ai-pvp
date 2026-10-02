package tourney

import (
	"slices"

	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// Standings is one participant's folded record: the rating chained from the
// run's start rating through the per-match law, plus the leaderboard
// tallies. Ratings live only in this fold and the close-of-run snapshot;
// tournament_games is the single source of truth and foldStandings is its
// only reader, so nothing can drift.
type Standings struct {
	Slot        int
	Rating      int
	Wins        int
	Losses      int
	Draws       int
	SeriesWon   int
	GamesPlayed int
}

// foldedGame is the projection of one persisted tournament game the fold
// consumes, in play order.
type foldedGame struct {
	RedSlot  int
	BlueSlot int
	Outcome  server.Outcome
}

// foldStandings replays games through server.RatingDeltas chained on start
// (the same law the PvP rating events ride, seeded per run instead of per
// user), counts W-L-D per slot, and credits one series won per decided
// series winner. seriesWinners may be partial while a run is in flight.
func foldStandings(start int, slots []int, games []foldedGame, seriesWinners []int) map[int]Standings {
	by := make(map[int]Standings, len(slots))
	for _, s := range slots {
		by[s] = Standings{Slot: s, Rating: start}
	}
	for _, g := range games {
		red, blue := by[g.RedSlot], by[g.BlueSlot]
		red.GamesPlayed++
		blue.GamesPlayed++
		switch g.Outcome {
		case server.RedWins:
			red.Wins++
			blue.Losses++
		case server.BlueWins:
			blue.Wins++
			red.Losses++
		case server.Draw:
			red.Draws++
			blue.Draws++
		}
		_, _, afterRed, afterBlue := server.RatingDeltas(red.Rating, blue.Rating, g.Outcome)
		red.Rating, blue.Rating = afterRed, afterBlue
		by[g.RedSlot], by[g.BlueSlot] = red, blue
	}
	for _, w := range seriesWinners {
		st := by[w]
		st.SeriesWon++
		by[w] = st
	}
	return by
}

// leaderboard orders the fold: rating desc, then wins desc, then slot asc.
func leaderboard(by map[int]Standings) []Standings {
	out := make([]Standings, 0, len(by))
	for _, st := range by {
		out = append(out, st)
	}
	slices.SortFunc(out, func(a, b Standings) int {
		if a.Rating != b.Rating {
			return b.Rating - a.Rating
		}
		if a.Wins != b.Wins {
			return b.Wins - a.Wins
		}
		return a.Slot - b.Slot
	})
	return out
}

// settleSeries applies the bo majority law the room series rides:
// boLen/2+1 wins ends a series early, a full schedule crowns the plurality
// leader, and equal wins at exhaustion leave the series drawn (nil winner).
func settleSeries(boLen, redFirstSlot, blueFirstSlot, redFirstWins, blueFirstWins, gamesPlayed int) (winner *int, finished bool) {
	need := boLen/2 + 1
	switch {
	case redFirstWins >= need:
		return &redFirstSlot, true
	case blueFirstWins >= need:
		return &blueFirstSlot, true
	}
	if gamesPlayed < boLen {
		return nil, false
	}
	switch {
	case redFirstWins > blueFirstWins:
		return &redFirstSlot, true
	case blueFirstWins > redFirstWins:
		return &blueFirstSlot, true
	}
	return nil, true
}
