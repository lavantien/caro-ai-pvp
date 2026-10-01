package vcf

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Zero allocation pins for the solve loop: every buffer the search touches
// is preallocated on the Solver, so repeated solves reuse one object
// without heap traffic. The solver itself is constructed outside the
// measured run, matching how the engine owns its instances.

func TestSolveZeroAllocs(t *testing.T) {
	vcfBoard := buildAt(t, false, 8, 4, rules.Red, specFourChain)
	vctBoard := buildAt(t, false, 8, 9, rules.Red, specVCTDoubleThree)
	refuteBoard := buildAt(t, false, 8, 4, rules.Red, specCounterFour)
	for _, tc := range []struct {
		kind  Kind
		board *rules.Board
	}{
		{KindVCF, vcfBoard},
		{KindVCF, refuteBoard},
		{KindVCT, vctBoard},
		{KindVCT, refuteBoard},
	} {
		s := New(tc.kind)
		var stats SolverStats
		if allocs := testing.AllocsPerRun(20, func() {
			s.Solve(tc.board, config.SolverNodeBudget, nil, &stats)
		}); allocs != 0 {
			t.Fatalf("kind %d: solve allocates %v per run", tc.kind, allocs)
		}
	}
}

func benchmarkConstruction(b *testing.B, kind Kind, board *rules.Board) {
	b.Helper()
	s := New(kind)
	var stats SolverStats
	b.ResetTimer()
	for range b.N {
		s.Solve(board, config.SolverNodeBudget, nil, &stats)
	}
}

func BenchmarkSolveVCFChain(b *testing.B) {
	benchmarkConstruction(b, KindVCF, buildAt(b, false, 8, 4, rules.Red, specFourChain))
}

func BenchmarkSolveVCFRefuted(b *testing.B) {
	benchmarkConstruction(b, KindVCF, buildAt(b, false, 8, 4, rules.Red, specTwoDeadFours))
}

func BenchmarkSolveVCTDoubleThree(b *testing.B) {
	benchmarkConstruction(b, KindVCT, buildAt(b, false, 8, 9, rules.Red, specVCTDoubleThree))
}

func BenchmarkSolveVCTCounterFour(b *testing.B) {
	benchmarkConstruction(b, KindVCT, buildAt(b, false, 8, 9, rules.Red, specVCTCounterFour))
}
