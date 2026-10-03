package tourney

// One real end-to-end conductor run: the production RoomSource over a real
// room manager and real easy-tier engines, the smallest surface the room
// contract allows (1+0, bo3), proving the whole chain the scripted tests
// cannot: live rooms per pairing in red-first order, real M-lines in the
// logs, settled game units, the zero-sum leaderboard, retired rooms, and no
// goroutine leak.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/server"
)

func TestConductorEndToEndEasyTiers(t *testing.T) {
	logDir := pointLogsAt(t)
	srv, err := server.Open(filepath.Join(t.TempDir(), "caro.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	wq := server.NewWriteQueue(nil)
	hub := server.NewHub()
	rm := server.NewRoomManager(hub, srv, wq)
	t.Cleanup(func() {
		rm.Shutdown()
		wq.Close()
		_ = srv.Close()
		hub.Close()
	})
	ts := NewStore(srv)
	roster := []Participant{
		{Slot: 0, Name: "easy-1", Tier: config.TierEasy.Name},
		{Slot: 1, Name: "easy-2", Tier: config.TierEasy.Name},
	}
	base := runtime.NumGoroutine()

	// A generous ceiling so a stuck engine fails the test instead of the
	// suite hanging.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	start := time.Now()
	res, err := NewConductor(RoomSource{RM: rm}).Run(ctx, ts, roster,
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("e2e run: 2 easy bo3 series at 1+0 in %s", elapsed)

	// Both pairings ran to settled series with games on disk.
	sched := mustSchedule(t, srv, res.Run.ID)
	if len(sched) != 2 {
		t.Fatalf("schedule = %d series, want 2", len(sched))
	}
	if sched[0].RedFirstSlot != 0 || sched[1].RedFirstSlot != 1 {
		t.Errorf("red-first seats = %+v, want pairing order 0 then 1", sched)
	}
	games := 0
	for i, s := range sched {
		if s.FinishedAt == nil {
			t.Errorf("series %d unfinished", i)
		}
		g := persistedGames(t, ts, s.ID)
		if len(g) < 2 {
			t.Fatalf("series %d holds %d games, want the bo3 majority", i, len(g))
		}
		for _, row := range g {
			if row.Outcome == server.OutcomeDraw {
				continue
			}
			if row.WonBy == nil {
				t.Errorf("series %d game %d decisive without a won-by tag", i, row.Idx)
			}
		}
		games += len(g)
	}
	if games < 4 {
		t.Errorf("total games = %d, want at least 4 (two bo3)", games)
	}

	// Zero-sum leaderboard with every game billed exactly once.
	sum := 0
	played := 0
	for _, st := range res.Board {
		sum += st.Rating
		played += st.GamesPlayed
	}
	if sum != 2*config.TournamentStartRating {
		t.Errorf("rating sum = %d, want the zero-sum %d", sum, 2*config.TournamentStartRating)
	}
	if played != 2*games {
		t.Errorf("leaderboard games = %d, persisted games = %d, want two seats per game", played, games)
	}

	// Real M-lines landed in per-series logs beside the header, one summary
	// line per finished game, and the verdict.
	for _, s := range sched {
		body, err := os.ReadFile(seriesLogFile(t, logDir, s.ID))
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		for _, want := range []string{
			fmt.Sprintf("run %d series %d", res.Run.ID, s.ID),
			"M1, Red, ",
			"series ",
		} {
			if !strings.Contains(string(body), want) {
				t.Errorf("series %d log misses %q:\n%s", s.ID, want, body)
			}
		}
		// Every persisted game of the series carries its own trace line with
		// the outcome, stone count, and won-by tag.
		for _, g := range persistedGames(t, ts, s.ID) {
			want := fmt.Sprintf("game %d: %s, %d moves", g.Idx+1, g.Outcome, g.Stones)
			if !strings.Contains(string(body), want) {
				t.Errorf("series %d log misses %q:\n%s", s.ID, want, body)
			}
			if g.WonBy != nil && !strings.Contains(string(body), ", won by "+*g.WonBy) {
				t.Errorf("series %d log misses the won-by %q:\n%s", s.ID, *g.WonBy, body)
			}
		}
	}

	// Every room retired at its series end, and no worker or engine
	// goroutine outlives the run.
	if live := rm.List(); len(live) != 0 {
		t.Errorf("rooms after the run = %d, want 0", len(live))
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if runtime.NumGoroutine() <= base+1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("goroutines = %d after the run, want at most %d", runtime.NumGoroutine(), base+1)
}
