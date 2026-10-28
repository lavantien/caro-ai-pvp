package vcf

import (
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

type Deadline interface {
	Exceeded() bool
	Stop()
}

type Kind uint8

const (
	KindVCF Kind = iota
	KindVCT
)

type SolverStats struct {
	Nodes     uint64
	ElapsedNs int64
	Plies     int
	Found     bool
	PV        [config.SolverMaxPly]rules.Move
}

func (s *SolverStats) AppendPV(dst []byte) []byte {
	for i := 0; i < s.Plies && i < len(s.PV); i++ {
		if i > 0 {
			dst = append(dst, ' ')
		}
		dst = rules.AppendCellName(dst, rules.Cell(s.PV[i]))
	}
	return dst
}

type result uint8

const (
	resFail result = iota
	resWin
	resAbort
)

const maxWinCells = 48

type Solver struct {
	kind     Kind
	maxPly   int
	plyCap   int
	moves    [config.SolverMaxPly][config.MaxBoardCells]rules.Move
	ranks    [config.SolverMaxPly][config.MaxBoardCells]uint8
	defends  [config.SolverMaxPly][config.MaxBoardCells]rules.Cell
	line     [config.SolverMaxPly]rules.Move
	winA     [maxWinCells]rules.Cell
	ttKeys   [1 << config.SolverTTBits]uint64
	ttStamps [1 << config.SolverTTBits]uint16
	nodes    int
	budget   int
	check    int
	plies    int
	aborted  bool
	deadline Deadline
}

func New(kind Kind) *Solver {
	return &Solver{kind: kind, maxPly: config.SolverMaxPly}
}

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

func (s *Solver) ttProbe(hash uint64, remaining int) bool {
	slot := hash & uint64(len(s.ttKeys)-1)
	return s.ttStamps[slot] != 0 && s.ttKeys[slot] == hash && int(s.ttStamps[slot]) >= remaining
}

func (s *Solver) ttStoreFail(hash uint64, remaining int) {
	if remaining <= 0 {
		return
	}
	slot := hash & uint64(len(s.ttKeys)-1)
	s.ttKeys[slot] = hash
	s.ttStamps[slot] = uint16(remaining)
}
