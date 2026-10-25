package main

import (
	"context"
	"errors"
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
	"github.com/lavantien/caro-ai-pvp/internal/tourney"
)

type stubRunSource struct {
	detail     tourney.Detail
	detailErr  error
	ongoing    tourney.Detail
	ongoingOK  bool
	ongoingErr error
}

func (s *stubRunSource) Detail(context.Context, int64) (tourney.Detail, error) {
	return s.detail, s.detailErr
}

func (s *stubRunSource) OngoingRun(context.Context) (tourney.Detail, bool, error) {
	return s.ongoing, s.ongoingOK, s.ongoingErr
}

type stubBoards struct{ boards []server.LiveBotBoard }

func (s stubBoards) BotBoards() []server.LiveBotBoard { return s.boards }

func int64Ptr(i int64) *int64 { return &i }

func realDetail() tourney.Detail {
	return tourney.Detail{
		Run: tourney.Run{
			ID: 41, CreatedAt: 1791310000, TCIdx: 0, BOLen: config.SeriesBO3,
			StartRating: config.TournamentStartRating, Variant: "hex",
			Status: tourney.RunStateOngoing,
		},
		Roster: []tourney.Participant{
			{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
			{Slot: 1, Name: "easy-2", Tier: config.TierEasy.Name},
		},
		Series: []tourney.Series{
			{PairingSlot: 0, RedFirstSlot: 0, BlueFirstSlot: 1, RedFirstWins: 1},
			{PairingSlot: 1, RedFirstSlot: 1, BlueFirstSlot: 0, RedFirstWins: 1, BlueFirstWins: 1,
				FinishedAt: int64Ptr(1791310600)},
		},
		Board: []tourney.Standings{
			{Slot: 0, Rating: 1006, Wins: 2, GamesPlayed: 3},
			{Slot: 1, Rating: 994, Wins: 1, GamesPlayed: 3},
		},
		Running: true,
	}
}

func TestRunsMergeRealAndScripted(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{detail: realDetail()}, stubBoards{}, 41)
	runs, err := tour.Runs(context.Background())
	if err != nil {
		t.Fatalf("runs: %v", err)
	}
	if len(runs) != 3 {
		t.Fatalf("runs = %d, want 3", len(runs))
	}
	if runs[0].ID != 41 || runs[0].Variant != "hex" || runs[0].Leader != "easy-1" {
		t.Errorf("real summary = %+v", runs[0])
	}
	if runs[1].ID != runningRunID || runs[2].ID != finishedRunID {
		t.Errorf("scripted summaries = %d, %d, want %d, %d",
			runs[1].ID, runs[2].ID, runningRunID, finishedRunID)
	}
}

func TestRunSnapshotMapsRealRun(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{detail: realDetail()}, stubBoards{}, 41)
	sn, err := tour.RunSnapshot(context.Background(), 41)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if sn.Run.ID != 41 || sn.Run.Variant != "hex" || !sn.Run.Running || sn.Run.Finished {
		t.Errorf("run info = %+v", sn.Run)
	}
	if len(sn.Seats) != 2 || sn.Seats[0].Name != "easy-1" || sn.Seats[0].Tier != config.TierEasy.Name {
		t.Errorf("seats = %+v", sn.Seats)
	}
	if len(sn.Series) != 2 || !sn.Series[1].Finished || sn.Series[0].Finished {
		t.Errorf("series = %+v", sn.Series)
	}
	if len(sn.Board) != 2 || sn.Board[0].Rating != 1006 || sn.Board[0].Wins != 2 {
		t.Errorf("board = %+v", sn.Board)
	}
	if _, err := tour.RunSnapshot(context.Background(), 99); !errors.Is(err, server.ErrNotFound) {
		t.Errorf("unknown run err = %v, want ErrNotFound", err)
	}
	if sn, err := tour.RunSnapshot(context.Background(), finishedRunID); err != nil || sn.Run.ID != finishedRunID {
		t.Errorf("scripted finished snapshot: (%+v, %v)", sn.Run, err)
	}
}

func TestOngoingRunBannerFallback(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{ongoing: realDetail(), ongoingOK: true}, stubBoards{}, 41)
	tour.bannerOn.Store(true)
	b, ok, err := tour.OngoingRun(context.Background())
	if err != nil || !ok || b.RunID != runningRunID || b.Done != 7 || b.Total != 12 {
		t.Errorf("scripted banner = (%+v, %t, %v)", b, ok, err)
	}
	tour.bannerOn.Store(false)
	b, ok, err = tour.OngoingRun(context.Background())
	if err != nil || !ok || b.RunID != 41 || b.Done != 1 || b.Total != 2 {
		t.Errorf("real banner = (%+v, %t, %v)", b, ok, err)
	}
	tour = newScriptTourney(&stubRunSource{}, stubBoards{}, 41)
	if _, ok, err := tour.OngoingRun(context.Background()); err != nil || ok {
		t.Errorf("no ongoing = (%t, %v), want (false, nil)", ok, err)
	}
}

func TestLiveBoardsDelegatesToRealRooms(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{}, stubBoards{boards: []server.LiveBotBoard{
		{RoomID: "a1", HostName: "host", GuestName: "guest", RedIsHost: true,
			HostWins: 1, GuestWins: 0, Turn: "red", Variant: "hex", Moves: []string{"L12", "M13"}},
		{RoomID: "b2", HostName: "host2", GuestName: "guest2", RedIsHost: false,
			HostWins: 0, GuestWins: 2, Turn: "blue", Moves: []string{"H8"}},
	}}, 41)
	boards := tour.LiveBoards()
	if len(boards) != 2 {
		t.Fatalf("boards = %d, want 2", len(boards))
	}
	if boards[0].RedName != "host" || boards[0].BlueName != "guest" ||
		boards[0].RedWins != 1 || boards[0].BlueWins != 0 || boards[0].Variant != "hex" {
		t.Errorf("host-red board = %+v", boards[0])
	}
	if boards[1].RedName != "guest2" || boards[1].BlueName != "host2" ||
		boards[1].RedWins != 2 || boards[1].BlueWins != 0 {
		t.Errorf("guest-red board = %+v", boards[1])
	}
	if got := tour.LiveBoards(); len(newScriptTourney(&stubRunSource{}, stubBoards{}, 41).LiveBoards()) != 0 || len(got) != 2 {
		t.Errorf("delegation not live: %d", len(got))
	}
}

func TestScriptedSnapshotsCarryVariant(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{}, stubBoards{}, 41)
	for _, id := range []int64{finishedRunID, runningRunID} {
		sn, err := tour.RunSnapshot(context.Background(), id)
		if err != nil {
			t.Fatalf("scripted run %d snapshot: %v", id, err)
		}
		if sn.Run.Variant != config.DefaultVariant {
			t.Errorf("scripted run %d variant = %q, want %q",
				id, sn.Run.Variant, config.DefaultVariant)
		}
	}
}

func TestStartRunBlockedByRealRun(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{}, stubBoards{}, 41)
	_, err := tour.StartRun(context.Background(), server.TourneySetup{})
	var blocked *server.TourneyBlockedError
	if !errors.As(err, &blocked) || blocked.RunID != 41 {
		t.Errorf("start run err = %v, want block on 41", err)
	}
}

func TestScriptedSnapshotsSetRealWinners(t *testing.T) {
	tour := newScriptTourney(&stubRunSource{}, stubBoards{}, 41)
	sn, err := tour.RunSnapshot(context.Background(), finishedRunID)
	if err != nil {
		t.Fatalf(`finished snapshot: %v`, err)
	}
	meets := finishedMeets()
	if len(sn.Series) != len(meets) {
		t.Fatalf(`series = %d, want %d`, len(sn.Series), len(meets))
	}
	for i, line := range sn.Series {
		if !line.Finished || line.WinnerSlot == nil {
			t.Fatalf(`series %d = %+v, want a finished series with a real winner`, i, line)
		}
		m := meets[i]
		want := m.b
		if m.redWins > m.blueWins {
			want = m.a
		}
		if *line.WinnerSlot != want {
			t.Errorf(`series %d winner = %d, want %d from the meet tally %d-%d`,
				i, *line.WinnerSlot, want, m.redWins, m.blueWins)
		}
	}
}
