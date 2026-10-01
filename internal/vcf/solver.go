// Package vcf implements the dedicated VCF and VCT solvers: proof search
// for forced wins by continuous threats, fours for VCF, threes and fours
// for VCT. Move generation reads the pattern tables exclusively, every
// claimed win verifies on the real board through rules.FastLastMoveWin, and
// resource exhaustion only ever reports not found. See vcf.md for the
// defender model, the search design, and the soundness argument.
package vcf

import (
	"strconv"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Deadline is the solver cancellation surface, structurally identical to
// the engine deadline interface, so engine budgets drive solvers with no
// import in either direction. nil means no wall clock limit.
type Deadline interface {
	Exceeded() bool
	Stop()
}

// Kind selects the attacker move set of the threat search.
type Kind uint8

const (
	// KindVCF searches victory by continuous fours: every attacker move
	// creates a four or open four, or wins on the spot.
	KindVCF Kind = iota
	// KindVCT extends the move set with three and broken three creations.
	KindVCT
)

// SolverStats reports one Solve call: expanded nodes, wall time, and the
// forced line when found. PV entries alternate attacker, defender, ending
// with the winning placement.
type SolverStats struct {
	Nodes     uint64
	ElapsedNs int64
	Plies     int
	Found     bool
	PV        [config.SolverMaxPly]rules.Move
}

// AppendPV appends the forced line in cell notation to dst, strconv only.
func (s *SolverStats) AppendPV(dst []byte) []byte {
	for i := 0; i < s.Plies && i < len(s.PV); i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = append(dst, byte('A'+s.PV[i]%config.BoardStride))
		dst = strconv.AppendInt(dst, int64(s.PV[i]/config.BoardStride)+1, 10)
	}
	return dst
}

// result is the three valued truth of one threat subtree. Wins are only
// ever returned from completely evaluated subtrees; aborts propagate.
type result uint8

const (
	resFail result = iota
	resWin
	resAbort
)

// Win cells at a defend node all arise through the attacker's single last
// move, so winA's margin holds every win cell. Reply sets are subsets of
// the empty cells, so defends sized by BoardCells can never overflow:
// truncating a defender reply set would drop a refutation and turn
// resource limits into false wins.
const maxWinCells = 48

// Solver is one threat space search instance with preallocated per-ply
// stacks and its own proof memo, nothing shared at runtime. The zobrist
// hash does not encode the region, so one instance must serve one board
// kind: cross-check runs get their own solver.
type Solver struct {
	kind       Kind
	maxPly     int
	plyCap     int
	moves      [config.SolverMaxPly][config.BoardCells]rules.Move
	ranks      [config.SolverMaxPly][config.BoardCells]uint8
	defends    [config.SolverMaxPly][config.BoardCells]rules.Cell
	line       [config.SolverMaxPly]rules.Move
	winA       [maxWinCells]rules.Cell
	ttKeys     [1 << config.SolverTTBits]uint64
	ttVerdicts [1 << config.SolverTTBits]uint8
	nodes      int
	budget     int
	check      int
	plies      int
	aborted    bool
	deadline   Deadline
}

// New allocates one solver of the given kind with its proof memo.
func New(kind Kind) *Solver {
	return &Solver{kind: kind, maxPly: config.SolverMaxPly}
}

// Solve decides whether the side to move forces a win by a continuous
// threat sequence within the node budget. It deepens iteratively over the
// forced line length, so shallow wins surface before deep refutation work
// exhausts the budget. The board is restored, the attacker is the side to
// move, and out, when non-nil, receives nodes, wall time and the forced
// line. Budget or deadline exhaustion always reports not found, never a
// win.
func (s *Solver) Solve(b *rules.Board, budget int, dl Deadline, out *SolverStats) bool {
	start := time.Now()
	s.nodes, s.budget, s.deadline = 0, budget, dl
	s.check, s.plies, s.aborted = config.SolverNodeCheckInterval, 0, false
	if dl != nil && dl.Exceeded() {
		s.aborted = true
	}
	found := false
	for depth := 1; depth <= s.maxPly && !s.aborted; depth += 2 {
		s.plyCap = depth
		if s.attack(b, 0) == resWin {
			found = true
			break
		}
	}
	if out != nil {
		out.Nodes = uint64(s.nodes)
		out.ElapsedNs = int64(time.Since(start))
		out.Found = found
		out.Plies = 0
		out.PV = [config.SolverMaxPly]rules.Move{}
		if found {
			out.Plies = s.plies
			copy(out.PV[:], s.line[:s.plies])
		}
	}
	return found
}

// ok is the per node resource gate every expansion passes through.
func (s *Solver) ok() bool {
	if s.aborted {
		return false
	}
	s.nodes++
	if s.nodes > s.budget {
		s.aborted = true
		return false
	}
	s.check--
	if s.check <= 0 {
		s.check = config.SolverNodeCheckInterval
		if s.deadline != nil && s.deadline.Exceeded() {
			s.aborted = true
			return false
		}
	}
	return true
}

// ttProbe and ttStore carry only refutations. Wins stay unrecorded so a
// forced line is always re-derived and the PV stays reconstructible; the
// expensive part of the search is refutation hunting, which fail verdicts
// keep pruned across visits.
func (s *Solver) ttProbe(hash uint64) (result, bool) {
	slot := hash & uint64(len(s.ttKeys)-1)
	if s.ttVerdicts[slot] != 0 && s.ttKeys[slot] == hash {
		return resFail, true
	}
	return resFail, false
}

func (s *Solver) ttStore(hash uint64, r result) {
	if r != resFail {
		return
	}
	slot := hash & uint64(len(s.ttKeys)-1)
	s.ttKeys[slot] = hash
	s.ttVerdicts[slot] = 2
}
