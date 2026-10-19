package engine

import (
	"testing"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

func doubledWeights() *[config.PatternClassCount]int {
	var w [config.PatternClassCount]int
	for i := range w {
		w[i] = 2 * config.PatternClassWeights[i]
	}
	return &w
}

func freeCellNear(t *testing.T, b *rules.Board, name string) rules.Cell {
	t.Helper()
	base := int(mustCell(t, name))
	for radius := 0; radius < config.BoardSize; radius++ {
		r, c := base/config.BoardStride, base%config.BoardStride
		for dr := -radius; dr <= radius; dr++ {
			for dc := -radius; dc <= radius; dc++ {
				if max(abs(dr), abs(dc)) != radius {
					continue
				}
				rr, cc := r+dr, c+dc
				if rr < 0 || rr >= config.BoardSize || cc < 0 || cc >= config.BoardSize {
					continue
				}
				cell := rules.Cell(rr*config.BoardStride + cc)
				if !b.Occupied(cell) {
					return cell
				}
			}
		}
	}
	t.Fatal("no free cell on the board")
	return 0
}

func (ev *evaluator) stateright(b *rules.Board) bool {
	var fresh evaluator
	fresh.w = ev.w
	fresh.reset(b)
	return ev.win == fresh.win && ev.wpair == fresh.wpair &&
		ev.score == fresh.score && ev.cell == fresh.cell && ev.fours == fresh.fours
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func TestEvaluatorOverrideWeightsArithmetic(t *testing.T) {
	b := playout(t, 7, 40)
	var def evaluator
	def.reset(b)
	if def.score[rules.Red] == 0 && def.score[rules.Blue] == 0 {
		t.Fatal("playout board must produce a nonzero class sum")
	}
	var over evaluator
	over.w = doubledWeights()
	over.reset(b)
	if over.score[rules.Red] != 2*def.score[rules.Red] || over.score[rules.Blue] != 2*def.score[rules.Blue] {
		t.Errorf("doubled weights scores %d %d, want %d %d", over.score[rules.Red], over.score[rules.Blue], 2*def.score[rules.Red], 2*def.score[rules.Blue])
	}
	var zero evaluator
	zero.w = &[config.PatternClassCount]int{}
	zero.reset(b)
	if got := zero.eval(rules.Red); got != config.EvalTempo {
		t.Errorf("zero weights eval = %d, want tempo %d", got, config.EvalTempo)
	}
	var base evaluator
	base.reset(b)
	mv := freeCellNear(t, b, "H8")
	zero.makeMove(rules.Red, mv)
	zero.unmakeMove(mv)
	if got := zero.eval(rules.Red); got != config.EvalTempo {
		t.Errorf("zero weights eval after make/unmake = %d, want tempo %d", got, config.EvalTempo)
	}
	if !zero.stateright(b) {
		t.Error("make/unmake under override weights desynced from the default evaluator")
	}
}

func TestEvaluatorResetPreservesOverride(t *testing.T) {
	b := playout(t, 11, 30)
	var def evaluator
	def.reset(b)
	var ev evaluator
	w := doubledWeights()
	ev.w = w
	ev.reset(b)
	if ev.score[rules.Red] != 2*def.score[rules.Red] {
		t.Errorf("override score %d, want %d", ev.score[rules.Red], 2*def.score[rules.Red])
	}
	ev.reset(b)
	if ev.w != w {
		t.Fatal("reset dropped the override weights pointer")
	}
	if ev.score[rules.Red] != 2*def.score[rules.Red] {
		t.Errorf("second reset score %d, want %d", ev.score[rules.Red], 2*def.score[rules.Red])
	}
}

func TestNewTieredEvalWeightsReachWorkers(t *testing.T) {
	tier := config.Tier{Name: "weights-probe", Cores: 2, TTBytes: 1 << 20, PonderThreads: 2}
	w := doubledWeights()
	s := NewTieredEval(tier, w)
	defer s.Close()
	if s.Workers() != 2 {
		t.Fatalf("workers = %d, want 2", s.Workers())
	}
	for i, wk := range s.workers {
		if wk.eval.w != w {
			t.Errorf("worker %d does not carry the override weights", i)
		}
	}
	d := NewTiered(tier)
	defer d.Close()
	for i, wk := range d.workers {
		if wk.eval.w != nil {
			t.Errorf("default worker %d carries override %v, want nil global default", i, wk.eval.w)
		}
	}
}
