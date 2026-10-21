package server

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type protocolSearcher struct {
	mu     sync.Mutex
	events []string
	open   bool
	closed bool
}

func (p *protocolSearcher) Search(b *rules.Board, _ engine.Deadline) (rules.Move, engine.SearchStats, string) {
	var buf [config.BoardCells]rules.Move
	n := b.LegalMoves(buf[:])
	if n < 2 {
		panic("protocol test: board with fewer than two legal moves")
	}
	var st engine.SearchStats
	st.PVLen = 2
	st.PV[0] = buf[0]
	st.PV[1] = buf[1]
	p.mu.Lock()
	p.events = append(p.events, "search")
	p.mu.Unlock()
	return buf[0], st, ""
}

func (p *protocolSearcher) StartPonder(*rules.Board) {
	p.mu.Lock()
	p.open = true
	p.events = append(p.events, "start")
	p.mu.Unlock()
}

func (p *protocolSearcher) StopPonder() (rules.Move, engine.SearchStats, string) {
	p.mu.Lock()
	p.open = false
	p.events = append(p.events, "stop")
	p.mu.Unlock()
	return 0, engine.SearchStats{}, ""
}

func (p *protocolSearcher) Close() {
	p.mu.Lock()
	p.closed = true
	p.events = append(p.events, "close")
	p.mu.Unlock()
}

func (p *protocolSearcher) snapshot() (events []string, open, closed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]string(nil), p.events...), p.open, p.closed
}

func TestPonderSearcherCallProtocol(t *testing.T) {
	s := newStack(t)
	var regMu sync.Mutex
	var registry []*protocolSearcher
	s.rm.makeSearcher = func(config.Tier) searcher {
		p := &protocolSearcher{}
		regMu.Lock()
		registry = append(registry, p)
		regMu.Unlock()
		return p
	}
	r, err := s.rm.CreateBotVsBot(&config.TierMaster, "", &config.TierMaster, "", 0, config.SeriesBO3)
	if err != nil {
		t.Fatalf("create master vs master: %v", err)
	}
	waitFor(t, func() bool { _, ok := r.Info(); return !ok })
	starts := 0
	for i, p := range registry {
		events, open, closed := p.snapshot()
		if !closed {
			t.Errorf("engine %d: Close never reached, events %v", i, events)
		}
		if open {
			t.Errorf("engine %d: closed with a ponder open, events %v", i, events)
		}
		ponderOpen := false
		for _, ev := range events {
			switch ev {
			case "start":
				if ponderOpen {
					t.Errorf("engine %d: nested StartPonder, events %v", i, events)
				}
				ponderOpen = true
				starts++
			case "stop":
				if !ponderOpen {
					t.Errorf("engine %d: StopPonder without an open ponder, events %v", i, events)
				}
				ponderOpen = false
			case "search":
				if ponderOpen {
					t.Errorf("engine %d: Search while a ponder is open, events %v", i, events)
				}
			case "close":
				if ponderOpen {
					t.Errorf("engine %d: Close while a ponder is open, events %v", i, events)
				}
			}
		}
	}
	if starts == 0 {
		t.Fatal("no ponder was ever armed, the protocol was not exercised")
	}
}

func collectPonderCase(t *testing.T, bot searcher, side rules.Color, predict rules.Move, base int, board *rules.Board, moves []rules.Move) (rules.Move, engine.SearchStats, string, bool) {
	t.Helper()
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
	lane.start(bot, side, board, predict, base, nil, 0)
	r := &Room{ponder: lane, board: board, moves: moves}
	return r.collectPonder(side, 1, time.Now())
}

func TestCollectPonderAdoptsAtMoveCountOne(t *testing.T) {
	h8 := rules.Move(mustCellT(t, "H8"))
	k10 := rules.Move(mustCellT(t, "K10"))
	bot := &ponderBot{stopMove: k10, stopStats: adoptableStopStats(k10, 15, int64(time.Millisecond))}
	board := rules.NewBoard()
	board.Make(rules.Cell(h8))
	mv, _, tag, ok := collectPonderCase(t, bot, rules.Blue, h8, 0, board, []rules.Move{h8})
	if !ok || mv != k10 || tag != config.BotLogTagPonder {
		t.Fatalf("base-zero adoption = move %d tag %q ok %t, want the adopted %d with the ponder tag", mv, tag, ok, k10)
	}
}

func TestCollectPonderRejectsStalePonder(t *testing.T) {
	moves := movesOf(t, []string{"D4", "H8", "P16", "K10", "J10", "M4", "N14"})
	board := rules.NewBoard()
	for _, mv := range moves {
		board.Make(rules.Cell(mv))
	}
	k10 := rules.Move(mustCellT(t, "K10"))
	bot := &ponderBot{stopMove: k10, stopStats: adoptableStopStats(k10, 15, int64(time.Millisecond))}
	mv, _, _, ok := collectPonderCase(t, bot, rules.Blue, moves[len(moves)-1], 5, board, moves)
	if ok || mv != 0 {
		t.Fatalf("stale ponder = move %d ok %t, want rejection two moves past the arm", mv, ok)
	}
}

func TestDispatchPonderArmRefusesUnarmedPonder(t *testing.T) {
	eng := &recordingInner{}
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
	arm := ponderArm{eng: eng, side: rules.Blue, board: *rules.NewBoard(), predict: rules.Move(mustCellT(t, "H8"))}
	r := &Room{ponder: lane, board: rules.NewBoard(), engines: [2]searcher{rules.Blue: eng}}
	r.dispatchPonderArm(&arm)
	time.Sleep(100 * time.Millisecond)
	eng.mu.Lock()
	starts := eng.starts
	eng.mu.Unlock()
	if starts != 0 {
		t.Fatalf("unarmed dispatch reached the engine %d times, want the arm gate to refuse", starts)
	}
}

func TestDispatchPonderArmGuardMatrix(t *testing.T) {
	eng := &recordingInner{}
	swapped := &recordingInner{}
	h8 := rules.Move(mustCellT(t, "H8"))
	movedPast := rules.NewBoard()
	movedPast.Make(mustCellT(t, "D4"))
	movedPast.Make(mustCellT(t, "H8"))
	openQuit := make(chan struct{})
	closedQuit := make(chan struct{})
	close(closedQuit)
	arm := ponderArm{ok: true, eng: eng, side: rules.Blue, board: *rules.NewBoard(), predict: h8, base: 0}
	movedArm := arm
	movedArm.base = 1
	swappedArm := arm
	swappedArm.eng = swapped
	rows := []struct {
		name string
		room *Room
		arm  ponderArm
	}{
		{"retired", &Room{over: true, quit: openQuit, board: rules.NewBoard(), engines: [2]searcher{rules.Blue: eng}}, arm},
		{"completed", &Room{finished: true, quit: openQuit, board: rules.NewBoard(), engines: [2]searcher{rules.Blue: eng}}, arm},
		{"retire window", &Room{quit: closedQuit, board: rules.NewBoard(), engines: [2]searcher{rules.Blue: eng}}, arm},
		{"no board", &Room{quit: openQuit, engines: [2]searcher{rules.Blue: eng}}, arm},
		{"moved past base", &Room{quit: openQuit, board: movedPast, engines: [2]searcher{rules.Blue: eng}}, movedArm},
		{"engine swapped", &Room{quit: openQuit, board: rules.NewBoard(), engines: [2]searcher{rules.Blue: eng}}, swappedArm},
	}
	for _, row := range rows {
		lane := newPonderLane()
		row.room.ponder = lane
		row.room.dispatchPonderArm(&row.arm)
		lane.mu.Lock()
		queued := len(lane.queue)
		lane.mu.Unlock()
		if queued != 0 {
			t.Errorf("%s row: the state guard armed %d ponder start(s), want refusal", row.name, queued)
		}
	}
}
