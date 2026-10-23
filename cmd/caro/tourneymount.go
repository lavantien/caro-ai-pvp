package main

import (
	"context"
	"errors"

	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

type tourneyService struct {
	m     *tourney.Manager
	rooms *server.RoomManager
}

func newTourneyService(m *tourney.Manager, rooms *server.RoomManager) tourneyService {
	return tourneyService{m: m, rooms: rooms}
}

func (s tourneyService) StartRun(ctx context.Context, setup server.TourneySetup) (int64, error) {
	roster := make([]tourney.Participant, len(setup.Seats))
	for i, seat := range setup.Seats {
		roster[i] = tourney.Participant{Slot: seat.Slot, Name: seat.Name, Tier: seat.Tier}
	}
	run, err := s.m.StartRun(ctx, tourney.RunSpec{
		Roster: roster, TCIdx: setup.TCIdx, BOLen: setup.BOLen,
		StartRating: setup.StartRating, Variant: setup.Variant,
	}, setup.Parallel)
	if err != nil {
		var gate *tourney.RunInProgressError
		if errors.As(err, &gate) {
			return 0, &server.TourneyBlockedError{RunID: gate.RunID}
		}
		return 0, err
	}
	return run.ID, nil
}

func (s tourneyService) CloseStalledRun(ctx context.Context, runID int64) error {
	return s.m.CloseStalled(ctx, runID)
}

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

func (s tourneyService) OngoingRun(ctx context.Context) (server.TourneyBanner, bool, error) {
	d, ok, err := s.m.OngoingRun(ctx)
	if err != nil || !ok {
		return server.TourneyBanner{}, false, err
	}
	b := server.TourneyBanner{RunID: d.Run.ID, Total: len(d.Series)}
	for _, line := range d.Series {
		if line.FinishedAt != nil {
			b.Done++
		}
	}
	return b, true, nil
}

func (s tourneyService) LiveBoards() []server.TourneyLiveBoard {
	boards := s.rooms.BotBoards()
	out := make([]server.TourneyLiveBoard, 0, len(boards))
	for _, b := range boards {
		out = append(out, liveBoardOf(b))
	}
	return out
}

func liveBoardOf(b server.LiveBotBoard) server.TourneyLiveBoard {
	lb := server.TourneyLiveBoard{
		RoomID: b.RoomID, RedName: b.GuestName, BlueName: b.HostName,
		RedWins: b.GuestWins, BlueWins: b.HostWins,
		Turn: b.Turn, Moves: b.Moves, Variant: b.Variant,
	}
	if b.RedIsHost {
		lb.RedName, lb.BlueName = b.HostName, b.GuestName
		lb.RedWins, lb.BlueWins = b.HostWins, b.GuestWins
	}
	return lb
}

func runInfoOf(d tourney.Detail) server.TourneyRunInfo {
	info := server.TourneyRunInfo{
		ID: d.Run.ID, CreatedAt: d.Run.CreatedAt, TCIdx: d.Run.TCIdx,
		BOLen: d.Run.BOLen, StartRating: d.Run.StartRating, Variant: d.Run.Variant,
		Finished: d.Run.Status == tourney.RunStateFinished, Running: d.Running,
	}
	if d.Failure != nil {
		info.Failure = d.Failure.Error()
	}
	return info
}

func seatsOf(roster []tourney.Participant) []server.TourneySeat {
	seats := make([]server.TourneySeat, len(roster))
	for i, p := range roster {
		seats[i] = server.TourneySeat{Slot: p.Slot, Name: p.Name, Tier: p.Tier}
	}
	return seats
}

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
