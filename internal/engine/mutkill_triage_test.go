package engine

// Mutation-killer pins from the Oct 2026 survivor triage: the empty-board
// fast-path slot, the node-check cadence, the stopped-search return
// contracts, and the empty-candidate root sentinel. Equivalent survivors are
// allowlisted in .mutate-allow with proofs instead.

import (
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// kills movegen.go 34:17 and 34:22 x2: the empty-board fast path must
// normalize its single slot, overwriting stale values an earlier search left
// in the shared per-ply stacks: the move becomes the precomputed center and
// the score becomes exactly 0.
func TestMutKillEmptyBoardFastPathSlot(t *testing.T) {
	e := New(0)
	e.scores[3][0] = 777
	if n := e.generate(rules.NewBoard(), 3, moveNone); n != 1 {
		t.Fatalf("empty board generated %d candidates, want 1", n)
	}
	if got := e.moves[3][0]; got != rules.Move(config.SearchEmptyBoardCell) {
		t.Errorf("moves[3][0] = %d, want center %d", got, config.SearchEmptyBoardCell)
	}
	if got := e.scores[3][0]; got != 0 {
		t.Errorf("scores[3][0] = %d, want the fast path's 0 over the 777 sentinel", got)
	}
}

// kills search.go 236:32 and 236:35 x2: the countdown deadline flips on the
// 6th Exceeded consult, which lands on the first interior cadence consult of
// the depth 5 iteration (the four banked iterations total 1669 nodes, the
// fifth crosses the 2048-node interval mid-tree), so the stop latches at
// exactly node 2048 and depth 4 stays the last completed iteration. Every
// cadence splice moves the consult node off 2048.
func TestMutKillNodeCheckCadence(t *testing.T) {
	e := New(0)
	_, stats := e.SearchDepth(midgameBoard(t), &mutCountdownDL{calls: 6}, 6)
	if stats.Nodes != 2048 || stats.Depth != 4 {
		t.Errorf("countdown cadence stop: nodes=%d depth=%d, want 2048 and 4", stats.Nodes, stats.Depth)
	}
}

// kills search.go 212:11 x2: a stopped deadline mid-iteration makes
// searchRoot answer exactly (0, moveNone); the callers discard the pair, and
// this pin holds the contract at the source. The countdown's 6th consult is
// the 12288th node, inside the depth 7 iteration's tree.
func TestMutKillStoppedRootReturn(t *testing.T) {
	e := New(0)
	b := midgameBoard(t)
	e.resetForSearch(b)
	sc, mv := e.searchRoot(b, 7, &mutCountdownDL{calls: 6})
	if !e.stopped {
		t.Fatalf("countdown deadline did not stop the root search")
	}
	if sc != 0 || mv != moveNone {
		t.Errorf("stopped searchRoot return = (%d, %d), want (0, moveNone)", sc, mv)
	}
}

// kills search.go 243:10 x2: negamax entered with the stop already latched
// returns exactly 0 before touching any search state.
func TestMutKillStoppedEntryReturn(t *testing.T) {
	e := New(0)
	e.resetForSearch(midgameBoard(t))
	e.stopped = true
	rv := e.negamax(midgameBoard(t), 2, -config.EvalMateMax, config.EvalMateMax, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if rv != 0 {
		t.Errorf("stopped negamax entry return = %d, want 0", rv)
	}
}

// kills search.go 299:11 x2: when the stop latches deep inside the tree the
// outermost negamax unwinds through the post-loop return, which must answer
// exactly 0. The countdown's 6th consult is the 12288th node, inside the
// depth 7 tree of the direct call, so the flip is guaranteed mid-search.
func TestMutKillStoppedLoopReturn(t *testing.T) {
	e := New(0)
	b := midgameBoard(t)
	e.resetForSearch(b)
	rv := e.negamax(b, 7, -config.EvalMateMax, config.EvalMateMax, 1, config.SearchExtensionMaxPly, &mutCountdownDL{calls: 6})
	if !e.stopped {
		t.Fatalf("countdown deadline did not stop the search")
	}
	if rv != 0 {
		t.Errorf("stopped negamax loop return = %d, want 0", rv)
	}
}

// kills search.go 272:43: red to move with exactly one red stone and no
// blue stone is the one board shape whose constrained candidate set is empty
// (every dilation cell sits within Chebyshev 2 of the anchor, all under the
// opening minimum), so negamax returns the fail-soft sentinel itself; the
// sentinel lives exactly one below the mate lattice at -(EvalMateMax+1).
func TestMutKillEmptyCandidateSentinel(t *testing.T) {
	b := rules.NewBoard()
	place(t, b, rules.Red, "H8")
	b.Side = rules.Red
	e := New(0)
	e.resetForSearch(b)
	if n := e.generate(b, 0, moveNone); n != 0 {
		t.Fatalf("degenerate opening generated %d candidates, want 0", n)
	}
	rv := e.negamax(b, 2, -config.EvalMateMax, config.EvalMateMax, 1, config.SearchExtensionMaxPly, NewFixedBudget(time.Second))
	if rv != -(config.EvalMateMax + 1) {
		t.Errorf("empty-candidate negamax = %d, want -(EvalMateMax+1)", rv)
	}
}
