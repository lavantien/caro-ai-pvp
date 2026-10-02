package tourney

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

// TestFoldMatchesServerLaw chains the fold against direct server.RatingDeltas
// calls: every rating equals the law's own chaining and the field stays
// zero-sum around the run's start.
func TestFoldMatchesServerLaw(t *testing.T) {
	start := config.TournamentStartRating
	games := []foldedGame{
		{RedSlot: 0, BlueSlot: 1, Outcome: server.RedWins},
		{RedSlot: 1, BlueSlot: 0, Outcome: server.RedWins},
		{RedSlot: 0, BlueSlot: 2, Outcome: server.Draw},
		{RedSlot: 2, BlueSlot: 0, Outcome: server.BlueWins},
	}
	ratings := map[int]int{0: start, 1: start, 2: start}
	for _, g := range games {
		_, _, afterRed, afterBlue := server.RatingDeltas(ratings[g.RedSlot], ratings[g.BlueSlot], g.Outcome)
		ratings[g.RedSlot], ratings[g.BlueSlot] = afterRed, afterBlue
	}
	by := foldStandings(start, []int{0, 1, 2}, games, nil)
	sum := 0
	for slot := range ratings {
		if by[slot].Rating != ratings[slot] {
			t.Errorf("slot %d rating = %d, want the law's %d", slot, by[slot].Rating, ratings[slot])
		}
		sum += by[slot].Rating
	}
	if sum != 3*start {
		t.Errorf("ratings sum = %d, want the zero-sum %d", sum, 3*start)
	}
}

func TestFoldDrawsMoveNothing(t *testing.T) {
	by := foldStandings(config.TournamentStartRating, []int{0, 1},
		[]foldedGame{{RedSlot: 0, BlueSlot: 1, Outcome: server.Draw}, {RedSlot: 1, BlueSlot: 0, Outcome: server.Draw}}, nil)
	for slot, want := range map[int]Standings{
		0: {Slot: 0, Rating: config.TournamentStartRating, Draws: 2, GamesPlayed: 2},
		1: {Slot: 1, Rating: config.TournamentStartRating, Draws: 2, GamesPlayed: 2},
	} {
		if by[slot] != want {
			t.Errorf("slot %d standings = %+v, want %+v", slot, by[slot], want)
		}
	}
}

func TestFoldWLDSeriesWon(t *testing.T) {
	games := []foldedGame{
		{RedSlot: 0, BlueSlot: 1, Outcome: server.RedWins},
		{RedSlot: 1, BlueSlot: 0, Outcome: server.RedWins},
		{RedSlot: 0, BlueSlot: 1, Outcome: server.BlueWins},
	}
	by := foldStandings(config.TournamentStartRating, []int{0, 1, 2}, games, []int{1})
	for slot, want := range map[int]struct {
		wins, losses, draws, series, played int
	}{
		0: {wins: 1, losses: 2, played: 3},
		1: {wins: 2, losses: 1, series: 1, played: 3},
		2: {},
	} {
		st := by[slot]
		if st.Wins != want.wins || st.Losses != want.losses || st.Draws != want.draws ||
			st.SeriesWon != want.series || st.GamesPlayed != want.played {
			t.Errorf("slot %d = W%d L%d D%d series%d of %d games, want W%d L%d D%d series%d of %d",
				slot, st.Wins, st.Losses, st.Draws, st.SeriesWon, st.GamesPlayed,
				want.wins, want.losses, want.draws, want.series, want.played)
		}
	}
}

func TestLeaderboardOrder(t *testing.T) {
	by := map[int]Standings{
		0: {Slot: 0, Rating: 1000, Wins: 1},
		1: {Slot: 1, Rating: 1010, Wins: 0},
		2: {Slot: 2, Rating: 1000, Wins: 3},
		3: {Slot: 3, Rating: 1010, Wins: 2},
		5: {Slot: 5, Rating: 1000, Wins: 3},
	}
	// Rating desc decides first (3 and 1 lead), equal ratings fall to wins
	// desc (3 before 1, 2 and 5 before 0), equal rating and wins fall to
	// slot asc (2 before 5).
	want := []int{3, 1, 2, 5, 0}
	got := leaderboard(by)
	for i, st := range got {
		if st.Slot != want[i] {
			t.Fatalf("order = %v, want %v", slotsOfBoard(got), want)
		}
	}
}

func slotsOfBoard(board []Standings) []int {
	out := make([]int, len(board))
	for i, st := range board {
		out[i] = st.Slot
	}
	return out
}

func TestSettleSeriesBo3(t *testing.T) {
	const red, blue = 10, 20
	cases := []struct {
		name               string
		bo, rw, bw, played int
		wantWinner         int // -1 drawn/none
		wantFinished       bool
	}{
		{"empty", 3, 0, 0, 0, -1, false},
		{"one up", 3, 1, 0, 1, -1, false},
		{"split unfinished", 3, 1, 1, 2, -1, false},
		{"red majority early", 3, 2, 0, 2, red, true},
		{"blue majority early", 3, 0, 2, 2, blue, true},
		{"red majority late", 3, 2, 1, 3, red, true},
		{"exhausted plurality blue", 3, 0, 1, 3, blue, true},
		{"exhausted drawn", 3, 1, 1, 3, -1, true},
		{"bo5 unfinished split", 5, 2, 2, 4, -1, false},
		{"bo5 exhausted plurality", 5, 2, 1, 5, red, true},
		{"bo5 exhausted drawn", 5, 1, 1, 5, -1, true},
	}
	for _, c := range cases {
		winner, finished := settleSeries(c.bo, red, blue, c.rw, c.bw, c.played)
		if finished != c.wantFinished {
			t.Errorf("%s: finished = %v, want %v", c.name, finished, c.wantFinished)
		}
		switch {
		case c.wantWinner == -1:
			if winner != nil {
				t.Errorf("%s: winner = %d, want nil", c.name, *winner)
			}
		case winner == nil:
			t.Errorf("%s: winner = nil, want %d", c.name, c.wantWinner)
		case *winner != c.wantWinner:
			t.Errorf("%s: winner = %d, want %d", c.name, *winner, c.wantWinner)
		}
	}
}
