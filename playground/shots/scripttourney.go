package main

import (
	"context"
	"sync/atomic"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

const (
	finishedRunID = 3
	runningRunID  = 4
)

type runSource interface {
	Detail(ctx context.Context, runID int64) (tourney.Detail, error)
	OngoingRun(ctx context.Context) (tourney.Detail, bool, error)
}

type boardSource interface {
	BotBoards() []server.LiveBotBoard
}

type scriptTourney struct {
	bannerOn atomic.Bool
	runs     runSource
	boards   boardSource
	realRun  int64
}

func newScriptTourney(runs runSource, boards boardSource, realRun int64) *scriptTourney {
	return &scriptTourney{runs: runs, boards: boards, realRun: realRun}
}

func (t *scriptTourney) StartRun(context.Context, server.TourneySetup) (int64, error) {
	var blocked int64 = runningRunID
	if t.realRun != 0 {
		blocked = t.realRun
	}
	return 0, &server.TourneyBlockedError{RunID: blocked}
}

func (t *scriptTourney) CloseStalledRun(context.Context, int64) error { return nil }

func (t *scriptTourney) Runs(ctx context.Context) ([]server.TourneyRunSummary, error) {
	out := make([]server.TourneyRunSummary, 0, 3)
	if t.realRun != 0 {
		d, err := t.runs.Detail(ctx, t.realRun)
		if err != nil {
			return nil, err
		}
		out = append(out, summaryOf(snapshotOf(d)))
	}
	running, _ := t.runningSnapshot()
	finished, _ := t.finishedSnapshot()
	return append(out, summaryOf(running), summaryOf(finished)), nil
}

func (t *scriptTourney) RunSnapshot(ctx context.Context, runID int64) (server.TourneySnapshot, error) {
	if t.realRun != 0 && runID == t.realRun {
		d, err := t.runs.Detail(ctx, runID)
		if err != nil {
			return server.TourneySnapshot{}, err
		}
		return snapshotOf(d), nil
	}
	switch runID {
	case finishedRunID:
		return t.finishedSnapshot()
	case runningRunID:
		return t.runningSnapshot()
	}
	return server.TourneySnapshot{}, server.ErrNotFound
}

func (t *scriptTourney) OngoingRun(ctx context.Context) (server.TourneyBanner, bool, error) {
	if t.bannerOn.Load() {
		return server.TourneyBanner{RunID: runningRunID, Done: 7, Total: 12}, true, nil
	}
	d, ok, err := t.runs.OngoingRun(ctx)
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

func (t *scriptTourney) LiveBoards() []server.TourneyLiveBoard {
	boards := t.boards.BotBoards()
	out := make([]server.TourneyLiveBoard, 0, len(boards))
	for _, b := range boards {
		out = append(out, liveBoardOf(b))
	}
	return out
}

func snapshotOf(d tourney.Detail) server.TourneySnapshot {
	return server.TourneySnapshot{
		Run:    runInfoOf(d),
		Seats:  seatsOf(d.Roster),
		Series: seriesOf(d.Series),
		Board:  boardOf(d.Board),
	}
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

type meet struct{ a, b, redWins, blueWins int }

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
