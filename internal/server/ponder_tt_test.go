package server

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type ttReplyBot struct {
	*ponderBot
	mu     sync.Mutex
	probes int
	lastTT rules.Board
	reply  rules.Move
}

func (b *ttReplyBot) TTMove(bd *rules.Board) rules.Move {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probes++
	b.lastTT = *bd
	return b.reply
}

func (b *ttReplyBot) ttState() (probes int, lastTT rules.Board) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.probes, b.lastTT
}

func ttFallbackRoom(t *testing.T, bot searcher) (User, *Room) {
	t.Helper()
	s := newStack(t)
	alice := seedUser(t, s.store, "alice")
	tier := config.TierMaster
	r, err := s.rm.Create(alice.ID, 0, config.SeriesBO3, &tier)
	if err != nil {
		t.Fatalf("create vs master: %v", err)
	}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return bot }
	r.budgetCap = 3 * time.Millisecond
	r.mu.Unlock()
	return alice, r
}

func TestPonderTTFallbackArmsAndAdopts(t *testing.T) {
	ms := movesOf(t, []string{"D4", "H8", "P16", "K10"})
	d4, h8, p16, k10 := ms[0], ms[1], ms[2], ms[3]
	inner := &ponderBot{}
	inner.answers = []ponderAnswer{{move: h8, pv: []rules.Move{h8}, stats: engine.SearchStats{Depth: 12, Threads: 8}}}
	inner.stopMove = k10
	inner.stopStats = adoptableStopStats(k10, 12, int64(10*time.Millisecond))
	bot := &ttReplyBot{ponderBot: inner, reply: p16}
	alice, r := ttFallbackRoom(t, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	waitFor(t, func() bool {
		_, starts, _ := bot.counts()
		return starts == 1
	})
	probes, lastTT := bot.ttState()
	if probes != 1 {
		t.Fatalf("TT probes = %d, want exactly the one arm probe", probes)
	}
	if lastTT.MoveCount != 2 || lastTT.Side != rules.Red {
		t.Fatalf("probe board = %d stones side %v, want the room board after the answer move", lastTT.MoveCount, lastTT.Side)
	}
	bot.mu.Lock()
	startBoard := bot.lastStart
	bot.mu.Unlock()
	if startBoard.MoveCount != 3 || startBoard.Side != rules.Blue {
		t.Fatalf("ponder board = %d stones side %v, want 3 with blue to move", startBoard.MoveCount, startBoard.Side)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(p16)); err != nil {
		t.Fatalf("predicted reply: %v", err)
	}
	waitBotMove(t, r, 4, k10)
	waitFor(t, func() bool {
		probes, _ := bot.ttState()
		return probes == 2
	})
	searches, starts, stops := bot.counts()
	if searches != 1 {
		t.Fatalf("searches after the TT-fallback adoption = %d, want 1: adoption must not search", searches)
	}
	if starts != 1 || stops != 1 {
		t.Fatalf("ponder counts = %d starts %d stops, want 1 and 1", starts, stops)
	}
	r.mu.Lock()
	rec := r.lastM
	r.mu.Unlock()
	if rec.Tag != config.BotLogTagPonder || rec.Move != k10 {
		t.Fatalf("adopted record = move %d tag %q, want %d pondered", rec.Move, rec.Tag, k10)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderTTFallbackRefusesIllegalReply(t *testing.T) {
	ms := movesOf(t, []string{"D4", "H8"})
	d4, h8 := ms[0], ms[1]
	inner := &ponderBot{}
	inner.answers = []ponderAnswer{{move: h8, pv: []rules.Move{h8}, stats: engine.SearchStats{Depth: 12, Threads: 8}}}
	bot := &ttReplyBot{ponderBot: inner, reply: d4}
	alice, r := ttFallbackRoom(t, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	time.Sleep(100 * time.Millisecond)
	if _, starts, _ := bot.counts(); starts != 0 {
		t.Fatalf("starts after an illegal TT reply = %d, want no arm", starts)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderTTFallbackRefusesSentinelReply(t *testing.T) {
	ms := movesOf(t, []string{"D4", "H8"})
	d4, h8 := ms[0], ms[1]
	inner := &ponderBot{}
	inner.answers = []ponderAnswer{{move: h8, pv: []rules.Move{h8}, stats: engine.SearchStats{Depth: 12, Threads: 8}}}
	bot := &ttReplyBot{ponderBot: inner, reply: rules.Move(config.BoardCells)}
	alice, r := ttFallbackRoom(t, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	time.Sleep(100 * time.Millisecond)
	if _, starts, _ := bot.counts(); starts != 0 {
		t.Fatalf("starts after the miss sentinel = %d, want no arm", starts)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

func TestPonderTTFallbackNeedsAnAnswerMove(t *testing.T) {
	ms := movesOf(t, []string{"D4", "H8"})
	d4, h8 := ms[0], ms[1]
	inner := &ponderBot{}
	inner.answers = []ponderAnswer{{move: h8, pv: []rules.Move{}, stats: engine.SearchStats{Depth: 12, Threads: 8}}}
	bot := &ttReplyBot{ponderBot: inner, reply: rules.Move(mustCellT(t, "P16"))}
	alice, r := ttFallbackRoom(t, bot)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, rules.Cell(d4)); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitBotMove(t, r, 2, h8)
	time.Sleep(100 * time.Millisecond)
	if _, starts, _ := bot.counts(); starts != 0 {
		t.Fatalf("starts after a PVLen-0 answer = %d, want no arm without an answer move", starts)
	}
	if probes, _ := bot.ttState(); probes != 0 {
		t.Fatalf("TT probes after a PVLen-0 answer = %d, want the table untouched", probes)
	}
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
}

type scriptedTTBot struct {
	*recordingInner
	mu     sync.Mutex
	probes int
	last   rules.Board
	reply  rules.Move
}

func (b *scriptedTTBot) TTMove(bd *rules.Board) rules.Move {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.probes++
	b.last = *bd
	return b.reply
}

func TestTTMoveDelegatesThroughTheSearcherStack(t *testing.T) {
	b := rules.NewBoard()
	b.Make(mustCellT(t, "H8"))
	probe := &scriptedTTBot{recordingInner: &recordingInner{}, reply: rules.Move(mustCellT(t, "K10"))}
	solver := &solverSearcher{inner: probe, cores: 2}
	if mv := solver.TTMove(b); mv != probe.reply {
		t.Fatalf("solver TTMove = %d, want the inner probe's %d", mv, probe.reply)
	}
	booked := &bookSearcher{inner: solver, maxPly: 8, cores: 2}
	if mv := booked.TTMove(b); mv != probe.reply {
		t.Fatalf("book TTMove = %d, want the inner probe's %d", mv, probe.reply)
	}
	probe.mu.Lock()
	probes, last := probe.probes, probe.last
	probe.mu.Unlock()
	if probes != 2 || last.MoveCount != 1 {
		t.Fatalf("inner probes = %d on %d stones, want 2 on the passed board", probes, last.MoveCount)
	}
}

func TestSearchTTMoveMissesWithoutAProber(t *testing.T) {
	b := rules.NewBoard()
	b.Make(mustCellT(t, "H8"))
	if mv := searchTTMove(&recordingInner{}, b); mv != rules.Move(config.BoardCells) {
		t.Fatalf("non-prober TTMove = %d, want the always-illegal sentinel", mv)
	}
	if mv := searchTTMove(&solverSearcher{inner: &recordingInner{}, cores: 1}, b); mv != rules.Move(config.BoardCells) {
		t.Fatalf("solver over a non-prober = %d, want the always-illegal sentinel", mv)
	}
	tier := tierSearcher{smp: engine.NewTiered(config.Tier{Name: "ponder-probe", Cores: 2, TTBytes: 1 << 20, PonderThreads: 2})}
	defer tier.Close()
	if mv := tier.TTMove(b); mv != rules.Move(config.BoardCells) {
		t.Fatalf("tier TTMove on an unsearched board = %d, want the miss sentinel", mv)
	}
}

func TestTTPonderPredictGatesOnLegality(t *testing.T) {
	b := rules.NewBoard()
	b.Make(mustCellT(t, "H8"))
	if _, ok := ttPonderPredict(&recordingInner{}, b); ok {
		t.Fatal("non-prober predict must refuse")
	}
	occupied := &scriptedTTBot{recordingInner: &recordingInner{}, reply: rules.Move(mustCellT(t, "H8"))}
	if _, ok := ttPonderPredict(occupied, b); ok {
		t.Fatal("occupied-cell predict must refuse")
	}
	sentinel := &scriptedTTBot{recordingInner: &recordingInner{}, reply: rules.Move(config.BoardCells)}
	if _, ok := ttPonderPredict(sentinel, b); ok {
		t.Fatal("sentinel predict must refuse")
	}
	legal := &scriptedTTBot{recordingInner: &recordingInner{}, reply: rules.Move(mustCellT(t, "K10"))}
	mv, ok := ttPonderPredict(legal, b)
	if !ok || mv != legal.reply {
		t.Fatalf("legal predict = (%d, %t), want (%d, true)", mv, ok, legal.reply)
	}
}
