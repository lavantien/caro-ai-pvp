package clock

// PID tuning benchmarks for the clock law, two layers.
//
// 1. Simulation sweeps driving the real GameClock through the
//    NewGameClockWithGains seam with injectable gains against synthetic cost
//    models, so alternative gain sets compare without touching production
//    config. No mirrored law: the sweep runs the production law, including
//    Budget's nanosecond Duration truncation (sub-nanosecond per grant,
//    negligible at every control's millisecond scale).
// 2. One live 1+0 game driving the real engine through the real GameClock,
//    the first end-to-end wiring of the clock to the engine.

import (
	"fmt"
	"math"
	"math/rand/v2"
	"testing"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Simulation shape. Sixty moves per side is double the expected-moves anchor,
// long enough that endgame drain behavior shows in the metrics.
const (
	benchSimMoves        = 60
	benchSlowStartMoves  = 10
	benchSlowStartFactor = 3.0
	benchOvershootMin    = 1.0
	benchOvershootMax    = 1.3
	benchOvershootSeedHi = 0x5EEDC10C
	benchOvershootSeedLo = 0x17
)

// Live-game shape: the hard cap per side at the 1+0 control.
const benchLiveCapPerSide = 60

// benchGainSets are the swept alternatives against the committed per-TC
// gains. Aggressive triples the proportional and doubles the integral and
// derivative action: strong tracking, expected to slam into the correction
// clamp under the overshoot models.
var benchGainSets = []struct {
	name      string
	transform func(config.PIDGains) config.PIDGains
}{
	{"committed", func(g config.PIDGains) config.PIDGains { return g }},
	{"Kp2", func(g config.PIDGains) config.PIDGains {
		return config.PIDGains{Kp: 2 * g.Kp, Ki: g.Ki, Kd: g.Kd}
	}},
	{"Ki2", func(g config.PIDGains) config.PIDGains {
		return config.PIDGains{Kp: g.Kp, Ki: 2 * g.Ki, Kd: g.Kd}
	}},
	{"KiHalf", func(g config.PIDGains) config.PIDGains {
		return config.PIDGains{Kp: g.Kp, Ki: g.Ki / 2, Kd: g.Kd}
	}},
	{"Kd0", func(g config.PIDGains) config.PIDGains {
		return config.PIDGains{Kp: g.Kp, Ki: g.Ki}
	}},
	{"zero-ff", func(config.PIDGains) config.PIDGains {
		return config.PIDGains{}
	}},
	{"aggressive", func(g config.PIDGains) config.PIDGains {
		return config.PIDGains{Kp: 3 * g.Kp, Ki: 2 * g.Ki, Kd: 2 * g.Kd}
	}},
}

// benchCostModel turns a grant into a spend via per-move factors, so every
// gain set under one (model, tc) cell consumes the identical factor sequence
// and the comparison stays fair.
type benchCostModel struct {
	name    string
	factors func(tcIdx int) []float64
}

func benchConstFactors(v float64) []float64 {
	f := make([]float64, benchSimMoves)
	for i := range f {
		f[i] = v
	}
	return f
}

func benchUniformFactors(tcIdx int, lo, hi float64) []float64 {
	rng := rand.New(rand.NewPCG(benchOvershootSeedHi, benchOvershootSeedLo+uint64(tcIdx)))
	f := make([]float64, benchSimMoves)
	for i := range f {
		f[i] = lo + (hi-lo)*rng.Float64()
	}
	return f
}

func benchSlowStartFactors() []float64 {
	f := benchConstFactors(1)
	for i := range benchSlowStartMoves {
		f[i] = benchSlowStartFactor
	}
	return f
}

var (
	benchExactModel     = benchCostModel{name: "exact", factors: func(int) []float64 { return benchConstFactors(1) }}
	benchOvershootModel = benchCostModel{name: "overshoot[1.0,1.3]", factors: func(tcIdx int) []float64 {
		return benchUniformFactors(tcIdx, benchOvershootMin, benchOvershootMax)
	}}
	benchSlowStartModel = benchCostModel{name: "slowstart-3x-first-10", factors: func(int) []float64 { return benchSlowStartFactors() }}
)

type benchSimResult struct {
	leftoverMs    float64
	distReserveMs float64
	collapseMs    float64
	stdevMs       float64
}

func benchRunSim(tcIdx int, gains config.PIDGains, factors []float64) benchSimResult {
	nsPerMs := float64(time.Millisecond)
	c := NewGameClockWithGains(tcIdx, gains)
	budgets := make([]float64, benchSimMoves)
	for m := range benchSimMoves {
		budgets[m] = float64(c.Budget()) / nsPerMs
		c.Commit(time.Duration(budgets[m] * factors[m] * nsPerMs))
	}
	leftoverMs := float64(c.Remaining()) / nsPerMs
	return benchSimResult{
		leftoverMs:    leftoverMs,
		distReserveMs: math.Abs(leftoverMs - config.SearchSafetyMarginMs),
		collapseMs:    benchWorstCollapse(budgets),
		stdevMs:       benchStdev(budgets),
	}
}

// benchWorstCollapse is the largest single-step drop of the granted budget
// below the running mean of the grants so far.
func benchWorstCollapse(b []float64) float64 {
	worst, sum := 0.0, 0.0
	for k, v := range b {
		sum += v
		if drop := sum/float64(k+1) - v; drop > worst {
			worst = drop
		}
	}
	return worst
}

func benchStdev(b []float64) float64 {
	var mean float64
	for _, v := range b {
		mean += v
	}
	mean /= float64(len(b))
	var ss float64
	for _, v := range b {
		d := v - mean
		ss += d * d
	}
	return math.Sqrt(ss / float64(len(b)))
}

func benchSweep(b *testing.B, model benchCostModel) {
	for tcIdx := range config.TimeControls {
		ctl := config.TimeControls[tcIdx]
		b.Run(fmt.Sprintf("%d+%d", ctl.InitialMin, ctl.IncrementSec), func(b *testing.B) {
			results := make([]benchSimResult, len(benchGainSets))
			for b.Loop() {
				factors := model.factors(tcIdx)
				for i, gs := range benchGainSets {
					results[i] = benchRunSim(tcIdx, gs.transform(config.ClockPID[tcIdx]), factors)
				}
			}
			b.Logf("model=%s tc=%d+%d reserve=%dms all metrics in ms", model.name, ctl.InitialMin, ctl.IncrementSec, config.SearchSafetyMarginMs)
			b.Logf("%-12s %10s %10s %10s %10s", "gains", "leftover", "dist-res", "collapse", "stdev")
			for i, gs := range benchGainSets {
				r := results[i]
				b.Logf("%-12s %10.1f %10.1f %10.1f %10.1f", gs.name, r.leftoverMs, r.distReserveMs, r.collapseMs, r.stdevMs)
			}
		})
	}
}

func BenchmarkClockPIDExactSpend(b *testing.B) { benchSweep(b, benchExactModel) }

func BenchmarkClockPIDOvershoot(b *testing.B) { benchSweep(b, benchOvershootModel) }

func BenchmarkClockPIDSlowStart(b *testing.B) { benchSweep(b, benchSlowStartModel) }

type benchGameReport struct {
	outcome          string
	movesPerSide     [2]int
	finalRemMs       [2]float64
	minBudgetMs      float64
	maxBudgetMs      float64
	worstOverrunFrac float64
}

// benchPlayGame plays one full TierHard game at 1+0: each turn grants
// NewFixedBudget(clock.Budget()), searches, commits the wall-clock elapsed,
// and makes the returned move on a rules board. Every move must be legal,
// Remaining must never go negative for either side, and the game must end by
// win, full board, or the per-side move cap.
func benchPlayGame(b *testing.B, eng *engine.SMP) benchGameReport {
	board := rules.NewBoard()
	clocks := [2]*GameClock{NewGameClock(0), NewGameClock(0)} // tc 0 is 1+0
	rep := benchGameReport{minBudgetMs: math.Inf(1)}
	for ply := 0; ; ply++ {
		if board.IsFull() {
			rep.outcome = "full-board draw"
			break
		}
		side := ply % 2
		if clocks[side].Moves() >= benchLiveCapPerSide {
			rep.outcome = "move-cap draw"
			break
		}
		budget := clocks[side].Budget()
		bMs := float64(budget) / float64(time.Millisecond)
		rep.minBudgetMs = math.Min(rep.minBudgetMs, bMs)
		rep.maxBudgetMs = math.Max(rep.maxBudgetMs, bMs)
		dl := engine.NewFixedBudget(budget)
		start := time.Now()
		mv, _ := eng.Search(board, dl)
		elapsed := time.Since(start)
		cell := rules.Cell(mv)
		if !board.IsLegal(cell) {
			b.Fatalf("ply %d: illegal engine move %d", ply, mv)
		}
		mover := board.Side
		board.Make(cell)
		clocks[side].Commit(elapsed)
		for s, c := range clocks {
			if c.Remaining() < 0 {
				b.Fatalf("ply %d: side %d remaining went negative: %v", ply, s, c.Remaining())
			}
		}
		rep.worstOverrunFrac = math.Max(rep.worstOverrunFrac, float64(elapsed)/float64(budget)-1)
		if board.FastLastMoveWin(mover, cell) {
			rep.outcome = "blue win"
			if mover == rules.Red {
				rep.outcome = "red win"
			}
			break
		}
	}
	if rep.outcome == "" {
		b.Fatal("game did not terminate")
	}
	for s, c := range clocks {
		rep.movesPerSide[s] = c.Moves()
		rep.finalRemMs[s] = float64(c.Remaining()) / float64(time.Millisecond)
	}
	return rep
}

// BenchmarkLiveGameClock1plus0 is seconds per iteration: one full game of
// real searches, so b.N stays at 1 under the default benchtime.
func BenchmarkLiveGameClock1plus0(b *testing.B) {
	eng := engine.NewTiered(config.TierHard)
	defer eng.Close()
	var rep benchGameReport
	for b.Loop() {
		rep = benchPlayGame(b, eng)
	}
	b.Logf("1+0 TierHard outcome=%s moves red/blue=%d/%d final remaining red/blue=%.0fms/%.0fms budget min/max=%.0f/%.0fms worst overrun=%.0f%%",
		rep.outcome, rep.movesPerSide[0], rep.movesPerSide[1], rep.finalRemMs[0], rep.finalRemMs[1],
		rep.minBudgetMs, rep.maxBudgetMs, 100*rep.worstOverrunFrac)
}
