package main

// The serve composition's adapter from the tourney manager onto the page
// service. The manager owns the tournament domain and stays free of UI
// shapes; the pages own the render shapes and know no tourney type (the
// server package cannot import tourney, tourney already sits on server).
// This file is the one mapping layer between them.

import (
	"context"

	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

// tourneyService implements server.TourneyService over one manager.
type tourneyService struct {
	m *tourney.Manager
}

// newTourneyService adapts the manager the serve composition drives.
func newTourneyService(m *tourney.Manager) tourneyService {
	return tourneyService{m: m}
}

// StartRun hands the page's validated setup to the manager: the run id comes
// back the moment the row persists.
func (s tourneyService) StartRun(ctx context.Context, setup server.TourneySetup) (int64, error) {
	roster := make([]tourney.Participant, len(setup.Seats))
	for i, seat := range setup.Seats {
		roster[i] = tourney.Participant{Slot: seat.Slot, Name: seat.Name, Tier: seat.Tier}
	}
	run, err := s.m.StartRun(ctx, tourney.RunSpec{
		Roster: roster, TCIdx: setup.TCIdx, BOLen: setup.BOLen,
		StartRating: setup.StartRating,
	}, setup.Parallel)
	if err != nil {
		return 0, err
	}
	return run.ID, nil
}

// RunSnapshot maps one run's whole detail onto the page shapes.
func (s tourneyService) RunSnapshot(ctx context.Context, runID int64) (server.TourneySnapshot, error) {
	d, err := s.m.Detail(ctx, runID)
	if err != nil {
		return server.TourneySnapshot{}, err
	}
	snap := server.TourneySnapshot{
		Run:    runInfoOf(d),
		Seats:  seatsOf(d.Roster),
		Series: seriesOf(d.Series),
		Board:  boardOf(d.Board),
	}
	return snap, nil
}

// Runs maps the run headers with each run's roster and standings head.
func (s tourneyService) Runs(ctx context.Context) ([]server.TourneyRunSummary, error) {
	runs, err := s.m.Runs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]server.TourneyRunSummary, len(runs))
	for i, run := range runs {
		d, err := s.m.Detail(ctx, run.ID)
		if err != nil {
			return nil, err
		}
		out[i] = server.TourneyRunSummary{
			TourneyRunInfo: runInfoOf(d),
			Seats:          seatsOf(d.Roster),
			Leader:         leaderName(d),
		}
	}
	return out, nil
}

// runInfoOf maps one detail's header half.
func runInfoOf(d tourney.Detail) server.TourneyRunInfo {
	info := server.TourneyRunInfo{
		ID: d.Run.ID, CreatedAt: d.Run.CreatedAt, TCIdx: d.Run.TCIdx,
		BOLen: d.Run.BOLen, StartRating: d.Run.StartRating,
		Finished: d.Run.Status == tourney.RunStateFinished, Running: d.Running,
	}
	if d.Failure != nil {
		info.Failure = d.Failure.Error()
	}
	return info
}

// seatsOf maps the roster; tier select options aside, the shapes agree.
func seatsOf(roster []tourney.Participant) []server.TourneySeat {
	seats := make([]server.TourneySeat, len(roster))
	for i, p := range roster {
		seats[i] = server.TourneySeat{Slot: p.Slot, Name: p.Name, Tier: p.Tier}
	}
	return seats
}

// seriesOf maps the pairing lines, keeping the winner slots as pointers.
func seriesOf(series []tourney.Series) []server.TourneySeriesLine {
	lines := make([]server.TourneySeriesLine, len(series))
	for i, s := range series {
		lines[i] = server.TourneySeriesLine{
			PairingSlot: s.PairingSlot, RedFirstSlot: s.RedFirstSlot, BlueFirstSlot: s.BlueFirstSlot,
			RedFirstWins: s.RedFirstWins, BlueFirstWins: s.BlueFirstWins,
			WinnerSlot: s.WinnerSlot, Finished: s.FinishedAt != nil,
		}
	}
	return lines
}

// boardOf maps the standings rows.
func boardOf(board []tourney.Standings) []server.TourneyStanding {
	rows := make([]server.TourneyStanding, len(board))
	for i, st := range board {
		rows[i] = server.TourneyStanding{
			Slot: st.Slot, Rating: st.Rating, Wins: st.Wins, Losses: st.Losses,
			Draws: st.Draws, SeriesWon: st.SeriesWon, GamesPlayed: st.GamesPlayed,
		}
	}
	return rows
}

// leaderName resolves the standings head onto its roster name, empty when
// the board is empty.
func leaderName(d tourney.Detail) string {
	if len(d.Board) == 0 {
		return ""
	}
	for _, p := range d.Roster {
		if p.Slot == d.Board[0].Slot {
			return p.Name
		}
	}
	return ""
}
