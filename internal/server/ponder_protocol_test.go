package server

import (
	"sync"
	"testing"

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
