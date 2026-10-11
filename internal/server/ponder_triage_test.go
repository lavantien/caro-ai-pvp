package server

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func TestGrantDepthRingDepthZeroIsAReference(t *testing.T) {
	var ring GrantDepthRing
	ring.Append(1_000_000, 0)
	if ref, ok := ring.ReferenceDepth(1_000_000); !ok || ref != 0 {
		t.Fatalf("depth-zero reference = (%d, %t), want (0, true)", ref, ok)
	}
}

type panicStopBot struct{ ponderBot }

func (b *panicStopBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	b.stops++
	b.mu.Unlock()
	panic("engine: stop ponder regression")
}

func TestPonderLaneStopSurvivesEngineStopPanic(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	bot := &panicStopBot{}
	lane.start(bot, rules.Red, rules.NewBoard(), 5, 0)
	waitFor(t, func() bool {
		bot.mu.Lock()
		defer bot.mu.Unlock()
		return bot.starts == 1
	})
	stopDone := make(chan ponderResult, 1)
	go func() { stopDone <- lane.stop(rules.Red) }()
	select {
	case res := <-stopDone:
		if res.active {
			t.Fatal("stop after the engine's StopPonder panic must report the empty result")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("stop hung after the engine's StopPonder panic")
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("lane did not exit after the engine's StopPonder panic")
	}
}

func TestPonderLaneAbortReleasesQueuedWaiters(t *testing.T) {
	lane := newPonderLane()
	bot := &panicStartBot{}
	lane.start(bot, rules.Red, rules.NewBoard(), 5, 0)
	stopDone := make(chan ponderResult, 1)
	go func() { stopDone <- lane.stop(rules.Red) }()
	waitFor(t, func() bool {
		lane.mu.Lock()
		defer lane.mu.Unlock()
		return len(lane.queue) == 2
	})
	closeDone := make(chan struct{})
	go func() {
		lane.close()
		close(closeDone)
	}()
	waitFor(t, func() bool {
		lane.mu.Lock()
		defer lane.mu.Unlock()
		return len(lane.queue) == 3
	})
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	select {
	case <-stopDone:
	case <-time.After(2 * time.Second):
		t.Fatal("abort starved the queued stop waiter")
	}
	select {
	case <-closeDone:
	case <-time.After(2 * time.Second):
		t.Fatal("abort starved the close waiter")
	}
	select {
	case <-exited:
	case <-time.After(2 * time.Second):
		t.Fatal("lane did not exit after the abort")
	}
}

func ponderRoomTier(t *testing.T, s *stack, name string, bot *ponderBot, tier config.Tier) (User, *Room) {
	t.Helper()
	alice := seedUser(t, s.store, name)
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier)
	if err != nil {
		t.Fatalf("create vs %s: %v", tier.Name, err)
	}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.budgetCap = 3 * time.Millisecond
	r.mu.Unlock()
	return alice, r
}

func TestPonderArmTierThreadMatrix(t *testing.T) {
	s := newStack(t)
	ms := movesOf(t, []string{"D4", "H8", "P16", "J10"})
	d4, h8, p16, j10 := ms[0], ms[1], ms[2], ms[3]
	for _, row := range []struct {
		tier config.Tier
		arms bool
	}{
		{tier: config.TierMedium, arms: true},
		{tier: config.TierEasy, arms: false},
	} {
		bot := &ponderBot{}
		bot.answers = []ponderAnswer{
			{move: h8, pv: []rules.Move{h8, p16}, stats: engine.SearchStats{Depth: 12, Threads: 8}},
			{move: j10, pv: []rules.Move{j10}, stats: engine.SearchStats{Depth: 11, Threads: 8}},
		}
		alice, r := ponderRoomTier(t, s, "alice-"+row.tier.Name, bot, row.tier)
		if err := r.Ready(alice.ID); err != nil {
			t.Fatalf("ready vs %s: %v", row.tier.Name, err)
		}
		if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
			t.Fatalf("human move vs %s: %v", row.tier.Name, err)
		}
		waitBotMove(t, r, 2, h8)
		if row.arms {
			waitFor(t, func() bool {
				_, starts, _ := bot.counts()
				return starts == 1
			})
		} else {
			time.Sleep(100 * time.Millisecond)
			if _, starts, _ := bot.counts(); starts != 0 {
				t.Fatalf("%s tier armed %d ponders, want the PonderThreads-0 tier to skip arming", row.tier.Name, starts)
			}
		}
		if err := r.Forfeit(alice.ID); err != nil {
			t.Fatalf("forfeit vs %s: %v", row.tier.Name, err)
		}
	}
}
