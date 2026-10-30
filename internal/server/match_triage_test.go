package server

import (
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/clock"
	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func TestSolverPassGrantlessBillsZeroGrant(t *testing.T) {
	s := &solverSearcher{inner: &solverProbeBot{}, vcf: vcf.New(vcf.KindVCF), cores: 1}
	mv, st, tag := s.Search(openFourBoard(t), neverDeadline{})
	s.Close()
	if tag != config.BotLogTagVCF {
		t.Fatalf("grantless tag = %q, want %q", tag, config.BotLogTagVCF)
	}
	if st.AllocNs != 0 {
		t.Errorf("grantless pass alloc = %d, want 0 with no budget to bill", st.AllocNs)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Errorf("grantless move = %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
}

func TestPonderProofStatsBillZeroGrant(t *testing.T) {
	s := &solverSearcher{inner: &solverProbeBot{}, vcf: vcf.New(vcf.KindVCF), cores: 1}
	s.StartPonder(openFourBoard(t))
	s.waitPass()
	mv, st, tag := s.StopPonder()
	s.Close()
	if tag != config.BotLogTagVCF {
		t.Fatalf("ponder proof tag = %q, want %q", tag, config.BotLogTagVCF)
	}
	if st.AllocNs != 0 {
		t.Errorf("ponder proof alloc = %d, want 0 with no turn grant behind it", st.AllocNs)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Errorf("ponder proof move = %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
}

func TestAuditRefutedStillAuditsAtTheFloorScore(t *testing.T) {
	s := &solverSearcher{inner: &solverProbeBot{}, cores: 1}
	mv := rules.Move(mustCellT(t, "E5"))
	st := engine.SearchStats{Score: config.EvalMateFloor, Depth: 4}
	if !s.auditRefuted(blueFourBoard(t), mv, st, time.Now(), 0, false) {
		t.Error("a floor-score claim must still run the refutation audit")
	}
}

func TestAuditRefutedRunsAtTheGrantGate(t *testing.T) {
	s := &solverSearcher{inner: &solverProbeBot{}, cores: 1}
	mv := rules.Move(mustCellT(t, "E5"))
	st := engine.SearchStats{Score: config.EvalMateMax - 5*config.EvalMateScoreStep, Depth: 4}
	grant := time.Duration(config.SolverMinGrantMs) * time.Millisecond
	if !s.auditRefuted(blueFourBoard(t), mv, st, time.Now(), grant, true) {
		t.Error("a grant exactly at the solver gate must still run the refutation audit")
	}
}

func TestAuditRefutedGrantlessRunsTheFullAudit(t *testing.T) {
	s := &solverSearcher{inner: &solverProbeBot{}, cores: 1}
	mv := rules.Move(mustCellT(t, "E5"))
	st := engine.SearchStats{Score: config.EvalMateMax - 5*config.EvalMateScoreStep, Depth: 4}
	if !s.auditRefuted(blueFourBoard(t), mv, st, time.Now(), 0, false) {
		t.Error("without a grant the audit must run unlimited and refute the fast kill")
	}
}

type excludedDeadlineBot struct {
	mu    sync.Mutex
	dls   []engine.Deadline
	mv    rules.Move
	stats engine.SearchStats
}

func (b *excludedDeadlineBot) Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats, string) {
	time.Sleep(2 * time.Millisecond)
	return b.mv, b.stats, ""
}

func (b *excludedDeadlineBot) SearchExcluded(_ *rules.Board, dl engine.Deadline, _ rules.Move) (rules.Move, engine.SearchStats) {
	b.mu.Lock()
	b.dls = append(b.dls, dl)
	b.mu.Unlock()
	return 0, engine.SearchStats{}
}

func (b *excludedDeadlineBot) StartPonder(*rules.Board) {}

func (b *excludedDeadlineBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *excludedDeadlineBot) Close() {}

func (b *excludedDeadlineBot) deadlines() []engine.Deadline {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]engine.Deadline(nil), b.dls...)
}

func refutedClaimBot(t *testing.T) *excludedDeadlineBot {
	return &excludedDeadlineBot{
		mv:    rules.Move(mustCellT(t, "E5")),
		stats: engine.SearchStats{Score: config.EvalMateMax - 5*config.EvalMateScoreStep, Depth: 4},
	}
}

func TestSolverDemotionGrantlessKeepsCallerDeadline(t *testing.T) {
	inner := refutedClaimBot(t)
	s := &solverSearcher{inner: inner, cores: 1}
	s.Search(blueFourBoard(t), neverDeadline{})
	s.Close()
	dls := inner.deadlines()
	if len(dls) != 1 {
		t.Fatalf("demotion re-searches = %d, want 1", len(dls))
	}
	if _, ok := dls[0].(neverDeadline); !ok {
		t.Errorf("demotion re-search deadline = %T, want the caller's grant-less deadline untouched", dls[0])
	}
}

func TestSolverDemotionReSearchBudgetStaysWithinGrant(t *testing.T) {
	inner := refutedClaimBot(t)
	s := &solverSearcher{inner: inner, cores: 1}
	grant := 2 * time.Second
	s.Search(blueFourBoard(t), engine.NewFixedBudget(grant))
	s.Close()
	dls := inner.deadlines()
	if len(dls) != 1 {
		t.Fatalf("demotion re-searches = %d, want 1", len(dls))
	}
	fb, ok := dls[0].(*engine.FixedBudget)
	if !ok {
		t.Fatalf("demotion re-search deadline = %T, want a rebuilt fixed budget", dls[0])
	}
	if fb.Budget() >= grant {
		t.Errorf("demotion re-search budget = %s, want strictly inside the %s grant", fb.Budget(), grant)
	}
}

func TestCloseEnginesToleratesRoomWithoutManager(t *testing.T) {
	r := &Room{}
	r.mu.Lock()
	r.closeEnginesLocked()
	r.mu.Unlock()
}

func bareBotTurnRoom(t *testing.T, s *stack, eng searcher) *Room {
	t.Helper()
	_, g, err := resolveVariant("")
	if err != nil {
		t.Fatalf("resolve variant: %v", err)
	}
	series, err := NewSeriesFor(1, 2, 0, config.SeriesBO3, "")
	if err != nil {
		t.Fatalf("series: %v", err)
	}
	return &Room{
		manager: s.rm, geom: g, series: series,
		host:    seat{userID: 1, bot: &config.TierEasy},
		board:   rules.NewBoardFor(g),
		clock:   [2]*clock.GameClock{clock.NewGameClock(0), clock.NewGameClock(0)},
		engines: [2]searcher{eng, nil},
	}
}

func TestBotTurnWithoutEngineIsANoop(t *testing.T) {
	s := newStack(t)
	r := bareBotTurnRoom(t, s, nil)
	if r.runBotTurn() {
		t.Error("a bot seat without a built engine must not run a turn")
	}
}

type panickingBot struct{}

func (panickingBot) Search(*rules.Board, engine.Deadline) (rules.Move, engine.SearchStats, string) {
	panic("bot test: searcher blew up")
}

func (panickingBot) StartPonder(*rules.Board) {}

func (panickingBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (panickingBot) Close() {}

func TestBotTurnSearchPanicReleasesMachineCores(t *testing.T) {
	s := newStack(t)
	r := bareBotTurnRoom(t, s, panickingBot{})
	before := s.rm.adm.LiveCores()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the panicking searcher never panicked")
			}
		}()
		r.runBotTurn()
	}()
	if got := s.rm.adm.LiveCores(); got != before {
		t.Errorf("live cores after the panic = %d, want the held %d released back", got, before)
	}
}

func injectFailingRebuildBot(t *testing.T, r *Room) {
	t.Helper()
	var calls int32
	var filler []string
	for i, name := range hostWinsRed {
		if i%2 == 1 {
			filler = append(filler, name)
		}
	}
	r.mu.Lock()
	r.makeSearcher = func(_ string, _ config.Tier) searcher {
		if atomic.AddInt32(&calls, 1) > 1 {
			panic("bot test: second game engine construction refused")
		}
		return &scriptedBot{script: movesOf(t, filler), stats: fakeStats()}
	}
	r.mu.Unlock()
}

func playHumanWinOverScriptedBot(t *testing.T, r *Room, alice User, breakStore func()) error {
	t.Helper()
	for i := 0; i < len(hostWinsRed); i += 2 {
		waitFor(t, func() bool { return humanTurn(r, alice.ID) })
		if i == len(hostWinsRed)-1 {
			if breakStore != nil {
				breakStore()
			}
			return r.PlayMove(alice.ID, mustCellT(t, hostWinsRed[i]))
		}
		if err := r.PlayMove(alice.ID, mustCellT(t, hostWinsRed[i])); err != nil {
			t.Fatalf("human move %d %s: %v", i+1, hostWinsRed[i], err)
		}
	}
	return nil
}

func TestNextGameStartFailureSurfacesThroughHumanMove(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	injectFailingRebuildBot(t, r)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	err := playHumanWinOverScriptedBot(t, r, alice, nil)
	if err == nil {
		t.Fatal("the human sweep's closing move swallowed the next game's start failure")
	}
	if !strings.Contains(err.Error(), "engine construction failed") {
		t.Errorf("closing move err = %v, want the engine construction failure", err)
	}
}

func TestPersistErrorBeatsStartErrorInCompletion(t *testing.T) {
	s := newStack(t)
	alice, r := botRoom(t, s, config.TierEasy)
	injectFailingRebuildBot(t, r)
	if err := r.Ready(alice.ID); err != nil {
		t.Fatalf("ready: %v", err)
	}
	err := playHumanWinOverScriptedBot(t, r, alice, func() {
		if _, derr := s.store.db.Exec(`DROP TABLE games`); derr != nil {
			t.Fatalf("drop games: %v", derr)
		}
	})
	if err == nil {
		t.Fatal("the closing move reported no error while both persistence and the next start failed")
	}
	if !strings.Contains(err.Error(), "no such table") {
		t.Errorf("closing move err = %v, want the persistence failure kept over the start failure", err)
	}
	if errors.Is(err, ErrMachineBusy) || strings.Contains(err.Error(), "engine construction failed") {
		t.Errorf("closing move err = %v, want the persistence failure, not the start failure", err)
	}
}
