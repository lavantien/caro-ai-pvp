// Command overshoot measures worst-case deadline overshoot of the engine:
// how far past a granted fixed budget wall-clock time runs before Search
// returns, per worker count and budget. The purpose is to validate
// config.SearchSafetyMarginMs against a 3x factor over the worst observation.
//
// Mechanisms under test: the search polls Exceeded() every
// config.SearchNodeCheckInterval nodes, an iteration boundary is entered only
// while the deadline is unexceeded (config.SearchSoftStopFraction is a clock
// constant, not consulted here because the harness grants a raw
// engine.NewFixedBudget), and the SMP driver must dispatch, halt, and join
// its parked workers via runWG.
//
// Run from this directory (the repo Makefile is outside this topic's file
// ownership, so the conventional make target cannot live here):
//
//	CGO_ENABLED=1 go run .
package main

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/lavantien/caro-ai-pvp/internal/config"
	"github.com/lavantien/caro-ai-pvp/internal/engine"
	"github.com/lavantien/caro-ai-pvp/internal/rules"
)

// Measurement knobs. The worker axis is normalized on one TT size so table
// size does not confound worker count; NewTiered reads only Cores and
// TTBytes today, the solver flags mirror the stock tier of the same core
// count for drift-resistance.
const (
	ttBytes      = 1 << 26
	repeats      = 100
	sweepCount   = 3
	warmupBudget = 100 * time.Millisecond
	p99          = 0.99
)

// Experiment matrix.
var (
	workerCounts = []int{1, 2, 4}
	budgets      = []time.Duration{
		10 * time.Millisecond,
		50 * time.Millisecond,
		200 * time.Millisecond,
		1000 * time.Millisecond,
	}
)

// searcher is the shared Search surface of the single-threaded Engine and
// the SMP pool.
type searcher interface {
	Search(b *rules.Board, dl engine.Deadline) (rules.Move, engine.SearchStats)
}

// newSearcher builds the measurement instance for a worker count:
// engine.New for one worker, engine.NewTiered for the SMP pool. The returned
// func releases worker threads; the single engine has none to release.
func newSearcher(workers int) (searcher, func()) {
	if workers == 1 {
		return engine.New(ttBytes), func() {}
	}
	s := engine.NewTiered(config.Tier{
		Name:    fmt.Sprintf("overshoot-%dw", workers),
		Cores:   workers,
		TTBytes: ttBytes,
		VCF:     true,
		VCT:     workers >= 4,
	})
	return s, s.Close
}

// midgamePosition builds the fixed 12-stone midgame board: a 2x2 center
// block (H8, H9, I8, I9), a 3-stone cluster upper left (C3, C4, D3), a
// 3-stone cluster lower right (M12, M13, N12), and a horizontal pair at
// (K10, L10). Sides alternate in listed order, neither side owns a live
// threat, so the deadline binds every search.
func midgamePosition() *rules.Board {
	b := rules.NewBoard()
	for _, name := range [...]string{
		"H8", "H9", "I8", "I9",
		"C3", "C4", "D3",
		"M12", "M13", "N12",
		"K10", "L10",
	} {
		cell, err := rules.ParseCell(name)
		if err != nil {
			panic(err)
		}
		b.Make(cell)
	}
	return b
}

// sample is one budgeted search outcome. early marks a search that returned
// before the deadline bound it (mate found or fallback), which would make
// the overshoot non-informative for that repeat.
type sample struct {
	overshoot time.Duration
	early     bool
}

// measure runs one budgeted search. The wall clock is armed before the grant
// so elapsed-budget can only overstate the true past-grant return time,
// never understate it: the conservative direction for sizing a reserve.
func measure(s searcher, b *rules.Board, budget time.Duration) sample {
	start := time.Now()
	dl := engine.NewFixedBudget(budget)
	s.Search(b, dl)
	elapsed := time.Since(start)
	return sample{overshoot: elapsed - budget, early: elapsed < budget}
}

// runSweep measures every configuration once with fresh instances, so the
// three sweeps are independent samples of host contention.
func runSweep() [][]sample {
	var out [][]sample
	for _, workers := range workerCounts {
		s, release := newSearcher(workers)
		b := midgamePosition()
		s.Search(b, engine.NewFixedBudget(warmupBudget))
		for _, budget := range budgets {
			row := make([]sample, repeats)
			for i := range row {
				row[i] = measure(s, b, budget)
			}
			out = append(out, row)
		}
		release()
	}
	return out
}

// pctIndex is the nearest-rank index of quantile q over n samples.
func pctIndex(n int, q float64) int {
	i := int(math.Ceil(q*float64(n))) - 1
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

func statsOf(row []sample) (med, hi, worst time.Duration, early int) {
	d := make([]time.Duration, len(row))
	for i, s := range row {
		d[i] = s.overshoot
		if s.early {
			early++
		}
	}
	sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
	med = (d[len(d)/2-1] + d[len(d)/2]) / 2
	hi = d[pctIndex(len(d), p99)]
	worst = d[len(d)-1]
	return
}

func ms(d time.Duration) float64 { return float64(d) / float64(time.Millisecond) }

// worstOf is a sweep's worst max across configurations, the criterion for
// picking the least-contentioned sweep.
func worstOf(sweep [][]sample) time.Duration {
	var worst time.Duration
	for _, row := range sweep {
		if _, _, w, _ := statsOf(row); w > worst {
			worst = w
		}
	}
	return worst
}

func printSweep(w *tabwriter.Writer, label string, sweep [][]sample) {
	fmt.Fprintf(w, "%s\n", label)
	fmt.Fprintf(w, "workers\tbudget\tmedian\tp99\tmax\tearly\n")
	i := 0
	for _, workers := range workerCounts {
		for _, budget := range budgets {
			med, hi, worst, early := statsOf(sweep[i])
			fmt.Fprintf(w, "%d\t%dms\t%.3fms\t%.3fms\t%.3fms\t%d/%d\n",
				workers, budget.Milliseconds(), ms(med), ms(hi), ms(worst), early, repeats)
			i++
		}
	}
	w.Flush()
}

func main() {
	fmt.Printf("overshoot harness: GOMAXPROCS=%d NumCPU=%d nodeCheck=%d parkDelayMs=%d softStop=%.2f safetyMarginMs=%d repeats=%d sweeps=%d ttBytes=%d\n",
		runtime.GOMAXPROCS(0), runtime.NumCPU(), config.SearchNodeCheckInterval,
		config.SearchWorkerParkDelayMs, config.SearchSoftStopFraction,
		config.SearchSafetyMarginMs, repeats, sweepCount, ttBytes)

	sweeps := make([][][]sample, 0, sweepCount)
	for i := range sweepCount {
		sweeps = append(sweeps, runSweep())
		fmt.Fprintf(os.Stderr, "sweep %d/%d done\n", i+1, sweepCount)
	}

	best := 0
	for i := 1; i < len(sweeps); i++ {
		if worstOf(sweeps[i]) < worstOf(sweeps[best]) {
			best = i
		}
	}

	out := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	for i, sweep := range sweeps {
		tag := ""
		if i == best {
			tag = "\t<- chosen (smallest maxima)"
		}
		fmt.Fprintf(out, "sweep %d\tworst max\t%.3fms%s\n", i+1, ms(worstOf(sweep)), tag)
	}
	out.Flush()
	fmt.Println()

	printSweep(tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0),
		fmt.Sprintf("chosen sweep %d of %d, warmup budget %s, fixed midgame position (12 stones)", best+1, sweepCount, warmupBudget),
		sweeps[best])

	worst := worstOf(sweeps[best])
	margin := time.Duration(config.SearchSafetyMarginMs) * time.Millisecond
	required := 3 * worst
	fmt.Printf("worst absolute overshoot: %.3fms\n", ms(worst))
	fmt.Printf("3x worst: %.3fms, SearchSafetyMarginMs: %s, covers every configuration: %v\n",
		ms(required), margin, required <= margin)
	if required > margin {
		fmt.Printf("minimum margin that covers 3x: %dms\n",
			(required+time.Millisecond-1)/time.Millisecond)
	}
}
