package tourney

import (
	"context"
	"fmt"
	"math"
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
	rm.SetMemoryProbe(func() (uint64, error) { return math.MaxUint64, nil })
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

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	start := time.Now()
	res, err := NewConductor(RoomSource{RM: rm}).Run(ctx, ts, roster,
		mustTC(1, 0), config.SeriesBO3, config.TournamentStartRating, 1, "test", "")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	elapsed := time.Since(start)
	t.Logf("e2e run: 2 easy bo3 series at 1+0 in %s", elapsed)

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
		names := [2]string{roster[0].Name, roster[1].Name}
		for _, g := range persistedGames(t, ts, s.ID) {
			want := gameSummary(g.Idx+1, names[g.RedSlot], names[g.BlueSlot],
				g.Outcome, g.Stones, g.WonBy)
			if !strings.Contains(string(body), want) {
				t.Errorf("series %d log misses %q:\n%s", s.ID, want, body)
			}
		}
	}

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
