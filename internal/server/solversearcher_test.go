package server

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// The tier solver wiring of Implication 2.1: medium runs VCF, hard runs
// VCF and VCT, easy runs neither, a proven forced line ends the move with
// the matching Implication 1.5 tag, and a miss leaves the untagged
// standard search the unspent remainder of the grant.

// anchorBoard places stones as (row, col) pairs relative to the anchor I9
// and leaves the given side to move.
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
		// Made unconditionally the way the solver suite's own crafts are:
		// adjacent same-color stones would violate the opening rule, which
		// these post-opening shapes never see in play.
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

// hardWiredSingleCore is the hard tier's solver wiring over the plain
// single-threaded engine. The wiring under test is core-agnostic, and the
// nodes assertion below needs an engine whose whole grant cannot vanish
// into a worker-pool scheduling loss before depth 1 commits, which the
// tag runs caught as red-on-green flakes on the 4-worker pool.
var hardWiredSingleCore = config.Tier{Name: "hard-wired", Cores: 1, VCF: true, VCT: true}

// openFourBoard gives red the open four J9-M9 with both completions free.
func openFourBoard(t testing.TB) *rules.Board {
	return anchorBoard(t, rules.Red, r(0, 1), r(0, 2), r(0, 3), r(0, 4))
}

// doubleThreeBoard is the VCT craft of the solver suite: red wins by
// continuous threes, never by fours alone, so only the VCT pass proves it.
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

	// Medium lacks VCT, so the same board falls through to the standard
	// search: an untagged, legal answer.
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
	// The grant must clear the shared-runner stall zone: CI run 37487619913
	// lost a whole 500ms window to descheduling before depth 1 committed on
	// this single-core engine, so stalls near half a second are proven on the
	// runner class. Five seconds holds ten times that, beside the 2s grants
	// the forced-win tests have never lost across the gate history.
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

// neverDeadline is a Deadline with no Budget method: the wiring must treat
// it as grant-less and still run the passes, the path every caller that is
// not the room's FixedBudget takes.
type neverDeadline struct{}

func (neverDeadline) Exceeded() bool { return false }
func (neverDeadline) Stop()          {}

// The floor-zone gate of the corrected-clock 1+0 smoke: a drained clock
// grants its SearchMinMoveTimeMs floor, where two compounding solver shares
// left hard's inner search a 2.5ms slice that finished no iteration, 497
// zero-node moves in the run, while medium's single 5ms slice starved only
// a tenth of its floor moves. At the floor itself the passes must not run:
// the provable open four falls to the untagged inner search holding the
// whole grant. Nodes stay unasserted here on purpose: the same smoke shows
// a plain 10ms grant occasionally completing no iteration (3 zero-node
// moves on easy seats that have no solvers), a floor-versus-quantum
// residual of the time manager, not of this gate.
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

// A grant just under the gate skips the passes and the whole grant is
// billed to the untagged search. Nodes stay unasserted here by design:
// the grant is structurally pinned under SolverMinGrantMs, inside the
// shared-runner stall zone, so the funded-engine property lives in the
// miss test's stall-sized grant instead.
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

// The gate's boundary sits at exactly SolverMinGrantMs: the passes still
// run there, proving the forced win on the solver's own tagged line.
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

// A deadline without a budget keeps the passes running under a nil share,
// the grant-less path of the wiring.
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

// The exported surface the playground harnesses drive: the constructor
// returns the same tier wiring the room layer uses, tag law intact.
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
