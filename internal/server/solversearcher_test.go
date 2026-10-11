package server

import (
	"sync"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
	"github.com/lavantien/caro-ai-pvp/internal/vcf"
)

func anchorBoard(t testing.TB, side rules.Color, stones ...struct {
	row, col int
	red      bool
}) *rules.Board {
	t.Helper()
	b := rules.NewBoard()
	for _, s := range stones {
		b.Side = rules.Blue
		if s.red {
			b.Side = rules.Red
		}
		b.Make(rules.Cell((8+s.row)*config.BoardStride + 8 + s.col))
	}
	b.Side = side
	return b
}

type stone = struct {
	row, col int
	red      bool
}

func r(row, col int) stone { return stone{row, col, true} }
func x(row, col int) stone { return stone{row, col, false} }

var hardWiredSingleCore = config.Tier{Name: "hard-wired", Cores: 1, VCF: true, VCT: true}

func openFourBoard(t testing.TB) *rules.Board {
	return anchorBoard(t, rules.Red, r(0, 1), r(0, 2), r(0, 3), r(0, 4))
}

func doubleThreeBoard(t testing.TB) *rules.Board {
	return anchorBoard(t, rules.Red, r(0, -2), r(0, -1), r(-2, 0), r(-1, 0), x(3, -5), x(3, -4))
}

func quietBoard(t testing.TB) *rules.Board {
	return anchorBoard(t, rules.Red, r(0, 0), r(3, 3), x(1, 2), x(4, 5))
}

func TestSolverWiringTagsForcedWins(t *testing.T) {
	b := openFourBoard(t)
	for _, tier := range []config.Tier{config.TierMedium, config.TierHard} {
		s := newBotSearcher(tier)
		mv, st, tag := s.Search(b, engine.NewFixedBudget(2*time.Second))
		s.Close()
		if tag != config.BotLogTagVCF {
			t.Fatalf("%s open four tag = %q, want %q", tier.Name, tag, config.BotLogTagVCF)
		}
		if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
			t.Fatalf("%s solver move = %v, want a completion of J9-M9", tier.Name, cellName(rules.Cell(mv)))
		}
		if want := config.EvalMateMax - config.EvalMateScoreStep; st.Score != want {
			t.Fatalf("%s solver score = %d, want the M1 lattice point %d", tier.Name, st.Score, want)
		}
		if st.Depth != 1 || st.PVLen != 1 || st.PV[0] != mv {
			t.Fatalf("%s solver stats depth %d pvlen %d pv0 %v, want 1 1 the move", tier.Name, st.Depth, st.PVLen, st.PV[0])
		}
		if st.Threads != tier.Cores || st.AllocNs != int64(2*time.Second) {
			t.Fatalf("%s solver stats threads %d alloc %d, want %d and the full grant", tier.Name, st.Threads, st.AllocNs, tier.Cores)
		}
	}
}

func TestSolverWiringSplitsVCTFromVCF(t *testing.T) {
	b := doubleThreeBoard(t)

	hard := newBotSearcher(config.TierHard)
	mv, st, tag := hard.Search(b, engine.NewFixedBudget(2*time.Second))
	hard.Close()
	if tag != config.BotLogTagVCT {
		t.Fatalf("hard double three tag = %q, want %q", tag, config.BotLogTagVCT)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("hard solver move %v illegal", mv)
	}
	if want := (config.EvalMateMax - st.Score) / config.EvalMateScoreStep; st.PVLen == 0 || want != st.Depth {
		t.Fatalf("hard stats plies %d score %d disagree, want the same mate distance", st.Depth, want)
	}

	medium := newBotSearcher(config.TierMedium)
	mv, st, tag = medium.Search(b, engine.NewFixedBudget(500*time.Millisecond))
	medium.Close()
	if tag != "" {
		t.Fatalf("medium double three tag = %q, want the untagged search", tag)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("medium search move %v illegal", mv)
	}
	if st.AllocNs != int64(500*time.Millisecond) {
		t.Fatalf("medium alloc = %d, want the whole grant billed", st.AllocNs)
	}
}

func TestSolverWiringEasyHasNoSolvers(t *testing.T) {
	b := openFourBoard(t)
	s := newBotSearcher(config.TierEasy)
	defer s.Close()
	mv, st, tag := s.Search(b, engine.NewFixedBudget(500*time.Millisecond))
	if tag != "" {
		t.Fatalf("easy tag = %q, want the untagged plain engine", tag)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("easy move %v illegal", mv)
	}
	if st.Threads != 1 {
		t.Fatalf("easy threads = %d, want the single core", st.Threads)
	}
}

func TestSolverWiringMissKeepsSearchBudget(t *testing.T) {
	b := quietBoard(t)
	grant := 5 * time.Second
	s := newBotSearcher(hardWiredSingleCore)
	mv, st, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	if tag != "" {
		t.Fatalf("quiet tag = %q, want the untagged search", tag)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("quiet move %v illegal", mv)
	}
	if st.AllocNs != int64(grant) {
		t.Fatalf("quiet alloc = %d, want the whole grant billed", st.AllocNs)
	}
	if st.Nodes == 0 {
		t.Fatal("quiet search reported no nodes, want the inner engine to have run")
	}
}

type neverDeadline struct{}

func (neverDeadline) Exceeded() bool { return false }
func (neverDeadline) Stop()          {}

func TestSolverWiringFloorGrantSkipsPasses(t *testing.T) {
	b := openFourBoard(t)
	grant := time.Duration(config.SearchMinMoveTimeMs) * time.Millisecond
	s := newBotSearcher(config.TierHard)
	mv, st, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	if tag != "" {
		t.Fatalf("floor grant tag = %q, want the passes skipped and the untagged search", tag)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("floor move %v illegal", mv)
	}
	if st.AllocNs != int64(grant) {
		t.Fatalf("floor alloc = %d, want the whole grant billed", st.AllocNs)
	}
}

func TestSolverWiringSkipFundsTheSearch(t *testing.T) {
	b := quietBoard(t)
	grant := time.Duration(config.SolverMinGrantMs-10) * time.Millisecond
	s := newBotSearcher(hardWiredSingleCore)
	mv, st, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	if tag != "" {
		t.Fatalf("skip grant tag = %q, want the untagged search", tag)
	}
	if !b.IsLegal(rules.Cell(mv)) {
		t.Fatalf("skip move %v illegal", mv)
	}
	if st.AllocNs != int64(grant) {
		t.Fatalf("skip alloc = %d, want the whole grant billed", st.AllocNs)
	}
}

func TestSolverWiringGateKeepsRealGrants(t *testing.T) {
	b := openFourBoard(t)
	grant := time.Duration(config.SolverMinGrantMs) * time.Millisecond
	s := newBotSearcher(config.TierHard)
	mv, _, tag := s.Search(b, engine.NewFixedBudget(grant))
	s.Close()
	if tag != config.BotLogTagVCF {
		t.Fatalf("grant at the gate tag = %q, want %q from the solver pass", tag, config.BotLogTagVCF)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Fatalf("grant at the gate move %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
}

func TestSolverWiringGrantlessDeadlineRunsPasses(t *testing.T) {
	b := openFourBoard(t)
	s := newBotSearcher(config.TierHard)
	mv, _, tag := s.Search(b, neverDeadline{})
	s.Close()
	if tag != config.BotLogTagVCF {
		t.Fatalf("grantless tag = %q, want %q from the solver pass", tag, config.BotLogTagVCF)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Fatalf("grantless move %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
}

func TestNewBotSearcherExportedSurface(t *testing.T) {
	s := NewBotSearcher(config.TierHard)
	defer s.Close()
	mv, _, tag := s.Search(openFourBoard(t), engine.NewFixedBudget(2*time.Second))
	if tag != config.BotLogTagVCF {
		t.Fatalf("exported tag = %q, want %q", tag, config.BotLogTagVCF)
	}
	if mv != rules.Move(mustCellT(t, "I9")) && mv != rules.Move(mustCellT(t, "N9")) {
		t.Fatalf("exported move %v, want a completion of J9-M9", cellName(rules.Cell(mv)))
	}
}

type solverProbeBot struct {
	mu       sync.Mutex
	searches int
	dls      []engine.Deadline
}

func (b *solverProbeBot) Search(_ *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats, string) {
	b.mu.Lock()
	b.searches++
	b.dls = append(b.dls, dl)
	b.mu.Unlock()
	return 0, fakeStats(), ""
}
func (b *solverProbeBot) StartPonder(*rules.Board) {}

func (b *solverProbeBot) StopPonder() (rules.Move, engine.SearchStats, string) {
	return 0, engine.SearchStats{}, ""
}

func (b *solverProbeBot) Close() {}

func (b *solverProbeBot) snapshot() (int, []engine.Deadline) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.searches, append([]engine.Deadline(nil), b.dls...)
}

func TestSolverSearcherNilPassSkipsToTheNextSolver(t *testing.T) {
	inner := &solverProbeBot{}
	s := &solverSearcher{inner: inner, vct: vcf.New(vcf.KindVCT), cores: 2}
	mv, st, tag := s.Search(doubleThreeBoard(t), engine.NewFixedBudget(2*time.Second))
	s.Close()
	if tag != config.BotLogTagVCT {
		t.Fatalf("tag = %q, want %q from the VCT pass behind the nil VCF slot", tag, config.BotLogTagVCT)
	}
	if !doubleThreeBoard(t).IsLegal(rules.Cell(mv)) {
		t.Fatalf("VCT move %v illegal on the double three", cellName(rules.Cell(mv)))
	}
	if st.Threads != 2 || st.AllocNs != int64(2*time.Second) {
		t.Fatalf("VCT stats threads %d alloc %d, want 2 and the full grant", st.Threads, st.AllocNs)
	}
	if n, _ := inner.snapshot(); n != 0 {
		t.Errorf("inner searches = %d, want the VCT proof to answer without the inner engine", n)
	}
}

func TestSolverSearcherRestBudgetStaysWithinGrant(t *testing.T) {
	inner := &solverProbeBot{}
	s := &solverSearcher{inner: inner, vcf: vcf.New(vcf.KindVCF), cores: 1}
	grant := 2 * time.Second
	dl := engine.NewFixedBudget(grant)
	s.Search(quietBoard(t), dl)
	s.Close()
	_, dls := inner.snapshot()
	if len(dls) != 1 {
		t.Fatalf("inner searches = %d, want the single fallback", len(dls))
	}
	fb, ok := dls[0].(*engine.FixedBudget)
	if !ok {
		t.Fatalf("inner deadline = %T, want a recomputed fixed budget", dls[0])
	}
	if fb == dl {
		t.Error("inner received the caller's deadline object, want the remainder budget rebuilt after the passes")
	}
	if fb.Budget() > grant {
		t.Errorf("inner budget = %s, want at most the grant", fb.Budget())
	}
}

func TestSolverSearcherGrantlessDeadlinePassesThrough(t *testing.T) {
	inner := &solverProbeBot{}
	s := &solverSearcher{inner: inner, vcf: vcf.New(vcf.KindVCF), cores: 1}
	s.Search(quietBoard(t), neverDeadline{})
	s.Close()
	_, dls := inner.snapshot()
	if len(dls) != 1 {
		t.Fatalf("inner searches = %d, want the single fallback", len(dls))
	}
	if _, ok := dls[0].(neverDeadline); !ok {
		t.Fatalf("inner deadline = %T, want the caller's grant-less deadline passed through untouched", dls[0])
	}
}
