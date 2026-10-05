package server

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The bot-turn safety nets of match.go: the first-legal fallback under an
// engine contract violation, the full-board draw a bot closes itself, and the
// real engine seam of the easy tier.

// TestBotIllegalAnswerFallsBackToFirstLegal pins the safety net under the
// engine contract: a searcher that answers with an occupied cell (the room
// validates every answer) must not corrupt the board, the first legal cell
// plays instead, and the game continues normally around it.
func TestBotIllegalAnswerFallsBackToFirstLegal(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	illegal := &scriptedBot{script: movesOf(t, []string{"D4", "P16"}), stats: fakeStats()}
	r.mu.Lock()
	r.makeSearcher = func(config.Tier) searcher { return illegal }
	r.mu.Unlock()

	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}

	// The expected fallback is the first cell of the enumeration of the
	// post-D4 board, replayed on an independent board: reading the room's
	// own board races the bot's fallback application (PlayMove returns
	// before the worker answers), which once made the copy already hold
	// the fallback and turned the expectation into B1.
	probe := rules.NewBoard()
	probe.Make(mustCellT(t, "D4"))
	var buf [config.BoardCells]rules.Move
	n := probe.LegalMoves(buf[:])
	if n == 0 {
		t.Fatal("no legal move after D4")
	}
	want := rules.Cell(buf[0])

	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.board != nil && r.board.MoveCount == 2
	})
	r.mu.Lock()
	got := r.moves[1]
	stones := r.board.MoveCount
	r.mu.Unlock()
	if rules.Cell(got) != want {
		t.Errorf("fallback move = %s, want the first legal cell %s", cellName(rules.Cell(got)), cellName(want))
	}
	if stones != 2 {
		t.Errorf("stones = %d, want 2", stones)
	}
	if illegal.searchCount() != 1 {
		t.Errorf("bot searches after the fallback = %d, want the single illegal answer", illegal.searchCount())
	}

	// The game continues: red holds the turn and lands another stone, the
	// bot's next answer is legal and plays as itself.
	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.PlayMove(alice.ID, mustCellT(t, "H8")); err != nil {
		t.Fatalf("game did not continue past the fallback: %v", err)
	}
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.board != nil && r.board.MoveCount == 4
	})
	r.mu.Lock()
	second, stones := r.moves[3], r.board.MoveCount
	r.mu.Unlock()
	if rules.Cell(second) != mustCellT(t, "P16") {
		t.Errorf("second bot answer = %s, want the scripted P16", cellName(rules.Cell(second)))
	}
	if stones != 4 {
		t.Errorf("stones after two rounds = %d, want 4", stones)
	}
	if illegal.searchCount() != 2 {
		t.Errorf("bot searches = %d, want one per turn", illegal.searchCount())
	}
}

// TestBotMoveFillsBoardToDraw drives the full-board draw whose last stone is
// the bot's: blue's half of the no-five fill script plays through the worker,
// the 256th stone fills the board, and the completion path books the draw and
// starts the next game instead of waiting for a human move to notice.
func TestBotMoveFillsBoardToDraw(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	script := drawMoves(t)
	var human, bot []string
	for i, name := range script {
		if i%2 == 0 {
			human = append(human, name)
		} else {
			bot = append(bot, name)
		}
	}
	injectScriptedBot(t, r, movesOf(t, bot))
	sub, err := r.Subscribe()
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	var mu sync.Mutex
	var gameEnds []string
	var mlines, moves int
	drained := make(chan struct{})
	go func() {
		defer close(drained)
		for ev := range sub.Events() {
			mu.Lock()
			switch ev.Kind {
			case EventKindMove:
				moves++
			case EventKindMLine:
				mlines++
			case EventKindGameEnd:
				gameEnds = append(gameEnds, ev.Payload)
			}
			mu.Unlock()
		}
	}()

	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	for _, name := range human {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if err := r.PlayMove(alice.ID, mustCellT(t, name)); err != nil {
			t.Fatalf("human move %s: %v", name, err)
		}
	}

	// The drawn game closed on the bot's own filling stone. The completion
	// path now persists a real unit (bot matches record), and the stream
	// reader drains asynchronously, so the honest observation point is the
	// reader's own counts plus the reset: all four hold only after the
	// gameend event published, the unit applied, and game 2 reset.
	waitFor(t, func() bool {
		mu.Lock()
		done := moves == config.BoardCells && mlines == len(bot) && len(gameEnds) == 1
		mu.Unlock()
		if !done {
			return false
		}
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.series.GamesPlayed() == 1 && r.board != nil && r.board.MoveCount == 0
	})
	mu.Lock()
	gotEnds, gotMoves, gotMLines := gameEnds, moves, mlines
	mu.Unlock()
	if len(gotEnds) != 1 || gotEnds[0] != OutcomeDraw {
		t.Errorf("game ends = %v, want the single draw", gotEnds)
	}
	if gotMoves != config.BoardCells || gotMLines != len(bot) {
		t.Errorf("stream counts = %d moves %d mlines, want %d and %d",
			gotMoves, gotMLines, config.BoardCells, len(bot))
	}

	// The series lives on at 0-0 with a fresh board and red kept by the host.
	r.mu.Lock()
	hostWins, guestWins := r.series.Score()
	boardStones := r.board.MoveCount
	red := r.series.RedUserID()
	r.mu.Unlock()
	if hostWins != 0 || guestWins != 0 {
		t.Errorf("score after the draw = %d-%d, want 0-0", hostWins, guestWins)
	}
	if boardStones != 0 || red != alice.ID {
		t.Errorf("game 2 = %d stones red %d, want a fresh board with red kept", boardStones, red)
	}

	// Retire and prove the stream closed on the terminal series frame.
	if err := r.Forfeit(alice.ID); err != nil {
		t.Fatalf("forfeit: %v", err)
	}
	sub.Unsubscribe()
	<-drained
	mu.Lock()
	finalEnds := append([]string(nil), gameEnds...)
	mu.Unlock()
	if len(finalEnds) != 1 || !strings.HasPrefix(finalEnds[0], OutcomeDraw) {
		t.Errorf("game ends after retire = %v, want still the single draw", finalEnds)
	}
}

// TestBotEasyTierRealEngineAnswersLegally runs the real single-threaded engine
// of the easy tier through the room: newBotSearcher sizes it, one search under
// a small budget answers, and the answer is a legal stone the game continues
// from.
func TestBotEasyTierRealEngineAnswersLegally(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	r.mu.Lock()
	r.budgetCap = 5 * time.Millisecond
	r.mu.Unlock()

	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	if err := r.PlayMove(alice.ID, mustCellT(t, "D4")); err != nil {
		t.Fatalf("human move: %v", err)
	}
	waitFor(t, func() bool {
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.board != nil && r.board.MoveCount == 2
	})

	r.mu.Lock()
	answer := r.moves[1]
	stats := r.lastM.stats
	r.mu.Unlock()
	if rules.Cell(answer) == rules.Cell(mustCellT(t, "D4")) {
		t.Error("bot replayed the occupied D4")
	}
	if stats.Nodes == 0 {
		t.Error("bot stats show no search nodes, want a real engine search")
	}

	// The room stays playable: red moves again and the turn comes back.
	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.PlayMove(alice.ID, mustCellT(t, "H8")); err != nil {
		t.Fatalf("game did not continue after the engine answer: %v", err)
	}

	// The tier seam itself: easy is the plain engine, and a direct search on
	// a fresh board answers inside the board. The budget must guarantee one
	// completed iteration even under coverage instrumentation on a loaded
	// runner: a search that dies before its first iteration answers with the
	// legal fallback and zero nodes, which is a budget artifact, not the
	// seam's behavior.
	eng := newBotSearcher(config.TierEasy)
	defer eng.Close()
	if _, isTiered := eng.(interface{ Workers() int }); isTiered {
		t.Error("easy tier built the SMP pool, want the single-threaded engine")
	}
	fresh := rules.NewBoard()
	mv, st, _ := eng.Search(fresh, engine.NewFixedBudget(1*time.Second))
	if !fresh.IsLegal(rules.Cell(mv)) {
		t.Errorf("engine answer %s is illegal on a fresh board", cellName(rules.Cell(mv)))
	}
	if st.Nodes == 0 {
		t.Error("direct engine search reported zero nodes")
	}
}
