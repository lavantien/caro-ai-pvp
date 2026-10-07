package main

// scriptTourney stands in for the conductor: every tournament surface the
// pages render (setup runs list, running run with live boards, finished run
// with a frozen leaderboard, home banner) reads this script, never a drive.
// The leaderboard derives from the scripted series lines so the arithmetic
// cannot drift.

import (
	"context"
	"sync/atomic"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

const (
	finishedRunID = 3
	runningRunID  = 4
)

type scriptTourney struct {
	bannerOn atomic.Bool
}

func newScriptTourney() *scriptTourney { return &scriptTourney{} }

func (t *scriptTourney) StartRun(context.Context, server.TourneySetup) (int64, error) {
	return 0, &server.TourneyBlockedError{RunID: runningRunID}
}

func (t *scriptTourney) CloseStalledRun(context.Context, int64) error { return nil }

func (t *scriptTourney) Runs(context.Context) ([]server.TourneyRunSummary, error) {
	finished, _ := t.finishedSnapshot()
	running, _ := t.runningSnapshot()
	return []server.TourneyRunSummary{summaryOf(running), summaryOf(finished)}, nil
}

func (t *scriptTourney) RunSnapshot(_ context.Context, runID int64) (server.TourneySnapshot, error) {
	switch runID {
	case finishedRunID:
		return t.finishedSnapshot()
	case runningRunID:
		return t.runningSnapshot()
	}
	return server.TourneySnapshot{}, server.ErrNotFound
}

func (t *scriptTourney) OngoingRun(context.Context) (server.TourneyBanner, bool, error) {
	if !t.bannerOn.Load() {
		return server.TourneyBanner{}, false, nil
	}
	return server.TourneyBanner{RunID: runningRunID, Done: 7, Total: 12}, true, nil
}

func (t *scriptTourney) LiveBoards() []server.TourneyLiveBoard {
	return []server.TourneyLiveBoard{{
		RoomID: "4d2e7f90a1b3c5d6", RedName: "hard-1", BlueName: "hard-2",
		RedWins: 1, BlueWins: 2, Turn: "blue",
		Moves: []string{"H8", "I9", "H10", "J9", "K10", "J10"},
	}}
}

// meet is one scripted series: the seats and the game score from the
// red-first seat's perspective.
type meet struct{ a, b, redWins, blueWins int }

// finishedMeets scripts the 12-series, 4-bot finished run: hard-2 sweeps,
// hard-1 second, medium-1 third, easy-1 last.
func finishedMeets() []meet {
	return []meet{
		{0, 1, 2, 1}, {1, 0, 1, 2},
		{0, 2, 2, 0}, {2, 0, 1, 2},
		{0, 3, 2, 0}, {3, 0, 0, 2},
		{1, 2, 2, 1}, {2, 1, 2, 0},
		{1, 3, 2, 0}, {3, 1, 2, 1},
		{2, 3, 2, 1}, {3, 2, 1, 2},
	}
}

// runningMeets scripts the 2-bot run mid-flight: series 1 settled 2-1 for
// hard-2 (1-2 from the red-first seat's perspective), series 2 at 1-1 and
// playing.
func runningMeets() []server.TourneySeriesLine {
	return []server.TourneySeriesLine{
		{PairingSlot: 0, RedFirstSlot: 0, BlueFirstSlot: 1, RedFirstWins: 1, BlueFirstWins: 2,
			WinnerSlot: intPtr(1), Finished: true},
		{PairingSlot: 1, RedFirstSlot: 1, BlueFirstSlot: 0, RedFirstWins: 1, BlueFirstWins: 1},
	}
}

func intPtr(i int) *int { return &i }

func finishedSeats() []server.TourneySeat {
	return []server.TourneySeat{
		{Slot: 0, Name: "hard-2", Tier: config.TierHard.Name},
		{Slot: 1, Name: "hard-1", Tier: config.TierHard.Name},
		{Slot: 2, Name: "medium-1", Tier: config.TierMedium.Name},
		{Slot: 3, Name: "easy-1", Tier: config.TierEasy.Name},
	}
}

func (t *scriptTourney) finishedSnapshot() (server.TourneySnapshot, error) {
	seats := finishedSeats()
	series := make([]server.TourneySeriesLine, 0, len(finishedMeets()))
	board := make([]server.TourneyStanding, len(seats))
	ratings := []int{1204, 1176, 982, 638}
	for i := range seats {
		board[i] = server.TourneyStanding{Slot: i, Rating: ratings[i]}
	}
	for i, m := range finishedMeets() {
		series = append(series, server.TourneySeriesLine{
			PairingSlot: i / 2, RedFirstSlot: m.a, BlueFirstSlot: m.b,
			RedFirstWins: m.redWins, BlueFirstWins: m.blueWins, Finished: true,
		})
		tallyMeet(&board[m.a], &board[m.b], m)
	}
	return server.TourneySnapshot{
		Run: server.TourneyRunInfo{
			ID: finishedRunID, CreatedAt: 1791000000, TCIdx: stageTCIdx, BOLen: stageBO,
			StartRating: config.TournamentStartRating, Finished: true,
		},
		Seats: seats, Series: series, Board: board,
	}, nil
}

func (t *scriptTourney) runningSnapshot() (server.TourneySnapshot, error) {
	seats := []server.TourneySeat{
		{Slot: 0, Name: "hard-1", Tier: config.TierHard.Name},
		{Slot: 1, Name: "hard-2", Tier: config.TierHard.Name},
	}
	board := []server.TourneyStanding{{Slot: 0, Rating: 988}, {Slot: 1, Rating: 1012}}
	for _, line := range runningMeets() {
		red, blue := &board[line.RedFirstSlot], &board[line.BlueFirstSlot]
		red.Wins += line.RedFirstWins
		blue.Wins += line.BlueFirstWins
		red.Losses += line.BlueFirstWins
		blue.Losses += line.RedFirstWins
		red.GamesPlayed += line.RedFirstWins + line.BlueFirstWins
		blue.GamesPlayed += line.RedFirstWins + line.BlueFirstWins
		if line.Finished && line.WinnerSlot != nil {
			board[*line.WinnerSlot].SeriesWon++
		}
	}
	return server.TourneySnapshot{
		Run: server.TourneyRunInfo{
			ID: runningRunID, CreatedAt: 1791300000, TCIdx: stageTCIdx, BOLen: stageBO,
			StartRating: config.TournamentStartRating, Running: true,
		},
		Seats: seats, Series: runningMeets(), Board: board,
	}, nil
}

// tallyMeet folds one series into the standings: wins, losses, games played,
// and the series win for the seat that took it.
func tallyMeet(a, b *server.TourneyStanding, m meet) {
	a.Wins += m.redWins
	a.Losses += m.blueWins
	b.Wins += m.blueWins
	b.Losses += m.redWins
	a.GamesPlayed += m.redWins + m.blueWins
	b.GamesPlayed += m.redWins + m.blueWins
	if m.redWins > m.blueWins {
		a.SeriesWon++
	} else {
		b.SeriesWon++
	}
}

// summaryOf flattens a snapshot into the setup page's runs-list row.
func summaryOf(sn server.TourneySnapshot) server.TourneyRunSummary {
	s := server.TourneyRunSummary{TourneyRunInfo: sn.Run, Seats: sn.Seats}
	best := -1
	for _, st := range sn.Board {
		if st.Rating > best {
			best = st.Rating
			s.Leader = seatName(sn.Seats, st.Slot)
		}
	}
	return s
}

func seatName(seats []server.TourneySeat, slot int) string {
	for _, s := range seats {
		if s.Slot == slot {
			return s.Name
		}
	}
	return ""
}
