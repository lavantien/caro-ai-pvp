package server

import (
	"bytes"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

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

func TestBotEasyTierRealEngineAnswersLegally(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
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
	stats := r.lastM.Stats
	r.mu.Unlock()
	if rules.Cell(answer) == rules.Cell(mustCellT(t, "D4")) {
		t.Error("bot replayed the occupied D4")
	}
	if stats.Nodes == 0 {
		t.Error("bot stats show no search nodes, want a real engine search")
	}

	waitFor(t, func() bool { return humanTurn(r, alice.ID) })
	if err := r.PlayMove(alice.ID, mustCellT(t, "H8")); err != nil {
		t.Fatalf("game did not continue after the engine answer: %v", err)
	}

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

func TestSolverPonderPassStaysSilentOnNormalCompletion(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		log.SetFlags(log.LstdFlags)
	})
	s := &solverSearcher{
		inner: &scriptedBot{script: movesOf(t, []string{"H8"}), stats: fakeStats()},
		vct:   vcf.New(vcf.KindVCT), cores: 2,
	}
	s.StartPonder(openFourBoard(t))
	s.StopPonder()
	s.Close()
	if out := buf.String(); strings.Contains(out, "ponder solver pass aborted") {
		t.Errorf("normal ponder pass logged %q, want silence from the recover guard", out)
	}
}

func TestNewBotSearcherPoolsOnlyAboveOneCore(t *testing.T) {
	easy := newBotSearcher(config.TierEasy)
	if _, ok := easy.(singleSearcher); !ok {
		t.Errorf("easy wiring = %T, want the single engine behind the one-core tier", easy)
	}
	easy.Close()
	for _, tier := range []config.Tier{config.TierMedium, config.TierHard} {
		s := newBotSearcher(tier)
		wrapped, ok := s.(*solverSearcher)
		if !ok {
			t.Fatalf("%s wiring = %T, want the solver wrapper around the pool", tier.Name, s)
		}
		if _, pooled := wrapped.inner.(tierSearcher); !pooled {
			t.Errorf("%s inner wiring = %T, want the tiered pool behind %d cores", tier.Name, wrapped.inner, tier.Cores)
		}
		s.Close()
	}
}
