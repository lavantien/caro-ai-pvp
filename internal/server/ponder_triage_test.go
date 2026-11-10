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
	lane.start(bot, rules.Red, rules.NewBoard(), 5, 0, nil, 0)
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
	lane.start(bot, rules.Red, rules.NewBoard(), 5, 0, nil, 0)
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
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier, "")
	if err != nil {
		t.Fatalf("create vs %s: %v", tier.Name, err)
	}
	r.mu.Lock()
	r.makeSearcher = func(_ string, _ config.Tier) searcher { return bot }
	r.budgetCap = 3 * time.Millisecond
	r.mu.Unlock()
	return alice, r
}

func TestPonderLaneAbortReleasesASingleCoreGrant(t *testing.T) {
	adm := newAdmission()
	lane := newPonderLane()
	done := make(chan struct{})
	go func() {
		lane.run()
		close(done)
	}()
	bot := &panicStartBot{}
	lane.start(bot, rules.Red, rules.NewBoard(), 0, 0, adm, 1)
	<-done
	if got := adm.LiveCores(); got != 0 {
		t.Fatalf("cores after the single-core abort = %d, want 0: the abort leaked the ponder grant", got)
	}
	lane.close()
}

func TestPonderLaneDrainReleasesASingleCoreGrant(t *testing.T) {
	adm := newAdmission()
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	eng := &recordingInner{}
	lane.start(eng, rules.Red, rules.NewBoard(), 0, 0, adm, 1)
	waitFor(t, func() bool {
		eng.mu.Lock()
		defer eng.mu.Unlock()
		return eng.starts == 1
	})
	if got := adm.LiveCores(); got != 1 {
		t.Fatalf("cores while ponding = %d, want the 1-core grant held", got)
	}
	lane.drain()
	if got := adm.LiveCores(); got != 0 {
		t.Fatalf("cores after the drain = %d, want 0: the drain leaked the single-core ponder grant", got)
	}
	lane.close()
	<-exited
}

func TestPonderLaneDrainSkipsAdmissionFreeArms(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	eng := &recordingInner{}
	lane.start(eng, rules.Red, rules.NewBoard(), 0, 0, nil, 1)
	waitFor(t, func() bool {
		eng.mu.Lock()
		defer eng.mu.Unlock()
		return eng.starts == 1
	})
	lane.drain()
	lane.mu.Lock()
	active := lane.active[rules.Red]
	lane.mu.Unlock()
	if active != nil {
		t.Fatal("drain must clear the armed seat without touching the nil admission")
	}
	lane.close()
	<-exited
}

func TestPonderLaneHaltSkipsAdmissionFreeArms(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	eng := &recordingInner{}
	lane.start(eng, rules.Red, rules.NewBoard(), 0, 0, nil, 1)
	waitFor(t, func() bool {
		eng.mu.Lock()
		defer eng.mu.Unlock()
		return eng.starts == 1
	})
	if res := stopLaneBounded(t, lane, rules.Red); !res.active {
		t.Fatal("stop after an admission-free arm must report the drained ponder")
	}
	lane.close()
	<-exited
}

func TestCollectPonderWithoutALaneReturnsNothing(t *testing.T) {
	r := &Room{}
	if mv, _, _, ok := r.collectPonder(rules.Blue, 1, time.Now()); ok || mv != 0 {
		t.Fatalf("laneless collect = move %d ok %t, want no move and rejection", mv, ok)
	}
}

func TestCollectPonderIdleLaneReturnsNoMove(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	defer func() {
		lane.close()
		<-exited
	}()
	r := &Room{ponder: lane}
	if mv, _, _, ok := r.collectPonder(rules.Blue, 1, time.Now()); ok || mv != 0 {
		t.Fatalf("idle collect = move %d ok %t, want no move and rejection", mv, ok)
	}
}

func TestCollectPonderRejectedGateReturnsNoMove(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	defer func() {
		lane.close()
		<-exited
	}()
	h8 := rules.Move(mustCellT(t, "H8"))
	k10 := rules.Move(mustCellT(t, "K10"))
	other := rules.Move(mustCellT(t, "L9"))
	bot := &ponderBot{}
	bot.stopMove = k10
	bot.stopStats = adoptableStopStats(k10, 12, int64(10*time.Millisecond))
	bot.stopStats.RootMoves[config.PonderStableIters-1] = other
	lane.start(bot, rules.Blue, rules.NewBoard(), h8, 0, nil, 0)
	b := rules.NewBoard()
	b.Make(rules.Cell(h8))
	r := &Room{ponder: lane}
	r.mu.Lock()
	r.board = b
	r.moves = []rules.Move{h8}
	r.mu.Unlock()
	if mv, _, _, ok := r.collectPonder(rules.Blue, int64(time.Millisecond), time.Now()); ok || mv != 0 {
		t.Fatalf("gate-rejected collect = move %d ok %t, want no move and rejection", mv, ok)
	}
}

func TestCollectPonderBoardlessRoomRejectsAdoptableStats(t *testing.T) {
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	defer func() {
		lane.close()
		<-exited
	}()
	k10 := rules.Move(mustCellT(t, "K10"))
	bot := &ponderBot{}
	bot.stopMove = k10
	bot.stopStats = adoptableStopStats(k10, 12, int64(10*time.Millisecond))
	lane.start(bot, rules.Blue, rules.NewBoard(), 0, 0, nil, 0)
	r := &Room{ponder: lane}
	if mv, _, _, ok := r.collectPonder(rules.Blue, int64(time.Millisecond), time.Now()); ok || mv != 0 {
		t.Fatalf("boardless collect = move %d ok %t, want rejection despite adoptable stats", mv, ok)
	}
}

func refutedClaimPonderRoom(t *testing.T, tag string) (*Room, *ponderBot) {
	t.Helper()
	lane := newPonderLane()
	exited := make(chan struct{})
	go func() {
		defer close(exited)
		lane.run()
	}()
	t.Cleanup(func() {
		lane.close()
		<-exited
	})
	b := blueFourBoard(t)
	slow := rules.Move(mustCellT(t, "E5"))
	bot := &ponderBot{}
	bot.stopMove = slow
	bot.stopStats = adoptableStopStats(slow, 12, int64(10*time.Millisecond))
	bot.stopStats.Score = config.EvalMateMax - 5*config.EvalMateScoreStep
	bot.mu.Lock()
	bot.stopTag = tag
	bot.mu.Unlock()
	ms := movesOf(t, []string{"J9", "K9", "L9", "M9", "F12", "G12"})
	r := &Room{ponder: lane}
	r.mu.Lock()
	r.board = b
	r.moves = ms
	r.mu.Unlock()
	lane.start(bot, rules.Blue, b, ms[len(ms)-1], b.MoveCount-1, nil, 0)
	return r, bot
}

func TestCollectPonderRefutedClaimRejectsUntagged(t *testing.T) {
	r, _ := refutedClaimPonderRoom(t, "")
	if mv, _, _, ok := r.collectPonder(rules.Blue, int64(time.Millisecond), time.Now()); ok || mv != 0 {
		t.Fatalf("refuted untagged collect = move %d ok %t, want the audited claim rejected", mv, ok)
	}
}

func TestCollectPonderTaggedProofSkipsTheRefutationAudit(t *testing.T) {
	r, bot := refutedClaimPonderRoom(t, config.BotLogTagVCF)
	mv, _, tag, ok := r.collectPonder(rules.Blue, int64(time.Millisecond), time.Now())
	if !ok || mv != bot.stopMove || tag != config.BotLogTagVCF {
		t.Fatalf("tagged collect = move %d tag %q ok %t, want the refuted-but-proven claim adopted un-audited", mv, tag, ok)
	}
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
		if err := readyRoomBounded(t, r, alice.ID); err != nil {
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
